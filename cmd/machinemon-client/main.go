package main

import (
	"flag"
	"fmt"
	"log/slog"
	"os"

	"github.com/machinemon/machinemon/internal/client"
	"github.com/machinemon/machinemon/internal/client/wizard"
	"github.com/machinemon/machinemon/internal/service"
	"github.com/machinemon/machinemon/internal/version"
)

func main() {
	configPath := flag.String("config", client.DefaultConfigPath(), "path to config file")
	setup := flag.Bool("setup", false, "run interactive setup wizard")
	serverURL := flag.String("server", "", "collector URL (non-interactive setup)")
	password := flag.String("password", "", "client password (non-interactive setup)")
	noDaemon := flag.Bool("no-daemon", false, "exit after setup, don't run daemon")
	insecure := flag.Bool("insecure", false, "allow self-signed TLS certificates")
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
		if err := service.Install("machinemon-client", binPath); err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}
		os.Exit(0)
	}
	if *serviceUninstall {
		if err := service.Uninstall("machinemon-client"); err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}
		os.Exit(0)
	}

	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{
		Level: slog.LevelInfo,
	}))

	cfg, err := client.LoadConfig(*configPath)
	if err != nil {
		logger.Error("failed to load config", "path", *configPath, "err", err)
		os.Exit(1)
	}

	// Apply CLI overrides
	if *serverURL != "" {
		cfg.ServerURL = *serverURL
	}
	if *password != "" {
		cfg.Password = *password
	}
	if *insecure {
		cfg.InsecureSkipTLS = true
	}

	if *setup {
		updatedCfg, err := wizard.Run(cfg)
		if err != nil {
			logger.Error("setup wizard failed", "err", err)
			os.Exit(1)
		}
		cfg = updatedCfg
		if err := client.SaveConfig(cfg, *configPath); err != nil {
			logger.Error("failed to save config", "err", err)
			os.Exit(1)
		}
		logger.Info("config saved", "path", *configPath)
	}

	if !cfg.IsConfigured() {
		fmt.Println("MachineMon Client is not configured.")
		fmt.Println("Run with --setup for interactive setup, or provide --server and --password flags.")
		os.Exit(1)
	}

	// Save config (in case CLI flags updated it)
	if *serverURL != "" || *password != "" || *insecure {
		if err := client.SaveConfig(cfg, *configPath); err != nil {
			logger.Error("failed to save config", "err", err)
		} else {
			logger.Info("config saved", "path", *configPath)
		}
	}

	if *noDaemon {
		return
	}

	client.RunDaemon(cfg, *configPath, logger)
}
