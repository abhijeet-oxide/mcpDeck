package tokens

import "testing"

// REQ: SES-3
func TestCount_UsesNamedOfflineEncoder(t *testing.T) {
	if got := Count(`{"name":"Northwind Traders","tier":"gold"}`); got < 5 || got > 20 {
		t.Fatalf("unexpected token count %d", got)
	}
	if Count("") != 0 {
		t.Fatal("empty string should be 0 tokens")
	}
}
