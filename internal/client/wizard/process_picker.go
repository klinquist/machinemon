package wizard

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/huh"
	"github.com/machinemon/machinemon/internal/client"
)

func runProcessPicker(cfg *client.Config) error {
	var wantProcesses bool
	form := huh.NewForm(
		huh.NewGroup(
			huh.NewConfirm().
				Title("Would you like to monitor specific processes?").
				Description("You can select from currently running processes").
				Value(&wantProcesses),
		),
	)
	if err := form.Run(); err != nil {
		return err
	}
	if !wantProcesses {
		return nil
	}

	fmt.Println("  Scanning running processes...")
	candidates, err := client.ListProcessCandidates()
	if err != nil {
		return fmt.Errorf("list processes: %w", err)
	}

	if len(candidates) == 0 {
		fmt.Println("  No suitable processes found.")
		return nil
	}

	// Build options for multi-select
	options := make([]huh.Option[int], 0, len(candidates))
	for i, c := range candidates {
		// Truncate cmdline for display
		display := c.Cmdline
		if len(display) > 80 {
			display = display[:77] + "..."
		}
		label := fmt.Sprintf("[%d] %s", c.PID, display)
		options = append(options, huh.NewOption(label, i))
	}

	var selected []int
	selectForm := huh.NewForm(
		huh.NewGroup(
			huh.NewMultiSelect[int]().
				Title("Select processes to monitor").
				Description("Use space to select, enter to confirm").
				Options(options...).
				Value(&selected),
		),
	)
	if err := selectForm.Run(); err != nil {
		return err
	}

	if len(selected) == 0 {
		fmt.Println("  No processes selected.")
		return nil
	}

	// For each selected process, ask for a friendly name
	var processes []client.ProcessConfig
	for _, idx := range selected {
		c := candidates[idx]
		suggestedName := client.SuggestFriendlyName(c)
		matchPattern := client.SuggestMatchPattern(c)

		friendlyName := suggestedName

		nameForm := huh.NewForm(
			huh.NewGroup(
				huh.NewInput().
					Title(fmt.Sprintf("Friendly name for: %s", truncate(c.Cmdline, 60))).
					Description(fmt.Sprintf("Match pattern: %s", matchPattern)).
					Value(&friendlyName),
			),
		)
		if err := nameForm.Run(); err != nil {
			return err
		}

		if friendlyName == "" {
			friendlyName = suggestedName
		}

		processes = append(processes, client.ProcessConfig{
			FriendlyName: friendlyName,
			MatchPattern: matchPattern,
			MatchType:    "substring",
		})
	}

	cfg.Processes = processes
	return nil
}

func truncate(s string, maxLen int) string {
	s = strings.TrimSpace(s)
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen-3] + "..."
}
