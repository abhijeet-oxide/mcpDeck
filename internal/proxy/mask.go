package proxy

import (
	"encoding/json"
	"net/url"
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

// stateKey matches hidden framework fields (CSRF tokens, view state, one-time
// page state) that the call log doesn't keep.
var stateKey = regexp.MustCompile(`(?i)^_|csrf|viewstate|xsrf|nonce`)

// MaskBody masks a request body for the call log: JSON as MaskJSON, and
// URL-encoded forms by field name, so session and page state stay out of
// the log.
func MaskBody(b []byte, contentType string) []byte {
	if !strings.Contains(strings.ToLower(contentType), "x-www-form-urlencoded") {
		return MaskJSON(b)
	}
	q, err := url.ParseQuery(string(b))
	if err != nil {
		return b
	}
	for k := range q {
		if IsSensitiveKey(k) || stateKey.MatchString(k) {
			q[k] = []string{"[masked]"}
		}
	}
	return []byte(q.Encode())
}

func mustJSON(v any) []byte {
	b, _ := json.Marshal(v)
	return b
}
