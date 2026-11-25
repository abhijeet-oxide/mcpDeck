// Package connect writes MCP client configs, the builder subagent and the
// skills for Claude Code, VS Code, Cursor and Claude Desktop (AGT-3, SKL-2).
//
// Build makes a Plan: the client's own command, a one-click link where the
// client has one, a JSON snippet, and the exact files mcpDeck would write.
// Apply writes them, keeping a backup of every file it changes.
package connect

import (
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"net/url"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
)

// OS gives the paths that differ per platform. Tests use a fake.
type OS interface {
	Home() string
	GOOS() string
	// AppData is %APPDATA% on Windows, ~/Library/Application Support on
	// macOS and $XDG_CONFIG_HOME (or ~/.config) on Linux.
	AppData() string
	Executable() string
}

// RealOS is the running machine.
type RealOS struct{}

func (RealOS) Home() string { h, _ := os.UserHomeDir(); return h }
func (RealOS) GOOS() string { return runtime.GOOS }
func (RealOS) AppData() string {
	switch runtime.GOOS {
	case "windows":
		if v := os.Getenv("APPDATA"); v != "" {
			return v
		}
	case "darwin":
		return filepath.Join(RealOS{}.Home(), "Library", "Application Support")
	}
	if v := os.Getenv("XDG_CONFIG_HOME"); v != "" {
		return v
	}
	return filepath.Join(RealOS{}.Home(), ".config")
}
func (RealOS) Executable() string {
	p, err := os.Executable()
	if err != nil {
		return "mcpdeck"
	}
	return p
}

// Toolkit is one grant to write into the client.
type Toolkit struct {
	Name     string `json:"name"`
	CanBuild bool   `json:"canBuild"`
}

// Input describes what to connect.
type Input struct {
	Client     string    `json:"client"`
	Toolkits   []Toolkit `json:"toolkits"` // every toolkit the agent has; the token is shared
	Token      string    `json:"-"`
	BaseURL    string    `json:"baseUrl"`
	Scope      string    `json:"scope"` // user | project
	ProjectDir string    `json:"projectDir"`
	Skills     bool      `json:"skills"`
	DataDir    string    `json:"dataDir"`
}

// File is one file the plan writes.
type File struct {
	Path    string `json:"path"`
	Kind    string `json:"kind"` // config | subagent | skill | instructions
	Exists  bool   `json:"exists"`
	Content string `json:"content"`
	Summary string `json:"summary"`
}

// Plan is what connecting does, shown before anything is written.
type Plan struct {
	Client      string   `json:"client"`
	Command     string   `json:"command"`
	Link        string   `json:"link,omitempty"`
	Snippet     string   `json:"snippet"`
	SnippetPath string   `json:"snippetPath"`
	Files       []File   `json:"files"`
	Notes       []string `json:"notes"`
	FirstPrompt string   `json:"firstPrompt"`
}

// ServerName is the client-side name of a toolkit's server.
func ServerName(toolkit string, build bool) string {
	if build {
		return "mcpdeck-" + toolkit + "-build"
	}
	return "mcpdeck-" + toolkit
}

// BuilderName is the builder subagent's name.
func BuilderName(toolkit string) string { return "mcpdeck-" + toolkit + "-builder" }

func endpoint(base, tk string, build bool) string {
	u := strings.TrimSuffix(base, "/") + "/mcp/" + tk
	if build {
		u += "/build"
	}
	return u
}

func httpServer(in Input, tk string, build bool, typed bool) map[string]any {
	m := map[string]any{"url": endpoint(in.BaseURL, tk, build), "headers": map[string]string{"Authorization": "Bearer " + in.Token}}
	if typed {
		m["type"] = "http"
	}
	return m
}

// Build makes the plan. skills holds the bundled skill folders.
func Build(in Input, sys OS, skills fs.FS, skillNames []string) (*Plan, error) {
	if len(in.Toolkits) == 0 {
		return nil, errors.New("no toolkit to connect")
	}
	if in.Scope == "" {
		in.Scope = "user"
	}
	if in.Scope == "project" && in.ProjectDir == "" {
		return nil, errors.New("project scope needs the project folder")
	}
	p := &Plan{Client: in.Client, Files: []File{}, Notes: []string{}}
	first := in.Toolkits[0]
	p.FirstPrompt = fmt.Sprintf("Use the mcpdeck-%s tools to list what you can do for me, then answer one real question with them.", first.Name)
	switch in.Client {
	case "claude-code":
		claudeCode(in, sys, p)
	case "vscode":
		vscode(in, sys, p)
	case "cursor":
		cursor(in, sys, p)
	case "claude-desktop":
		claudeDesktop(in, sys, p)
	case "other":
		servers := map[string]any{}
		for _, tk := range in.Toolkits {
			servers[ServerName(tk.Name, false)] = httpServer(in, tk.Name, false, true)
		}
		p.Snippet = pretty(map[string]any{"mcpServers": servers})
		p.Notes = append(p.Notes, "Add the server to your client as Streamable HTTP with the Authorization header.")
	default:
		return nil, fmt.Errorf("unknown client %q", in.Client)
	}
	if in.Skills {
		if err := addSkills(in, sys, skills, skillNames, p); err != nil {
			return nil, err
		}
	}
	for i := range p.Files {
		if _, err := os.Stat(p.Files[i].Path); err == nil {
			p.Files[i].Exists = true
		}
	}
	p.Notes = append(p.Notes, "The token is stored in the client's config in plain text, as MCP clients require. Revoke it in mcpDeck if it leaks.")
	return p, nil
}

func pretty(v any) string {
	b, _ := json.MarshalIndent(v, "", "  ")
	return string(b)
}

// mergeJSON sets key path in a JSON file, keeping everything else (AGT-3).
func mergeJSON(path string, set func(root map[string]any)) (string, error) {
	root := map[string]any{}
	if b, err := os.ReadFile(path); err == nil && strings.TrimSpace(string(b)) != "" {
		if err := json.Unmarshal(stripJSONC(b), &root); err != nil {
			return "", fmt.Errorf("%s isn't valid JSON (%v); fix it or paste the snippet by hand", path, err)
		}
	}
	set(root)
	return pretty(root) + "\n", nil
}

// stripJSONC removes // and /* */ comments and trailing commas (VS Code files).
func stripJSONC(b []byte) []byte {
	var out []byte
	inStr, esc := false, false
	for i := 0; i < len(b); i++ {
		c := b[i]
		if inStr {
			out = append(out, c)
			if esc {
				esc = false
			} else if c == '\\' {
				esc = true
			} else if c == '"' {
				inStr = false
			}
			continue
		}
		if c == '"' {
			inStr = true
			out = append(out, c)
			continue
		}
		if c == '/' && i+1 < len(b) && b[i+1] == '/' {
			for i < len(b) && b[i] != '\n' {
				i++
			}
			out = append(out, '\n')
			continue
		}
		if c == '/' && i+1 < len(b) && b[i+1] == '*' {
			i += 2
			for i+1 < len(b) && !(b[i] == '*' && b[i+1] == '/') {
				i++
			}
			i++
			continue
		}
		if c == ',' {
			j := i + 1
			for j < len(b) && (b[j] == ' ' || b[j] == '\n' || b[j] == '\r' || b[j] == '\t') {
				j++
			}
			if j < len(b) && (b[j] == '}' || b[j] == ']') {
				continue
			}
		}
		out = append(out, c)
	}
	return out
}

func obj(m map[string]any, key string) map[string]any {
	if v, ok := m[key].(map[string]any); ok {
		return v
	}
	v := map[string]any{}
	m[key] = v
	return v
}

func shellQuote(s string, goos string) string {
	if goos == "windows" {
		return `"` + strings.ReplaceAll(s, `"`, `\"`) + `"`
	}
	return "'" + strings.ReplaceAll(s, "'", `'\''`) + "'"
}

// claudeCode: `claude mcp add`, ~/.claude.json or .mcp.json, and the builder
// subagent in ~/.claude/agents or .claude/agents with its own MCP server.
func claudeCode(in Input, sys OS, p *Plan) {
	var cmds []string
	scope := "user"
	if in.Scope == "project" {
		scope = "project"
	}
	for _, tk := range in.Toolkits {
		cmds = append(cmds, fmt.Sprintf(`claude mcp add --transport http --scope %s %s %s --header "Authorization: Bearer %s"`,
			scope, ServerName(tk.Name, false), endpoint(in.BaseURL, tk.Name, false), in.Token))
	}
	p.Command = strings.Join(cmds, "\n")
	cfg := filepath.Join(sys.Home(), ".claude.json")
	agentsDir := filepath.Join(sys.Home(), ".claude", "agents")
	if in.Scope == "project" {
		cfg = filepath.Join(in.ProjectDir, ".mcp.json")
		agentsDir = filepath.Join(in.ProjectDir, ".claude", "agents")
	}
	servers := map[string]any{}
	for _, tk := range in.Toolkits {
		servers[ServerName(tk.Name, false)] = httpServer(in, tk.Name, false, true)
	}
	p.Snippet = pretty(map[string]any{"mcpServers": servers})
	p.SnippetPath = cfg
	content, err := mergeJSON(cfg, func(root map[string]any) {
		ms := obj(root, "mcpServers")
		for k, v := range servers {
			ms[k] = v
		}
	})
	if err != nil {
		p.Notes = append(p.Notes, err.Error())
	} else {
		p.Files = append(p.Files, File{Path: cfg, Kind: "config", Content: content, Summary: "adds " + keys(servers) + " under mcpServers; other entries are kept"})
	}
	for _, tk := range in.Toolkits {
		if !tk.CanBuild {
			continue
		}
		p.Files = append(p.Files, File{Path: filepath.Join(agentsDir, BuilderName(tk.Name)+".md"), Kind: "subagent",
			Content: claudeBuilder(in, tk.Name), Summary: "builder subagent with the " + tk.Name + " build tools; the main session never loads them"})
	}
	if in.Scope == "project" {
		p.Notes = append(p.Notes, "Project files hold the token: add .mcp.json and .claude/agents/mcpdeck-* to .gitignore, or use user scope.")
	}
}

func claudeBuilder(in Input, tk string) string {
	server := ServerName(tk, true)
	skills := ""
	if in.Skills {
		skills = "skills:\n  - mcpdeck-build-tool\n"
	}
	return fmt.Sprintf(`---
name: %s
description: Builds a new mcpDeck tool for the %s toolkit when no listed mcpdeck-%s tool fits the task. Use proactively instead of exploring APIs in the main conversation. Brief it in one paragraph (the question, the inputs, the fields needed); it replies with the tool's name and status.
tools: mcp__%s__*, Read, Write, Bash
%smcpServers:
  - %s:
      type: http
      url: %s
      headers:
        Authorization: "Bearer %s"
---

You build one small, fast tool in the mcpDeck toolkit %q and report back.

1. search_tools and list_tool_requests: reuse or fulfil, don't duplicate.
2. search_apis, then describe_api for the requests you need.
3. Try calls with run_api or run_script. Return only the fields the brief asks for; run independent calls with Promise.all.
4. test_tool, then save_tool with at least one pm.test test.
5. Reply in two lines: the tool name, its status (live, or pending and the rule that held it) and what it returns. Nothing else.
`, BuilderName(tk), tk, tk, server, skills, server, endpoint(in.BaseURL, tk, true), in.Token, tk)
}

func keys(m map[string]any) string {
	var k []string
	for x := range m {
		k = append(k, x)
	}
	sort.Strings(k)
	return strings.Join(k, ", ")
}

func vscodeUserDir(sys OS) string {
	return filepath.Join(sys.AppData(), "Code", "User")
}

// vscode: `code --add-mcp`, vscode:mcp/install links, mcp.json, and a custom
// agent restricted to the build server for runSubagent.
func vscode(in Input, sys OS, p *Plan) {
	servers := map[string]any{}
	var cmds []string
	for _, tk := range in.Toolkits {
		s := httpServer(in, tk.Name, false, true)
		servers[ServerName(tk.Name, false)] = s
		withName := map[string]any{"name": ServerName(tk.Name, false), "type": "http", "url": s["url"], "headers": s["headers"]}
		j, _ := json.Marshal(withName)
		cmds = append(cmds, "code --add-mcp "+shellQuote(string(j), sys.GOOS()))
		if p.Link == "" {
			p.Link = "vscode:mcp/install?" + url.QueryEscape(string(j))
		}
		if tk.CanBuild {
			servers[ServerName(tk.Name, true)] = httpServer(in, tk.Name, true, true)
		}
	}
	p.Command = strings.Join(cmds, "\n")
	cfg := filepath.Join(vscodeUserDir(sys), "mcp.json")
	agentDir := filepath.Join(vscodeUserDir(sys), "prompts")
	if in.Scope == "project" {
		cfg = filepath.Join(in.ProjectDir, ".vscode", "mcp.json")
		agentDir = filepath.Join(in.ProjectDir, ".github", "agents")
	}
	p.Snippet = pretty(map[string]any{"servers": servers})
	p.SnippetPath = cfg
	content, err := mergeJSON(cfg, func(root map[string]any) {
		ss := obj(root, "servers")
		for k, v := range servers {
			ss[k] = v
		}
	})
	if err != nil {
		p.Notes = append(p.Notes, err.Error())
	} else {
		p.Files = append(p.Files, File{Path: cfg, Kind: "config", Content: content, Summary: "adds " + keys(servers) + " under servers; other entries are kept"})
	}
	for _, tk := range in.Toolkits {
		if !tk.CanBuild {
			continue
		}
		body := fmt.Sprintf(`---
name: %s
description: Builds a new mcpDeck tool for the %s toolkit when no listed tool fits. Run it as a subagent with a one-paragraph brief.
tools: ['%s/*']
user-invocable: false
---

You build one small, fast tool in the mcpDeck toolkit %q and report back.
Use search_tools, search_apis, describe_api, run_api or run_script, test_tool, then save_tool with at least one pm.test test.
Reply in two lines: the tool name, its status (live, or pending and why) and what it returns.
`, BuilderName(tk.Name), tk.Name, ServerName(tk.Name, true), tk.Name)
		p.Files = append(p.Files, File{Path: filepath.Join(agentDir, BuilderName(tk.Name)+".agent.md"), Kind: "subagent", Content: body,
			Summary: "custom agent limited to the " + tk.Name + " build tools, for runSubagent"})
		p.Notes = append(p.Notes, "In VS Code's tool picker you can turn off "+ServerName(tk.Name, true)+" for the main chat; the builder agent still has it.")
	}
}

// cursor: deeplink, ~/.cursor/mcp.json or .cursor/mcp.json.
func cursor(in Input, sys OS, p *Plan) {
	servers := map[string]any{}
	for _, tk := range in.Toolkits {
		s := httpServer(in, tk.Name, false, false)
		servers[ServerName(tk.Name, false)] = s
		if p.Link == "" {
			j, _ := json.Marshal(s)
			p.Link = "cursor://anysphere.cursor-deeplink/mcp/install?name=" + url.QueryEscape(ServerName(tk.Name, false)) + "&config=" + base64.StdEncoding.EncodeToString(j)
		}
		if tk.CanBuild {
			servers[ServerName(tk.Name, true)] = httpServer(in, tk.Name, true, false)
		}
	}
	cfg := filepath.Join(sys.Home(), ".cursor", "mcp.json")
	if in.Scope == "project" {
		cfg = filepath.Join(in.ProjectDir, ".cursor", "mcp.json")
	}
	p.Snippet = pretty(map[string]any{"mcpServers": servers})
	p.SnippetPath = cfg
	p.Command = "Open the link, or add the snippet to " + cfg
	content, err := mergeJSON(cfg, func(root map[string]any) {
		ms := obj(root, "mcpServers")
		for k, v := range servers {
			ms[k] = v
		}
	})
	if err != nil {
		p.Notes = append(p.Notes, err.Error())
	} else {
		p.Files = append(p.Files, File{Path: cfg, Kind: "config", Content: content, Summary: "adds " + keys(servers) + " under mcpServers; other entries are kept"})
	}
	for _, tk := range in.Toolkits {
		if tk.CanBuild {
			p.Notes = append(p.Notes, "Cursor can't give one subagent its own MCP server, so "+ServerName(tk.Name, true)+" is a separate server: turn it on in the chat that builds tools and off elsewhere.")
		}
	}
}

// claudeDesktop: stdio bridge with the token in MCPDECK_TOKEN.
func claudeDesktop(in Input, sys OS, p *Plan) {
	dir := filepath.Join(sys.AppData(), "Claude")
	cfg := filepath.Join(dir, "claude_desktop_config.json")
	servers := map[string]any{}
	for _, tk := range in.Toolkits {
		servers[ServerName(tk.Name, false)] = map[string]any{
			"command": sys.Executable(),
			"args":    []string{"stdio", "--toolkit", tk.Name, "--url", strings.TrimSuffix(in.BaseURL, "/")},
			"env":     map[string]string{"MCPDECK_TOKEN": in.Token},
		}
	}
	p.Snippet = pretty(map[string]any{"mcpServers": servers})
	p.SnippetPath = cfg
	p.Command = "Write the config below, then restart Claude Desktop"
	content, err := mergeJSON(cfg, func(root map[string]any) {
		ms := obj(root, "mcpServers")
		for k, v := range servers {
			ms[k] = v
		}
	})
	if err != nil {
		p.Notes = append(p.Notes, err.Error())
	} else {
		p.Files = append(p.Files, File{Path: cfg, Kind: "config", Content: content, Summary: "adds " + keys(servers) + " under mcpServers; other entries are kept"})
	}
	p.Notes = append(p.Notes, "Claude Desktop has no subagents, so building is left to Claude Code, VS Code or Cursor.")
}

// addSkills installs the bundled skills for the client (SKL-2).
func addSkills(in Input, sys OS, skills fs.FS, names []string, p *Plan) error {
	for _, name := range names {
		body, err := fs.ReadFile(skills, name+"/SKILL.md")
		if err != nil {
			return err
		}
		switch in.Client {
		case "claude-code":
			base := filepath.Join(sys.Home(), ".claude", "skills", name)
			if in.Scope == "project" {
				base = filepath.Join(in.ProjectDir, ".claude", "skills", name)
			}
			err := fs.WalkDir(skills, name, func(path string, d fs.DirEntry, err error) error {
				if err != nil || d.IsDir() {
					return err
				}
				b, _ := fs.ReadFile(skills, path)
				rel := strings.TrimPrefix(path, name+"/")
				p.Files = append(p.Files, File{Path: filepath.Join(base, filepath.FromSlash(rel)), Kind: "skill", Content: string(b), Summary: "skill " + name})
				return nil
			})
			if err != nil {
				return err
			}
		case "vscode", "cursor":
			scripts := filepath.Join(in.DataDir, "skills", name, "scripts")
			text := strings.ReplaceAll(string(body), "scripts/", filepath.ToSlash(scripts)+"/")
			desc, rest := splitFrontmatter(text)
			// Scripts live in mcpDeck's data folder; the instructions point there.
			_ = fs.WalkDir(skills, name+"/scripts", func(path string, d fs.DirEntry, err error) error {
				if err != nil || d.IsDir() {
					return nil
				}
				b, _ := fs.ReadFile(skills, path)
				p.Files = append(p.Files, File{Path: filepath.Join(in.DataDir, "skills", filepath.FromSlash(path)), Kind: "skill", Content: string(b), Summary: "script for " + name})
				return nil
			})
			if in.Client == "vscode" {
				dir := filepath.Join(vscodeUserDir(sys), "prompts")
				if in.Scope == "project" {
					dir = filepath.Join(in.ProjectDir, ".github", "instructions")
				}
				content := "---\ndescription: " + yamlString(desc) + "\n---\n" + rest
				p.Files = append(p.Files, File{Path: filepath.Join(dir, name+".instructions.md"), Kind: "instructions", Content: content, Summary: "instructions " + name + " (loaded when relevant)"})
			} else {
				if in.Scope != "project" {
					if name == names[0] {
						p.Notes = append(p.Notes, "Cursor keeps rules per project; choose project scope to install the skills as rules.")
					}
					continue
				}
				content := "---\ndescription: " + yamlString(desc) + "\nalwaysApply: false\n---\n" + rest
				p.Files = append(p.Files, File{Path: filepath.Join(in.ProjectDir, ".cursor", "rules", name+".mdc"), Kind: "instructions", Content: content, Summary: "rule " + name + " (applied when relevant)"})
			}
		}
	}
	return nil
}

func splitFrontmatter(s string) (desc, rest string) {
	if !strings.HasPrefix(s, "---") {
		return "", s
	}
	end := strings.Index(s[3:], "\n---")
	if end < 0 {
		return "", s
	}
	fm := s[3 : 3+end]
	rest = strings.TrimLeft(s[3+end+4:], "\n")
	for _, line := range strings.Split(fm, "\n") {
		if strings.HasPrefix(line, "description:") {
			desc = strings.TrimSpace(strings.TrimPrefix(line, "description:"))
		}
	}
	return desc, rest
}

func yamlString(s string) string {
	b, _ := json.Marshal(s)
	return string(b)
}

// Apply writes the plan's files, backing up each existing one to
// <file>.mcpdeck-backup. It returns the paths written.
func Apply(p *Plan) ([]string, error) {
	var written []string
	for _, f := range p.Files {
		if err := os.MkdirAll(filepath.Dir(f.Path), 0o755); err != nil {
			return written, fmt.Errorf("create %s: %w", filepath.Dir(f.Path), err)
		}
		if old, err := os.ReadFile(f.Path); err == nil {
			if string(old) == f.Content {
				continue
			}
			if f.Kind == "config" {
				if err := os.WriteFile(f.Path+".mcpdeck-backup", old, 0o600); err != nil {
					return written, fmt.Errorf("back up %s: %w", f.Path, err)
				}
			}
		}
		mode := os.FileMode(0o644)
		if f.Kind == "config" || f.Kind == "subagent" {
			mode = 0o600 // holds the token
		}
		if err := os.WriteFile(f.Path, []byte(f.Content), mode); err != nil {
			return written, fmt.Errorf("write %s: %w", f.Path, err)
		}
		written = append(written, f.Path)
	}
	return written, nil
}
