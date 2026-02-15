package client

import (
	"fmt"
	"os"
	"os/user"
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
	Processes       []ProcessConfig `toml:"process"`
	Checks          []CheckConfig   `toml:"check"`

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
	RunAsUser  string `toml:"run_as_user,omitempty"`

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
	home := realUserHome()
	if home != "" {
		switch runtime.GOOS {
		case "darwin":
			return filepath.Join(home, "Library", "Application Support", "MachineMon", "client.toml")
		default:
			return filepath.Join(home, ".config", "machinemon", "client.toml")
		}
	}
	return "/etc/machinemon/client.toml"
}

// realUserHome returns the home directory of the real user, even under sudo.
func realUserHome() string {
	// If running under sudo, use the invoking user's home
	if sudoUser := os.Getenv("SUDO_USER"); sudoUser != "" {
		if u, err := user.Lookup(sudoUser); err == nil && u.HomeDir != "" {
			return u.HomeDir
		}
		switch runtime.GOOS {
		case "darwin":
			home := filepath.Join("/Users", sudoUser)
			if _, err := os.Stat(home); err == nil {
				return home
			}
		default:
			home := filepath.Join("/home", sudoUser)
			if _, err := os.Stat(home); err == nil {
				return home
			}
		}
	}
	if home, err := os.UserHomeDir(); err == nil {
		return home
	}
	return ""
}
