package client

import (
	"log/slog"
	"os"
	"os/signal"
	"syscall"
	"time"
)

func RunDaemon(cfg *Config, configPath string, logger *slog.Logger) {
	sessionID := bootSessionID()
	reporter := NewReporter(cfg.ServerURL, cfg.Password, cfg.InsecureSkipTLS)
	interval := time.Duration(cfg.CheckInInterval) * time.Second

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGTERM, syscall.SIGINT)

	doCheckIn := func() {
		logger.Info("collecting metrics")
		metrics, err := CollectSystemMetrics()
		if err != nil {
			logger.Error("failed to collect metrics", "err", err)
			return
		}

		var procs []ProcessStatus
		if len(cfg.Processes) > 0 {
			procs, err = MatchProcesses(cfg.Processes)
			if err != nil {
				logger.Error("failed to match processes", "err", err)
			}
		}

		var checks []CheckResult
		if len(cfg.Checks) > 0 {
			logger.Info("running checks", "count", len(cfg.Checks))
			checks = RunChecks(cfg.Checks)
			for _, c := range checks {
				if !c.Healthy {
					logger.Warn("check failed", "name", c.FriendlyName, "type", c.CheckType, "message", c.Message)
				}
			}
		}

		logger.Info("sending check-in",
			"cpu", metrics.CPUPercent,
			"mem", metrics.MemPercent,
			"disk", metrics.DiskPercent,
			"processes", len(procs),
			"checks", len(checks))

		resp, err := reporter.CheckIn(cfg.ClientID, sessionID, metrics, procs, checks)
		if err != nil {
			logger.Error("check-in failed", "err", err)
			return
		}

		logger.Info("check-in successful", "client_id", resp.ClientID)

		// Save client_id if this was first check-in
		if cfg.ClientID == "" && resp.ClientID != "" {
			cfg.ClientID = resp.ClientID
			if err := SaveConfig(cfg, configPath); err != nil {
				logger.Error("failed to save config with client_id", "err", err)
			} else {
				logger.Info("saved client_id to config", "client_id", resp.ClientID)
			}
		}

		// Adjust interval if server requests it
		if resp.NextCheckInSeconds > 0 {
			newInterval := time.Duration(resp.NextCheckInSeconds) * time.Second
			if newInterval != interval {
				interval = newInterval
				logger.Info("adjusted check-in interval", "seconds", resp.NextCheckInSeconds)
			}
		}
	}

	logger.Info("starting daemon",
		"server", cfg.ServerURL,
		"interval", interval,
		"session_id", sessionID,
		"processes", len(cfg.Processes),
		"checks", len(cfg.Checks))

	// Immediate first check-in
	doCheckIn()

	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			doCheckIn()
			// Reset ticker in case interval changed
			ticker.Reset(interval)
		case sig := <-sigCh:
			logger.Info("received signal, shutting down", "signal", sig)
			return
		}
	}
}
