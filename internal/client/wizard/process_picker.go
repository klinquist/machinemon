package wizard

import (
	"fmt"
	"strconv"
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
		options = append(options, huh.NewOption("Back to setup menu", "done"))

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

	removed := 0
	for len(cfg.Processes) > 0 {
		options := make([]huh.Option[string], 0, len(cfg.Processes)+1)
		options = append(options, huh.NewOption("< Back to process menu >", "back"))
		for i, p := range cfg.Processes {
			label := fmt.Sprintf("%s (%s)", p.FriendlyName, truncate(p.MatchPattern, 50))
			options = append(options, huh.NewOption(label, "proc:"+strconv.Itoa(i)))
		}

		var choice string
		removeForm := huh.NewForm(
			huh.NewGroup(
				huh.NewSelect[string]().
					Title("Select one process to stop monitoring").
					Description("Type to filter. Enter to select.").
					Filtering(true).
					Height(14).
					Options(options...).
					Value(&choice),
			),
		)
		if err := removeForm.Run(); err != nil {
			return err
		}
		if choice == "back" {
			break
		}
		if !strings.HasPrefix(choice, "proc:") {
			fmt.Println("  Invalid selection.")
			fmt.Println()
			continue
		}
		choice = strings.TrimPrefix(choice, "proc:")

		idx, err := strconv.Atoi(choice)
		if err != nil || idx < 0 || idx >= len(cfg.Processes) {
			fmt.Println("  Invalid selection.")
			fmt.Println()
			continue
		}

		removedProc := cfg.Processes[idx]
		cfg.Processes = append(cfg.Processes[:idx], cfg.Processes[idx+1:]...)
		removed++
		fmt.Printf("  Removed: %s\n\n", removedProc.FriendlyName)
	}

	if removed > 0 {
		fmt.Printf("  Removed %d process(es).\n\n", removed)
	} else {
		fmt.Println("  No processes removed.")
		fmt.Println()
	}
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

	existingNames := make(map[string]bool, len(cfg.Processes))
	for _, p := range cfg.Processes {
		existingNames[strings.ToLower(strings.TrimSpace(p.FriendlyName))] = true
	}

	added := 0
	for {
		options := make([]huh.Option[string], 0, len(candidates)+1)
		options = append(options, huh.NewOption("< Back to process menu >", "back"))
		for i, c := range candidates {
			display := c.Cmdline
			if len(display) > 80 {
				display = display[:77] + "..."
			}
			label := fmt.Sprintf("[%d] %s", c.PID, display)
			options = append(options, huh.NewOption(label, "proc:"+strconv.Itoa(i)))
		}

		var choice string
		selectForm := huh.NewForm(
			huh.NewGroup(
				huh.NewSelect[string]().
					Title("Select one running process to add").
					Description("Type to filter. Enter to select.").
					Filtering(true).
					Height(16).
					Options(options...).
					Value(&choice),
			),
		)
		if err := selectForm.Run(); err != nil {
			return err
		}
		if choice == "back" {
			break
		}
		if !strings.HasPrefix(choice, "proc:") {
			fmt.Println("  Invalid selection.")
			fmt.Println()
			continue
		}
		choice = strings.TrimPrefix(choice, "proc:")

		idx, err := strconv.Atoi(choice)
		if err != nil || idx < 0 || idx >= len(candidates) {
			fmt.Println("  Invalid selection.")
			fmt.Println()
			continue
		}

		c := candidates[idx]
		suggestedName := client.SuggestFriendlyName(c)
		matchPattern := client.SuggestMatchPattern(c)
		if isAlreadyMonitored(cfg.Processes, matchPattern) {
			fmt.Printf("  Already monitored: %s\n\n", matchPattern)
			continue
		}

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

		cfg.Processes = append(cfg.Processes, client.ProcessConfig{
			FriendlyName: friendlyName,
			MatchPattern: matchPattern,
			MatchType:    "substring",
		})
		added++
		fmt.Printf("  Added: %s (%s)\n\n", friendlyName, matchPattern)

		var addMore bool
		addMoreForm := huh.NewForm(
			huh.NewGroup(
				huh.NewConfirm().
					Title("Add another process?").
					Value(&addMore),
			),
		)
		if err := addMoreForm.Run(); err != nil {
			return err
		}
		if !addMore {
			break
		}
	}

	if added > 0 {
		fmt.Printf("  Added %d process(es).\n\n", added)
	} else {
		fmt.Println("  No processes added.")
		fmt.Println()
	}
	return nil
}

func isAlreadyMonitored(processes []client.ProcessConfig, matchPattern string) bool {
	for _, p := range processes {
		if normalizeMatchType(p.MatchType) == "substring" && p.MatchPattern == matchPattern {
			return true
		}
	}
	return false
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
