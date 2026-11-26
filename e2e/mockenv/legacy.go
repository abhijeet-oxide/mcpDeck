package mockenv

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/xml"
	"fmt"
	"io"
	"net/http"
	"strings"
	"sync"
	"time"
)

// legacy is an old ERP web app, Django style: GET /accounts/login/ sets a
// csrftoken cookie and puts the same token in a hidden form field; POST the
// form with that token to get a sessionid cookie (answered with a 302, as
// browsers expect). JSON endpoints need the session; writes also need an
// X-CSRFToken header matching the csrftoken cookie. An expired session is
// answered with a 302 to the login page, not a 401. Credit data lives
// behind a SOAP 1.1 service.

func randHex(n int) string {
	b := make([]byte, n)
	_, _ = rand.Read(b)
	return hex.EncodeToString(b)
}

func (e *Env) legacy() http.Handler {
	mux := http.NewServeMux()
	var mu sync.Mutex
	sessions := map[string]time.Time{}
	notes := map[string][]string{}
	e.expireLegacy = func() {
		mu.Lock()
		defer mu.Unlock()
		for k := range sessions {
			delete(sessions, k)
		}
	}
	customers := []map[string]any{
		{"id": "C100", "name": "Northwind Traders", "credit_limit": 50000, "status": "ACTIVE"},
		{"id": "C200", "name": "Contoso Ltd", "credit_limit": 120000, "status": "ON_HOLD"},
		{"id": "C300", "name": "Fabrikam Inc", "credit_limit": 15000, "status": "ACTIVE"},
	}
	session := func(r *http.Request) bool {
		c, err := r.Cookie("sessionid")
		if err != nil {
			return false
		}
		mu.Lock()
		defer mu.Unlock()
		exp, ok := sessions[c.Value]
		return ok && time.Now().Before(exp)
	}
	toLogin := func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, "/accounts/login/?next="+r.URL.Path, http.StatusFound)
	}
	mux.HandleFunc("GET /accounts/login/", func(w http.ResponseWriter, r *http.Request) {
		tok := randHex(16)
		http.SetCookie(w, &http.Cookie{Name: "csrftoken", Value: tok, Path: "/", Secure: true, SameSite: http.SameSiteLaxMode})
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		fmt.Fprintf(w, `<!doctype html><html><body><form method="post" action="/accounts/login/">
<input type="hidden" name="csrfmiddlewaretoken" value="%s">
<input name="username"><input name="password" type="password"><button>Sign in</button></form></body></html>`, tok)
	})
	mux.HandleFunc("POST /accounts/login/", func(w http.ResponseWriter, r *http.Request) {
		_ = r.ParseForm()
		c, err := r.Cookie("csrftoken")
		if err != nil || c.Value == "" || r.PostForm.Get("csrfmiddlewaretoken") != c.Value {
			e.inc("legacy.csrf_fail")
			w.WriteHeader(403)
			_, _ = w.Write([]byte("<h1>Forbidden (403)</h1><p>CSRF verification failed. Request aborted.</p>"))
			return
		}
		if r.PostForm.Get("username") != LegacyUser || r.PostForm.Get("password") != LegacyPass {
			w.WriteHeader(200)
			_, _ = w.Write([]byte("<p class=error>Please enter a correct username and password.</p>"))
			return
		}
		sid := randHex(20)
		mu.Lock()
		sessions[sid] = time.Now().Add(e.ttl())
		mu.Unlock()
		e.inc("legacy.login")
		http.SetCookie(w, &http.Cookie{Name: "sessionid", Value: sid, Path: "/", HttpOnly: true, Secure: true, SameSite: http.SameSiteLaxMode})
		http.Redirect(w, r, "/home/", http.StatusFound)
	})
	mux.HandleFunc("GET /api/customers/", func(w http.ResponseWriter, r *http.Request) {
		if !session(r) {
			toLogin(w, r)
			return
		}
		q := strings.ToLower(r.URL.Query().Get("q"))
		var out []any
		for _, c := range customers {
			if q == "" || strings.Contains(strings.ToLower(c["name"].(string)), q) {
				out = append(out, c)
			}
		}
		writeJSON(w, 200, map[string]any{"count": len(out), "results": out})
	})
	mux.HandleFunc("POST /api/customers/{id}/notes/", func(w http.ResponseWriter, r *http.Request) {
		if !session(r) {
			toLogin(w, r)
			return
		}
		c, err := r.Cookie("csrftoken")
		if err != nil || r.Header.Get("X-CSRFToken") == "" || r.Header.Get("X-CSRFToken") != c.Value {
			e.inc("legacy.csrf_fail")
			writeJSON(w, 403, map[string]any{"detail": "CSRF Failed: CSRF token missing or incorrect."})
			return
		}
		var in struct{ Text string }
		_ = readJSON(r, &in)
		mu.Lock()
		notes[r.PathValue("id")] = append(notes[r.PathValue("id")], in.Text)
		n := len(notes[r.PathValue("id")])
		mu.Unlock()
		e.inc("legacy.note")
		writeJSON(w, 201, map[string]any{"id": n, "customer": r.PathValue("id"), "text": in.Text})
	})
	mux.HandleFunc("POST /soap/CreditService", func(w http.ResponseWriter, r *http.Request) {
		if !session(r) {
			toLogin(w, r)
			return
		}
		b, _ := io.ReadAll(io.LimitReader(r.Body, 1<<16))
		var env struct {
			Body struct {
				Inner []byte `xml:",innerxml"`
			} `xml:"Body"`
		}
		if err := xml.Unmarshal(b, &env); err != nil {
			w.Header().Set("Content-Type", "text/xml; charset=utf-8")
			w.WriteHeader(500)
			_, _ = w.Write([]byte(soapFault("Client", "Malformed envelope")))
			return
		}
		var req struct {
			XMLName    xml.Name
			CustomerID string `xml:"CustomerId"`
		}
		_ = xml.Unmarshal(env.Body.Inner, &req)
		action := strings.Trim(r.Header.Get("SOAPAction"), `"`)
		w.Header().Set("Content-Type", "text/xml; charset=utf-8")
		var found map[string]any
		for _, c := range customers {
			if c["id"] == req.CustomerID {
				found = c
			}
		}
		if found == nil {
			w.WriteHeader(500)
			_, _ = w.Write([]byte(soapFault("Client", "Unknown customer "+req.CustomerID)))
			return
		}
		switch action {
		case "urn:GetCreditLimit", "GetCreditLimit":
			fmt.Fprintf(w, `<?xml version="1.0" encoding="utf-8"?><soap:Envelope xmlns:soap="http://schemas.xmlsoap.org/soap/envelope/"><soap:Body><GetCreditLimitResponse xmlns="urn:erp"><CustomerId>%s</CustomerId><CreditLimit currency="USD">%v</CreditLimit><Status>%s</Status><Holds><Hold><Code>LATE</Code><Since>2026-10-01</Since></Hold></Holds></GetCreditLimitResponse></soap:Body></soap:Envelope>`,
				found["id"], found["credit_limit"], found["status"])
		default:
			w.WriteHeader(500)
			_, _ = w.Write([]byte(soapFault("Client", "Unknown SOAPAction "+action)))
		}
	})
	return mux
}

func soapFault(code, msg string) string {
	return `<?xml version="1.0"?><soap:Envelope xmlns:soap="http://schemas.xmlsoap.org/soap/envelope/"><soap:Body><soap:Fault><faultcode>soap:` + code + `</faultcode><faultstring>` + msg + `</faultstring></soap:Fault></soap:Body></soap:Envelope>`
}

// LegacyCurl is what someone copies from the browser's network tab.
func (e *Env) LegacyCurl() string {
	u := e.URLs["legacy"]
	return strings.Join([]string{
		`curl "` + u + `/accounts/login/"`,
		`curl -X POST "` + u + `/accounts/login/" -H "Content-Type: application/x-www-form-urlencoded" --data-raw "csrfmiddlewaretoken=abc&username=clerk&password=` + LegacyPass + `"`,
		`curl "` + u + `/api/customers/?q=north" -H "Cookie: sessionid=abc"`,
		`curl -X POST "` + u + `/api/customers/C100/notes/" -H "Content-Type: application/json" -H "X-CSRFToken: abc" -d '{"text":"called"}'`,
		`curl -X POST "` + u + `/soap/CreditService" -H "Content-Type: text/xml; charset=utf-8" -H "SOAPAction: urn:GetCreditLimit" -d '<soap:Envelope xmlns:soap="http://schemas.xmlsoap.org/soap/envelope/"><soap:Body><GetCreditLimit xmlns="urn:erp"><CustomerId>C100</CustomerId></GetCreditLimit></soap:Body></soap:Envelope>'`,
	}, "\n\n")
}

// ExpireSessions ends every legacy session, as a server restart or an idle
// timeout would.
func (e *Env) ExpireSessions() { e.expireLegacy() }
