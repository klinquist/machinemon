package server

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"

	"github.com/BurntSushi/toml"
)

type Config struct {
	ListenAddr   string `toml:"listen_addr"`
	DatabasePath string `toml:"database_path"`
	BinariesDir  string `toml:"binaries_dir"` // directory containing client .tar.gz binaries

	// TLS
	TLSMode      string `toml:"tls_mode"` // "autocert", "selfsigned", "manual", "none"
	Domain       string `toml:"domain"`    // for autocert
	CertFile     string `toml:"cert_file"` // for manual
	KeyFile      string `toml:"key_file"`  // for manual
	CertCacheDir string `toml:"cert_cache_dir"`

	// Auth
	AdminPasswordHash  string `toml:"admin_password_hash"`
	ClientPasswordHash string `toml:"client_password_hash"`

	// Dev mode
	DevMode       bool   `toml:"dev_mode"`
	DevProxyURL   string `toml:"dev_proxy_url"`
}

func DefaultServerConfig() *Config {
	return &Config{
		ListenAddr:   ":8080",
		DatabasePath: defaultDatabasePath(),
		BinariesDir:  defaultBinariesDir(),
		TLSMode:      "none",
		CertCacheDir: defaultCertCacheDir(),
	}
}

func LoadServerConfig(path string) (*Config, error) {
	cfg := DefaultServerConfig()
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return cfg, nil
		}
		return nil, fmt.Errorf("read server config: %w", err)
	}
	if err := toml.Unmarshal(data, cfg); err != nil {
		return nil, fmt.Errorf("parse server config: %w", err)
	}
	return cfg, nil
}

func SaveServerConfig(cfg *Config, path string) error {
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

func DefaultServerConfigPath() string {
	switch runtime.GOOS {
	case "darwin":
		if home, err := os.UserHomeDir(); err == nil {
			return filepath.Join(home, "Library", "Application Support", "MachineMon", "server.toml")
		}
	default:
		if home, err := os.UserHomeDir(); err == nil {
			return filepath.Join(home, ".config", "machinemon", "server.toml")
		}
	}
	return "/etc/machinemon/server.toml"
}

func defaultDatabasePath() string {
	switch runtime.GOOS {
	case "darwin":
		if home, err := os.UserHomeDir(); err == nil {
			return filepath.Join(home, "Library", "Application Support", "MachineMon", "machinemon.db")
		}
	default:
		if home, err := os.UserHomeDir(); err == nil {
			return filepath.Join(home, ".local", "share", "machinemon", "machinemon.db")
		}
	}
	return "/var/lib/machinemon/machinemon.db"
}

func defaultBinariesDir() string {
	switch runtime.GOOS {
	case "darwin":
		if home, err := os.UserHomeDir(); err == nil {
			return filepath.Join(home, "Library", "Application Support", "MachineMon", "binaries")
		}
	default:
		if home, err := os.UserHomeDir(); err == nil {
			return filepath.Join(home, ".local", "share", "machinemon", "binaries")
		}
	}
	return "/var/lib/machinemon/binaries"
}

func defaultCertCacheDir() string {
	switch runtime.GOOS {
	case "darwin":
		if home, err := os.UserHomeDir(); err == nil {
			return filepath.Join(home, "Library", "Application Support", "MachineMon", "certs")
		}
	default:
		if home, err := os.UserHomeDir(); err == nil {
			return filepath.Join(home, ".local", "share", "machinemon", "certs")
		}
	}
	return "/var/lib/machinemon/certs"
}
