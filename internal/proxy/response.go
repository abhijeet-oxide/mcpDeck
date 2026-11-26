package proxy

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"mime"
	"net/http"
	"regexp"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"
)

// isBinary reports a response body that isn't text: files, images,
// archives. Tools get its facts instead of mangled bytes (RESP-2).
func isBinary(contentType string, body []byte) bool {
	mt, _, _ := mime.ParseMediaType(contentType)
	mt = strings.ToLower(mt)
	switch {
	case mt == "":
	case strings.HasPrefix(mt, "text/"), strings.HasSuffix(mt, "+json"), strings.HasSuffix(mt, "+xml"),
		mt == "application/json", mt == "application/xml", mt == "application/javascript", mt == "application/x-www-form-urlencoded",
		mt == "application/yaml", mt == "application/x-yaml", mt == "application/graphql-response+json", mt == "application/problem+json":
		return !utf8.Valid(body)
	default:
		return true
	}
	return !utf8.Valid(body)
}

// binaryFacts is what a tool sees for a binary response.
func binaryFacts(h http.Header, body []byte) []byte {
	sum := sha256.Sum256(body)
	out := map[string]any{"binary": true, "contentType": h.Get("Content-Type"), "size": len(body), "sha256": hex.EncodeToString(sum[:])}
	if _, params, err := mime.ParseMediaType(h.Get("Content-Disposition")); err == nil && params["filename"] != "" {
		out["filename"] = params["filename"]
	}
	b, _ := json.Marshal(out)
	return b
}

var tagRe = regexp.MustCompile(`(?s)<script.*?</script>|<style.*?</style>|<[^>]+>`)
var spaceRe = regexp.MustCompile(`\s+`)

// UpstreamMessage is the short reason an API gave for an error, from the
// fields APIs commonly use, or the text of an HTML or XML error page.
func UpstreamMessage(body []byte, contentType string) string {
	clip := func(s string) string {
		s = strings.TrimSpace(spaceRe.ReplaceAllString(s, " "))
		if len(s) > 240 {
			s = s[:240] + "…"
		}
		return s
	}
	var v any
	if json.Unmarshal(body, &v) == nil {
		if s := findMessage(v, 0); s != "" {
			return clip(s)
		}
		if len(body) <= 240 {
			return clip(string(body))
		}
		return ""
	}
	s := string(body)
	if i := strings.Index(s, "<faultstring>"); i >= 0 {
		if j := strings.Index(s[i:], "</faultstring>"); j > 0 {
			return clip(s[i+len("<faultstring>") : i+j])
		}
	}
	if strings.Contains(strings.ToLower(contentType), "html") || strings.HasPrefix(strings.TrimSpace(s), "<") {
		return clip(tagRe.ReplaceAllString(s, " "))
	}
	if !utf8.ValidString(s) {
		return ""
	}
	return clip(s)
}

func findMessage(v any, depth int) string {
	if depth > 3 {
		return ""
	}
	switch t := v.(type) {
	case string:
		return t
	case []any:
		var parts []string
		for _, x := range t {
			if s := findMessage(x, depth+1); s != "" {
				parts = append(parts, s)
			}
			if len(parts) == 3 {
				break
			}
		}
		return strings.Join(parts, "; ")
	case map[string]any:
		for _, k := range []string{"message", "error_description", "errorMessages", "detail", "title", "errors", "error", "faultstring", "msg", "reason", "description"} {
			if x, ok := t[k]; ok {
				if s := findMessage(x, depth+1); s != "" {
					if k == "error" && t["error_description"] == nil {
						return s
					}
					return s
				}
			}
		}
	}
	return ""
}

// retryAfter is how long to wait before retrying a 429 or 503 (RFC 9110
// 10.2.3): Retry-After seconds or a date, else a short backoff. ok is
// false when the wait is too long to be worth it inside a tool call.
func retryAfter(h http.Header, attempt int, now time.Time) (time.Duration, bool) {
	ra := strings.TrimSpace(h.Get("Retry-After"))
	d := time.Duration(500*(1<<attempt)) * time.Millisecond
	if ra != "" {
		if n, err := strconv.Atoi(ra); err == nil {
			d = time.Duration(n) * time.Second
		} else if t, err := http.ParseTime(ra); err == nil {
			d = t.Sub(now)
		}
	}
	if d < 0 {
		d = 0
	}
	return d, d <= 10*time.Second
}
