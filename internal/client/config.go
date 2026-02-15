package client

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"

	"github.com/BurntSushi/toml"
)

type Config struct {
	ClientID        string          `toml:"client_id"`
	ServerURL       string          `toml:"server_url"`
	Password        string          `toml:"password"`
	CheckInInterval int             `toml:"check_in_interval"` // seconds
	InsecureSkipTLS bool            `toml:"insecure_skip_tls"` // allow self-signed certs
	Processes []ProcessConfig `toml:"process"`
	Checks   []CheckConfig   `toml:"check"`

	path string `toml:"-"` // file path, not serialized
}

// CheckConfig defines a client-side check. The Type field determines what
// kind of check is run. Currently supported: "script". Future types like
// "http" and "file_touch" use their own fields.
type CheckConfig struct {
	FriendlyName string `toml:"friendly_name"`
	Type         string `toml:"type"` // "script", "http", "file_touch", ...

	// Script check fields
	ScriptPath string `toml:"script_path,omitempty"`

	// HTTP check fields (future)
	URL            string `toml:"url,omitempty"`
	ExpectedStatus int    `toml:"expected_status,omitempty"`

	// File touch check fields (future)
	FilePath   string `toml:"file_path,omitempty"`
	MaxAgeSecs int    `toml:"max_age_secs,omitempty"`
}

type ProcessConfig struct {
	FriendlyName string `toml:"friendly_name"`
	MatchPattern string `toml:"match_pattern"`
	MatchType    string `toml:"match_type"` // "substring" or "regex"
}

func DefaultConfig() *Config {
	return &Config{
		CheckInInterval: 120,
	}
}

func LoadConfig(path string) (*Config, error) {
	cfg := DefaultConfig()
	cfg.path = path

	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return cfg, nil
		}
		return nil, fmt.Errorf("read config: %w", err)
	}
	if err := toml.Unmarshal(data, cfg); err != nil {
		return nil, fmt.Errorf("parse config: %w", err)
	}
	// Default match_type
	for i := range cfg.Processes {
		if cfg.Processes[i].MatchType == "" {
			cfg.Processes[i].MatchType = "substring"
		}
	}
	return cfg, nil
}

func SaveConfig(cfg *Config, path string) error {
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0700); err != nil {
		return fmt.Errorf("create config dir: %w", err)
	}
	f, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0600)
	if err != nil {
		return fmt.Errorf("open config for writing: %w", err)
	}
	defer f.Close()
	return toml.NewEncoder(f).Encode(cfg)
}

func (c *Config) IsConfigured() bool {
	return c.ServerURL != "" && c.Password != ""
}

func DefaultConfigPath() string {
	switch runtime.GOOS {
	case "darwin":
		if home, err := os.UserHomeDir(); err == nil {
			return filepath.Join(home, "Library", "Application Support", "MachineMon", "client.toml")
		}
	default:
		if home, err := os.UserHomeDir(); err == nil {
			return filepath.Join(home, ".config", "machinemon", "client.toml")
		}
	}
	return "/etc/machinemon/client.toml"
}
