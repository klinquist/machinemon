package main

import (
	"context"
	"embed"
	"flag"
	"fmt"
	"io/fs"
	"log/slog"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"

	"github.com/machinemon/machinemon/internal/alerting"
	"github.com/machinemon/machinemon/internal/server"
	"github.com/machinemon/machinemon/internal/service"
	"github.com/machinemon/machinemon/internal/store"
	"github.com/machinemon/machinemon/internal/version"
)

//go:embed web_dist
var webDistEmbed embed.FS

func main() {
	configPath := flag.String("config", server.DefaultServerConfigPath(), "path to config file")
	setup := flag.Bool("setup", false, "run initial setup")
	serviceInstall := flag.Bool("service-install", false, "install as a system service (auto-detects init system)")
	serviceUninstall := flag.Bool("service-uninstall", false, "remove the system service")
	versionFlag := flag.Bool("version", false, "print version and exit")
	flag.Parse()

	if *versionFlag {
		fmt.Println(version.String())
		os.Exit(0)
	}

	if *serviceInstall {
		binPath, _ := os.Executable()
		cfgAbs, _ := filepath.Abs(*configPath)
		if err := service.Install("machinemon-server", binPath, cfgAbs); err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}
		os.Exit(0)
	}
	if *serviceUninstall {
		if err := service.Uninstall("machinemon-server"); err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}
		os.Exit(0)
	}

	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{
		Level: slog.LevelInfo,
	}))

	cfg, err := server.LoadServerConfig(*configPath)
	if err != nil {
		logger.Error("failed to load config", "path", *configPath, "err", err)
		os.Exit(1)
	}

	if *setup || cfg.AdminPasswordHash == "" {
		if err := runSetup(cfg, *configPath); err != nil {
			logger.Error("setup failed", "err", err)
			os.Exit(1)
		}
	}

	// Ensure database directory exists
	dbDir := filepath.Dir(cfg.DatabasePath)
	if err := os.MkdirAll(dbDir, 0750); err != nil {
		logger.Error("failed to create database directory", "path", dbDir, "err", err)
		os.Exit(1)
	}

	st, err := store.NewSQLiteStore(cfg.DatabasePath)
	if err != nil {
		logger.Error("failed to open database", "path", cfg.DatabasePath, "err", err)
		os.Exit(1)
	}
	defer st.Close()

	logger.Info("database ready", "path", cfg.DatabasePath)

	// Set up embedded web filesystem
	webFS, err := fs.Sub(webDistEmbed, "web_dist")
	if err != nil {
		logger.Error("failed to set up embedded web filesystem", "err", err)
		os.Exit(1)
	}
	server.SetWebFS(webFS)

	// Start alert engine
	alertEngine := alerting.NewEngine(st, logger)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go alertEngine.Run(ctx)

	srv := server.New(cfg, st, alertEngine, logger)

	logger.Info("MachineMon Server starting",
		"version", version.Version,
		"addr", cfg.ListenAddr,
		"tls", cfg.TLSMode)

	// Graceful shutdown on SIGTERM/SIGINT
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGTERM, syscall.SIGINT)

	errCh := make(chan error, 1)
	go func() {
		errCh <- srv.ListenAndServeTLS()
	}()

	select {
	case sig := <-sigCh:
		logger.Info("received signal, shutting down gracefully", "signal", sig)
		cancel() // Stop alert engine
		logger.Info("server stopped")
	case err := <-errCh:
		if err != nil {
			logger.Error("server error", "err", err)
			cancel()
			os.Exit(1)
		}
	}
}

func runSetup(cfg *server.Config, configPath string) error {
	fmt.Println("=== MachineMon Server Setup ===")
	fmt.Println()

	// Admin password
	if cfg.AdminPasswordHash == "" {
		fmt.Print("Set admin password: ")
		var pw string
		fmt.Scanln(&pw)
		if pw == "" {
			return fmt.Errorf("admin password is required")
		}
		hash, err := server.HashPassword(pw)
		if err != nil {
			return fmt.Errorf("hash password: %w", err)
		}
		cfg.AdminPasswordHash = hash
	}

	// Client password
	if cfg.ClientPasswordHash == "" {
		fmt.Print("Set client password (used by all monitoring clients): ")
		var pw string
		fmt.Scanln(&pw)
		if pw == "" {
			return fmt.Errorf("client password is required")
		}
		hash, err := server.HashPassword(pw)
		if err != nil {
			return fmt.Errorf("hash password: %w", err)
		}
		cfg.ClientPasswordHash = hash
	}

	// TLS mode
	fmt.Println()
	fmt.Println("TLS mode options:")
	fmt.Println("  1. none     - HTTP only (use with reverse proxy like nginx)")
	fmt.Println("  2. autocert - Let's Encrypt automatic HTTPS")
	fmt.Println("  3. selfsigned - Generate self-signed certificate")
	fmt.Print("Choose TLS mode [1]: ")
	var tlsChoice string
	fmt.Scanln(&tlsChoice)
	switch tlsChoice {
	case "2", "autocert":
		cfg.TLSMode = "autocert"
		fmt.Print("Domain name for HTTPS certificate: ")
		fmt.Scanln(&cfg.Domain)
		if cfg.Domain == "" {
			return fmt.Errorf("domain is required for autocert")
		}
		cfg.ListenAddr = ":443"
		cfg.ExternalURL = "https://" + cfg.Domain
	case "3", "selfsigned":
		cfg.TLSMode = "selfsigned"
		fmt.Print("Listen address [0.0.0.0:8443]: ")
		var addr string
		fmt.Scanln(&addr)
		if addr != "" {
			cfg.ListenAddr = addr
		} else {
			cfg.ListenAddr = "0.0.0.0:8443"
		}
	default:
		cfg.TLSMode = "none"
		fmt.Print("Listen address [0.0.0.0:8080]: ")
		var addr string
		fmt.Scanln(&addr)
		if addr != "" {
			cfg.ListenAddr = addr
		} else {
			cfg.ListenAddr = "0.0.0.0:8080"
		}
		fmt.Println()
		fmt.Println("Since you're running behind a reverse proxy, what is the public URL")
		fmt.Println("that clients and browsers will use to reach this server?")
		fmt.Print("External URL (e.g. https://monitor.example.com): ")
		var extURL string
		fmt.Scanln(&extURL)
		if extURL != "" {
			cfg.ExternalURL = extURL
		}
	}

	if err := server.SaveServerConfig(cfg, configPath); err != nil {
		return fmt.Errorf("save config: %w", err)
	}
	fmt.Println()
	fmt.Printf("Config saved to %s\n", configPath)

	if cfg.TLSMode == "none" {
		// Extract port from listen address for nginx example
		proxyAddr := cfg.ListenAddr
		if strings.Contains(proxyAddr, "0.0.0.0") {
			port := proxyAddr[strings.LastIndex(proxyAddr, ":"):]
			proxyAddr = "127.0.0.1" + port
		} else if strings.HasPrefix(proxyAddr, ":") {
			proxyAddr = "127.0.0.1" + proxyAddr
		}

		fmt.Println()
		fmt.Println("Running in HTTP mode. For HTTPS, put behind a reverse proxy.")
		fmt.Println("Example nginx config:")
		fmt.Println()
		fmt.Println("  server {")
		fmt.Println("    listen 443 ssl;")
		fmt.Println("    server_name monitor.example.com;")
		fmt.Println("    ssl_certificate /etc/letsencrypt/live/monitor.example.com/fullchain.pem;")
		fmt.Println("    ssl_certificate_key /etc/letsencrypt/live/monitor.example.com/privkey.pem;")
		fmt.Println("    location / {")
		fmt.Printf("      proxy_pass http://%s;\n", proxyAddr)
		fmt.Println("      proxy_set_header Host $host;")
		fmt.Println("      proxy_set_header X-Real-IP $remote_addr;")
		fmt.Println("      proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;")
		fmt.Println("      proxy_set_header X-Forwarded-Proto $scheme;")
		fmt.Println("    }")
		fmt.Println("  }")
	}

	return nil
}
