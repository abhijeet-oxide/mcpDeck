package mock

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

// REQ: NET-2
func TestSlow_DelaysOnlyWhenAsked(t *testing.T) {
	h := slow(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(204) }))
	start := time.Now()
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest("GET", "/x?_delay=150ms", nil))
	if rec.Code != 204 || time.Since(start) < 150*time.Millisecond {
		t.Fatalf("code %d after %s", rec.Code, time.Since(start))
	}
	start = time.Now()
	h.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest("GET", "/x", nil))
	if time.Since(start) > 50*time.Millisecond {
		t.Fatal("delayed without being asked")
	}
}
