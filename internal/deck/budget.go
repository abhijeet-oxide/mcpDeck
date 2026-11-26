package deck

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/abhijeet-oxide/mcpdeck/internal/tokens"
)

// OutputBudget is the most tokens one tool result sends an agent. Claude
// Code cuts tool results over 25,000 tokens by default, mid-JSON; mcpDeck
// cuts the longest list instead and says how to get the rest (USE-4).
const OutputBudget = 20000

// fitOutput shortens the longest list in a JSON output until it fits the
// budget, and adds a _mcpdeck note with the counts and a hint.
func fitOutput(out string, budget int) (string, bool) {
	n := tokens.Count(out)
	if n <= budget {
		return out, false
	}
	var v any
	if json.Unmarshal([]byte(out), &v) != nil {
		r := []rune(out)
		return string(r[:len(r)*budget/n]) + fmt.Sprintf("\n… cut by mcpDeck: the output was %d tokens, over the %d-token budget. Ask for less (a limit, page or filter).", n, budget), true
	}
	keys, items := longestList(v, nil)
	if len(items) < 2 {
		return out, false
	}
	label := budget
	budget -= budget / 50 // headroom: token counts are estimates
	render := func(k int) string {
		b, _ := json.Marshal(withNote(setAt(v, keys, items[:k]), keys, k, len(items), n, label))
		return string(b)
	}
	lo, hi := 1, len(items)-1
	for lo < hi { // the most items that fit
		mid := (lo + hi + 1) / 2
		if tokens.Count(render(mid)) <= budget {
			lo = mid
		} else {
			hi = mid - 1
		}
	}
	return render(lo), true
}

// longestList finds the longest list reachable through object keys.
func longestList(v any, keys []string) ([]string, []any) {
	switch t := v.(type) {
	case []any:
		return keys, t
	case map[string]any:
		var bk []string
		var bl []any
		for k, c := range t {
			ck, cl := longestList(c, append(append([]string{}, keys...), k))
			if len(cl) > len(bl) {
				bk, bl = ck, cl
			}
		}
		return bk, bl
	}
	return nil, nil
}

// setAt returns a copy of v with the value at keys replaced.
func setAt(v any, keys []string, nv any) any {
	if len(keys) == 0 {
		return nv
	}
	m, _ := v.(map[string]any)
	c := make(map[string]any, len(m))
	for k, x := range m {
		c[k] = x
	}
	c[keys[0]] = setAt(m[keys[0]], keys[1:], nv)
	return c
}

func withNote(v any, keys []string, shown, total, was, budget int) any {
	list := strings.Join(keys, ".")
	if list == "" {
		list = "the list"
	}
	note := map[string]any{"truncated": true, "list": list, "shown": shown, "total": total,
		"hint": fmt.Sprintf("The output was %d tokens, over mcpDeck's %d-token budget, so %s was cut to the first %d of %d items. Call again with a narrower input (a limit, page or filter) for the rest.", was, budget, list, shown, total)}
	if m, ok := v.(map[string]any); ok {
		m["_mcpdeck"] = note
		return m
	}
	return map[string]any{"items": v, "_mcpdeck": note}
}
