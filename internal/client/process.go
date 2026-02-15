package client

import (
	"path/filepath"
	"regexp"
	"sort"
	"strings"

	"github.com/shirou/gopsutil/v4/process"
)

type ProcessStatus struct {
	FriendlyName string
	MatchPattern string
	IsRunning    bool
	PID          int32
	CPUPercent   float64
	MemPercent   float64
	Cmdline      string
}

// MatchProcesses scans running processes and matches against watched process patterns.
func MatchProcesses(watched []ProcessConfig) ([]ProcessStatus, error) {
	allProcs, err := process.Processes()
	if err != nil {
		return nil, err
	}

	results := make([]ProcessStatus, len(watched))
	for i, w := range watched {
		results[i] = ProcessStatus{
			FriendlyName: w.FriendlyName,
			MatchPattern: w.MatchPattern,
		}
		for _, p := range allProcs {
			cmdline, ok := processSearchText(p)
			if !ok {
				continue
			}
			if matchesCmdline(w.MatchPattern, w.MatchType, cmdline) {
				results[i].IsRunning = true
				results[i].PID = p.Pid
				results[i].Cmdline = cmdline
				cpuPct, _ := p.CPUPercent()
				results[i].CPUPercent = cpuPct
				memPct, _ := p.MemoryPercent()
				results[i].MemPercent = float64(memPct)
				break
			}
		}
	}
	return results, nil
}

func matchesCmdline(pattern, matchType, cmdline string) bool {
	switch matchType {
	case "regex":
		matched, _ := regexp.MatchString(pattern, cmdline)
		return matched
	default: // "substring"
		return strings.Contains(cmdline, pattern)
	}
}

// ProcessCandidate represents a running process for the picker UI.
type ProcessCandidate struct {
	PID     int32
	Name    string
	Cmdline string
}

// ListProcessCandidates returns all running processes suitable for monitoring.
func ListProcessCandidates() ([]ProcessCandidate, error) {
	procs, err := process.Processes()
	if err != nil {
		return nil, err
	}

	var candidates []ProcessCandidate

	for _, p := range procs {
		name, err := p.Name()
		if err != nil || strings.TrimSpace(name) == "" {
			continue
		}

		cmdline, ok := processSearchText(p)
		if !ok {
			continue
		}

		// Skip kernel threads (Linux)
		if strings.HasPrefix(cmdline, "[") && strings.HasSuffix(cmdline, "]") {
			continue
		}

		candidates = append(candidates, ProcessCandidate{
			PID:     p.Pid,
			Name:    name,
			Cmdline: cmdline,
		})
	}
	sort.Slice(candidates, func(i, j int) bool {
		if candidates[i].Name == candidates[j].Name {
			return candidates[i].PID < candidates[j].PID
		}
		return strings.ToLower(candidates[i].Name) < strings.ToLower(candidates[j].Name)
	})
	return candidates, nil
}

func processSearchText(p *process.Process) (string, bool) {
	cmdline, _ := p.Cmdline()
	cmdline = strings.TrimSpace(cmdline)
	if cmdline != "" {
		return cmdline, true
	}

	exe, _ := p.Exe()
	exe = strings.TrimSpace(exe)
	if exe != "" {
		return exe, true
	}

	name, _ := p.Name()
	name = strings.TrimSpace(name)
	if name != "" {
		return name, true
	}

	return "", false
}

// SuggestMatchPattern returns a good match pattern for a process.
// For Node.js processes, uses the script path instead of just "node".
func SuggestMatchPattern(candidate ProcessCandidate) string {
	if isNodeProcess(candidate) {
		if script := nodeScriptArg(candidate.Cmdline); script != "" {
			// Match on node + script path so we disambiguate different node apps.
			return "node " + script
		}
	}
	name := canonicalProcessName(candidate)
	if name != "" {
		return name
	}
	return strings.TrimSpace(candidate.Cmdline)
}

// SuggestFriendlyName returns a suggested friendly name for a process.
func SuggestFriendlyName(candidate ProcessCandidate) string {
	if isNodeProcess(candidate) {
		if script := nodeScriptArg(candidate.Cmdline); script != "" {
			scriptName := filepath.Base(script)
			scriptName = strings.TrimSuffix(scriptName, ".js")
			scriptName = strings.TrimSuffix(scriptName, ".mjs")
			scriptName = strings.TrimSuffix(scriptName, ".cjs")
			if scriptName != "" {
				return scriptName
			}
		}
	}
	if name := canonicalProcessName(candidate); name != "" {
		return name
	}
	return strings.TrimSpace(candidate.Cmdline)
}

func canonicalProcessName(candidate ProcessCandidate) string {
	name := strings.TrimSpace(candidate.Name)
	if name != "" && !strings.Contains(name, " ") {
		return name
	}
	parts := strings.Fields(candidate.Cmdline)
	if len(parts) == 0 {
		return name
	}
	base := filepath.Base(parts[0])
	base = strings.TrimSpace(base)
	if base != "" {
		return base
	}
	return name
}

func isNodeProcess(candidate ProcessCandidate) bool {
	name := strings.ToLower(strings.TrimSpace(canonicalProcessName(candidate)))
	return name == "node" || name == "nodejs"
}

func nodeScriptArg(cmdline string) string {
	parts := strings.Fields(cmdline)
	for i, part := range parts {
		if i == 0 {
			continue
		}
		if strings.HasPrefix(part, "-") {
			continue
		}
		return part
	}
	return ""
}
