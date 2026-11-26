package proxy

import (
	"encoding/json"
	"regexp"
	"sort"
	"strings"
)

// sensitiveKey matches field names whose values are masked before storage (EX-1).
var sensitiveKey = regexp.MustCompile(`(?i)(password|passwd|token|secret|authorization|api[-_]?key|ssn|card|iban|cookie|session)`)

// IsSensitiveKey reports whether a field or header name holds a secret.
func IsSensitiveKey(k string) bool { return sensitiveKey.MatchString(k) }

// MaskJSON replaces values of sensitive fields in a JSON document. Non-JSON
// input is returned unchanged.
func MaskJSON(b []byte) []byte {
	var v any
	if err := json.Unmarshal(b, &v); err != nil {
		return b
	}
	out, err := json.Marshal(maskValue(v))
	if err != nil {
		return b
	}
	return out
}

func maskValue(v any) any {
	switch t := v.(type) {
	case map[string]any:
		for k, x := range t {
			if IsSensitiveKey(k) {
				if _, isObj := x.(map[string]any); !isObj {
					t[k] = "[masked]"
					continue
				}
			}
			t[k] = maskValue(x)
		}
		return t
	case []any:
		for i := range t {
			t[i] = maskValue(t[i])
		}
		return t
	default:
		return v
	}
}

// redactor replaces known secret values anywhere in text (CRED-4).
type redactor []string

func (r redactor) String(s string) string {
	for _, sec := range r {
		if len(sec) >= 4 {
			s = strings.ReplaceAll(s, sec, "[secret]")
		}
	}
	return s
}

func (r redactor) Bytes(b []byte) []byte {
	if len(r) == 0 {
		return b
	}
	return []byte(r.String(string(b)))
}

func (r redactor) sortLongestFirst() {
	sort.Slice(r, func(i, j int) bool { return len(r[i]) > len(r[j]) })
}
