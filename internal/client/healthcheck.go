package client

import (
	"context"
	"encoding/json"
	"os/exec"
	"time"

	"github.com/machinemon/machinemon/internal/models"
)

// CheckResult is the internal result of running a single check.
type CheckResult struct {
	FriendlyName string
	CheckType    string
	Healthy      bool
	Message      string
	State        string // JSON blob
}

// RunChecks executes all configured checks and returns payloads ready for the server.
func RunChecks(checks []CheckConfig) []CheckResult {
	results := make([]CheckResult, len(checks))
	for i, check := range checks {
		results[i] = runCheck(check)
	}
	return results
}

func runCheck(check CheckConfig) CheckResult {
	switch check.Type {
	case models.CheckTypeScript, "":
		return runScriptCheck(check)
	case models.CheckTypeHTTP:
		return runHTTPCheck(check)
	case models.CheckTypeFileTouch:
		return runFileTouchCheck(check)
	default:
		return CheckResult{
			FriendlyName: check.FriendlyName,
			CheckType:    check.Type,
			Healthy:      false,
			Message:      "unknown check type: " + check.Type,
		}
	}
}

func runScriptCheck(check CheckConfig) CheckResult {
	result := CheckResult{
		FriendlyName: check.FriendlyName,
		CheckType:    models.CheckTypeScript,
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	cmd := exec.CommandContext(ctx, "/bin/sh", "-c", check.ScriptPath)
	output, err := cmd.CombinedOutput()

	// Capture last 500 chars of output
	outputStr := string(output)
	if len(outputStr) > 500 {
		outputStr = outputStr[len(outputStr)-500:]
	}

	var exitCode int
	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			exitCode = exitErr.ExitCode()
		} else {
			exitCode = -1
			outputStr = err.Error()
		}
		result.Healthy = false
		result.Message = "exit code " + itoa(exitCode)
	} else {
		exitCode = 0
		result.Healthy = true
		result.Message = "OK"
	}

	state, _ := json.Marshal(models.ScriptCheckState{
		ScriptPath: check.ScriptPath,
		ExitCode:   exitCode,
		Output:     outputStr,
	})
	result.State = string(state)

	return result
}

// runHTTPCheck performs an HTTP check (placeholder for future implementation).
func runHTTPCheck(check CheckConfig) CheckResult {
	// TODO: implement HTTP check
	return CheckResult{
		FriendlyName: check.FriendlyName,
		CheckType:    models.CheckTypeHTTP,
		Healthy:      false,
		Message:      "HTTP checks not yet implemented",
	}
}

// runFileTouchCheck performs a file-touch check (placeholder for future implementation).
func runFileTouchCheck(check CheckConfig) CheckResult {
	// TODO: implement file touch check
	return CheckResult{
		FriendlyName: check.FriendlyName,
		CheckType:    models.CheckTypeFileTouch,
		Healthy:      false,
		Message:      "file touch checks not yet implemented",
	}
}

func itoa(i int) string {
	if i == 0 {
		return "0"
	}
	s := ""
	neg := false
	if i < 0 {
		neg = true
		i = -i
	}
	for i > 0 {
		s = string(rune('0'+i%10)) + s
		i /= 10
	}
	if neg {
		s = "-" + s
	}
	return s
}
