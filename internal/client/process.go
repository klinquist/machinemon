package client

import (
	"regexp"
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
			cmdline, err := p.Cmdline()
			if err != nil || cmdline == "" {
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

	seen := make(map[string]bool)
	var candidates []ProcessCandidate

	for _, p := range procs {
		name, err := p.Name()
		if err != nil || name == "" {
			continue
		}

		cmdline, _ := p.Cmdline()
		if cmdline == "" {
			continue
		}

		// Skip kernel threads (Linux)
		if strings.HasPrefix(cmdline, "[") && strings.HasSuffix(cmdline, "]") {
			continue
		}

		// For deduplication, use the full cmdline as key
		key := cmdline
		if seen[key] {
			continue
		}
		seen[key] = true

		candidates = append(candidates, ProcessCandidate{
			PID:     p.Pid,
			Name:    name,
			Cmdline: cmdline,
		})
	}
	return candidates, nil
}

// SuggestMatchPattern returns a good match pattern for a process.
// For Node.js processes, uses the script path instead of just "node".
func SuggestMatchPattern(candidate ProcessCandidate) string {
	name := strings.ToLower(candidate.Name)
	if name == "node" || name == "nodejs" {
		// For Node.js, use the script path from the cmdline
		parts := strings.Fields(candidate.Cmdline)
		for _, part := range parts[1:] {
			if !strings.HasPrefix(part, "-") {
				return part // First non-flag argument is typically the script
			}
		}
	}
	return candidate.Name
}

// SuggestFriendlyName returns a suggested friendly name for a process.
func SuggestFriendlyName(candidate ProcessCandidate) string {
	name := strings.ToLower(candidate.Name)
	if name == "node" || name == "nodejs" {
		parts := strings.Fields(candidate.Cmdline)
		for _, part := range parts[1:] {
			if !strings.HasPrefix(part, "-") {
				// Use the script filename without extension
				segments := strings.Split(part, "/")
				scriptName := segments[len(segments)-1]
				scriptName = strings.TrimSuffix(scriptName, ".js")
				scriptName = strings.TrimSuffix(scriptName, ".mjs")
				scriptName = strings.TrimSuffix(scriptName, ".cjs")
				return scriptName
			}
		}
	}
	return candidate.Name
}
