package proxy

import (
	"encoding/json"
	"math"
	"net/url"
	"regexp"
	"strings"

	"github.com/abhijeet-oxide/mcpdeck/internal/store"
)

// MatchRequest finds the saved request a raw "METHOD /path" call means,
// and the path parameter values in it. Literal segments beat parameters.
func MatchRequest(reqs []store.Request, method, path string) (*store.Request, map[string]string) {
	method = strings.ToUpper(method)
	var best *store.Request
	var bestParams map[string]string
	bestScore := math.MinInt
	for i := range reqs {
		r := &reqs[i]
		if strings.ToUpper(r.Method) != method {
			continue
		}
		tmpl, ok := strings.CutPrefix(r.URL, "{{baseUrl}}")
		if !ok {
			continue
		}
		tmpl, _, _ = strings.Cut(tmpl, "?")
		re, names := templateRe(tmpl)
		m := re.FindStringSubmatch(path)
		if m == nil {
			continue
		}
		score := len(tmpl) - 10*len(names)
		if score > bestScore {
			bestScore, best = score, r
			bestParams = map[string]string{}
			for j, n := range names {
				bestParams[n] = m[j+1]
			}
		}
	}
	return best, bestParams
}

func templateRe(tmpl string) (*regexp.Regexp, []string) {
	var names []string
	var b strings.Builder
	b.WriteString("^")
	last := 0
	for _, loc := range pathParamRe.FindAllStringSubmatchIndex(tmpl, -1) {
		b.WriteString(regexp.QuoteMeta(tmpl[last:loc[0]]))
		if loc[2] >= 0 {
			names = append(names, tmpl[loc[2]:loc[3]])
			b.WriteString("([^/]+)")
		} else {
			names = append(names, tmpl[loc[4]:loc[5]])
			b.WriteString("/([^/]+)")
		}
		last = loc[1]
	}
	b.WriteString(regexp.QuoteMeta(tmpl[last:]))
	b.WriteString("/?$")
	return regexp.MustCompile(b.String()), names
}

func withPathArgs(a Args, params map[string]string) Args {
	if len(params) == 0 {
		return a
	}
	p := map[string]any{}
	for k, v := range a.Path {
		p[k] = v
	}
	for k, v := range params {
		if _, set := p[k]; !set {
			p[k] = unescape(v)
		}
	}
	a.Path = p
	return a
}

func unescape(s string) string {
	if u, err := pathUnescape(s); err == nil {
		return u
	}
	return s
}

// ClassifyRaw is the access of a call with no saved request: by method,
// except a GraphQL POST, which reads unless it sends a mutation.
func ClassifyRaw(method, path string, a Args) string {
	if strings.ToUpper(method) == "POST" && IsGraphQLPath(path) {
		body := a.Body
		if !a.HasBody && a.Flat != nil {
			body = a.Flat
		}
		if b, err := json.Marshal(body); err == nil && !IsGraphQLMutation(string(b)) {
			return "read"
		}
		return "write"
	}
	return ClassifyMethod(method)
}

// IsGraphQLPath reports an endpoint path that serves GraphQL.
func IsGraphQLPath(path string) bool {
	p := strings.ToLower(strings.TrimSuffix(strings.SplitN(path, "?", 2)[0], "/"))
	return strings.HasSuffix(p, "/graphql") || strings.HasSuffix(p, "/gql")
}

var mutationRe = regexp.MustCompile(`(^|[^A-Za-z0-9_])mutation([^A-Za-z0-9_]|$)`)

// IsGraphQLMutation reports a GraphQL document (or code holding one) that changes data.
func IsGraphQLMutation(doc string) bool { return mutationRe.MatchString(doc) }

func pathUnescape(s string) (string, error) { return url.PathUnescape(s) }
