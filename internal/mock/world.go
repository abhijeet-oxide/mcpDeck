// Package mock serves the demo: four related fake APIs with seeded,
// consistent data (MOCK-3), used by `mcpdeck dev`, tests and demos.
//
//	crm      bearer token          accounts, contacts, owners
//	billing  X-Api-Key header      customer summaries (deliberately bloated), invoices
//	support  bearer token          Jira-style issue search, issues, comments
//	legacy   cookie login + SOAP   credit holds behind a session cookie
package mock

import (
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"strings"
	"sync"
	"time"
)

// Demo credentials. They only open the local mock servers.
const (
	CRMToken      = "demo-crm-token-4f2a"
	BillingKey    = "demo-billing-key-81c3"
	SupportToken  = "demo-support-token-6d0e"
	LegacyUser    = "deck"
	LegacyPass    = "demo-legacy-pass-2b9f"
	legacySession = "LEGACY-SESSION-7731"
)

type account struct {
	ID        string `json:"id"`
	Name      string `json:"name"`
	Tier      string `json:"tier"`
	Industry  string `json:"industry"`
	Country   string `json:"country"`
	OwnerID   string `json:"ownerId"`
	BillingID string `json:"billingId"`
	JiraOrg   string `json:"jiraOrg"`
	Created   string `json:"createdAt"`
}

var accounts = []account{
	{"001Ab00000NWTRD", "Northwind Traders", "gold", "Wholesale", "US", "005OWN01", "C-1001", "NWT", "2021-03-14T09:12:00Z"},
	{"001Ab00000CNTSO", "Contoso Ltd", "platinum", "Manufacturing", "DE", "005OWN02", "C-1002", "CTS", "2019-11-02T15:40:00Z"},
	{"001Ab00000FBRKM", "Fabrikam Inc", "silver", "Retail", "GB", "005OWN01", "C-1003", "FAB", "2022-06-21T11:05:00Z"},
	{"001Ab00000GLBEX", "Globex Corporation", "gold", "Energy", "US", "005OWN03", "C-1004", "GLX", "2020-01-30T08:00:00Z"},
	{"001Ab00000INTCH", "Initech", "bronze", "Software", "CA", "005OWN02", "C-1005", "INI", "2023-09-09T17:22:00Z"},
}

var owners = map[string]map[string]any{
	"005OWN01": {"id": "005OWN01", "name": "Dana Whitfield", "email": "dana@acme.example", "team": "Enterprise East"},
	"005OWN02": {"id": "005OWN02", "name": "Ravi Menon", "email": "ravi@acme.example", "team": "EMEA"},
	"005OWN03": {"id": "005OWN03", "name": "Lucia Ortega", "email": "lucia@acme.example", "team": "Enterprise West"},
}

type invoice struct {
	Number     string  `json:"number"`
	CustomerID string  `json:"customerId"`
	Amount     float64 `json:"amount"`
	Paid       float64 `json:"amountPaid"`
	Currency   string  `json:"currency"`
	Issued     string  `json:"issuedAt"`
	Due        string  `json:"dueDate"`
	Status     string  `json:"status"`
}

var invoices = []invoice{
	{"INV-2041", "C-1001", 12400, 12400, "USD", "2026-08-01", "2026-08-31", "paid"},
	{"INV-2087", "C-1001", 8650, 0, "USD", "2026-09-01", "2026-10-01", "overdue"},
	{"INV-2113", "C-1001", 3120.5, 1000, "USD", "2026-10-01", "2026-10-31", "overdue"},
	{"INV-2150", "C-1001", 9900, 0, "USD", "2026-11-01", "2026-12-01", "open"},
	{"INV-2042", "C-1002", 45000, 45000, "EUR", "2026-08-01", "2026-08-31", "paid"},
	{"INV-2151", "C-1002", 47250, 0, "EUR", "2026-11-01", "2026-12-01", "open"},
	{"INV-2099", "C-1003", 2200, 0, "GBP", "2026-09-10", "2026-10-10", "overdue"},
	{"INV-2152", "C-1004", 18300, 0, "USD", "2026-11-01", "2026-12-01", "open"},
	{"INV-2153", "C-1005", 640, 640, "CAD", "2026-11-03", "2026-12-03", "paid"},
}

type issue struct {
	Key      string `json:"key"`
	Org      string `json:"org"`
	Summary  string `json:"summary"`
	Status   string `json:"status"`
	Priority string `json:"priority"`
	Assignee string `json:"assignee"`
	Updated  string `json:"updated"`
}

var seedIssues = []issue{
	{"NWT-482", "NWT", "Invoice PDF shows wrong VAT number", "In Progress", "High", "dana@acme.example", "2026-11-24T10:02:00Z"},
	{"NWT-479", "NWT", "Can't add a second billing contact", "Open", "Medium", "", "2026-11-22T16:45:00Z"},
	{"NWT-455", "NWT", "Refund for duplicate payment", "Done", "High", "ravi@acme.example", "2026-11-10T09:30:00Z"},
	{"CTS-301", "CTS", "SSO login loops on Safari", "Open", "Critical", "lucia@acme.example", "2026-11-25T07:15:00Z"},
	{"FAB-090", "FAB", "Export to CSV drops accents", "Open", "Low", "", "2026-11-18T12:00:00Z"},
	{"GLX-212", "GLX", "Usage report missing October", "In Progress", "Medium", "dana@acme.example", "2026-11-20T14:20:00Z"},
}

// World is the running demo.
type World struct {
	Ports    map[string]int
	servers  []*http.Server
	mu       sync.Mutex
	issues   []issue
	comments map[string][]map[string]any
	holds    map[string]bool
}

// Start serves the four APIs from basePort (crm), +1 billing, +2 support, +3 legacy.
func Start(basePort int) (*World, error) {
	w := &World{Ports: map[string]int{}, issues: append([]issue{}, seedIssues...), comments: map[string][]map[string]any{},
		holds: map[string]bool{"001Ab00000FBRKM": true}}
	handlers := []struct {
		name string
		h    http.Handler
	}{{"crm", w.crm()}, {"billing", w.billing()}, {"support", w.support()}, {"legacy", w.legacy()}}
	for i, x := range handlers {
		port := 0 // 0: any free port, for each API
		if basePort != 0 {
			port = basePort + i
		}
		ln, err := net.Listen("tcp", fmt.Sprintf("127.0.0.1:%d", port))
		if err != nil {
			w.Close()
			return nil, fmt.Errorf("demo %s API can't use port %d: %w; pass --mock-port to use other ports", x.name, port, err)
		}
		if basePort == 0 {
			port = ln.Addr().(*net.TCPAddr).Port
		}
		w.Ports[x.name] = port
		srv := &http.Server{Handler: x.h, ReadHeaderTimeout: 5 * time.Second}
		w.servers = append(w.servers, srv)
		go func() { _ = srv.Serve(ln) }()
	}
	return w, nil
}

// URL of one demo API.
func (w *World) URL(name string) string { return fmt.Sprintf("http://127.0.0.1:%d", w.Ports[name]) }

// Close stops the servers.
func (w *World) Close() {
	for _, s := range w.servers {
		_ = s.Close()
	}
}

func writeJSON(rw http.ResponseWriter, status int, v any) {
	rw.Header().Set("Content-Type", "application/json")
	rw.WriteHeader(status)
	_ = json.NewEncoder(rw).Encode(v)
}

func findAccount(id string) (account, bool) {
	for _, a := range accounts {
		if a.ID == id {
			return a, true
		}
	}
	return account{}, false
}

func (w *World) crm() http.Handler {
	mux := http.NewServeMux()
	auth := func(h http.HandlerFunc) http.HandlerFunc {
		return func(rw http.ResponseWriter, r *http.Request) {
			if r.Header.Get("Authorization") != "Bearer "+CRMToken {
				writeJSON(rw, 401, map[string]any{"error": "invalid or missing bearer token"})
				return
			}
			h(rw, r)
		}
	}
	mux.HandleFunc("GET /accounts", auth(func(rw http.ResponseWriter, r *http.Request) {
		q := strings.ToLower(r.URL.Query().Get("q"))
		out := []account{}
		for _, a := range accounts {
			if q == "" || strings.Contains(strings.ToLower(a.Name), q) || strings.ToLower(a.JiraOrg) == q {
				out = append(out, a)
			}
		}
		writeJSON(rw, 200, map[string]any{"totalSize": len(out), "records": out})
	}))
	mux.HandleFunc("GET /accounts/{id}", auth(func(rw http.ResponseWriter, r *http.Request) {
		a, ok := findAccount(r.PathValue("id"))
		if !ok {
			writeJSON(rw, 404, map[string]any{"error": "no account " + r.PathValue("id")})
			return
		}
		writeJSON(rw, 200, a)
	}))
	mux.HandleFunc("PATCH /accounts/{id}", auth(func(rw http.ResponseWriter, r *http.Request) {
		a, ok := findAccount(r.PathValue("id"))
		if !ok {
			writeJSON(rw, 404, map[string]any{"error": "no account"})
			return
		}
		var patch map[string]any
		_ = json.NewDecoder(r.Body).Decode(&patch)
		writeJSON(rw, 200, map[string]any{"id": a.ID, "updated": patch})
	}))
	mux.HandleFunc("GET /accounts/{id}/contacts", auth(func(rw http.ResponseWriter, r *http.Request) {
		a, ok := findAccount(r.PathValue("id"))
		if !ok {
			writeJSON(rw, 404, map[string]any{"error": "no account"})
			return
		}
		short := strings.ToLower(strings.Fields(a.Name)[0])
		writeJSON(rw, 200, map[string]any{"records": []map[string]any{
			{"id": "003" + a.JiraOrg + "1", "name": "Alex Kim", "email": "alex@" + short + ".example", "role": "Finance"},
			{"id": "003" + a.JiraOrg + "2", "name": "Sam Patel", "email": "sam@" + short + ".example", "role": "IT"},
		}})
	}))
	mux.HandleFunc("GET /owners/{id}", auth(func(rw http.ResponseWriter, r *http.Request) {
		o, ok := owners[r.PathValue("id")]
		if !ok {
			writeJSON(rw, 404, map[string]any{"error": "no owner"})
			return
		}
		writeJSON(rw, 200, o)
	}))
	return mux
}

func (w *World) billing() http.Handler {
	mux := http.NewServeMux()
	auth := func(h http.HandlerFunc) http.HandlerFunc {
		return func(rw http.ResponseWriter, r *http.Request) {
			if r.Header.Get("X-Api-Key") != BillingKey {
				writeJSON(rw, 401, map[string]any{"error": "missing or wrong X-Api-Key"})
				return
			}
			h(rw, r)
		}
	}
	mux.HandleFunc("GET /customers/{id}/summary", auth(func(rw http.ResponseWriter, r *http.Request) {
		id := r.PathValue("id")
		var mine []invoice
		balance := 0.0
		for _, i := range invoices {
			if i.CustomerID == id {
				mine = append(mine, i)
				balance += i.Amount - i.Paid
			}
		}
		if mine == nil {
			writeJSON(rw, 404, map[string]any{"error": "no customer " + id})
			return
		}
		// Deliberately bloated, like many real billing APIs.
		full := []map[string]any{}
		for _, i := range mine {
			full = append(full, map[string]any{"number": i.Number, "amount": i.Amount, "amountPaid": i.Paid, "currency": i.Currency, "dueDate": i.Due,
				"status": i.Status, "overdue": i.Status == "overdue", "issuedAt": i.Issued,
				"lineItems": []map[string]any{{"sku": "PLAT-SEAT", "description": "Platform seat licence, annual, billed monthly", "qty": 12, "unitPrice": i.Amount / 12},
					{"sku": "SUP-PRM", "description": "Premium support add-on", "qty": 1, "unitPrice": 0}},
				"taxBreakdown": map[string]any{"rate": 0.2, "jurisdiction": "default", "exempt": false, "registration": "VAT-REG-000000"},
				"audit":        map[string]any{"createdBy": "billing-batch", "createdVia": "scheduler", "revision": 3, "lastSyncedAt": "2026-11-25T03:00:00Z"},
				"links":        map[string]any{"pdf": "/invoices/" + i.Number + ".pdf", "self": "/invoices/" + i.Number}})
		}
		writeJSON(rw, 200, map[string]any{"customerId": id, "currency": mine[0].Currency, "balance": balance, "creditLimit": 50000,
			"paymentTerms": "NET30", "dunningLevel": 1, "autopay": false, "invoices": full,
			"preferences": map[string]any{"language": "en", "invoiceDelivery": "email", "ccEmails": []string{}, "poRequired": false},
			"metadata":    map[string]any{"source": "erp", "region": "na-1", "schemaVersion": "2024-07", "featureFlags": []string{"new-dunning", "multi-currency"}}})
	}))
	mux.HandleFunc("GET /invoices", auth(func(rw http.ResponseWriter, r *http.Request) {
		cid, st := r.URL.Query().Get("customerId"), r.URL.Query().Get("status")
		out := []invoice{}
		for _, i := range invoices {
			if (cid == "" || i.CustomerID == cid) && (st == "" || i.Status == st) {
				out = append(out, i)
			}
		}
		writeJSON(rw, 200, map[string]any{"items": out, "count": len(out)})
	}))
	mux.HandleFunc("GET /invoices/{number}", auth(func(rw http.ResponseWriter, r *http.Request) {
		for _, i := range invoices {
			if i.Number == r.PathValue("number") {
				writeJSON(rw, 200, i)
				return
			}
		}
		writeJSON(rw, 404, map[string]any{"error": "no invoice"})
	}))
	mux.HandleFunc("POST /invoices/{number}/remind", auth(func(rw http.ResponseWriter, r *http.Request) {
		writeJSON(rw, 202, map[string]any{"queued": true, "invoice": r.PathValue("number")})
	}))
	return mux
}

func (w *World) support() http.Handler {
	mux := http.NewServeMux()
	auth := func(h http.HandlerFunc) http.HandlerFunc {
		return func(rw http.ResponseWriter, r *http.Request) {
			if r.Header.Get("Authorization") != "Bearer "+SupportToken {
				writeJSON(rw, 401, map[string]any{"errorMessages": []string{"You are not authenticated."}})
				return
			}
			h(rw, r)
		}
	}
	render := func(i issue) map[string]any {
		var assignee any
		if i.Assignee != "" {
			assignee = map[string]any{"emailAddress": i.Assignee, "active": true, "timeZone": "UTC"}
		}
		return map[string]any{"key": i.Key, "self": "http://127.0.0.1/rest/api/2/issue/" + i.Key, "fields": map[string]any{
			"summary": i.Summary, "status": map[string]any{"name": i.Status, "statusCategory": map[string]any{"key": strings.ToLower(strings.ReplaceAll(i.Status, " ", "-"))}},
			"priority": map[string]any{"name": i.Priority}, "assignee": assignee, "updated": i.Updated,
			"description": "Reported by the customer through the portal. Steps to reproduce and screenshots are attached to the ticket. " + i.Summary + ".",
			"labels":      []string{"customer", strings.ToLower(i.Org)}, "watches": map[string]any{"watchCount": 3, "isWatching": false}}}
	}
	mux.HandleFunc("POST /rest/api/2/search", auth(func(rw http.ResponseWriter, r *http.Request) {
		var body struct {
			JQL        string `json:"jql"`
			MaxResults int    `json:"maxResults"`
		}
		_ = json.NewDecoder(r.Body).Decode(&body)
		jql := body.JQL
		org := ""
		if i := strings.Index(jql, `org = "`); i >= 0 {
			rest := jql[i+7:]
			org = rest[:strings.Index(rest+`"`, `"`)]
		}
		notDone := strings.Contains(jql, "status != Done")
		w.mu.Lock()
		out := []map[string]any{}
		for _, is := range w.issues {
			if (org == "" || is.Org == org) && (!notDone || is.Status != "Done") {
				out = append(out, render(is))
			}
		}
		w.mu.Unlock()
		if body.MaxResults > 0 && len(out) > body.MaxResults {
			out = out[:body.MaxResults]
		}
		writeJSON(rw, 200, map[string]any{"total": len(out), "maxResults": body.MaxResults, "issues": out})
	}))
	mux.HandleFunc("GET /rest/api/2/issue/{key}", auth(func(rw http.ResponseWriter, r *http.Request) {
		w.mu.Lock()
		defer w.mu.Unlock()
		for _, is := range w.issues {
			if is.Key == r.PathValue("key") {
				m := render(is)
				m["fields"].(map[string]any)["comments"] = w.comments[is.Key]
				writeJSON(rw, 200, m)
				return
			}
		}
		writeJSON(rw, 404, map[string]any{"errorMessages": []string{"Issue does not exist"}})
	}))
	mux.HandleFunc("POST /rest/api/2/issue", auth(func(rw http.ResponseWriter, r *http.Request) {
		var body struct {
			Fields struct {
				Project string `json:"project"`
				Summary string `json:"summary"`
			} `json:"fields"`
		}
		_ = json.NewDecoder(r.Body).Decode(&body)
		w.mu.Lock()
		key := fmt.Sprintf("%s-%d", body.Fields.Project, 500+len(w.issues))
		w.issues = append(w.issues, issue{Key: key, Org: body.Fields.Project, Summary: body.Fields.Summary, Status: "Open", Priority: "Medium", Updated: time.Now().UTC().Format(time.RFC3339)})
		w.mu.Unlock()
		writeJSON(rw, 201, map[string]any{"key": key})
	}))
	mux.HandleFunc("PUT /rest/api/2/issue/{key}/assignee", auth(func(rw http.ResponseWriter, r *http.Request) {
		var body struct {
			Email string `json:"email"`
		}
		_ = json.NewDecoder(r.Body).Decode(&body)
		w.mu.Lock()
		defer w.mu.Unlock()
		for i := range w.issues {
			if w.issues[i].Key == r.PathValue("key") {
				w.issues[i].Assignee = body.Email
				writeJSON(rw, 200, map[string]any{"key": w.issues[i].Key, "assignee": body.Email})
				return
			}
		}
		writeJSON(rw, 404, map[string]any{"errorMessages": []string{"Issue does not exist"}})
	}))
	mux.HandleFunc("POST /rest/api/2/issue/{key}/comment", auth(func(rw http.ResponseWriter, r *http.Request) {
		var body map[string]any
		_ = json.NewDecoder(r.Body).Decode(&body)
		w.mu.Lock()
		w.comments[r.PathValue("key")] = append(w.comments[r.PathValue("key")], body)
		w.mu.Unlock()
		writeJSON(rw, 201, map[string]any{"id": fmt.Sprint(time.Now().UnixNano() % 100000), "body": body["body"]})
	}))
	return mux
}

func (w *World) legacy() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("POST /login", func(rw http.ResponseWriter, r *http.Request) {
		_ = r.ParseForm()
		if r.Form.Get("username") != LegacyUser || r.Form.Get("password") != LegacyPass {
			rw.WriteHeader(403)
			_, _ = io.WriteString(rw, "login failed")
			return
		}
		http.SetCookie(rw, &http.Cookie{Name: "JSESSIONID", Value: legacySession, Path: "/", HttpOnly: true})
		rw.WriteHeader(204)
	})
	authed := func(r *http.Request) bool {
		c, err := r.Cookie("JSESSIONID")
		return err == nil && c.Value == legacySession
	}
	mux.HandleFunc("GET /api/credit-hold/{account}", func(rw http.ResponseWriter, r *http.Request) {
		if !authed(r) {
			writeJSON(rw, 401, map[string]any{"error": "session expired"})
			return
		}
		a, ok := findAccount(r.PathValue("account"))
		if !ok {
			writeJSON(rw, 404, map[string]any{"error": "unknown account"})
			return
		}
		w.mu.Lock()
		hold := w.holds[a.ID]
		w.mu.Unlock()
		writeJSON(rw, 200, map[string]any{"ACCOUNT_ID": a.ID, "CREDIT_HOLD": hold, "HOLD_REASON": map[bool]string{true: "Overdue > 60 days", false: ""}[hold], "LAST_REVIEW": "2026-11-01"})
	})
	mux.HandleFunc("POST /soap/CreditService", func(rw http.ResponseWriter, r *http.Request) {
		if !authed(r) {
			rw.WriteHeader(401)
			return
		}
		b, _ := io.ReadAll(r.Body)
		rw.Header().Set("Content-Type", "text/xml")
		if strings.Contains(string(b), "SetCreditHold") {
			_, _ = io.WriteString(rw, `<soap:Envelope xmlns:soap="http://schemas.xmlsoap.org/soap/envelope/"><soap:Body><SetCreditHoldResponse><ok>true</ok></SetCreditHoldResponse></soap:Body></soap:Envelope>`)
			return
		}
		_, _ = io.WriteString(rw, `<soap:Envelope xmlns:soap="http://schemas.xmlsoap.org/soap/envelope/"><soap:Body><Fault>unknown action</Fault></soap:Body></soap:Envelope>`)
	})
	return mux
}
