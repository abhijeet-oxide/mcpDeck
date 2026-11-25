package store

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"
)

// Typed helpers over the document tables.

func (s *Store) PutAPI(ctx context.Context, a API) error {
	return s.Put(ctx, "api", a.ID, a, map[string]any{"name": a.Name})
}
func (s *Store) API(ctx context.Context, id string) (API, error) { return Get[API](ctx, s, "api", id) }
func (s *Store) APIByName(ctx context.Context, name string) (API, error) {
	return Find[API](ctx, s, "api", "name = ?", name)
}
func (s *Store) APIs(ctx context.Context) ([]API, error) {
	return List[API](ctx, s, "api", "ORDER BY name")
}

func (s *Store) PutRequest(ctx context.Context, r Request) error {
	return s.Put(ctx, "request", r.ID, r, map[string]any{"api_id": r.APIID})
}
func (s *Store) Request(ctx context.Context, id string) (Request, error) {
	return Get[Request](ctx, s, "request", id)
}
func (s *Store) Requests(ctx context.Context, apiID string) ([]Request, error) {
	if apiID == "" {
		return List[Request](ctx, s, "request", "ORDER BY id")
	}
	return List[Request](ctx, s, "request", "api_id = ? ORDER BY id", apiID)
}

func (s *Store) PutEnvironment(ctx context.Context, e Environment) error {
	return s.Put(ctx, "environment", e.ID, e, map[string]any{"api_id": e.APIID})
}
func (s *Store) Environment(ctx context.Context, id string) (Environment, error) {
	return Get[Environment](ctx, s, "environment", id)
}
func (s *Store) Environments(ctx context.Context, apiID string) ([]Environment, error) {
	return List[Environment](ctx, s, "environment", "api_id = ? ORDER BY id", apiID)
}

func (s *Store) PutVariable(ctx context.Context, v Variable) error {
	return s.Put(ctx, "variable", v.ID, v, map[string]any{"api_id": v.APIID, "env_id": v.EnvID})
}
func (s *Store) Variable(ctx context.Context, id string) (Variable, error) {
	return Get[Variable](ctx, s, "variable", id)
}
func (s *Store) Variables(ctx context.Context, apiID string) ([]Variable, error) {
	return List[Variable](ctx, s, "variable", "api_id = ? ORDER BY id", apiID)
}

func (s *Store) PutToolkit(ctx context.Context, t Toolkit) error {
	return s.Put(ctx, "toolkit", t.ID, t, map[string]any{"name": t.Name})
}
func (s *Store) Toolkit(ctx context.Context, id string) (Toolkit, error) {
	return Get[Toolkit](ctx, s, "toolkit", id)
}
func (s *Store) ToolkitByName(ctx context.Context, name string) (Toolkit, error) {
	return Find[Toolkit](ctx, s, "toolkit", "name = ?", name)
}
func (s *Store) Toolkits(ctx context.Context) ([]Toolkit, error) {
	return List[Toolkit](ctx, s, "toolkit", "ORDER BY name")
}

func (s *Store) PutTool(ctx context.Context, t Tool) error {
	return s.Put(ctx, "tool", t.ID, t, map[string]any{"name": t.Name})
}
func (s *Store) Tool(ctx context.Context, id string) (Tool, error) {
	return Get[Tool](ctx, s, "tool", id)
}
func (s *Store) ToolByName(ctx context.Context, name string) (Tool, error) {
	return Find[Tool](ctx, s, "tool", "name = ?", name)
}
func (s *Store) Tools(ctx context.Context) ([]Tool, error) {
	return List[Tool](ctx, s, "tool", "ORDER BY name")
}

// ToolkitTools lists the tools in a toolkit.
func (s *Store) ToolkitTools(ctx context.Context, toolkitID string) ([]Tool, error) {
	return List[Tool](ctx, s, "tool", "id IN (SELECT tool_id FROM toolkit_tool WHERE toolkit_id = ?) ORDER BY name", toolkitID)
}

// ToolToolkits lists the toolkit IDs that contain a tool.
func (s *Store) ToolToolkits(ctx context.Context, toolID string) ([]string, error) {
	rows, err := s.DB.QueryContext(ctx, `SELECT toolkit_id FROM toolkit_tool WHERE tool_id = ?`, toolID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []string{}
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		out = append(out, id)
	}
	return out, rows.Err()
}

func (s *Store) AddToolToToolkit(ctx context.Context, toolkitID, toolID string) error {
	_, err := s.DB.ExecContext(ctx, `INSERT OR IGNORE INTO toolkit_tool(toolkit_id, tool_id) VALUES (?, ?)`, toolkitID, toolID)
	return err
}
func (s *Store) RemoveToolFromToolkit(ctx context.Context, toolkitID, toolID string) error {
	_, err := s.DB.ExecContext(ctx, `DELETE FROM toolkit_tool WHERE toolkit_id = ? AND tool_id = ?`, toolkitID, toolID)
	return err
}

// AddVersion inserts a new version. Versions are never rewritten except for
// their status (VER-1).
func (s *Store) AddVersion(ctx context.Context, v ToolVersion) error {
	data, _ := json.Marshal(v)
	_, err := s.DB.ExecContext(ctx, `INSERT INTO tool_version(id, tool_id, n, status, data) VALUES (?, ?, ?, ?, ?)`,
		v.ID, v.ToolID, v.N, v.Status, string(data))
	if err != nil {
		return fmt.Errorf("save version: %w", err)
	}
	return nil
}

// SetVersionStatus changes only the status and decision time.
func (s *Store) SetVersionStatus(ctx context.Context, id, status string, at time.Time) error {
	v, err := s.Version(ctx, id)
	if err != nil {
		return err
	}
	v.Status = status
	v.DecidedAt = &at
	data, _ := json.Marshal(v)
	_, err = s.DB.ExecContext(ctx, `UPDATE tool_version SET status = ?, data = ? WHERE id = ?`, status, string(data), id)
	return err
}
func (s *Store) Version(ctx context.Context, id string) (ToolVersion, error) {
	return Get[ToolVersion](ctx, s, "tool_version", id)
}
func (s *Store) Versions(ctx context.Context, toolID string) ([]ToolVersion, error) {
	return List[ToolVersion](ctx, s, "tool_version", "tool_id = ? ORDER BY n DESC", toolID)
}
func (s *Store) PendingVersions(ctx context.Context) ([]ToolVersion, error) {
	return List[ToolVersion](ctx, s, "tool_version", "status = 'pending' ORDER BY id DESC")
}

// LiveVersion is the newest live version of a tool.
func (s *Store) LiveVersion(ctx context.Context, toolID string) (ToolVersion, error) {
	return Find[ToolVersion](ctx, s, "tool_version", "tool_id = ? AND status = 'live' ORDER BY n DESC", toolID)
}

// NextVersion returns the next version number for a tool.
func (s *Store) NextVersion(ctx context.Context, toolID string) (int, error) {
	var n sql.NullInt64
	err := s.DB.QueryRowContext(ctx, `SELECT max(n) FROM tool_version WHERE tool_id = ?`, toolID).Scan(&n)
	return int(n.Int64) + 1, err
}

func (s *Store) PutAgent(ctx context.Context, a Agent) error {
	return s.Put(ctx, "agent", a.ID, agentDoc{Agent: a, Hash: a.TokenHash}, map[string]any{"token_hash": a.TokenHash})
}
func (s *Store) Agent(ctx context.Context, id string) (Agent, error) {
	d, err := Get[agentDoc](ctx, s, "agent", id)
	d.Agent.TokenHash = d.Hash
	return d.Agent, err
}
func (s *Store) AgentByTokenHash(ctx context.Context, hash string) (Agent, error) {
	d, err := Find[agentDoc](ctx, s, "agent", "token_hash = ?", hash)
	d.Agent.TokenHash = d.Hash
	return d.Agent, err
}
func (s *Store) Agents(ctx context.Context) ([]Agent, error) {
	docs, err := List[agentDoc](ctx, s, "agent", "ORDER BY id")
	out := make([]Agent, 0, len(docs))
	for _, d := range docs {
		d.Agent.TokenHash = d.Hash
		out = append(out, d.Agent)
	}
	return out, err
}

func (s *Store) PutGrant(ctx context.Context, g Grant) error {
	return s.Put(ctx, "grant_", g.ID, g, map[string]any{"agent_id": g.AgentID, "toolkit_id": g.ToolkitID})
}
func (s *Store) Grant(ctx context.Context, id string) (Grant, error) {
	return Get[Grant](ctx, s, "grant_", id)
}
func (s *Store) GrantFor(ctx context.Context, agentID, toolkitID string) (Grant, error) {
	return Find[Grant](ctx, s, "grant_", "agent_id = ? AND toolkit_id = ?", agentID, toolkitID)
}
func (s *Store) Grants(ctx context.Context, agentID, toolkitID string) ([]Grant, error) {
	var conds []string
	var args []any
	if agentID != "" {
		conds = append(conds, "agent_id = ?")
		args = append(args, agentID)
	}
	if toolkitID != "" {
		conds = append(conds, "toolkit_id = ?")
		args = append(args, toolkitID)
	}
	where := strings.Join(conds, " AND ")
	if where == "" {
		where = "1=1"
	}
	return List[Grant](ctx, s, "grant_", where+" ORDER BY id", args...)
}

func (s *Store) PutSession(ctx context.Context, x Session) error {
	return s.Put(ctx, "session", x.ID, x, map[string]any{"agent_id": x.AgentID})
}
func (s *Store) Session(ctx context.Context, id string) (Session, error) {
	return Get[Session](ctx, s, "session", id)
}
func (s *Store) Sessions(ctx context.Context, agentID string, limit int) ([]Session, error) {
	if agentID == "" {
		return List[Session](ctx, s, "session", "ORDER BY id DESC LIMIT ?", limit)
	}
	return List[Session](ctx, s, "session", "agent_id = ? ORDER BY id DESC LIMIT ?", agentID, limit)
}

func (s *Store) PutToolRequest(ctx context.Context, r ToolRequest) error {
	return s.Put(ctx, "tool_request", r.ID, r, map[string]any{"toolkit_id": r.ToolkitID, "state": r.State})
}
func (s *Store) ToolRequest(ctx context.Context, id string) (ToolRequest, error) {
	return Get[ToolRequest](ctx, s, "tool_request", id)
}
func (s *Store) ToolRequests(ctx context.Context, toolkitID, state string) ([]ToolRequest, error) {
	where, args := "1=1", []any{}
	if toolkitID != "" {
		where += " AND toolkit_id = ?"
		args = append(args, toolkitID)
	}
	if state != "" {
		where += " AND state = ?"
		args = append(args, state)
	}
	return List[ToolRequest](ctx, s, "tool_request", where+" ORDER BY id DESC", args...)
}

func (s *Store) PutFieldReport(ctx context.Context, r FieldReport) error {
	return s.Put(ctx, "field_report", r.ID, r, map[string]any{"tool_id": r.ToolID})
}
func (s *Store) FieldReports(ctx context.Context, toolID string) ([]FieldReport, error) {
	return List[FieldReport](ctx, s, "field_report", "tool_id = ? ORDER BY id DESC", toolID)
}

// InsertCall records a call (SES-2).
func (s *Store) InsertCall(ctx context.Context, c Call) error {
	data, _ := json.Marshal(c)
	_, err := s.DB.ExecContext(ctx, `INSERT INTO call(id, session_id, parent_id, actor_kind, actor_id, toolkit_id, kind, name, status, started_at, duration_ms, tokens, output_bytes, data)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		c.ID, c.SessionID, c.ParentID, c.ActorKind, c.ActorID, c.ToolkitID, c.Kind, c.Name, c.Status,
		c.StartedAt.UTC().Format(time.RFC3339Nano), c.DurationMS, c.Tokens, c.OutputBytes, string(data))
	if err != nil {
		return fmt.Errorf("record call: %w", err)
	}
	return nil
}

// CallFilter narrows ListCalls.
type CallFilter struct {
	SessionID string
	ParentID  string
	ActorID   string
	ToolkitID string
	Kind      string
	Status    string
	Name      string
	Since     time.Time
	Limit     int
	TopLevel  bool
}

// Calls lists calls, newest first.
func (s *Store) Calls(ctx context.Context, f CallFilter) ([]Call, error) {
	conds := []string{"1=1"}
	var args []any
	add := func(cond string, v any) { conds = append(conds, cond); args = append(args, v) }
	if f.SessionID != "" {
		add("session_id = ?", f.SessionID)
	}
	if f.ParentID != "" {
		add("parent_id = ?", f.ParentID)
	}
	if f.ActorID != "" {
		add("actor_id = ?", f.ActorID)
	}
	if f.ToolkitID != "" {
		add("toolkit_id = ?", f.ToolkitID)
	}
	if f.Kind != "" {
		add("kind = ?", f.Kind)
	}
	if f.Status != "" {
		add("status = ?", f.Status)
	}
	if f.Name != "" {
		add("name LIKE ?", "%"+f.Name+"%")
	}
	if !f.Since.IsZero() {
		add("started_at >= ?", f.Since.UTC().Format(time.RFC3339Nano))
	}
	if f.TopLevel {
		conds = append(conds, "parent_id = ''")
	}
	if f.Limit <= 0 {
		f.Limit = 200
	}
	q := `SELECT data FROM call WHERE ` + strings.Join(conds, " AND ") + ` ORDER BY started_at DESC, id DESC LIMIT ?`
	args = append(args, f.Limit)
	rows, err := s.DB.QueryContext(ctx, q, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []Call{}
	for rows.Next() {
		var data string
		if err := rows.Scan(&data); err != nil {
			return nil, err
		}
		var c Call
		if err := json.Unmarshal([]byte(data), &c); err != nil {
			return nil, err
		}
		out = append(out, c)
	}
	return out, rows.Err()
}

// Call returns one call.
func (s *Store) Call(ctx context.Context, id string) (Call, error) {
	return Get[Call](ctx, s, "call", id)
}

// PruneCalls drops call bodies older than bodies and rows older than meta (SES-5).
func (s *Store) PruneCalls(ctx context.Context, bodiesBefore, rowsBefore time.Time) error {
	if _, err := s.DB.ExecContext(ctx, `DELETE FROM call WHERE started_at < ?`, rowsBefore.UTC().Format(time.RFC3339Nano)); err != nil {
		return err
	}
	_, err := s.DB.ExecContext(ctx, `UPDATE call SET data = json_remove(data, '$.input', '$.output') WHERE started_at < ?`,
		bodiesBefore.UTC().Format(time.RFC3339Nano))
	return err
}

// LastAudit returns the newest audit event, or ok=false when empty.
func (s *Store) LastAudit(ctx context.Context, tx *sql.Tx) (AuditEvent, bool, error) {
	row := tx.QueryRowContext(ctx, `SELECT seq, id, at, actor_kind, actor_id, actor_name, action, target, detail, prev_hash, hash FROM audit_event ORDER BY seq DESC LIMIT 1`)
	e, err := scanAudit(row)
	if errors.Is(err, sql.ErrNoRows) {
		return AuditEvent{}, false, nil
	}
	return e, err == nil, err
}

// InsertAudit appends an event inside tx.
func (s *Store) InsertAudit(ctx context.Context, tx *sql.Tx, e AuditEvent) error {
	_, err := tx.ExecContext(ctx, `INSERT INTO audit_event(id, at, actor_kind, actor_id, actor_name, action, target, detail, prev_hash, hash) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		e.ID, e.At.UTC().Format(time.RFC3339Nano), e.ActorKind, e.ActorID, e.ActorName, e.Action, e.Target, e.Detail, e.PrevHash, e.Hash)
	return err
}

// AuditEvents lists events, oldest first when asc.
func (s *Store) AuditEvents(ctx context.Context, asc bool, limit int) ([]AuditEvent, error) {
	order := "DESC"
	if asc {
		order = "ASC"
	}
	if limit <= 0 {
		limit = -1
	}
	rows, err := s.DB.QueryContext(ctx, `SELECT seq, id, at, actor_kind, actor_id, actor_name, action, target, detail, prev_hash, hash FROM audit_event ORDER BY seq `+order+` LIMIT ?`, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []AuditEvent{}
	for rows.Next() {
		e, err := scanAudit(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, e)
	}
	return out, rows.Err()
}

type scanner interface{ Scan(dest ...any) error }

func scanAudit(r scanner) (AuditEvent, error) {
	var e AuditEvent
	var at string
	err := r.Scan(&e.Seq, &e.ID, &at, &e.ActorKind, &e.ActorID, &e.ActorName, &e.Action, &e.Target, &e.Detail, &e.PrevHash, &e.Hash)
	if err != nil {
		return e, err
	}
	e.At, _ = time.Parse(time.RFC3339Nano, at)
	return e, nil
}
