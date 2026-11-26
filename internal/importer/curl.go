package importer

import (
	"encoding/base64"
	"fmt"
	"net/url"
	"regexp"
	"strings"

	"github.com/abhijeet-oxide/mcpdeck/internal/store"
)

// Curl imports one or more curl commands (IMP-4). Credential headers and
// query keys are moved into secret variables, never left on the request.
func Curl(text string) (*Result, error) {
	cmds := splitCurl(text)
	if len(cmds) == 0 {
		return nil, fmt.Errorf("no curl command found")
	}
	res := &Result{Kind: "curl"}
	seenVar := map[string]bool{}
	addSecret := func(name, value string) string {
		if !seenVar[name] {
			seenVar[name] = true
			res.Vars = append(res.Vars, Var{Name: name, Value: value, Secret: true, Preview: maskPreview(value)})
		}
		return "{{" + name + "}}"
	}
	for _, cmd := range cmds {
		args, err := shellSplit(cmd)
		if err != nil {
			return nil, err
		}
		r := store.Request{Method: "", BodyType: "none"}
		var ra store.Auth // this command's auth
		var rawURL string
		var dataParts []string
		user := ""
		for i := 1; i < len(args); i++ {
			a := args[i]
			next := func() string {
				if i+1 < len(args) {
					i++
					return args[i]
				}
				return ""
			}
			switch {
			case a == "-X" || a == "--request":
				r.Method = strings.ToUpper(next())
			case a == "-H" || a == "--header":
				h := next()
				k, v, _ := strings.Cut(h, ":")
				k, v = strings.TrimSpace(k), strings.TrimSpace(v)
				lk := strings.ToLower(k)
				switch {
				case lk == "cookie" || strings.Contains(lk, "csrf") || strings.Contains(lk, "xsrf"):
					// Session values come from signing in, not from the example (AUTH-2).
					res.Warnings = appendOnce(res.Warnings, "dropped the "+k+" header from the examples; set up a login on the Auth tab and mcpDeck keeps the session")
				case lk == "authorization" && strings.HasPrefix(strings.ToLower(v), "bearer "):
					ra = store.Auth{Type: "bearer", Token: addSecret("token", strings.TrimSpace(v[7:]))}
				case lk == "authorization" && strings.HasPrefix(strings.ToLower(v), "basic "):
					dec, _ := base64.StdEncoding.DecodeString(strings.TrimSpace(v[6:]))
					u, p, _ := strings.Cut(string(dec), ":")
					ra = store.Auth{Type: "basic", Username: u, Password: addSecret("password", p)}
				case IsSecretName(k):
					ra = store.Auth{Type: "apikey", Key: k, In: "header", Value: addSecret("apiKey", v)}
				default:
					r.Headers = append(r.Headers, store.KV{Key: k, Value: v})
				}
			case a == "-d" || a == "--data" || a == "--data-raw" || a == "--data-binary" || a == "--data-urlencode" || a == "--json":
				dataParts = append(dataParts, next())
				if a == "--json" {
					r.BodyType = "json"
				}
			case a == "-u" || a == "--user":
				user = next()
			case a == "--url":
				rawURL = next()
			case a == "-b" || a == "--cookie":
				next()
				res.Warnings = appendOnce(res.Warnings, "dropped cookies from the examples; set up a login on the Auth tab and mcpDeck keeps the session")
			case strings.HasPrefix(a, "-"):
				// flags without values we don't need: -s, -L, -k, --compressed, -i, -v
				if a == "-o" || a == "--output" || a == "-A" || a == "--user-agent" || a == "-e" || a == "--referer" {
					next()
				}
			default:
				if rawURL == "" {
					rawURL = a
				}
			}
		}
		if user != "" {
			u, p, _ := strings.Cut(user, ":")
			ra = store.Auth{Type: "basic", Username: u, Password: addSecret("password", p)}
		}
		if rawURL == "" {
			return nil, fmt.Errorf("a curl command has no URL")
		}
		pu, err := url.Parse(rawURL)
		if err != nil || pu.Host == "" {
			return nil, fmt.Errorf("can't read URL %q", rawURL)
		}
		q := pu.Query()
		for k, vs := range q {
			if IsSecretName(k) {
				ra = store.Auth{Type: "apikey", Key: k, In: "query", Value: addSecret("apiKey", vs[0])}
				q.Del(k)
			}
		}
		base := pu.Scheme + "://" + pu.Host
		if res.BaseURL == "" {
			res.BaseURL = base
			res.Vars = append([]Var{{Name: "baseUrl", Value: base, Preview: base}}, res.Vars...)
		}
		prefix := "{{baseUrl}}"
		if base != res.BaseURL {
			prefix = base
		}
		path, params := paramPath(pu.EscapedPath())
		r.URL = prefix + path
		for k, vs := range q {
			r.Query = append(r.Query, store.KV{Key: k, Value: vs[0]})
		}
		if len(dataParts) > 0 {
			r.Body = strings.Join(dataParts, "&")
			if r.BodyType == "" || r.BodyType == "none" {
				r.BodyType = "form"
				if strings.HasPrefix(strings.TrimSpace(r.Body), "{") || strings.HasPrefix(strings.TrimSpace(r.Body), "[") {
					r.BodyType = "json"
				} else if strings.HasPrefix(strings.TrimSpace(r.Body), "<") {
					r.BodyType = "xml"
				}
			}
			if r.Method == "" {
				r.Method = "POST"
			}
		}
		if r.Method == "" {
			r.Method = "GET"
		}
		r.Name = NameFromPath(r.Method, path)
		r.Title = r.Method + " " + path
		if len(params) > 0 {
			r.Description = "Example values: " + strings.Join(params, ", ")
		}
		Classify(&r)
		if ra.Type != "" {
			a := ra
			r.Auth = &a
		}
		res.Requests = append(res.Requests, r)
	}
	settleAuth(res)
	res.Title = res.BaseURL
	if u, err := url.Parse(res.BaseURL); err == nil {
		h := strings.Split(u.Hostname(), ".")
		name := h[0]
		if (name == "api" || name == "www") && len(h) > 1 {
			name = h[1]
		}
		res.SuggestName = ClientName(name)
	}
	uniqueNames(res.Requests)
	return res, nil
}

var idLike = regexp.MustCompile(`[0-9]`)

// paramPath turns ID-like segments into {params}: /customers/C-1001/summary
// becomes /customers/{customerId}/summary. It returns the example values.
func paramPath(p string) (string, []string) {
	segs := strings.Split(p, "/")
	var examples []string
	for i, s := range segs {
		if s == "" || !idLike.MatchString(s) || (len(s) < 3 && !regexp.MustCompile(`^[0-9]+$`).MatchString(s)) {
			continue
		}
		if regexp.MustCompile(`^v[0-9]+$`).MatchString(s) {
			continue // API versions like v1
		}
		name := "id"
		if i > 0 && segs[i-1] != "" && !strings.HasPrefix(segs[i-1], "{") {
			prev := strings.TrimSuffix(strings.ToLower(segs[i-1]), "s")
			name = MethodName(prev) + "Id"
		}
		examples = append(examples, name+"="+s)
		segs[i] = "{" + name + "}"
	}
	return strings.Join(segs, "/"), examples
}

func splitCurl(text string) []string {
	text = strings.ReplaceAll(text, "\\\r\n", " ")
	text = strings.ReplaceAll(text, "\\\n", " ")
	text = strings.ReplaceAll(text, "^\r\n", " ") // Windows cmd continuation
	var out []string
	var cur strings.Builder
	for _, line := range strings.Split(text, "\n") {
		t := strings.TrimSpace(line)
		if strings.HasPrefix(t, "curl ") && cur.Len() > 0 {
			out = append(out, cur.String())
			cur.Reset()
		}
		if t != "" {
			cur.WriteString(t + " ")
		}
	}
	if strings.TrimSpace(cur.String()) != "" {
		out = append(out, cur.String())
	}
	var cmds []string
	for _, c := range out {
		if strings.HasPrefix(strings.TrimSpace(c), "curl") {
			cmds = append(cmds, c)
		}
	}
	return cmds
}

// shellSplit splits a command line honouring single and double quotes.
func shellSplit(s string) ([]string, error) {
	var out []string
	var cur strings.Builder
	inS, inD, esc, has := false, false, false, false
	for _, r := range s {
		switch {
		case esc:
			cur.WriteRune(r)
			esc = false
		case r == '\\' && !inS:
			esc = true
		case r == '\'' && !inD:
			inS = !inS
			has = true
		case r == '"' && !inS:
			inD = !inD
			has = true
		case (r == ' ' || r == '\t' || r == '\n' || r == '\r') && !inS && !inD:
			if cur.Len() > 0 || has {
				out = append(out, cur.String())
				cur.Reset()
				has = false
			}
		default:
			cur.WriteRune(r)
		}
	}
	if inS || inD {
		return nil, fmt.Errorf("unclosed quote in curl command")
	}
	if cur.Len() > 0 || has {
		out = append(out, cur.String())
	}
	return out, nil
}

// settleAuth makes the auth most examples use the API's auth and keeps a
// different one only on the requests that use it, like a token request
// with basic auth among calls with a bearer token.
func settleAuth(res *Result) {
	count := map[string]int{}
	var byKey = map[string]store.Auth{}
	for _, r := range res.Requests {
		if r.Auth != nil {
			k := r.Auth.Type + "|" + r.Auth.Key + "|" + r.Auth.In
			count[k]++
			byKey[k] = *r.Auth
		}
	}
	best, n := "", 0
	for k, c := range count {
		if c > n || (c == n && k < best) {
			best, n = k, c
		}
	}
	if best == "" {
		return
	}
	res.Auth = byKey[best]
	for i := range res.Requests {
		r := &res.Requests[i]
		if r.Auth == nil {
			continue
		}
		if r.Auth.Type+"|"+r.Auth.Key+"|"+r.Auth.In == best {
			r.Auth = nil
		}
	}
}

func appendOnce(list []string, s string) []string {
	for _, x := range list {
		if x == s {
			return list
		}
	}
	return append(list, s)
}
