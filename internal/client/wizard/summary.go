package wizard

import (
	"fmt"

	"github.com/charmbracelet/huh"
	"github.com/machinemon/machinemon/internal/client"
)

func runSummary(cfg *client.Config) error {
	fmt.Println()
	fmt.Println("  ┌─────────────── Summary ───────────────┐")
	fmt.Printf("  │ Server:  %-29s │\n", truncate(cfg.ServerURL, 29))
	fmt.Printf("  │ Password: %-28s │\n", "********")
	fmt.Printf("  │ TLS Skip: %-28v │\n", cfg.InsecureSkipTLS)
	fmt.Printf("  │ Interval: %-28s │\n", fmt.Sprintf("%d seconds", cfg.CheckInInterval))
	fmt.Printf("  │ Processes: %-27d │\n", len(cfg.Processes))

	for _, p := range cfg.Processes {
		fmt.Printf("  │   - %-33s │\n", truncate(p.FriendlyName, 33))
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
		return err
	}
	if !confirmed {
		return fmt.Errorf("setup cancelled by user")
	}
	return nil
}
