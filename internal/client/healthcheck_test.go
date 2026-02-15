package client

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/machinemon/machinemon/internal/models"
)

func TestRunScriptCheckExitCodeZeroIsHealthy(t *testing.T) {
	result := runScriptCheck(CheckConfig{
		FriendlyName: "ok",
		Type:         models.CheckTypeScript,
		ScriptPath:   "exit 0",
	})
	if !result.Healthy {
		t.Fatalf("expected healthy result, got %+v", result)
	}
	if result.Message != "OK" {
		t.Fatalf("expected OK message, got %q", result.Message)
	}
}

func TestRunScriptCheckExitCodeOneIsUnhealthy(t *testing.T) {
	result := runScriptCheck(CheckConfig{
		FriendlyName: "fail",
		Type:         models.CheckTypeScript,
		ScriptPath:   "exit 1",
	})
	if result.Healthy {
		t.Fatalf("expected unhealthy result, got %+v", result)
	}
	if !strings.Contains(result.Message, "1") {
		t.Fatalf("expected exit code in message, got %q", result.Message)
	}

	var state models.ScriptCheckState
	if err := json.Unmarshal([]byte(result.State), &state); err != nil {
		t.Fatalf("unmarshal state: %v", err)
	}
	if state.ExitCode != 1 {
		t.Fatalf("expected state exit code 1, got %d", state.ExitCode)
	}
}

func TestRunScriptCheckUnknownRunAsUserFails(t *testing.T) {
	result := runScriptCheck(CheckConfig{
		FriendlyName: "unknown-user",
		Type:         models.CheckTypeScript,
		ScriptPath:   "exit 0",
		RunAsUser:    "__machinemon_no_such_user__",
	})
	if result.Healthy {
		t.Fatalf("expected unhealthy result, got %+v", result)
	}
	if !strings.Contains(result.Message, "not found") {
		t.Fatalf("expected not found error message, got %q", result.Message)
	}

	var state models.ScriptCheckState
	if err := json.Unmarshal([]byte(result.State), &state); err != nil {
		t.Fatalf("unmarshal state: %v", err)
	}
	if state.RunAsUser != "__machinemon_no_such_user__" {
		t.Fatalf("unexpected state run_as_user: %+v", state)
	}
}
