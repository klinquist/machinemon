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

	// Step 1: Server connection
	if err := runServerForm(cfg); err != nil {
		return nil, fmt.Errorf("server setup: %w", err)
	}

	// Step 2: Process picker
	if err := runProcessPicker(cfg); err != nil {
		return nil, fmt.Errorf("process picker: %w", err)
	}

	// Step 3: Summary and confirm
	if err := runSummary(cfg); err != nil {
		return nil, fmt.Errorf("summary: %w", err)
	}

	return cfg, nil
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
