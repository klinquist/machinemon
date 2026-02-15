package wizard

import (
	"fmt"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/charmbracelet/huh"
	"github.com/machinemon/machinemon/internal/client"
	"github.com/machinemon/machinemon/internal/models"
)

func runScriptCheckPicker(cfg *client.Config) error {
	for {
		printScriptCheckTable(cfg.Checks)

		options := []huh.Option[string]{
			huh.NewOption("Add script check", "add"),
		}
		if scriptCheckCount(cfg.Checks) > 0 {
			options = append(options, huh.NewOption("Delete script check", "remove"))
		}
		options = append(options, huh.NewOption("Back to setup menu", "done"))

		var action string
		form := huh.NewForm(
			huh.NewGroup(
				huh.NewSelect[string]().
					Title("Script checks").
					Description("Add/delete script checks. Exit code 0 = healthy. Exit code 1 = alert.").
					Options(options...).
					Value(&action),
			),
		)
		if err := form.Run(); err != nil {
			return err
		}

		switch action {
		case "add":
			if err := maybeAddScriptChecks(cfg); err != nil {
				return err
			}
		case "remove":
			if err := maybeRemoveScriptChecks(cfg); err != nil {
				return err
			}
		default:
			return nil
		}
	}
}

func maybeAddScriptChecks(cfg *client.Config) error {
	existingNames := make(map[string]bool, len(cfg.Checks))
	for _, c := range cfg.Checks {
		existingNames[strings.ToLower(strings.TrimSpace(c.FriendlyName))] = true
	}

	added := 0
	for {
		var scriptCmd string
		scriptForm := huh.NewForm(
			huh.NewGroup(
				huh.NewInput().
					Title("Shell script command or path").
					Description("Examples: /usr/local/bin/check.sh or curl -sf http://localhost:8080/health").
					Placeholder("/usr/local/bin/check.sh").
					Value(&scriptCmd),
			),
		)
		if err := scriptForm.Run(); err != nil {
			return err
		}
		scriptCmd = strings.TrimSpace(scriptCmd)
		if scriptCmd == "" {
			fmt.Println("  Script command cannot be empty.")
			fmt.Println()
			continue
		}

		suggestedName := suggestScriptCheckName(scriptCmd)
		friendlyName := suggestedName
		runAsUser := ""

		detailsForm := huh.NewForm(
			huh.NewGroup(
				huh.NewInput().
					Title("Friendly name").
					Description("Shown in dashboard and alerts.").
					Value(&friendlyName),
				huh.NewInput().
					Title("Run as user (optional)").
					Description("Leave blank to run as the client service user.").
					Placeholder("www-data").
					Value(&runAsUser),
			),
		)
		if err := detailsForm.Run(); err != nil {
			return err
		}

		friendlyName = uniqueFriendlyName(friendlyName, existingNames)
		cfg.Checks = append(cfg.Checks, client.CheckConfig{
			FriendlyName: friendlyName,
			Type:         models.CheckTypeScript,
			ScriptPath:   scriptCmd,
			RunAsUser:    strings.TrimSpace(runAsUser),
		})
		added++
		fmt.Printf("  Added script check: %s\n\n", friendlyName)

		var addMore bool
		addMoreForm := huh.NewForm(
			huh.NewGroup(
				huh.NewConfirm().
					Title("Add another script check?").
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
		fmt.Printf("  Added %d script check(s).\n\n", added)
	}
	return nil
}

func maybeRemoveScriptChecks(cfg *client.Config) error {
	removed := 0
	for {
		entries := scriptCheckEntries(cfg.Checks)
		if len(entries) == 0 {
			break
		}

		options := make([]huh.Option[string], 0, len(entries)+1)
		options = append(options, huh.NewOption("< Back to script check menu >", "back"))
		for _, entry := range entries {
			runAs := strings.TrimSpace(entry.Check.RunAsUser)
			if runAs == "" {
				runAs = "service user"
			}
			label := fmt.Sprintf("%s [user=%s] (%s)",
				entry.Check.FriendlyName, runAs, truncate(entry.Check.ScriptPath, 42))
			options = append(options, huh.NewOption(label, "check:"+strconv.Itoa(entry.Index)))
		}

		var choice string
		removeForm := huh.NewForm(
			huh.NewGroup(
				huh.NewSelect[string]().
					Title("Select one script check to delete").
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
		if !strings.HasPrefix(choice, "check:") {
			fmt.Println("  Invalid selection.")
			fmt.Println()
			continue
		}
		choice = strings.TrimPrefix(choice, "check:")
		idx, err := strconv.Atoi(choice)
		if err != nil || idx < 0 || idx >= len(cfg.Checks) {
			fmt.Println("  Invalid selection.")
			fmt.Println()
			continue
		}

		removedCheck := cfg.Checks[idx]
		cfg.Checks = append(cfg.Checks[:idx], cfg.Checks[idx+1:]...)
		removed++
		fmt.Printf("  Removed: %s\n\n", removedCheck.FriendlyName)
	}

	if removed > 0 {
		fmt.Printf("  Removed %d script check(s).\n\n", removed)
	}
	return nil
}

func scriptCheckCount(checks []client.CheckConfig) int {
	return len(scriptCheckEntries(checks))
}

type scriptCheckEntry struct {
	Index int
	Check client.CheckConfig
}

func scriptCheckEntries(checks []client.CheckConfig) []scriptCheckEntry {
	entries := make([]scriptCheckEntry, 0, len(checks))
	for i, check := range checks {
		checkType := strings.TrimSpace(strings.ToLower(check.Type))
		if checkType == "" || checkType == models.CheckTypeScript {
			entries = append(entries, scriptCheckEntry{Index: i, Check: check})
		}
	}
	return entries
}

func printScriptCheckTable(checks []client.CheckConfig) {
	const (
		nameWidth    = 22
		userWidth    = 14
		commandWidth = 32
	)

	entries := scriptCheckEntries(checks)
	fmt.Println("  Configured script checks:")
	border := fmt.Sprintf("  +----+-%s-+-%s-+-%s-+",
		strings.Repeat("-", nameWidth),
		strings.Repeat("-", userWidth),
		strings.Repeat("-", commandWidth),
	)
	fmt.Println(border)
	fmt.Printf("  | %-2s | %-*s | %-*s | %-*s |\n",
		"#", nameWidth, "Friendly Name", userWidth, "Run As User", commandWidth, "Command")
	fmt.Println(border)

	if len(entries) == 0 {
		fmt.Printf("  | %-2s | %-*s | %-*s | %-*s |\n", "", nameWidth, "<none>", userWidth, "", commandWidth, "")
		fmt.Println(border)
		fmt.Println()
		return
	}

	for i, entry := range entries {
		runAs := strings.TrimSpace(entry.Check.RunAsUser)
		if runAs == "" {
			runAs = "(service user)"
		}
		fmt.Printf("  | %2d | %-*s | %-*s | %-*s |\n",
			i+1,
			nameWidth, truncate(entry.Check.FriendlyName, nameWidth),
			userWidth, truncate(runAs, userWidth),
			commandWidth, truncate(entry.Check.ScriptPath, commandWidth),
		)
	}
	fmt.Println(border)
	fmt.Println()
}

func suggestScriptCheckName(scriptCmd string) string {
	scriptCmd = strings.TrimSpace(scriptCmd)
	if scriptCmd == "" {
		return "script-check"
	}
	first := strings.Fields(scriptCmd)[0]
	first = filepath.Base(first)
	first = strings.TrimSuffix(first, ".sh")
	first = strings.TrimSpace(first)
	if first == "" {
		return "script-check"
	}
	return first
}
