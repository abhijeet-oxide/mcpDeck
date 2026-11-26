package store

import (
	"encoding/json"
	"time"
)

// KV is a header or query parameter.
type KV struct {
	Key      string `json:"key"`
	Value    string `json:"value"`
	Disabled bool   `json:"disabled,omitempty"`
}

// Auth is set on an API or a request (AUTH-1). Values may contain {{vars}}.
type Auth struct {
	Type           string `json:"type"` // inherit | none | bearer | apikey | basic | login
	Token          string `json:"token,omitempty"`
	Key            string `json:"key,omitempty"`
	Value          string `json:"value,omitempty"`
	In             string `json:"in,omitempty"` // header | query
	Username       string `json:"username,omitempty"`
	Password       string `json:"password,omitempty"`
	LoginRequestID string `json:"loginRequestId,omitempty"`
	Capture        string `json:"capture,omitempty"` // cookie (default), a response header name, or json:<field> sent as a bearer token

	// Login chains (AUTH-2). Steps run in order before the API's calls;
	// each step's captures become {{login.<name>}} for later steps, for
	// Apply headers and for any request template. LoginRequestID with
	// Capture is the one-step shorthand.
	Steps            []LoginStep `json:"steps,omitempty"`
	RefreshRequestID string      `json:"refreshRequestId,omitempty"` // run instead of the steps once the session expires, while {{login.refresh_token}} works
	ExpiresFrom      string      `json:"expiresFrom,omitempty"`      // json:<path> in the last response, or a number of seconds; default 30 minutes
	// Apply adds headers to every call once signed in (login, oauth2, hmac),
	// e.g. Authorization: Bearer {{login.access_token}}.
	Apply []KV `json:"apply,omitempty"`

	// OAuth 2.0 (AUTH-3): client_credentials, password or refresh_token
	// grants against TokenURL. Values may be {{vars}}.
	Grant        string `json:"grant,omitempty"`
	TokenURL     string `json:"tokenUrl,omitempty"`
	ClientID     string `json:"clientId,omitempty"`
	ClientSecret string `json:"clientSecret,omitempty"`
	ClientAuth   string `json:"clientAuth,omitempty"` // basic (default) | body
	Scope        string `json:"scope,omitempty"`
	Audience     string `json:"audience,omitempty"`

	// HMAC request signing (AUTH-4). StringToSign may use {{method}},
	// {{path}} (path and query as sent), {{pathOnly}}, {{query}}, {{host}},
	// {{timestamp}}, {{timestampMs}}, {{isoTime}}, {{nonce}}, {{body}},
	// {{bodySha256}}, {{bodyMd5}} and any variable. Apply headers can use
	// {{sign.signature}}, {{sign.timestamp}} and {{sign.nonce}}.
	Algorithm    string `json:"algorithm,omitempty"` // sha256 (default) | sha1 | sha512
	Secret       string `json:"secret,omitempty"`
	StringToSign string `json:"stringToSign,omitempty"`
	Encoding     string `json:"encoding,omitempty"` // hex (default) | base64
}

// LoginStep is one request in a login chain and what to keep from its response.
type LoginStep struct {
	RequestID string    `json:"requestId"`
	Captures  []Capture `json:"captures,omitempty"`
}

// Capture keeps one value from a login response as {{login.<Name>}}.
// From is json:<path>, header:<name>, cookie:<name> or regex:<pattern with one group>.
type Capture struct {
	Name string `json:"name"`
	From string `json:"from"`
}

// AuthRequestIDs are the requests an auth setup sends itself (login steps
// and refresh). Tools and agents can't call them (CRED-8).
func (a Auth) AuthRequestIDs() []string {
	if a.Type != "login" {
		return nil
	}
	var out []string
	if a.LoginRequestID != "" {
		out = append(out, a.LoginRequestID)
	}
	for _, s := range a.Steps {
		if s.RequestID != "" {
			out = append(out, s.RequestID)
		}
	}
	if a.RefreshRequestID != "" {
		out = append(out, a.RefreshRequestID)
	}
	return out
}

// API is one system you call (APP-1). Name is the client name in tool code.
type API struct {
	ID             string    `json:"id"`
	Name           string    `json:"name"`
	Title          string    `json:"title"`
	Description    string    `json:"description"`
	Source         string    `json:"source"`
	AllowedHosts   []string  `json:"allowedHosts"`
	Auth           Auth      `json:"auth"`
	ActiveEnvID    string    `json:"activeEnvId"`
	ScriptsTrusted bool      `json:"scriptsTrusted"`
	TLS            TLS       `json:"tls"`
	CreatedAt      time.Time `json:"createdAt"`
}

// TLS is how the proxy checks an API's certificates (API-9).
type TLS struct {
	// Insecure skips certificate verification, for dev servers with
	// self-signed certificates.
	Insecure bool `json:"insecure"`
	// CACerts are extra PEM certificates trusted for this API, on top of
	// the system roots (a company CA, a self-signed server).
	CACerts string `json:"caCerts,omitempty"`
	// ClientCert and ClientKey are PEM for mutual TLS (API-10). ClientKey
	// is a {{variable}} holding a secret; a pasted key is moved into one.
	ClientCert string `json:"clientCert,omitempty"`
	ClientKey  string `json:"clientKey,omitempty"`
}

// SavedResponse is a response kept on a request, one per outcome you care
// about (200 ok, 404 not found…). Bodies are masked (EX-1, EX-3).
type SavedResponse struct {
	ID      string          `json:"id"`
	Name    string          `json:"name"`
	Status  int             `json:"status"`
	Body    json.RawMessage `json:"body"`
	SavedAt time.Time       `json:"savedAt"`
}

// Request is one saved request in an API (HTTP-1).
type Request struct {
	ID             string          `json:"id"`
	APIID          string          `json:"apiId"`
	Folder         string          `json:"folder"`
	Name           string          `json:"name"` // camelCase method name: crm.getAccount
	Title          string          `json:"title"`
	Description    string          `json:"description"`
	Method         string          `json:"method"`
	URL            string          `json:"url"`
	Headers        []KV            `json:"headers"`
	Query          []KV            `json:"query"`
	PathParams     []KV            `json:"pathParams,omitempty"` // values for {id} or :id when you send it; agents pass their own
	BodyType       string          `json:"bodyType"` // none | json | text | xml | form
	Body           string          `json:"body"`
	Auth           *Auth           `json:"auth,omitempty"`
	Access         string          `json:"access"` // read | write
	AccessReason   string          `json:"accessReason"`
	AccessFlag     bool            `json:"accessFlag"` // "check access" until confirmed (API-1)
	ResponseSchema json.RawMessage `json:"responseSchema,omitempty"`
	Example        json.RawMessage `json:"example,omitempty"` // masked response body (EX-1)
	Responses      []SavedResponse `json:"responses,omitempty"`
	TimeoutSeconds float64         `json:"timeoutSeconds,omitempty"` // 0: the default in Settings (NET-2)
	CreatedAt      time.Time       `json:"createdAt"`
	UpdatedAt      time.Time       `json:"updatedAt"`
}

// EffectiveAccess is write when the request writes or is flagged unconfirmed.
func (r Request) EffectiveAccess() string {
	if r.AccessFlag || r.Access != "read" {
		return "write"
	}
	return "read"
}

// Environment is a named set of variables (ENV-1).
type Environment struct {
	ID    string `json:"id"`
	APIID string `json:"apiId"`
	Name  string `json:"name"`
}

// Variable is a {{name}} value at API scope (EnvID "") or environment scope.
// Secret values are only stored encrypted (CRED-2) and never returned.
type Variable struct {
	ID         string    `json:"id"`
	APIID      string    `json:"apiId"`
	EnvID      string    `json:"envId"`
	Name       string    `json:"name"`
	Source     string    `json:"source"` // literal | os_env | secret
	Value      string    `json:"value,omitempty"`
	Ciphertext string    `json:"ciphertext,omitempty"`
	UpdatedAt  time.Time `json:"updatedAt"`
}

// Rules are a toolkit's approval rules for agent-saved versions (AB-2).
type Rules struct {
	AutoApprove      bool `json:"autoApprove"`
	RequireTests     bool `json:"requireTests"`
	DescriptionCheck bool `json:"descriptionCheck"`
	MaxOutputTokens  int  `json:"maxOutputTokens"`
	NoDuplicates     bool `json:"noDuplicates"`
}

// DefaultRules are the AB-2 defaults.
func DefaultRules() Rules {
	return Rules{AutoApprove: true, RequireTests: true, DescriptionCheck: true, MaxOutputTokens: 2000, NoDuplicates: true}
}

// Toolkit is a named set of tools served as one MCP endpoint (TK-1).
type Toolkit struct {
	ID          string    `json:"id"`
	Name        string    `json:"name"`
	Description string    `json:"description"`
	BuildScope  []string  `json:"buildScope"` // API IDs
	Rules       Rules     `json:"rules"`
	Hints       bool      `json:"hints"`
	HintTokens  int       `json:"hintTokens"`
	CreatedAt   time.Time `json:"createdAt"`
}

// Tool is a library entry; its code lives in versions (TOOL-11).
type Tool struct {
	ID        string    `json:"id"`
	Name      string    `json:"name"`
	CreatedAt time.Time `json:"createdAt"`
}

// Check is one approval rule result.
type Check struct {
	Rule   string `json:"rule"`
	Label  string `json:"label"`
	Passed bool   `json:"passed"`
	Detail string `json:"detail,omitempty"`
}

// ToolTest is an input plus a pm.test script run against the output (TOOL-8).
type ToolTest struct {
	Name   string          `json:"name"`
	Input  json.RawMessage `json:"input"`
	Script string          `json:"script,omitempty"`
}

// TestResult is the outcome of one tool test.
type TestResult struct {
	Name     string   `json:"name"`
	Passed   bool     `json:"passed"`
	Error    string   `json:"error,omitempty"`
	Asserts  []Assert `json:"asserts,omitempty"`
	Tokens   int      `json:"tokens"`
	Duration int64    `json:"durationMs"`
}

// Assert is one pm.test result.
type Assert struct {
	Name   string `json:"name"`
	Passed bool   `json:"passed"`
	Error  string `json:"error,omitempty"`
}

// ToolVersion is immutable once created (VER-1); only status changes.
type ToolVersion struct {
	ID            string          `json:"id"`
	ToolID        string          `json:"toolId"`
	N             int             `json:"n"`
	Source        string          `json:"source"`
	Name          string          `json:"name"`
	Description   string          `json:"description"`
	UseWhen       string          `json:"useWhen"`
	DontUseWhen   string          `json:"dontUseWhen"`
	InputSchema   json.RawMessage `json:"inputSchema"`
	Access        string          `json:"access"` // read | write
	APIs          []string        `json:"apis"`
	Calls         []string        `json:"calls"` // "crm.getAccount"
	Tests         []ToolTest      `json:"tests"`
	TestResults   []TestResult    `json:"testResults"`
	Checks        []Check         `json:"checks"`
	Status        string          `json:"status"` // live | pending | rejected | retired
	CreatedByKind string          `json:"createdByKind"`
	CreatedByID   string          `json:"createdById"`
	CreatedByName string          `json:"createdByName"`
	SessionID     string          `json:"sessionId,omitempty"`
	ToolkitID     string          `json:"toolkitId,omitempty"`
	Fulfils       string          `json:"fulfils,omitempty"`
	Note          string          `json:"note,omitempty"`
	CreatedAt     time.Time       `json:"createdAt"`
	DecidedAt     *time.Time      `json:"decidedAt,omitempty"`
}

// Agent is a client you've added (AGT-1). Only a hash of its token is kept.
type Agent struct {
	ID          string     `json:"id"`
	Name        string     `json:"name"`
	Client      string     `json:"client"` // claude-code | vscode | cursor | claude-desktop | other
	TokenHash   string     `json:"-"`
	TokenPrefix string     `json:"tokenPrefix"`
	CreatedAt   time.Time  `json:"createdAt"`
	RevokedAt   *time.Time `json:"revokedAt,omitempty"`
	LastSeenAt  *time.Time `json:"lastSeenAt,omitempty"`
	ClientName  string     `json:"clientName,omitempty"`
}

// agentDoc stores the hash, which the JSON form above hides.
type agentDoc struct {
	Agent
	Hash string `json:"tokenHash"`
}

// Grant gives an agent one toolkit (AGT-2).
type Grant struct {
	ID        string    `json:"id"`
	AgentID   string    `json:"agentId"`
	ToolkitID string    `json:"toolkitId"`
	Access    string    `json:"access"` // read | read_write
	CanBuild  bool      `json:"canBuild"`
	CreatedAt time.Time `json:"createdAt"`
}

// Session is one MCP session (SES-1).
type Session struct {
	ID            string    `json:"id"`
	AgentID       string    `json:"agentId"`
	ToolkitID     string    `json:"toolkitId"`
	Mode          string    `json:"mode"` // use | build
	ClientName    string    `json:"clientName"`
	ClientVersion string    `json:"clientVersion"`
	StartedAt     time.Time `json:"startedAt"`
	LastSeenAt    time.Time `json:"lastSeenAt"`
}

// ToolRequest is an agent asking for a tool it can't build (AB-4).
type ToolRequest struct {
	ID          string    `json:"id"`
	ToolkitID   string    `json:"toolkitId"`
	AgentID     string    `json:"agentId"`
	AgentName   string    `json:"agentName"`
	SessionID   string    `json:"sessionId"`
	Description string    `json:"description"`
	Tried       string    `json:"tried"`
	State       string    `json:"state"` // open | closed
	FulfilledBy string    `json:"fulfilledBy,omitempty"`
	CreatedAt   time.Time `json:"createdAt"`
}

// FieldReport is an agent saying which output fields it used (FB-1).
type FieldReport struct {
	ID        string    `json:"id"`
	ToolID    string    `json:"toolId"`
	Version   int       `json:"version"`
	AgentID   string    `json:"agentId"`
	AgentName string    `json:"agentName"`
	SessionID string    `json:"sessionId"`
	Used      []string  `json:"used"`
	Unused    []string  `json:"unused"`
	Note      string    `json:"note"`
	CreatedAt time.Time `json:"createdAt"`
}

// Call is one recorded call (SES-2).
type Call struct {
	ID          string    `json:"id"`
	SessionID   string    `json:"sessionId"`
	ParentID    string    `json:"parentId"`
	ActorKind   string    `json:"actorKind"` // you | agent
	ActorID     string    `json:"actorId"`
	ActorName   string    `json:"actorName"`
	ToolkitID   string    `json:"toolkitId"`
	Kind        string    `json:"kind"` // tool | build | list | upstream
	Name        string    `json:"name"`
	Status      string    `json:"status"` // ok | error | blocked
	Error       string    `json:"error,omitempty"`
	StartedAt   time.Time `json:"startedAt"`
	DurationMS  int64     `json:"durationMs"`
	Input       string    `json:"input,omitempty"`
	Output      string    `json:"output,omitempty"`
	OutputBytes int       `json:"outputBytes"`
	Tokens      int       `json:"tokens"`
	Method      string    `json:"method,omitempty"`
	URL         string    `json:"url,omitempty"`
	HTTPStatus  int       `json:"httpStatus,omitempty"`
	APIID       string    `json:"apiId,omitempty"`
	RequestID   string    `json:"requestId,omitempty"`
	VersionID   string    `json:"versionId,omitempty"`
}

// AuditEvent is one entry in the hash-chained audit log (AUD-1).
type AuditEvent struct {
	Seq       int64     `json:"seq"`
	ID        string    `json:"id"`
	At        time.Time `json:"at"`
	ActorKind string    `json:"actorKind"`
	ActorID   string    `json:"actorId"`
	ActorName string    `json:"actorName"`
	Action    string    `json:"action"`
	Target    string    `json:"target"`
	Detail    string    `json:"detail"`
	PrevHash  string    `json:"prevHash"`
	Hash      string    `json:"hash"`
}
