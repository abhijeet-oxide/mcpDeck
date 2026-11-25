package importer

import (
	"bufio"
	"encoding/json"
	"fmt"
	"sort"
	"strings"
)

// Environment is an environment file found by an import (ENV-2).
type Environment struct {
	Kind string `json:"kind"` // postman | bruno | dotenv | json
	Name string `json:"name"`
	Vars []Var  `json:"vars"`
}

// ParseEnvironment reads an exported environment: a Postman environment,
// a Bruno environment (.bru or its JSON export), a .env file, or a flat
// JSON object of names to values. filename only helps pick a name.
func ParseEnvironment(data []byte, filename string) (*Environment, error) {
	text := strings.TrimSpace(strings.TrimPrefix(string(data), "\ufeff"))
	if text == "" {
		return nil, fmt.Errorf("the environment file is empty; export it from Postman or Bruno and pick that file")
	}
	var env *Environment
	var err error
	switch {
	case strings.HasPrefix(text, "{"):
		env, err = jsonEnv([]byte(text))
	case strings.HasPrefix(text, "vars") || strings.Contains(text, "\nvars {") || strings.Contains(text, "vars:secret"):
		env, err = bruEnv(text)
	default:
		env, err = dotEnv(text)
	}
	if err != nil {
		return nil, err
	}
	if env.Name == "" {
		env.Name = nameFromFile(filename)
	}
	for i := range env.Vars {
		v := &env.Vars[i]
		v.Env = env.Name
		if !v.Secret && IsSecretName(v.Name) {
			v.Secret = true
		}
		v.Preview = v.Value
		if v.Secret {
			v.Preview = maskPreview(v.Value)
		}
	}
	if len(env.Vars) == 0 {
		return nil, fmt.Errorf("no variables found in %s; check it's an environment export, not a collection", orDefault(filename, "the file"))
	}
	return env, nil
}

func orDefault(s, d string) string {
	if s == "" {
		return d
	}
	return s
}

func nameFromFile(f string) string {
	f = strings.ReplaceAll(f, "\\", "/")
	if i := strings.LastIndex(f, "/"); i >= 0 {
		f = f[i+1:]
	}
	for _, ext := range []string{".postman_environment.json", ".bru", ".json", ".env"} {
		f = strings.TrimSuffix(f, ext)
	}
	f = strings.TrimSpace(f)
	if f == "" || f == ".env" {
		return "Imported"
	}
	return f
}

func jsonEnv(data []byte) (*Environment, error) {
	var doc map[string]json.RawMessage
	if err := json.Unmarshal(data, &doc); err != nil {
		return nil, fmt.Errorf("not valid JSON: %w", err)
	}
	var name string
	_ = json.Unmarshal(doc["name"], &name)
	// Postman: { name, values: [{ key, value, type, enabled }] }
	if raw, ok := doc["values"]; ok {
		var vals []struct {
			Key     string `json:"key"`
			Value   any    `json:"value"`
			Type    string `json:"type"`
			Enabled *bool  `json:"enabled"`
		}
		if err := json.Unmarshal(raw, &vals); err != nil {
			return nil, fmt.Errorf("postman environment: %w", err)
		}
		e := &Environment{Kind: "postman", Name: name}
		for _, v := range vals {
			if v.Key == "" || (v.Enabled != nil && !*v.Enabled) {
				continue
			}
			e.Vars = append(e.Vars, Var{Name: v.Key, Value: kvString(v.Value), Secret: v.Type == "secret"})
		}
		return e, nil
	}
	// Bruno JSON export: { name, variables: [{ name, value, enabled, secret }] }
	if raw, ok := doc["variables"]; ok {
		var vals []struct {
			Name    string `json:"name"`
			Value   any    `json:"value"`
			Enabled *bool  `json:"enabled"`
			Secret  bool   `json:"secret"`
		}
		if err := json.Unmarshal(raw, &vals); err != nil {
			return nil, fmt.Errorf("bruno environment: %w", err)
		}
		e := &Environment{Kind: "bruno", Name: name}
		for _, v := range vals {
			if v.Name == "" || (v.Enabled != nil && !*v.Enabled) {
				continue
			}
			e.Vars = append(e.Vars, Var{Name: v.Name, Value: kvString(v.Value), Secret: v.Secret})
		}
		return e, nil
	}
	// A flat object: { "baseUrl": "…", "token": "…" }
	var flat map[string]any
	if err := json.Unmarshal(data, &flat); err != nil {
		return nil, err
	}
	keys := make([]string, 0, len(flat))
	for k := range flat {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	e := &Environment{Kind: "json"}
	for _, k := range keys {
		switch flat[k].(type) {
		case map[string]any, []any:
			return nil, fmt.Errorf("didn't recognise this JSON as a Postman or Bruno environment")
		}
		e.Vars = append(e.Vars, Var{Name: k, Value: kvString(flat[k])})
	}
	return e, nil
}

// bruEnv reads Bruno's .bru environment:
//
//	vars {
//	  baseUrl: https://api.example.com
//	  ~disabled: x
//	}
//	vars:secret [
//	  token
//	]
func bruEnv(text string) (*Environment, error) {
	e := &Environment{Kind: "bruno"}
	sc := bufio.NewScanner(strings.NewReader(text))
	block := ""
	for sc.Scan() {
		line := strings.TrimSpace(sc.Text())
		switch {
		case line == "":
		case strings.HasPrefix(line, "vars:secret"):
			block = "secret"
			// vars:secret [ a, b ] on one line
			if i, j := strings.Index(line, "["), strings.Index(line, "]"); i >= 0 && j > i {
				for _, n := range strings.Split(line[i+1:j], ",") {
					addSecret(e, strings.TrimSpace(n))
				}
				block = ""
			}
		case strings.HasPrefix(line, "vars"):
			block = "vars"
		case line == "}" || line == "]":
			block = ""
		case block == "vars":
			if strings.HasPrefix(line, "~") {
				continue
			}
			k, v, ok := strings.Cut(line, ":")
			if !ok {
				continue
			}
			e.Vars = append(e.Vars, Var{Name: strings.TrimSpace(k), Value: strings.TrimSpace(v)})
		case block == "secret":
			for _, n := range strings.Split(line, ",") {
				addSecret(e, strings.TrimSpace(n))
			}
		}
	}
	return e, nil
}

func addSecret(e *Environment, name string) {
	if name == "" || strings.HasPrefix(name, "~") {
		return
	}
	for i := range e.Vars {
		if e.Vars[i].Name == name {
			e.Vars[i].Secret = true
			return
		}
	}
	// Bruno doesn't export secret values: the variable comes in empty.
	e.Vars = append(e.Vars, Var{Name: name, Secret: true})
}

func dotEnv(text string) (*Environment, error) {
	e := &Environment{Kind: "dotenv"}
	sc := bufio.NewScanner(strings.NewReader(text))
	n := 0
	for sc.Scan() {
		n++
		line := strings.TrimSpace(sc.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		line = strings.TrimPrefix(line, "export ")
		k, v, ok := strings.Cut(line, "=")
		if !ok || strings.TrimSpace(k) == "" {
			return nil, fmt.Errorf("line %d isn't NAME=value; import a Postman or Bruno environment, or a .env file", n)
		}
		v = strings.TrimSpace(v)
		if len(v) >= 2 && (v[0] == '"' && v[len(v)-1] == '"' || v[0] == '\'' && v[len(v)-1] == '\'') {
			v = v[1 : len(v)-1]
		}
		e.Vars = append(e.Vars, Var{Name: strings.TrimSpace(k), Value: v})
	}
	return e, nil
}
