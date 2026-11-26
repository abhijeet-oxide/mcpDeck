package deck

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base32"
	"encoding/hex"
	"fmt"
	"strings"
	"time"

	"github.com/abhijeet-oxide/mcpdeck/internal/audit"
	"github.com/abhijeet-oxide/mcpdeck/internal/store"
)

// Clients mcpDeck can connect (AGT-3).
var Clients = map[string]string{
	"claude-code":    "Claude Code",
	"vscode":         "VS Code",
	"cursor":         "Cursor",
	"claude-desktop": "Claude Desktop",
	"other":          "Other MCP client",
}

// HashToken is how tokens are stored (AGT-1).
func HashToken(tok string) string {
	h := sha256.Sum256([]byte(tok))
	return hex.EncodeToString(h[:])
}

func newToken() string {
	b := make([]byte, 20)
	_, _ = rand.Read(b)
	return "mcpd_" + strings.ToLower(base32.StdEncoding.WithPadding(base32.NoPadding).EncodeToString(b))
}

// AgentSummary is a row on the Agents page.
type AgentSummary struct {
	store.Agent
	ClientLabel string     `json:"clientLabel"`
	Grants      []GrantRow `json:"grants"`
	CallsToday  int        `json:"callsToday"`
	ErrorsToday int        `json:"errorsToday"`
	Connected   bool       `json:"connected"`
}

func (d *Deck) connected(a store.Agent) bool {
	return a.RevokedAt == nil && a.LastSeenAt != nil && d.Clock.Now().Sub(*a.LastSeenAt) < 10*time.Minute
}

func (d *Deck) agentSummary(ctx context.Context, a store.Agent) AgentSummary {
	s := AgentSummary{Agent: a, ClientLabel: Clients[a.Client], Grants: []GrantRow{}, Connected: d.connected(a)}
	grants, _ := d.Store.Grants(ctx, a.ID, "")
	for _, g := range grants {
		tk, _ := d.Store.Toolkit(ctx, g.ToolkitID)
		s.Grants = append(s.Grants, GrantRow{Grant: g, Agent: a, ToolkitName: tk.Name, Connected: s.Connected})
	}
	y, m, dd := d.Clock.Now().Date()
	calls, _ := d.Store.Calls(ctx, store.CallFilter{ActorID: a.ID, Since: time.Date(y, m, dd, 0, 0, 0, 0, time.UTC), TopLevel: true, Limit: 10000})
	for _, c := range calls {
		if c.Kind == "list" {
			continue
		}
		s.CallsToday++
		if c.Status != "ok" {
			s.ErrorsToday++
		}
	}
	return s
}

func (d *Deck) ListAgents(ctx context.Context) ([]AgentSummary, error) {
	agents, err := d.Store.Agents(ctx)
	if err != nil {
		return nil, err
	}
	out := make([]AgentSummary, 0, len(agents))
	for _, a := range agents {
		out = append(out, d.agentSummary(ctx, a))
	}
	return out, nil
}

// AgentDetail is one agent's page.
type AgentDetail struct {
	AgentSummary
	Sessions []store.Session `json:"sessions"`
}

func (d *Deck) GetAgent(ctx context.Context, id string) (*AgentDetail, error) {
	a, err := d.Store.Agent(ctx, id)
	if err != nil {
		return nil, notFound("agent")
	}
	out := &AgentDetail{AgentSummary: d.agentSummary(ctx, a)}
	out.Sessions, _ = d.Store.Sessions(ctx, id, 50)
	return out, nil
}

// ConnectInput adds a toolkit to a client (AGT-3).
type ConnectInput struct {
	Client    string `json:"client"`
	ToolkitID string `json:"toolkitId"`
	AgentID   string `json:"agentId"` // reuse; otherwise the client's agent, or a new one
	Name      string `json:"name"`
	Access    string `json:"access"`
	CanBuild  bool   `json:"canBuild"`
}

// ConnectResult carries the token, shown once.
type ConnectResult struct {
	Agent   store.Agent   `json:"agent"`
	Token   string        `json:"token"`
	Grant   store.Grant   `json:"grant"`
	Toolkit store.Toolkit `json:"toolkit"`
	Grants  []GrantRow    `json:"grants"`
}

func (d *Deck) Connect(ctx context.Context, in ConnectInput) (*ConnectResult, error) {
	if _, ok := Clients[in.Client]; !ok {
		return nil, badRequest("client must be one of vscode, claude-code, cursor, claude-desktop, other")
	}
	tk, err := d.toolkitByRef(ctx, in.ToolkitID)
	if err != nil {
		return nil, err
	}
	if in.Access == "" {
		in.Access = "read"
	}
	if in.Access != "read" && in.Access != "read_write" {
		return nil, badRequest("access must be read or read_write")
	}
	var a store.Agent
	switch {
	case in.AgentID != "":
		a, err = d.Store.Agent(ctx, in.AgentID)
		if err != nil {
			return nil, notFound("agent")
		}
	default:
		agents, _ := d.Store.Agents(ctx)
		for _, x := range agents {
			if x.Client == in.Client && x.RevokedAt == nil && (in.Name == "" || x.Name == in.Name) {
				a = x
				break
			}
		}
	}
	now := d.Clock.Now()
	tok := newToken()
	if a.ID == "" {
		name := strings.TrimSpace(in.Name)
		if name == "" {
			name = Clients[in.Client] + " on this computer"
		}
		a = store.Agent{ID: d.IDs.New(), Name: name, Client: in.Client, CreatedAt: now}
		d.audit(ctx, audit.You, "agent.create", a.Name, Clients[in.Client])
	} else if a.RevokedAt != nil {
		return nil, conflict("%s is revoked; connect a new agent instead", a.Name)
	} else {
		d.audit(ctx, audit.You, "agent.rotate", a.Name, "new token for connect")
	}
	a.TokenHash, a.TokenPrefix = HashToken(tok), tok[:10]
	if err := d.Store.PutAgent(ctx, a); err != nil {
		return nil, err
	}
	g, err := d.Store.GrantFor(ctx, a.ID, tk.ID)
	if err != nil {
		g = store.Grant{ID: d.IDs.New(), AgentID: a.ID, ToolkitID: tk.ID, CreatedAt: now}
	}
	g.Access, g.CanBuild = in.Access, in.CanBuild
	if err := d.Store.PutGrant(ctx, g); err != nil {
		return nil, err
	}
	d.audit(ctx, audit.You, "grant.set", a.Name+" → "+tk.Name, fmt.Sprintf("%s, build %v", g.Access, g.CanBuild))
	d.agentRevoked(a.ID) // old token's sessions end
	res := &ConnectResult{Agent: a, Token: tok, Grant: g, Toolkit: tk}
	res.Grants = d.agentSummary(ctx, a).Grants
	return res, nil
}

// Rotate issues a new token (AGT-4).
func (d *Deck) Rotate(ctx context.Context, id string) (*ConnectResult, error) {
	a, err := d.Store.Agent(ctx, id)
	if err != nil {
		return nil, notFound("agent")
	}
	if a.RevokedAt != nil {
		return nil, conflict("%s is revoked", a.Name)
	}
	tok := newToken()
	a.TokenHash, a.TokenPrefix = HashToken(tok), tok[:10]
	_ = d.Store.PutAgent(ctx, a)
	d.audit(ctx, audit.You, "agent.rotate", a.Name, "")
	d.agentRevoked(a.ID)
	res := &ConnectResult{Agent: a, Token: tok}
	res.Grants = d.agentSummary(ctx, a).Grants
	return res, nil
}

// Revoke refuses the agent's token at once and ends its sessions (AGT-4).
func (d *Deck) Revoke(ctx context.Context, id string) error {
	a, err := d.Store.Agent(ctx, id)
	if err != nil {
		return notFound("agent")
	}
	now := d.Clock.Now()
	a.RevokedAt = &now
	_ = d.Store.PutAgent(ctx, a)
	d.audit(ctx, audit.You, "agent.revoke", a.Name, "")
	d.agentRevoked(id)
	return nil
}

// GrantInput changes a grant in place.
type GrantInput struct {
	Access   *string `json:"access"`
	CanBuild *bool   `json:"canBuild"`
}

func (d *Deck) UpdateGrant(ctx context.Context, id string, in GrantInput) (*store.Grant, error) {
	g, err := d.Store.Grant(ctx, id)
	if err != nil {
		return nil, notFound("grant")
	}
	if in.Access != nil {
		if *in.Access != "read" && *in.Access != "read_write" {
			return nil, badRequest("access must be read or read_write")
		}
		g.Access = *in.Access
	}
	if in.CanBuild != nil {
		g.CanBuild = *in.CanBuild
	}
	_ = d.Store.PutGrant(ctx, g)
	a, _ := d.Store.Agent(ctx, g.AgentID)
	tk, _ := d.Store.Toolkit(ctx, g.ToolkitID)
	d.audit(ctx, audit.You, "grant.set", a.Name+" → "+tk.Name, fmt.Sprintf("%s, build %v", g.Access, g.CanBuild))
	d.toolsChanged([]string{g.ToolkitID})
	return &g, nil
}

func (d *Deck) DeleteGrant(ctx context.Context, id string) error {
	g, err := d.Store.Grant(ctx, id)
	if err != nil {
		return notFound("grant")
	}
	_ = d.Store.Delete(ctx, "grant_", id)
	a, _ := d.Store.Agent(ctx, g.AgentID)
	tk, _ := d.Store.Toolkit(ctx, g.ToolkitID)
	d.audit(ctx, audit.You, "grant.remove", a.Name+" → "+tk.Name, "")
	d.agentRevoked(g.AgentID)
	return nil
}

// Authenticate resolves an agent token and its grant for a toolkit.
func (d *Deck) Authenticate(ctx context.Context, token, toolkit string) (store.Agent, store.Grant, store.Toolkit, error) {
	var a store.Agent
	var g store.Grant
	var tk store.Toolkit
	if token == "" {
		return a, g, tk, errf(401, "unauthorized", "missing token: add the header Authorization: Bearer <token> from mcpDeck's Connect dialog")
	}
	a, err := d.Store.AgentByTokenHash(ctx, HashToken(token))
	if err != nil {
		return a, g, tk, errf(401, "unauthorized", "unknown token: reconnect this client from mcpDeck (Agents › Connect)")
	}
	if a.RevokedAt != nil {
		return a, g, tk, errf(401, "revoked", "this agent was revoked in mcpDeck; ask the user to connect it again")
	}
	tk, err = d.Store.ToolkitByName(ctx, toolkit)
	if err != nil {
		return a, g, tk, errf(404, "not_found", "no toolkit named %s", toolkit)
	}
	g, err = d.Store.GrantFor(ctx, a.ID, tk.ID)
	if err != nil {
		return a, g, tk, errf(403, "no_grant", "%s has no access to the %s toolkit; ask the user to add it in mcpDeck", a.Name, toolkit)
	}
	return a, g, tk, nil
}

// SessionSeen records an MCP session and the client's name (SES-1).
func (d *Deck) SessionSeen(ctx context.Context, a store.Agent, tk store.Toolkit, mode, mcpSession, client, version string) {
	now := d.Clock.Now()
	if mcpSession != "" {
		s, err := d.Store.Session(ctx, mcpSession)
		if err != nil {
			s = store.Session{ID: mcpSession, AgentID: a.ID, ToolkitID: tk.ID, Mode: mode, StartedAt: now}
		}
		if client != "" {
			s.ClientName, s.ClientVersion = client, version
		}
		s.LastSeenAt = now
		_ = d.Store.PutSession(ctx, s)
	}
	a.LastSeenAt = &now
	if client != "" {
		a.ClientName = client
	}
	_ = d.Store.PutAgent(ctx, a)
}
