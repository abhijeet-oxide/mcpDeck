package server

import (
	"context"
	"net/http"
	"regexp"
	"strings"

	"github.com/abhijeet-oxide/mcpdeck/internal/audit"
	"github.com/abhijeet-oxide/mcpdeck/internal/deck"
)

type adminAgentKey struct{}

// AdminAgentHeader names the agent behind `mcpdeck mcp`, the admin MCP
// server. It rides on the CLI token, so it has the CLI's reach, but the
// actions are recorded as that agent's and review rules apply to it (PAR-2).
const AdminAgentHeader = "X-Mcpdeck-Agent"

// you is who is acting: the person, or the agent behind the admin MCP server.
func you(r *http.Request) audit.Actor {
	if name, ok := r.Context().Value(adminAgentKey{}).(string); ok {
		return audit.Actor{Kind: "agent", ID: "admin-mcp", Name: name}
	}
	return audit.You
}

func withAdminAgent(r *http.Request, who string) *http.Request {
	name := strings.TrimSpace(r.Header.Get(AdminAgentHeader))
	if who != "cli" || name == "" {
		return r
	}
	if len(name) > 60 {
		name = name[:60]
	}
	name += " (admin MCP)"
	ctx := context.WithValue(r.Context(), adminAgentKey{}, name)
	ctx = deck.WithActor(ctx, audit.Actor{Kind: "agent", ID: "admin-mcp", Name: name})
	return r.WithContext(ctx)
}

// personOnly are the decisions an agent never makes for itself, even with
// the admin MCP server: approving write tools, widening any agent's access,
// tokens, settings and revealing data (ACC-2, PAR-2).
var personOnly = []struct {
	method string
	path   *regexp.Regexp
	why    string
}{
	{"POST", regexp.MustCompile(`^/api/versions/[^/]+/approve$`), "approving a tool version"},
	{"PATCH", regexp.MustCompile(`^/api/grants/`), "changing an agent's access"},
	{"POST", regexp.MustCompile(`^/api/agents/[^/]+/(rotate|revoke)$`), "rotating or revoking agent tokens"},
	{"PUT", regexp.MustCompile(`^/api/settings$`), "changing settings"},
	{"POST", regexp.MustCompile(`^/api/reveal-data$`), "opening the data folder"},
	{"POST", regexp.MustCompile(`^/api/apis/[^/]+/scripts-trust$`), "trusting scripts"},
}

func blockedForAgent(r *http.Request) string {
	if _, ok := r.Context().Value(adminAgentKey{}).(string); !ok {
		return ""
	}
	for _, p := range personOnly {
		if r.Method == p.method && p.path.MatchString(r.URL.Path) {
			return p.why + " is for a person; ask the user to do it in mcpDeck"
		}
	}
	return ""
}
