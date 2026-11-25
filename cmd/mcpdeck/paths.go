package main

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
)

// dataDir is the per-user data folder (RUN-3).
func dataDir() string {
	switch runtime.GOOS {
	case "windows":
		if v := os.Getenv("LOCALAPPDATA"); v != "" {
			return filepath.Join(v, "mcpDeck")
		}
	case "darwin":
		h, _ := os.UserHomeDir()
		return filepath.Join(h, "Library", "Application Support", "mcpDeck")
	}
	if v := os.Getenv("XDG_DATA_HOME"); v != "" {
		return filepath.Join(v, "mcpdeck")
	}
	h, _ := os.UserHomeDir()
	return filepath.Join(h, ".local", "share", "mcpdeck")
}

// instance is written while a server runs, so the CLI can find it.
type instance struct {
	Port     int    `json:"port"`
	PID      int    `json:"pid"`
	CLIToken string `json:"cliToken"`
}

func instancePath(dir string) string { return filepath.Join(dir, "instance.json") }

func readInstance(dir string) (*instance, error) {
	b, err := os.ReadFile(instancePath(dir))
	if err != nil {
		return nil, err
	}
	var in instance
	return &in, json.Unmarshal(b, &in)
}

func writeInstance(dir string, in instance) error {
	b, _ := json.Marshal(in)
	return os.WriteFile(instancePath(dir), b, 0o600)
}

// opener opens a URL or folder with the OS's default handler.
type opener struct{}

func (opener) Open(target string) error {
	var cmd *exec.Cmd
	switch runtime.GOOS {
	case "windows":
		cmd = exec.Command("rundll32", "url.dll,FileProtocolHandler", target)
	case "darwin":
		cmd = exec.Command("open", target)
	default:
		cmd = exec.Command("xdg-open", target)
	}
	return cmd.Start()
}
