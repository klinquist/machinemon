package wizard

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/huh"
	"github.com/machinemon/machinemon/internal/client"
)

func runProcessPicker(cfg *client.Config) error {
	for {
		printMonitoredProcessTable(cfg.Processes)

		options := []huh.Option[string]{
			huh.NewOption("Add process to monitor", "add"),
		}
		if len(cfg.Processes) > 0 {
			options = append(options, huh.NewOption("Stop monitoring existing process(es)", "remove"))
		}
		options = append(options, huh.NewOption("Continue setup", "done"))

		var action string
		form := huh.NewForm(
			huh.NewGroup(
				huh.NewSelect[string]().
					Title("Process monitoring").
					Description("Choose an action").
					Options(options...).
					Value(&action),
			),
		)
		if err := form.Run(); err != nil {
			return err
		}

		switch action {
		case "add":
			if err := maybeAddProcesses(cfg); err != nil {
				return err
			}
		case "remove":
			if err := maybeRemoveProcesses(cfg); err != nil {
				return err
			}
		default:
			return nil
		}
	}
}

func maybeRemoveProcesses(cfg *client.Config) error {
	if len(cfg.Processes) == 0 {
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
	fmt.Printf("  Removed %d process(es).\n\n", len(selected))
	return nil
}

func maybeAddProcesses(cfg *client.Config) error {
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
		key := normalizeMatchType(p.MatchType) + "|" + p.MatchPattern
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
	fmt.Printf("  Added %d process(es).\n\n", len(additions))
	return nil
}

func printMonitoredProcessTable(processes []client.ProcessConfig) {
	const (
		nameWidth    = 24
		typeWidth    = 10
		patternWidth = 34
	)

	fmt.Println("  Currently monitored processes:")
	border := fmt.Sprintf("  +----+-%s-+-%s-+-%s-+",
		strings.Repeat("-", nameWidth),
		strings.Repeat("-", typeWidth),
		strings.Repeat("-", patternWidth),
	)
	fmt.Println(border)
	fmt.Printf("  | %-2s | %-*s | %-*s | %-*s |\n",
		"#", nameWidth, "Friendly Name", typeWidth, "Type", patternWidth, "Match Pattern")
	fmt.Println(border)

	if len(processes) == 0 {
		fmt.Printf("  | %-2s | %-*s | %-*s | %-*s |\n", "", nameWidth, "<none>", typeWidth, "", patternWidth, "")
		fmt.Println(border)
		fmt.Println()
		return
	}

	for i, p := range processes {
		matchType := normalizeMatchType(p.MatchType)
		fmt.Printf("  | %2d | %-*s | %-*s | %-*s |\n",
			i+1,
			nameWidth, truncate(p.FriendlyName, nameWidth),
			typeWidth, truncate(matchType, typeWidth),
			patternWidth, truncate(p.MatchPattern, patternWidth),
		)
	}
	fmt.Println(border)
	fmt.Println()
}

func normalizeMatchType(matchType string) string {
	matchType = strings.TrimSpace(matchType)
	if matchType == "" {
		return "substring"
	}
	return matchType
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
