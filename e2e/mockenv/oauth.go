package mockenv

import (
	"fmt"
	"net/http"
	"strconv"
	"strings"
)

// idp is an OAuth 2.0 authorization server on its own host (like Okta,
// Auth0, Entra ID or Keycloak): POST /oauth2/token supports
// client_credentials, password and refresh_token grants, with client
// authentication by HTTP basic or form fields (RFC 6749 §2.3.1). Its
// tokens open billing, a Stripe-style API on a different host with
// cursor pagination (starting_after / has_more).

func (e *Env) idp() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /.well-known/openid-configuration", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, 200, map[string]any{"issuer": origin(r), "token_endpoint": origin(r) + "/oauth2/token",
			"grant_types_supported": []string{"client_credentials", "password", "refresh_token"}})
	})
	mux.HandleFunc("POST /oauth2/token", func(w http.ResponseWriter, r *http.Request) {
		if !strings.HasPrefix(r.Header.Get("Content-Type"), "application/x-www-form-urlencoded") {
			writeJSON(w, 400, map[string]any{"error": "invalid_request", "error_description": "use application/x-www-form-urlencoded"})
			return
		}
		_ = r.ParseForm()
		f := r.PostForm
		id, secret, basic := r.BasicAuth()
		if !basic {
			id, secret = f.Get("client_id"), f.Get("client_secret")
		}
		if id != OAuthClientID || secret != OAuthSecret {
			e.inc("idp.bad_client")
			w.Header().Set("WWW-Authenticate", `Basic realm="idp"`)
			writeJSON(w, 401, map[string]any{"error": "invalid_client"})
			return
		}
		scope := f.Get("scope")
		if scope == "" {
			scope = "billing.read"
		}
		subject := "client:" + id
		switch f.Get("grant_type") {
		case "client_credentials":
			e.inc("idp.client_credentials")
		case "password":
			if f.Get("username") != OAuthUser || f.Get("password") != OAuthUserPass {
				writeJSON(w, 400, map[string]any{"error": "invalid_grant", "error_description": "bad username or password"})
				return
			}
			subject = f.Get("username")
			e.inc("idp.password")
		case "refresh_token":
			ti, ok := e.tokens.valid("billing", f.Get("refresh_token"), true)
			if !ok {
				writeJSON(w, 400, map[string]any{"error": "invalid_grant", "error_description": "refresh token expired or revoked"})
				return
			}
			e.tokens.revoke(f.Get("refresh_token"))
			subject = ti.subject
			e.inc("idp.refresh")
		default:
			writeJSON(w, 400, map[string]any{"error": "unsupported_grant_type"})
			return
		}
		out := map[string]any{"access_token": e.tokens.issue("billing", subject, e.ttl(), false), "token_type": "Bearer",
			"expires_in": int(e.ttl().Seconds()), "scope": scope}
		if f.Get("grant_type") != "client_credentials" {
			out["refresh_token"] = e.tokens.issue("billing", subject, 24*e.ttl(), true)
		}
		writeJSON(w, 200, out)
	})
	return mux
}

type invoiceRow struct {
	ID, Customer, Status, Currency string
	Amount                         int
	Created                        int64
}

func (e *Env) saasBilling() http.Handler {
	mux := http.NewServeMux()
	var invs []invoiceRow
	sts := []string{"paid", "open", "void", "uncollectible", "open"}
	for i := 0; i < 57; i++ {
		invs = append(invs, invoiceRow{ID: fmt.Sprintf("in_%05d", 90000-i), Customer: fmt.Sprintf("cus_%03d", i%13), Status: sts[i%len(sts)],
			Currency: "usd", Amount: 1500 + i*275, Created: 1795000000 - int64(i)*86400})
	}
	auth := func(h http.HandlerFunc) http.HandlerFunc {
		return func(w http.ResponseWriter, r *http.Request) {
			if _, ok := e.tokens.valid("billing", bearer(r), false); !ok {
				w.Header().Set("WWW-Authenticate", `Bearer error="invalid_token"`)
				writeJSON(w, 401, map[string]any{"error": map[string]any{"type": "invalid_request_error", "message": "Invalid or expired access token"}})
				return
			}
			h(w, r)
		}
	}
	mux.HandleFunc("GET /v1/invoices", auth(func(w http.ResponseWriter, r *http.Request) {
		lim, _ := strconv.Atoi(r.URL.Query().Get("limit"))
		if lim <= 0 || lim > 100 {
			lim = 10
		}
		after := r.URL.Query().Get("starting_after")
		status := r.URL.Query().Get("status")
		var list []invoiceRow
		for _, in := range invs {
			if status == "" || in.Status == status {
				list = append(list, in)
			}
		}
		start := 0
		if after != "" {
			for i, in := range list {
				if in.ID == after {
					start = i + 1
				}
			}
		}
		end := start + lim
		if end > len(list) {
			end = len(list)
		}
		var data []any
		for _, in := range list[start:end] {
			data = append(data, map[string]any{"id": in.ID, "object": "invoice", "customer": in.Customer, "status": in.Status, "currency": in.Currency,
				"amount_due": in.Amount, "amount_paid": map[bool]int{true: in.Amount}[in.Status == "paid"], "created": in.Created,
				"lines":    map[string]any{"object": "list", "data": []any{map[string]any{"id": "il_" + in.ID, "amount": in.Amount, "description": "Pro plan × 1"}}, "has_more": false},
				"metadata": map[string]any{}, "livemode": false})
		}
		writeJSON(w, 200, map[string]any{"object": "list", "url": "/v1/invoices", "has_more": end < len(list), "data": data})
	}))
	mux.HandleFunc("GET /v1/customers/{id}", auth(func(w http.ResponseWriter, r *http.Request) {
		id := r.PathValue("id")
		if !strings.HasPrefix(id, "cus_") {
			writeJSON(w, 404, map[string]any{"error": map[string]any{"type": "invalid_request_error", "message": "No such customer: '" + id + "'"}})
			return
		}
		writeJSON(w, 200, map[string]any{"id": id, "object": "customer", "email": id + "@corp.example", "name": "Customer " + id[4:], "balance": 0, "delinquent": id == "cus_003"})
	}))
	serveSpec(mux, "/openapi.json", func(r *http.Request) map[string]any {
		return specDoc{title: "Billing API", desc: "Invoices and customers.", server: origin(r),
			schemes: map[string]any{"oauth2": map[string]any{"type": "oauth2", "flows": map[string]any{"clientCredentials": map[string]any{
				"tokenUrl": e.URLs["idp"] + "/oauth2/token", "scopes": map[string]any{"billing.read": "Read billing"}}}}},
			security: []string{"oauth2"},
			ops: []op{
				{method: "GET", path: "/v1/invoices", id: "listInvoices", summary: "List invoices, newest first", params: []param{q("limit", 10), q("starting_after", nil), q("status", nil)}},
				{method: "GET", path: "/v1/customers/{id}", id: "getCustomer", summary: "Retrieve a customer", params: []param{pp("id")}},
			}}.v3()
	})
	return mux
}
