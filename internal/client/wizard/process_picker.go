package wizard

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/huh"
	"github.com/machinemon/machinemon/internal/client"
)

func runProcessPicker(cfg *client.Config) error {
	if len(cfg.Processes) > 0 {
		fmt.Println("  Currently monitored processes:")
		for _, p := range cfg.Processes {
			fmt.Printf("    - %s (%s)\n", p.FriendlyName, p.MatchPattern)
		}
		fmt.Println()
	}

	var manageProcesses bool
	title := "Would you like to monitor specific processes?"
	description := "You can select from currently running processes."
	if len(cfg.Processes) > 0 {
		title = "Would you like to update monitored processes?"
		description = "You can remove existing ones and/or add new running processes."
	}
	form := huh.NewForm(
		huh.NewGroup(
			huh.NewConfirm().
				Title(title).
				Description(description).
				Value(&manageProcesses),
		),
	)
	if err := form.Run(); err != nil {
		return err
	}
	if !manageProcesses {
		return nil
	}

	if err := maybeRemoveProcesses(cfg); err != nil {
		return err
	}
	if err := maybeAddProcesses(cfg); err != nil {
		return err
	}

	return nil
}

func maybeRemoveProcesses(cfg *client.Config) error {
	if len(cfg.Processes) == 0 {
		return nil
	}

	var removeAny bool
	form := huh.NewForm(
		huh.NewGroup(
			huh.NewConfirm().
				Title("Stop monitoring any existing processes?").
				Value(&removeAny),
		),
	)
	if err := form.Run(); err != nil {
		return err
	}
	if !removeAny {
		return nil
	}

	options := make([]huh.Option[int], 0, len(cfg.Processes))
	for i, p := range cfg.Processes {
		label := fmt.Sprintf("%s (%s)", p.FriendlyName, truncate(p.MatchPattern, 50))
		options = append(options, huh.NewOption(label, i))
	}

	var selected []int
	removeForm := huh.NewForm(
		huh.NewGroup(
			huh.NewMultiSelect[int]().
				Title("Select processes to stop monitoring").
				Description("Use space to select, enter to confirm").
				Options(options...).
				Value(&selected),
		),
	)
	if err := removeForm.Run(); err != nil {
		return err
	}
	if len(selected) == 0 {
		return nil
	}

	removeIdx := make(map[int]bool, len(selected))
	for _, idx := range selected {
		removeIdx[idx] = true
	}

	kept := make([]client.ProcessConfig, 0, len(cfg.Processes)-len(selected))
	for i, p := range cfg.Processes {
		if !removeIdx[i] {
			kept = append(kept, p)
		}
	}
	cfg.Processes = kept
	return nil
}

func maybeAddProcesses(cfg *client.Config) error {
	var addAny bool
	form := huh.NewForm(
		huh.NewGroup(
			huh.NewConfirm().
				Title("Add new running processes to monitor?").
				Value(&addAny),
		),
	)
	if err := form.Run(); err != nil {
		return err
	}
	if !addAny {
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

	existingPatterns := make(map[string]bool, len(cfg.Processes))
	for _, p := range cfg.Processes {
		key := p.MatchType + "|" + p.MatchPattern
		existingPatterns[key] = true
	}

	type indexedCandidate struct {
		candidate client.ProcessCandidate
	}
	var filtered []indexedCandidate
	for _, c := range candidates {
		matchPattern := client.SuggestMatchPattern(c)
		key := "substring|" + matchPattern
		if existingPatterns[key] {
			continue
		}
		filtered = append(filtered, indexedCandidate{candidate: c})
	}

	if len(filtered) == 0 {
		fmt.Println("  No additional processes to add.")
		return nil
	}

	options := make([]huh.Option[int], 0, len(filtered))
	for i, entry := range filtered {
		display := entry.candidate.Cmdline
		if len(display) > 80 {
			display = display[:77] + "..."
		}
		label := fmt.Sprintf("[%d] %s", entry.candidate.PID, display)
		options = append(options, huh.NewOption(label, i))
	}
	var selected []int
	selectForm := huh.NewForm(
		huh.NewGroup(
			huh.NewMultiSelect[int]().
				Title("Select running processes to add").
				Description("Use space to select, enter to confirm").
				Options(options...).
				Value(&selected),
		),
	)
	if err := selectForm.Run(); err != nil {
		return err
	}

	if len(selected) == 0 {
		return nil
	}

	existingNames := make(map[string]bool, len(cfg.Processes))
	for _, p := range cfg.Processes {
		existingNames[strings.ToLower(strings.TrimSpace(p.FriendlyName))] = true
	}

	additions := make([]client.ProcessConfig, 0, len(selected))
	for _, idx := range selected {
		c := filtered[idx].candidate
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
		friendlyName = uniqueFriendlyName(friendlyName, existingNames)

		additions = append(additions, client.ProcessConfig{
			FriendlyName: friendlyName,
			MatchPattern: matchPattern,
			MatchType:    "substring",
		})
	}

	cfg.Processes = append(cfg.Processes, additions...)
	return nil
}

func uniqueFriendlyName(base string, existing map[string]bool) string {
	base = strings.TrimSpace(base)
	if base == "" {
		base = "process"
	}
	name := base
	if !existing[strings.ToLower(name)] {
		existing[strings.ToLower(name)] = true
		return name
	}
	for i := 2; ; i++ {
		candidate := fmt.Sprintf("%s-%d", base, i)
		key := strings.ToLower(candidate)
		if !existing[key] {
			existing[key] = true
			return candidate
		}
	}
}

func truncate(s string, maxLen int) string {
	s = strings.TrimSpace(s)
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen-3] + "..."
}
