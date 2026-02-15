package client

import (
	"path/filepath"
	"testing"
)

func TestConfigRoundTripProcesses(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "client.toml")

	cfg := DefaultConfig()
	cfg.ServerURL = "https://example.com"
	cfg.Password = "secret"
	cfg.Processes = []ProcessConfig{
		{FriendlyName: "api", MatchPattern: "api.js", MatchType: "substring"},
		{FriendlyName: "worker", MatchPattern: "worker.js", MatchType: "substring"},
	}
	cfg.Checks = []CheckConfig{
		{FriendlyName: "health", Type: "script", ScriptPath: "curl -sf http://localhost:8080/health", RunAsUser: "www-data"},
	}

	if err := SaveConfig(cfg, path); err != nil {
		t.Fatalf("save config: %v", err)
	}

	loaded, err := LoadConfig(path)
	if err != nil {
		t.Fatalf("load config: %v", err)
	}
	if got, want := len(loaded.Processes), 2; got != want {
		t.Fatalf("processes length mismatch: got %d want %d", got, want)
	}
	if loaded.Processes[0].FriendlyName != "api" || loaded.Processes[0].MatchPattern != "api.js" {
		t.Fatalf("unexpected process[0]: %+v", loaded.Processes[0])
	}
	if got, want := len(loaded.Checks), 1; got != want {
		t.Fatalf("checks length mismatch: got %d want %d", got, want)
	}
	if loaded.Checks[0].RunAsUser != "www-data" {
		t.Fatalf("unexpected check run_as_user: %+v", loaded.Checks[0])
	}
}
