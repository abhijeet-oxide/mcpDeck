-- Config entities are JSON documents with the columns we query on.
CREATE TABLE setting (key TEXT PRIMARY KEY, value TEXT NOT NULL);
CREATE TABLE api (id TEXT PRIMARY KEY, name TEXT NOT NULL UNIQUE, data TEXT NOT NULL);
CREATE TABLE request (id TEXT PRIMARY KEY, api_id TEXT NOT NULL, data TEXT NOT NULL);
CREATE INDEX request_api ON request(api_id);
CREATE TABLE environment (id TEXT PRIMARY KEY, api_id TEXT NOT NULL, data TEXT NOT NULL);
CREATE TABLE variable (id TEXT PRIMARY KEY, api_id TEXT NOT NULL, env_id TEXT NOT NULL, data TEXT NOT NULL);
CREATE INDEX variable_api ON variable(api_id);
CREATE TABLE toolkit (id TEXT PRIMARY KEY, name TEXT NOT NULL UNIQUE, data TEXT NOT NULL);
CREATE TABLE tool (id TEXT PRIMARY KEY, name TEXT NOT NULL UNIQUE, data TEXT NOT NULL);
CREATE TABLE toolkit_tool (toolkit_id TEXT NOT NULL, tool_id TEXT NOT NULL, PRIMARY KEY (toolkit_id, tool_id));
CREATE TABLE tool_version (id TEXT PRIMARY KEY, tool_id TEXT NOT NULL, n INTEGER NOT NULL, status TEXT NOT NULL, data TEXT NOT NULL, UNIQUE (tool_id, n));
CREATE TABLE agent (id TEXT PRIMARY KEY, token_hash TEXT NOT NULL UNIQUE, data TEXT NOT NULL);
CREATE TABLE grant_ (id TEXT PRIMARY KEY, agent_id TEXT NOT NULL, toolkit_id TEXT NOT NULL, data TEXT NOT NULL, UNIQUE (agent_id, toolkit_id));
CREATE TABLE session (id TEXT PRIMARY KEY, agent_id TEXT NOT NULL, data TEXT NOT NULL);
CREATE TABLE tool_request (id TEXT PRIMARY KEY, toolkit_id TEXT NOT NULL, state TEXT NOT NULL, data TEXT NOT NULL);
CREATE TABLE field_report (id TEXT PRIMARY KEY, tool_id TEXT NOT NULL, data TEXT NOT NULL);

CREATE TABLE call (
  id TEXT PRIMARY KEY,
  session_id TEXT NOT NULL DEFAULT '',
  parent_id TEXT NOT NULL DEFAULT '',
  actor_kind TEXT NOT NULL,
  actor_id TEXT NOT NULL DEFAULT '',
  toolkit_id TEXT NOT NULL DEFAULT '',
  kind TEXT NOT NULL,
  name TEXT NOT NULL,
  status TEXT NOT NULL,
  started_at TEXT NOT NULL,
  duration_ms INTEGER NOT NULL DEFAULT 0,
  tokens INTEGER NOT NULL DEFAULT 0,
  output_bytes INTEGER NOT NULL DEFAULT 0,
  data TEXT NOT NULL
);
CREATE INDEX call_started ON call(started_at);
CREATE INDEX call_session ON call(session_id);
CREATE INDEX call_parent ON call(parent_id);

-- Append-only and hash-chained (AUD-1, AUD-2). Triggers refuse edits.
CREATE TABLE audit_event (
  seq INTEGER PRIMARY KEY AUTOINCREMENT,
  id TEXT NOT NULL UNIQUE,
  at TEXT NOT NULL,
  actor_kind TEXT NOT NULL,
  actor_id TEXT NOT NULL,
  actor_name TEXT NOT NULL,
  action TEXT NOT NULL,
  target TEXT NOT NULL,
  detail TEXT NOT NULL,
  prev_hash TEXT NOT NULL,
  hash TEXT NOT NULL
);
CREATE TRIGGER audit_no_update BEFORE UPDATE ON audit_event BEGIN SELECT RAISE(ABORT, 'audit log is append-only'); END;
CREATE TRIGGER audit_no_delete BEFORE DELETE ON audit_event BEGIN SELECT RAISE(ABORT, 'audit log is append-only'); END;
