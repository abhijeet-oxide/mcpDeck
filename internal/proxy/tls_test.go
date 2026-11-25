package proxy

import (
	"context"
	"encoding/pem"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/abhijeet-oxide/mcpdeck/internal/store"
)

// REQ: API-9
func TestDo_TLSSettingsPerAPI(t *testing.T) {
	ctx := context.Background()
	f := setup(t, nil)
	tlsSrv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = io.WriteString(w, `{"ok":true}`)
	}))
	t.Cleanup(tlsSrv.Close)
	v, _ := f.s.Variable(ctx, "v1")
	v.Value = tlsSrv.URL
	_ = f.s.PutVariable(ctx, v)

	// A self-signed certificate fails with a message that says what to do.
	_, err := f.p.Do(ctx, Call{APIID: "api1", RequestID: "rGET", Args: Args{Flat: map[string]any{"id": "1"}}})
	if !IsCode(err, CodeBadTLS) {
		t.Fatalf("self-signed: got %v, want %s", err, CodeBadTLS)
	}

	// Trusting the server's certificate makes it work.
	api, _ := f.s.API(ctx, "api1")
	api.TLS.CACerts = string(pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: tlsSrv.Certificate().Raw}))
	_ = f.s.PutAPI(ctx, api)
	if res, err := f.p.Do(ctx, Call{APIID: "api1", RequestID: "rGET", Args: Args{Flat: map[string]any{"id": "1"}}}); err != nil || res.Status != 200 {
		t.Fatalf("trusted CA: %v %v", res, err)
	}

	// Turning verification off works too.
	api.TLS = store.TLS{Insecure: true}
	_ = f.s.PutAPI(ctx, api)
	if res, err := f.p.Do(ctx, Call{APIID: "api1", RequestID: "rGET", Args: Args{Flat: map[string]any{"id": "1"}}}); err != nil || res.Status != 200 {
		t.Fatalf("insecure: %v %v", res, err)
	}

	// Bad PEM is an error, not a silent fallback.
	api.TLS.Insecure, api.TLS.CACerts = false, "not a cert"
	_ = f.s.PutAPI(ctx, api)
	if _, err := f.p.Do(ctx, Call{APIID: "api1", RequestID: "rGET", Args: Args{Flat: map[string]any{"id": "1"}}}); !IsCode(err, CodeBadTLS) {
		t.Fatalf("bad pem: got %v", err)
	}
}
