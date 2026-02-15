package wizard

import (
	"fmt"

	"github.com/charmbracelet/huh"
	"github.com/machinemon/machinemon/internal/client"
)

func runSummary(cfg *client.Config) (bool, error) {
	fmt.Println()
	fmt.Println("  ┌─────────────── Summary ───────────────┐")
	fmt.Printf("  │ Server:  %-29s │\n", truncate(cfg.ServerURL, 29))
	fmt.Printf("  │ Password: %-28s │\n", "********")
	fmt.Printf("  │ TLS Skip: %-28v │\n", cfg.InsecureSkipTLS)
	fmt.Printf("  │ Interval: %-28s │\n", fmt.Sprintf("%d seconds", cfg.CheckInInterval))
	fmt.Printf("  │ Processes: %-27d │\n", len(cfg.Processes))
	fmt.Printf("  │ Script checks: %-23d │\n", scriptCheckCount(cfg.Checks))

	for _, p := range cfg.Processes {
		fmt.Printf("  │   - %-33s │\n", truncate(p.FriendlyName, 33))
	}
	for _, check := range scriptCheckEntries(cfg.Checks) {
		display := check.Check.FriendlyName
		if check.Check.RunAsUser != "" {
			display += " as " + check.Check.RunAsUser
		}
		fmt.Printf("  │   * %-33s │\n", truncate(display, 33))
	}

	fmt.Println("  └────────────────────────────────────────┘")
	fmt.Println()

	var confirmed bool
	form := huh.NewForm(
		huh.NewGroup(
			huh.NewConfirm().
				Title("Save this configuration?").
				Value(&confirmed),
		),
	)
	if err := form.Run(); err != nil {
		return false, err
	}
	return confirmed, nil
}
