package client

import (
	"fmt"
	"os"
	"strings"

	"github.com/google/uuid"
	"github.com/shirou/gopsutil/v4/host"
)

func bootSessionIDFromIdentity(identity string) string {
	return uuid.NewSHA1(uuid.NameSpaceOID, []byte(identity)).String()
}

// bootSessionID returns a deterministic session identifier for the current host boot.
// This keeps session_id stable across client process restarts/upgrades, but changes
// after a machine reboot when host boot time changes.
func bootSessionID() string {
	bootTime, err := host.BootTime()
	if err != nil || bootTime == 0 {
		// Fallback keeps prior behavior if boot identity is unavailable.
		return uuid.New().String()
	}

	hostname, _ := os.Hostname()
	identity := fmt.Sprintf("%s:%d", strings.TrimSpace(hostname), bootTime)
	return bootSessionIDFromIdentity(identity)
}
