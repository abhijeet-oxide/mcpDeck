package connect

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/url"
	"path/filepath"
	"strings"
)

// BuilderServerName is the client-side name of the admin MCP server.
const BuilderServerName = "mcpdeck"

// builderAgent is the name the admin MCP server records actions under.
func builderAgent(client string) string {
	switch client {
	case "vscode":
		return "GitHub Copilot"
	case "claude-code":
		return "Claude Code"
	case "cursor":
		return "Cursor"
	case "claude-desktop":
		return "Claude Desktop"
	}
	return "Agent"
}

// Builder plans adding the admin MCP server (`mcpdeck mcp`) to a client, so
// an agent there can set mcpDeck up and build tools from a description
// (CON-1). It runs mcpDeck's own binary on stdio and needs no token: it
// reaches the running mcpDeck through the CLI's local connection.
func Builder(client, scope, projectDir string, sys OS) (*Plan, error) {
	exe := sys.Executable()
	args := []string{"mcp", "--agent", builderAgent(client)}
	stdio := map[string]any{"command": exe, "args": args}
	p := &Plan{Client: client, Files: []File{}, Notes: []string{}}
	p.FirstPrompt = "Use the mcpdeck tools to build tools for <the web app's URL>: <what you do there, e.g. list my claims and submit a claim with a receipt>. Open the browser and I'll sign in."
	merge := func(cfg, key string, entry map[string]any) {
		content, err := mergeJSON(cfg, func(root map[string]any) { obj(root, key)[BuilderServerName] = entry })
		if err != nil {
			p.Notes = append(p.Notes, err.Error())
			return
		}
		p.Files = append(p.Files, File{Path: cfg, Kind: "config", Content: content, Summary: "adds " + BuilderServerName + " under " + key + "; other entries are kept"})
		p.Snippet, p.SnippetPath = pretty(map[string]any{key: map[string]any{BuilderServerName: entry}}), cfg
	}
	switch client {
	case "vscode":
		entry := map[string]any{"type": "stdio", "command": exe, "args": args}
		withName := map[string]any{"name": BuilderServerName, "type": "stdio", "command": exe, "args": args}
		j, _ := json.Marshal(withName)
		p.Link = "vscode:mcp/install?" + url.QueryEscape(string(j))
		p.Command = "code --add-mcp " + shellQuote(string(j), sys.GOOS())
		cfg := filepath.Join(vscodeUserDir(sys), "mcp.json")
		if scope == "project" && projectDir != "" {
			cfg = filepath.Join(projectDir, ".vscode", "mcp.json")
		}
		merge(cfg, "servers", entry)
	case "claude-code":
		q := make([]string, 0, len(args)+1)
		q = append(q, shellQuote(exe, sys.GOOS()))
		for _, a := range args {
			q = append(q, shellQuote(a, sys.GOOS()))
		}
		p.Command = "claude mcp add --scope user " + BuilderServerName + " -- " + strings.Join(q, " ")
		p.Snippet = p.Command
	case "cursor":
		j, _ := json.Marshal(stdio)
		p.Link = "cursor://anysphere.cursor-deeplink/mcp/install?name=" + BuilderServerName + "&config=" + base64.StdEncoding.EncodeToString(j)
		merge(filepath.Join(sys.Home(), ".cursor", "mcp.json"), "mcpServers", stdio)
		p.Command = "Open the link, or add the snippet to ~/.cursor/mcp.json"
	case "claude-desktop":
		merge(filepath.Join(sys.AppData(), "Claude", "claude_desktop_config.json"), "mcpServers", stdio)
		p.Command = "Write the config below, then restart Claude Desktop"
	default:
		return nil, fmt.Errorf("unknown client %q; use vscode, claude-code, cursor or claude-desktop", client)
	}
	p.Notes = append(p.Notes, "This server sets mcpDeck up with your reach (it runs as you). It can't approve write tools, change access, pick a files folder or read secrets: those stay with you.")
	return p, nil
}
