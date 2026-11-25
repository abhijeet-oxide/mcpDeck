package importer

import (
	"testing"

	"github.com/google/go-cmp/cmp"
)

type ev struct {
	Name, Value string
	Secret      bool
}

// REQ: ENV-2
func TestParseEnvironment(t *testing.T) {
	tests := []struct {
		name, file, in string
		kind, env      string
		want           []ev
	}{
		{"postman", "Staging.postman_environment.json",
			`{"name":"Staging","values":[{"key":"baseUrl","value":"https://s.example","enabled":true},{"key":"apiKey","value":"k1","type":"secret"},{"key":"old","value":"x","enabled":false}]}`,
			"postman", "Staging", []ev{{"baseUrl", "https://s.example", false}, {"apiKey", "k1", true}}},
		{"bruno bru", "envs/Local.bru", "vars {\n  baseUrl: http://localhost:8080\n  ~skipped: 1\n  user: admin\n}\nvars:secret [\n  password,\n  token\n]\n",
			"bruno", "Local", []ev{{"baseUrl", "http://localhost:8080", false}, {"user", "admin", false}, {"password", "", true}, {"token", "", true}}},
		{"bruno json", "x.json", `{"name":"Prod","variables":[{"name":"host","value":"h","enabled":true,"secret":false},{"name":"pw","value":"p","secret":true}]}`,
			"bruno", "Prod", []ev{{"host", "h", false}, {"pw", "p", true}}},
		{"dotenv", ".env", "# comment\nexport BASE_URL=\"https://x\"\nAPI_TOKEN=abc\n",
			"dotenv", "Imported", []ev{{"BASE_URL", "https://x", false}, {"API_TOKEN", "abc", true}}},
		{"flat json", "dev.json", `{"baseUrl":"http://d","port":8080}`,
			"json", "dev", []ev{{"baseUrl", "http://d", false}, {"port", "8080", false}}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			e, err := ParseEnvironment([]byte(tt.in), tt.file)
			if err != nil {
				t.Fatal(err)
			}
			var got []ev
			for _, v := range e.Vars {
				got = append(got, ev{v.Name, v.Value, v.Secret})
				if v.Secret && v.Value != "" && v.Preview == v.Value {
					t.Errorf("%s: secret preview shows the value", v.Name)
				}
			}
			if e.Kind != tt.kind || e.Name != tt.env {
				t.Errorf("kind/name = %s/%s, want %s/%s", e.Kind, e.Name, tt.kind, tt.env)
			}
			if d := cmp.Diff(tt.want, got); d != "" {
				t.Error(d)
			}
		})
	}
	for _, bad := range []string{"", "not an env", `{"item":[{"a":1}]}`} {
		if _, err := ParseEnvironment([]byte(bad), "f"); err == nil {
			t.Errorf("%q: want error", bad)
		}
	}
}
