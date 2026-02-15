package wizard

import (
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/charmbracelet/huh"
	"github.com/machinemon/machinemon/internal/client"
)

// Run executes the interactive setup wizard and returns an updated config.
func Run(existingConfig *client.Config) (*client.Config, error) {
	cfg := existingConfig
	if cfg == nil {
		cfg = client.DefaultConfig()
	}

	fmt.Println()
	fmt.Println("  ╔══════════════════════════════════════╗")
	fmt.Println("  ║       MachineMon Client Setup        ║")
	fmt.Println("  ╚══════════════════════════════════════╝")
	fmt.Println()

	if cfg.IsConfigured() {
		fmt.Println("  Existing configuration detected.")
		fmt.Println()
	}

	for {
		action, err := runSetupMenu(cfg)
		if err != nil {
			return nil, err
		}
		switch action {
		case "server":
			if err := runServerForm(cfg); err != nil {
				return nil, fmt.Errorf("server setup: %w", err)
			}
		case "processes":
			if err := runProcessPicker(cfg); err != nil {
				return nil, fmt.Errorf("process picker: %w", err)
			}
		case "save":
			if !cfg.IsConfigured() {
				fmt.Println("  Server URL and client password are required before saving.")
				fmt.Println()
				continue
			}
			confirmed, err := runSummary(cfg)
			if err != nil {
				return nil, fmt.Errorf("summary: %w", err)
			}
			if confirmed {
				return cfg, nil
			}
		case "cancel":
			return nil, fmt.Errorf("setup cancelled by user")
		}
	}
}

func runSetupMenu(cfg *client.Config) (string, error) {
	serverLabel := cfg.ServerURL
	if strings.TrimSpace(serverLabel) == "" {
		serverLabel = "<not set>"
	}
	procLabel := fmt.Sprintf("%d process(es)", len(cfg.Processes))

	var action string
	form := huh.NewForm(
		huh.NewGroup(
			huh.NewSelect[string]().
				Title("Setup menu").
				Description(fmt.Sprintf("Server: %s | Monitored: %s", truncate(serverLabel, 36), procLabel)).
				Options(
					huh.NewOption("Configure server settings", "server"),
					huh.NewOption("Configure monitored processes", "processes"),
					huh.NewOption("Save and exit", "save"),
					huh.NewOption("Cancel setup", "cancel"),
				).
				Value(&action),
		),
	)
	if err := form.Run(); err != nil {
		return "", err
	}
	return action, nil
}

func runServerForm(cfg *client.Config) error {
	serverURL := cfg.ServerURL
	password := cfg.Password
	insecure := cfg.InsecureSkipTLS

	form := huh.NewForm(
		huh.NewGroup(
			huh.NewInput().
				Title("Collector URL").
				Description("The URL of your MachineMon server").
				Placeholder("https://monitor.example.com").
				Value(&serverURL),
			huh.NewInput().
				Title("Client Password").
				Description("The shared password configured on the server").
				EchoMode(huh.EchoModePassword).
				Value(&password),
			huh.NewConfirm().
				Title("Allow self-signed certificates?").
				Description("Enable if your server uses a self-signed TLS certificate").
				Value(&insecure),
		),
	)

	if err := form.Run(); err != nil {
		return err
	}

	// Normalize URL
	serverURL = strings.TrimRight(serverURL, "/")
	if !strings.HasPrefix(serverURL, "http://") && !strings.HasPrefix(serverURL, "https://") {
		serverURL = "https://" + serverURL
	}

	// Test connection
	fmt.Printf("\n  Testing connection to %s... ", serverURL)
	if err := testConnection(serverURL, insecure); err != nil {
		fmt.Printf("FAILED\n")
		fmt.Printf("  Error: %s\n\n", err)

		var retry bool
		retryForm := huh.NewForm(
			huh.NewGroup(
				huh.NewConfirm().
					Title("Connection failed. Continue anyway?").
					Value(&retry),
			),
		)
		if err := retryForm.Run(); err != nil {
			return err
		}
		if !retry {
			return fmt.Errorf("connection test failed")
		}
	} else {
		fmt.Printf("OK\n\n")
	}

	cfg.ServerURL = serverURL
	cfg.Password = password
	cfg.InsecureSkipTLS = insecure
	return nil
}

func testConnection(serverURL string, insecure bool) error {
	httpClient := &http.Client{Timeout: 10 * time.Second}
	if insecure {
		httpClient.Transport = &http.Transport{
			TLSClientConfig: nil, // will be set properly in reporter
		}
	}
	resp, err := httpClient.Get(serverURL + "/healthz")
	if err != nil {
		return err
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("server returned status %d", resp.StatusCode)
	}
	return nil
}
