package deck

import (
	"encoding/json"
	"fmt"
	"strings"
	"testing"

	"github.com/abhijeet-oxide/mcpdeck/internal/tokens"
)

func bigItems(n int) []any {
	var l []any
	for i := 0; i < n; i++ {
		l = append(l, map[string]any{"id": fmt.Sprintf("item-%05d", i), "note": strings.Repeat("lorem ipsum ", 8)})
	}
	return l
}

// REQ: USE-4
func TestFitOutput(t *testing.T) {
	small, _ := json.Marshal(map[string]any{"items": bigItems(3)})
	if out, cut := fitOutput(string(small), 1000); cut || out != string(small) {
		t.Fatal("small outputs pass through unchanged")
	}
	for _, v := range []any{map[string]any{"total": 5000, "page": map[string]any{"rows": bigItems(5000)}}, bigItems(5000)} {
		b, _ := json.Marshal(v)
		out, cut := fitOutput(string(b), 2000)
		if !cut || tokens.Count(out) > 2000 {
			t.Fatalf("cut=%v tokens=%d", cut, tokens.Count(out))
		}
		var got map[string]any
		if err := json.Unmarshal([]byte(out), &got); err != nil {
			t.Fatalf("output is not JSON: %v", err)
		}
		note, _ := got["_mcpdeck"].(map[string]any)
		if note == nil || note["total"] != float64(5000) || note["shown"].(float64) < 10 || !strings.Contains(note["hint"].(string), "limit") {
			t.Fatalf("note %+v", note)
		}
	}
	text := strings.Repeat("word ", 20000)
	if out, cut := fitOutput(text, 1000); !cut || !strings.Contains(out, "cut by mcpDeck") || tokens.Count(out) > 1100 {
		t.Fatalf("text cut=%v tokens=%d", cut, tokens.Count(out))
	}
}
