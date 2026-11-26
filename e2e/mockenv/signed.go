package mockenv

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"math"
	"net/http"
	"strconv"
	"sync"
	"time"
)

// payments signs every request with HMAC-SHA256, like Coinbase, Binance,
// Kraken and many payment gateways: X-Key-Id names the key, X-Timestamp
// is Unix seconds (±300 s), and X-Signature is the lowercase hex HMAC of
//
//	METHOD \n PATH?QUERY \n TIMESTAMP \n hex(sha256(body))
//
// Refunds also need an Idempotency-Key header.

// PaymentsStringToSign is the documented canonical string.
func PaymentsStringToSign(method, pathQuery, ts string, body []byte) string {
	h := sha256.Sum256(body)
	return method + "\n" + pathQuery + "\n" + ts + "\n" + hex.EncodeToString(h[:])
}

func (e *Env) payments() http.Handler {
	mux := http.NewServeMux()
	var mu sync.Mutex
	refunds := map[string]map[string]any{}
	verify := func(w http.ResponseWriter, r *http.Request) ([]byte, bool) {
		body, _ := io.ReadAll(io.LimitReader(r.Body, 1<<20))
		if r.Header.Get("X-Key-Id") != HMACKeyID {
			writeJSON(w, 401, map[string]any{"code": "unknown_key", "message": "X-Key-Id is missing or unknown"})
			return nil, false
		}
		ts := r.Header.Get("X-Timestamp")
		n, err := strconv.ParseInt(ts, 10, 64)
		if err != nil || math.Abs(float64(time.Now().Unix()-n)) > 300 {
			writeJSON(w, 401, map[string]any{"code": "stale_timestamp", "message": "X-Timestamp must be Unix seconds within 5 minutes"})
			return nil, false
		}
		mac := hmac.New(sha256.New, []byte(HMACSecret))
		mac.Write([]byte(PaymentsStringToSign(r.Method, r.URL.RequestURI(), ts, body)))
		if !hmac.Equal([]byte(hex.EncodeToString(mac.Sum(nil))), []byte(r.Header.Get("X-Signature"))) {
			e.inc("payments.bad_signature")
			writeJSON(w, 401, map[string]any{"code": "bad_signature", "message": "X-Signature doesn't match the request"})
			return nil, false
		}
		e.inc("payments.ok")
		return body, true
	}
	mux.HandleFunc("GET /v2/payments", func(w http.ResponseWriter, r *http.Request) {
		if _, ok := verify(w, r); !ok {
			return
		}
		lim, _ := strconv.Atoi(r.URL.Query().Get("limit"))
		if lim <= 0 {
			lim = 5
		}
		var out []any
		for i := 0; i < lim; i++ {
			out = append(out, map[string]any{"id": fmt.Sprintf("pay_%04d", i+1), "amount": 2500 + i*100, "currency": "EUR", "status": []string{"captured", "refunded", "failed"}[i%3],
				"card": map[string]any{"brand": "visa", "last4": "4242", "exp_month": 12, "exp_year": 2029}, "created": time.Now().Add(-time.Duration(i) * time.Hour).UTC().Format(time.RFC3339)})
		}
		writeJSON(w, 200, map[string]any{"payments": out})
	})
	mux.HandleFunc("GET /v2/payments/{id}", func(w http.ResponseWriter, r *http.Request) {
		if _, ok := verify(w, r); !ok {
			return
		}
		writeJSON(w, 200, map[string]any{"id": r.PathValue("id"), "amount": 2500, "currency": "EUR", "status": "captured"})
	})
	mux.HandleFunc("POST /v2/refunds", func(w http.ResponseWriter, r *http.Request) {
		if _, ok := verify(w, r); !ok {
			return
		}
		key := r.Header.Get("Idempotency-Key")
		if key == "" {
			writeJSON(w, 400, map[string]any{"code": "missing_idempotency_key", "message": "send an Idempotency-Key header"})
			return
		}
		mu.Lock()
		defer mu.Unlock()
		if prev, ok := refunds[key]; ok {
			writeJSON(w, 200, prev)
			return
		}
		ref := map[string]any{"id": fmt.Sprintf("re_%04d", len(refunds)+1), "status": "pending"}
		refunds[key] = ref
		e.inc("payments.refund")
		writeJSON(w, 201, ref)
	})
	serveSpec(mux, "/openapi.json", func(r *http.Request) map[string]any {
		return specDoc{title: "Payments API", desc: "Signed requests: see the HMAC section of the docs.", server: origin(r),
			schemes:  map[string]any{"keyId": map[string]any{"type": "apiKey", "in": "header", "name": "X-Key-Id"}},
			security: []string{"keyId"},
			ops: []op{
				{method: "GET", path: "/v2/payments", id: "listPayments", summary: "List recent payments", params: []param{q("limit", 5)}},
				{method: "GET", path: "/v2/payments/{id}", id: "getPayment", summary: "Get a payment", params: []param{pp("id")}},
				{method: "POST", path: "/v2/refunds", id: "createRefund", summary: "Refund a payment", params: []param{hp("Idempotency-Key", "")},
					body: jsonBody(map[string]any{"payment_id": "pay_0001", "amount": 100})},
			}}.v3()
	})
	return mux
}

// mtlsService needs a client certificate signed by the test CA plus an
// X-Vault-Token header, like HashiCorp Vault with TLS client auth.
func (e *Env) mtlsService() http.Handler {
	mux := http.NewServeMux()
	guard := func(h http.HandlerFunc) http.HandlerFunc {
		return func(w http.ResponseWriter, r *http.Request) {
			if r.TLS == nil || len(r.TLS.PeerCertificates) == 0 {
				writeJSON(w, 400, map[string]any{"errors": []string{"client certificate required"}})
				return
			}
			if r.Header.Get("X-Vault-Token") != MTLSToken {
				writeJSON(w, 403, map[string]any{"errors": []string{"permission denied"}})
				return
			}
			h(w, r)
		}
	}
	mux.HandleFunc("GET /v1/sys/health", func(w http.ResponseWriter, r *http.Request) {
		cn := ""
		if r.TLS != nil && len(r.TLS.PeerCertificates) > 0 {
			cn = r.TLS.PeerCertificates[0].Subject.CommonName
		}
		writeJSON(w, 200, map[string]any{"initialized": true, "sealed": false, "standby": false, "version": "1.17.3", "client_cn": cn})
	})
	mux.HandleFunc("GET /v1/secret/metadata/{path...}", guard(func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, 200, map[string]any{"data": map[string]any{"current_version": 3, "created_time": "2026-01-01T00:00:00Z",
			"updated_time": "2026-11-01T00:00:00Z", "max_versions": 10, "path": r.PathValue("path")}})
	}))
	return mux
}
