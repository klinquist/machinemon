package main

import (
	"flag"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"runtime"

	"github.com/charmbracelet/huh"
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

	if runtime.GOOS == "darwin" && os.Getuid() == 0 {
		if sudoUser := os.Getenv("SUDO_USER"); sudoUser != "" {
			fmt.Fprintf(os.Stderr, "Warning: running with sudo on macOS. launchd services should normally be installed as the logged-in user (%s).\n", sudoUser)
		} else {
			fmt.Fprintln(os.Stderr, "Warning: running as root on macOS. launchd services should normally be installed as a non-root user.")
		}
	}

	if *versionFlag {
		fmt.Println(version.String())
		os.Exit(0)
	}

	if *serviceInstall {
		binPath, _ := os.Executable()
		cfgAbs, _ := filepath.Abs(*configPath)
		if err := service.Install("machinemon-client", binPath, cfgAbs); err != nil {
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

		// Do not launch interactive daemon after setup.
		// If service is already running, offer restart so it picks up new config.
		running := false
		if isRunning, err := service.IsRunning("machinemon-client"); err != nil {
			logger.Warn("could not determine service status", "err", err)
		} else {
			running = isRunning
		}

		if running {
			var restartService bool
			form := huh.NewForm(
				huh.NewGroup(
					huh.NewConfirm().
						Title("MachineMon client service is already running. Restart it now to apply this configuration?").
						Value(&restartService),
				),
			)
			if err := form.Run(); err != nil {
				logger.Error("prompt failed", "err", err)
				os.Exit(1)
			}
			if restartService {
				if err := service.Restart("machinemon-client"); err != nil {
					logger.Error("failed to restart service", "err", err)
					os.Exit(1)
				}
				logger.Info("service restarted", "service", "machinemon-client")
			}
			return
		}

		var installAndStart bool
		form := huh.NewForm(
			huh.NewGroup(
				huh.NewConfirm().
					Title("MachineMon client service is not running. Install/update service and start it now?").
					Value(&installAndStart),
			),
		)
		if err := form.Run(); err != nil {
			logger.Error("prompt failed", "err", err)
			os.Exit(1)
		}

		if installAndStart {
			binPath, _ := os.Executable()
			cfgAbs, _ := filepath.Abs(*configPath)

			if err := service.Install("machinemon-client", binPath, cfgAbs); err != nil {
				logger.Error("failed to install service", "err", err)
				os.Exit(1)
			}
			if err := service.Start("machinemon-client"); err != nil {
				logger.Error("failed to start service", "err", err)
				os.Exit(1)
			}
			logger.Info("service installed and started", "service", "machinemon-client")
			return
		}

		printServiceNextSteps()
		return
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

func printServiceNextSteps() {
	fmt.Println()
	fmt.Println("Configuration saved.")
	fmt.Println("Run the client as a service:")

	switch service.Detect() {
	case service.Systemd:
		fmt.Println("  sudo machinemon-client --service-install")
		fmt.Println("Then start it:")
		fmt.Println("  sudo systemctl enable --now machinemon-client")
	case service.OpenRC:
		fmt.Println("  sudo machinemon-client --service-install")
		fmt.Println("Then start and enable it:")
		fmt.Println("  sudo rc-service machinemon-client start")
		fmt.Println("  sudo rc-update add machinemon-client default")
	case service.SysVInit:
		fmt.Println("  sudo machinemon-client --service-install")
		fmt.Println("Then start it:")
		fmt.Println("  sudo service machinemon-client start")
	case service.Upstart:
		fmt.Println("  sudo machinemon-client --service-install")
		fmt.Println("Then start it:")
		fmt.Println("  sudo start machinemon-client")
	case service.Launchd:
		fmt.Println("  machinemon-client --service-install")
		fmt.Println("Then start it:")
		fmt.Println("  launchctl bootstrap gui/$(id -u) ~/Library/LaunchAgents/com.machinemon.client.plist")
		fmt.Println("  launchctl kickstart -k gui/$(id -u)/com.machinemon.client")
	default:
		fmt.Println("  machinemon-client --service-install")
	}
}
