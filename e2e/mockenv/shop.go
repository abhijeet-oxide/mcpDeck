package mockenv

import (
	"fmt"
	"net/http"
	"strconv"
	"sync"
)

// shop is an e-commerce API: log in with email and password for a short
// lived JWT-style access token plus a rotating refresh token, then page
// through orders and their line items. Modelled on common REST shop APIs
// (Shopify Admin, commercetools, Saleor REST).

type shopOrder struct {
	ID         string `json:"id"`
	Number     string `json:"number"`
	Status     string `json:"status"`
	CustomerID string `json:"customer_id"`
	Total      int    `json:"total_cents"`
	Currency   string `json:"currency"`
	Created    string `json:"created_at"`
}

var shopStatuses = []string{"paid", "shipped", "delivered", "cancelled", "pending"}

func shopOrders() []shopOrder {
	out := make([]shopOrder, 0, 25)
	for i := 1; i <= 25; i++ {
		out = append(out, shopOrder{ID: fmt.Sprintf("ord_%04d", i), Number: fmt.Sprintf("#%d", 1000+i),
			Status: shopStatuses[i%len(shopStatuses)], CustomerID: fmt.Sprintf("cus_%02d", i%7+1),
			Total: 1999 * (i%5 + 1), Currency: "USD", Created: fmt.Sprintf("2026-11-%02dT10:%02d:00Z", i%27+1, i)})
	}
	return out
}

func shopItems(orderID string) []map[string]any {
	n := len(orderID)%3 + 2
	var out []map[string]any
	for i := 0; i < n; i++ {
		sku := fmt.Sprintf("SKU-%s-%d", orderID[4:], i)
		out = append(out, map[string]any{
			"id": fmt.Sprintf("li_%s_%d", orderID, i), "sku": sku, "quantity": i + 1, "unit_price_cents": 999 + i*500,
			"product": map[string]any{"id": "prod_" + sku, "title": fmt.Sprintf("Product %s", sku), "vendor": "Acme",
				"description_html": "<p>" + longText(40) + "</p>", "images": []any{map[string]any{"src": "https://cdn.shop.example/" + sku + ".jpg", "width": 2048, "height": 2048}},
				"tags": []string{"summer", "sale", "new"}, "weight_grams": 400 + i*10},
		})
	}
	return out
}

func longText(words int) string {
	s := ""
	for i := 0; i < words; i++ {
		s += "lorem "
	}
	return s
}

func (e *Env) shop() http.Handler {
	mux := http.NewServeMux()
	var mu sync.Mutex
	orders := shopOrders()
	auth := func(h http.HandlerFunc) http.HandlerFunc {
		return func(w http.ResponseWriter, r *http.Request) {
			if _, ok := e.tokens.valid("shop", bearer(r), false); !ok {
				writeJSON(w, 401, map[string]any{"error": "invalid_token", "message": "access token missing or expired"})
				return
			}
			h(w, r)
		}
	}
	issue := func(w http.ResponseWriter, subject string) {
		writeJSON(w, 200, map[string]any{
			"access_token":  e.tokens.issue("shop", subject, e.ttl(), false),
			"refresh_token": e.tokens.issue("shop", subject, 24*e.ttl()+1, true),
			"token_type":    "Bearer", "expires_in": int(e.ttl().Seconds()),
			"user": map[string]any{"id": "usr_1", "email": subject, "name": "Pat Buyer"},
		})
	}
	mux.HandleFunc("POST /api/auth/login", func(w http.ResponseWriter, r *http.Request) {
		var in struct{ Email, Password string }
		if err := readJSON(r, &in); err != nil || in.Email != ShopUser || in.Password != ShopPass {
			writeJSON(w, 401, map[string]any{"error": "invalid_credentials"})
			return
		}
		e.inc("shop.login") // password logins only; refreshes count as shop.refresh
		issue(w, in.Email)
	})
	mux.HandleFunc("POST /api/auth/refresh", func(w http.ResponseWriter, r *http.Request) {
		var in struct {
			RefreshToken string `json:"refresh_token"`
		}
		_ = readJSON(r, &in)
		ti, ok := e.tokens.valid("shop", in.RefreshToken, true)
		if !ok {
			writeJSON(w, 401, map[string]any{"error": "invalid_grant"})
			return
		}
		e.tokens.revoke(in.RefreshToken) // refresh tokens rotate
		e.inc("shop.refresh")
		issue(w, ti.subject)
	})
	mux.HandleFunc("GET /api/orders", auth(func(w http.ResponseWriter, r *http.Request) {
		page, _ := strconv.Atoi(r.URL.Query().Get("page"))
		per, _ := strconv.Atoi(r.URL.Query().Get("per_page"))
		if page < 1 {
			page = 1
		}
		if per < 1 || per > 100 {
			per = 10
		}
		status := r.URL.Query().Get("status")
		mu.Lock()
		var list []shopOrder
		for _, o := range orders {
			if status == "" || o.Status == status {
				list = append(list, o)
			}
		}
		mu.Unlock()
		total := len(list)
		lo, hi := (page-1)*per, page*per
		if lo > total {
			lo = total
		}
		if hi > total {
			hi = total
		}
		writeJSON(w, 200, map[string]any{"data": list[lo:hi], "meta": map[string]any{"page": page, "per_page": per, "total": total, "total_pages": (total + per - 1) / per}})
	}))
	find := func(id string) (shopOrder, bool) {
		mu.Lock()
		defer mu.Unlock()
		for _, o := range orders {
			if o.ID == id {
				return o, true
			}
		}
		return shopOrder{}, false
	}
	mux.HandleFunc("GET /api/orders/{id}", auth(func(w http.ResponseWriter, r *http.Request) {
		o, ok := find(r.PathValue("id"))
		if !ok {
			writeJSON(w, 404, map[string]any{"error": "not_found", "message": "no order " + r.PathValue("id")})
			return
		}
		writeJSON(w, 200, map[string]any{"order": o, "shipping_address": map[string]any{"line1": "1 Main St", "city": "Springfield", "country": "US", "postal_code": "12345"},
			"customer": map[string]any{"id": o.CustomerID, "email": o.CustomerID + "@mail.example"}, "notes": nil})
	}))
	mux.HandleFunc("GET /api/orders/{id}/items", auth(func(w http.ResponseWriter, r *http.Request) {
		if _, ok := find(r.PathValue("id")); !ok {
			writeJSON(w, 404, map[string]any{"error": "not_found"})
			return
		}
		writeJSON(w, 200, map[string]any{"items": shopItems(r.PathValue("id"))})
	}))
	mux.HandleFunc("POST /api/orders/{id}/cancel", auth(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		defer mu.Unlock()
		for i := range orders {
			if orders[i].ID == r.PathValue("id") {
				if orders[i].Status == "delivered" {
					writeJSON(w, 409, map[string]any{"error": "conflict", "message": "delivered orders can't be cancelled"})
					return
				}
				orders[i].Status = "cancelled"
				e.inc("shop.cancel")
				writeJSON(w, 200, map[string]any{"order": orders[i]})
				return
			}
		}
		writeJSON(w, 404, map[string]any{"error": "not_found"})
	}))
	serveSpec(mux, "/openapi.json", func(r *http.Request) map[string]any {
		return specDoc{title: "Shop API", desc: "Orders, line items and customers.", server: origin(r) + "/api",
			schemes:  map[string]any{"bearerAuth": map[string]any{"type": "http", "scheme": "bearer", "bearerFormat": "JWT"}},
			security: []string{"bearerAuth"},
			ops: []op{
				{method: "POST", path: "/auth/login", id: "login", summary: "Log in with email and password", tag: "auth", noSecurity: true,
					body: jsonBody(map[string]any{"email": "you@shop.example", "password": ""}),
					resp: map[string]any{"access_token": "eyJ...", "refresh_token": "rt...", "token_type": "Bearer", "expires_in": 3600}},
				{method: "POST", path: "/auth/refresh", id: "refreshToken", summary: "Exchange a refresh token", tag: "auth", noSecurity: true,
					body: jsonBody(map[string]any{"refresh_token": ""})},
				{method: "GET", path: "/orders", id: "listOrders", summary: "List orders, newest first", tag: "orders",
					params: []param{q("page", 1), q("per_page", 10), q("status", nil)},
					resp:   map[string]any{"data": []any{map[string]any{"id": "ord_0001", "number": "#1001", "status": "paid", "customer_id": "cus_01", "total_cents": 1999, "currency": "USD", "created_at": "2026-11-01T10:00:00Z"}}, "meta": map[string]any{"page": 1, "per_page": 10, "total": 1, "total_pages": 1}}},
				{method: "GET", path: "/orders/{id}", id: "getOrder", summary: "Get one order", tag: "orders", params: []param{pp("id")}},
				{method: "GET", path: "/orders/{id}/items", id: "listOrderItems", summary: "List an order's line items", tag: "orders", params: []param{pp("id")}},
				{method: "POST", path: "/orders/{id}/cancel", id: "cancelOrder", summary: "Cancel an order", tag: "orders", params: []param{pp("id")}},
			}}.v3()
	})
	return mux
}
