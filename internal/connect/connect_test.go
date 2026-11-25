package connect

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/abhijeet-oxide/mcpdeck/skills"
)

type fakeOS struct{ home, goos string }

func (f fakeOS) Home() string       { return f.home }
func (f fakeOS) GOOS() string       { return f.goos }
func (f fakeOS) AppData() string    { return filepath.Join(f.home, "AppData") }
func (f fakeOS) Executable() string { return filepath.Join(f.home, "bin", "mcpdeck") }

func in(client string, build bool) Input {
	return Input{Client: client, Toolkits: []Toolkit{{Name: "support", CanBuild: build}}, Token: "mcpd_tok", BaseURL: "http://127.0.0.1:47800", Skills: true}
}

// REQ: AGT-3
func TestClaudeCode_KeepsExistingEntriesAndAddsBuilderSubagent(t *testing.T) {
	home := t.TempDir()
	cfg := filepath.Join(home, ".claude.json")
	_ = os.WriteFile(cfg, []byte(`{"numStartups": 4, "mcpServers": {"github": {"type": "http", "url": "https://x"}}}`), 0o600)
	x := in("claude-code", true)
	x.DataDir = filepath.Join(home, "data")
	p, err := Build(x, fakeOS{home, "linux"}, skills.FS, skills.Names)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(p.Command, "claude mcp add --transport http --scope user mcpdeck-support http://127.0.0.1:47800/mcp/support") {
		t.Fatalf("command: %s", p.Command)
	}
	if _, err := Apply(p); err != nil {
		t.Fatal(err)
	}
	var root map[string]any
	b, _ := os.ReadFile(cfg)
	if err := json.Unmarshal(b, &root); err != nil {
		t.Fatal(err)
	}
	ms := root["mcpServers"].(map[string]any)
	if ms["github"] == nil || ms["mcpdeck-support"] == nil || root["numStartups"] == nil {
		t.Fatalf("existing entries lost: %s", b)
	}
	if ms["mcpdeck-support-build"] != nil {
		t.Fatal("the build server must not be in the main session's config")
	}
	if _, err := os.Stat(cfg + ".mcpdeck-backup"); err != nil {
		t.Fatal("no backup written")
	}
	agent, err := os.ReadFile(filepath.Join(home, ".claude", "agents", "mcpdeck-support-builder.md"))
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"name: mcpdeck-support-builder", "mcpServers:", "url: http://127.0.0.1:47800/mcp/support/build", "mcp__mcpdeck-support-build__*", "- mcpdeck-build-tool"} {
		if !strings.Contains(string(agent), want) {
			t.Fatalf("subagent missing %q:\n%s", want, agent)
		}
	}
	if _, err := os.Stat(filepath.Join(home, ".claude", "skills", "mcpdeck-trim-output", "scripts", "field-sizes.mjs")); err != nil {
		t.Fatal("skill scripts not installed")
	}
}

// REQ: AGT-3, SKL-2
func TestVSCode_JSONCConfigLinkAndInstructions(t *testing.T) {
	home := t.TempDir()
	proj := t.TempDir()
	_ = os.MkdirAll(filepath.Join(proj, ".vscode"), 0o755)
	_ = os.WriteFile(filepath.Join(proj, ".vscode", "mcp.json"), []byte("{\n  // mine\n  \"servers\": { \"other\": { \"type\": \"stdio\", \"command\": \"x\" }, },\n}\n"), 0o644)
	x := in("vscode", true)
	x.Scope, x.ProjectDir, x.DataDir = "project", proj, filepath.Join(home, "data")
	p, err := Build(x, fakeOS{home, "windows"}, skills.FS, skills.Names)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(p.Link, "vscode:mcp/install?") || !strings.Contains(p.Command, `code --add-mcp "`) {
		t.Fatalf("link/command: %s %s", p.Link, p.Command)
	}
	if _, err := Apply(p); err != nil {
		t.Fatal(err)
	}
	var root map[string]any
	b, _ := os.ReadFile(filepath.Join(proj, ".vscode", "mcp.json"))
	if err := json.Unmarshal(b, &root); err != nil {
		t.Fatalf("written file must parse: %v\n%s", err, b)
	}
	s := root["servers"].(map[string]any)
	if s["other"] == nil || s["mcpdeck-support"] == nil || s["mcpdeck-support-build"] == nil {
		t.Fatalf("servers: %v", s)
	}
	ag, _ := os.ReadFile(filepath.Join(proj, ".github", "agents", "mcpdeck-support-builder.agent.md"))
	if !strings.Contains(string(ag), "tools: ['mcpdeck-support-build/*']") {
		t.Fatalf("agent file: %s", ag)
	}
	ins, _ := os.ReadFile(filepath.Join(proj, ".github", "instructions", "mcpdeck-trim-output.instructions.md"))
	if !strings.Contains(string(ins), filepath.ToSlash(filepath.Join(home, "data", "skills", "mcpdeck-trim-output", "scripts"))) {
		t.Fatalf("instructions must point at the data folder scripts:\n%s", ins)
	}
}

// REQ: AGT-3, TK-2
func TestClaudeDesktop_UsesStdioBridge(t *testing.T) {
	home := t.TempDir()
	p, err := Build(in("claude-desktop", false), fakeOS{home, "darwin"}, skills.FS, nil)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(p.Snippet, `"MCPDECK_TOKEN": "mcpd_tok"`) || !strings.Contains(p.Snippet, `"stdio"`) {
		t.Fatalf("snippet: %s", p.Snippet)
	}
	if !strings.HasSuffix(p.Files[0].Path, filepath.Join("Claude", "claude_desktop_config.json")) {
		t.Fatalf("path: %s", p.Files[0].Path)
	}
}

func TestCursor_Deeplink(t *testing.T) {
	p, err := Build(in("cursor", false), fakeOS{t.TempDir(), "linux"}, skills.FS, nil)
	if err != nil || !strings.HasPrefix(p.Link, "cursor://anysphere.cursor-deeplink/mcp/install?name=mcpdeck-support&config=") {
		t.Fatalf("%v %s", err, p.Link)
	}
}
