package alerting

import (
	"context"
	"fmt"
	"log/slog"
	"strconv"
	"strings"
	"time"

	"github.com/machinemon/machinemon/internal/models"
	"github.com/machinemon/machinemon/internal/store"
)

type Engine struct {
	store      store.Store
	dispatcher *Dispatcher
	logger     *slog.Logger
	checkInCh  chan string
}

func NewEngine(st store.Store, logger *slog.Logger) *Engine {
	return &Engine{
		store:      st,
		dispatcher: NewDispatcher(st, logger),
		logger:     logger,
		checkInCh:  make(chan string, 100),
	}
}

// NotifyCheckIn tells the engine that a client just checked in.
func (e *Engine) NotifyCheckIn(clientID string) {
	select {
	case e.checkInCh <- clientID:
	default:
		e.logger.Warn("check-in notification channel full, dropping", "client_id", clientID)
	}
}

// SendTestAlert dispatches a test alert through a specific provider.
func (e *Engine) SendTestAlert(providerID int64) (*models.TestAlertResult, error) {
	return e.dispatcher.SendTestAlert(providerID)
}

// NotifyRestart fires an alert that a client process restarted (session_id changed).
func (e *Engine) NotifyRestart(clientID, hostname string) {
	e.fireAlert(clientID, models.AlertTypeClientRestarted, models.SeverityWarning,
		fmt.Sprintf("Client '%s' has restarted (new session detected)", hostname))
}

// Run starts the alert engine background loop.
func (e *Engine) Run(ctx context.Context) {
	offlineTicker := time.NewTicker(30 * time.Second)
	cleanupTicker := time.NewTicker(24 * time.Hour)
	defer offlineTicker.Stop()
	defer cleanupTicker.Stop()

	e.logger.Info("alert engine started")

	for {
		select {
		case <-ctx.Done():
			e.logger.Info("alert engine stopped")
			return
		case clientID := <-e.checkInCh:
			e.evaluateCheckIn(clientID)
		case <-offlineTicker.C:
			e.checkOfflineClients()
		case <-cleanupTicker.C:
			e.cleanupOldData()
		}
	}
}

func (e *Engine) checkOfflineClients() {
	thresholdStr, _ := e.store.GetSetting("offline_threshold_seconds")
	thresholdSecs := 240 // default 4 minutes
	if thresholdStr != "" {
		if n, err := fmt.Sscanf(thresholdStr, "%d", &thresholdSecs); n == 0 || err != nil {
			thresholdSecs = 240
		}
	}

	// Use SQL-side time comparison to avoid Go/SQLite timezone mismatches
	clients, err := e.store.GetStaleOnlineClients(thresholdSecs)
	if err != nil {
		e.logger.Error("failed to get stale clients", "err", err)
		return
	}

	for _, c := range clients {
		e.logger.Warn("client went offline", "client_id", c.ID, "hostname", c.Hostname,
			"last_seen", c.LastSeenAt)
		e.store.SetClientOnline(c.ID, false)
		e.fireAlert(c.ID, models.AlertTypeOffline, models.SeverityCritical,
			fmt.Sprintf("Client '%s' has gone offline (no check-in for %d+ seconds)",
				c.Hostname, thresholdSecs))
	}
}

func (e *Engine) evaluateCheckIn(clientID string) {
	client, err := e.store.GetClient(clientID)
	if err != nil || client == nil {
		e.logger.Error("failed to get client for evaluation", "client_id", clientID, "err", err)
		return
	}

	// Check mute status
	if client.AlertsMuted {
		if client.MutedUntil == nil || client.MutedUntil.After(time.Now()) {
			return // Still muted
		}
		// Mute expired, unmute
		e.store.SetClientMute(clientID, false, nil, "")
	}

	// 1. Was this client previously offline? Fire "online" alert
	// The UpsertClient already set is_online=true and returned wasOffline.
	// We check by looking for a recent offline alert without a corresponding online alert.
	lastOfflineAlert, _ := e.store.GetLastAlertByTypes(clientID, models.AlertTypeOffline, models.AlertTypeOnline)
	if lastOfflineAlert != nil && lastOfflineAlert.AlertType == models.AlertTypeOffline {
		e.fireAlert(clientID, models.AlertTypeOnline, models.SeverityInfo,
			fmt.Sprintf("Client '%s' is back online", client.Hostname))
	}

	// 2. Threshold checks
	latest, err := e.store.GetLatestMetrics(clientID)
	if err != nil || latest == nil {
		return
	}

	thresholds := e.resolveThresholds(client)
	e.checkThreshold(clientID, client.Hostname, "cpu", latest.CPUPercent, thresholds.CPUWarnPct, thresholds.CPUCritPct)
	e.checkThreshold(clientID, client.Hostname, "mem", latest.MemPercent, thresholds.MemWarnPct, thresholds.MemCritPct)
	e.checkThreshold(clientID, client.Hostname, "disk", latest.DiskPercent, thresholds.DiskWarnPct, thresholds.DiskCritPct)

	// 3. Process checks
	e.checkProcesses(clientID, client.Hostname)

	// 4. Check results (script, http, file_touch, ...)
	e.checkChecks(clientID, client.Hostname)
}

func (e *Engine) resolveThresholds(client *models.Client) models.Thresholds {
	t := models.DefaultThresholds

	// Load global settings overrides
	if v, _ := e.store.GetSetting("cpu_warn_pct_default"); v != "" {
		fmt.Sscanf(v, "%f", &t.CPUWarnPct)
	}
	if v, _ := e.store.GetSetting("cpu_crit_pct_default"); v != "" {
		fmt.Sscanf(v, "%f", &t.CPUCritPct)
	}
	if v, _ := e.store.GetSetting("mem_warn_pct_default"); v != "" {
		fmt.Sscanf(v, "%f", &t.MemWarnPct)
	}
	if v, _ := e.store.GetSetting("mem_crit_pct_default"); v != "" {
		fmt.Sscanf(v, "%f", &t.MemCritPct)
	}
	if v, _ := e.store.GetSetting("disk_warn_pct_default"); v != "" {
		fmt.Sscanf(v, "%f", &t.DiskWarnPct)
	}
	if v, _ := e.store.GetSetting("disk_crit_pct_default"); v != "" {
		fmt.Sscanf(v, "%f", &t.DiskCritPct)
	}

	// Per-client overrides
	if client.CPUWarnPct != nil {
		t.CPUWarnPct = *client.CPUWarnPct
	}
	if client.CPUCritPct != nil {
		t.CPUCritPct = *client.CPUCritPct
	}
	if client.MemWarnPct != nil {
		t.MemWarnPct = *client.MemWarnPct
	}
	if client.MemCritPct != nil {
		t.MemCritPct = *client.MemCritPct
	}
	if client.DiskWarnPct != nil {
		t.DiskWarnPct = *client.DiskWarnPct
	}
	if client.DiskCritPct != nil {
		t.DiskCritPct = *client.DiskCritPct
	}

	return t
}

func (e *Engine) checkThreshold(clientID, hostname, metric string, value, warnPct, critPct float64) {
	warnType := metric + "_warn"
	critType := metric + "_crit"
	recoverType := metric + "_recover"

	lastAlert, _ := e.store.GetLastAlertByTypes(clientID, warnType, critType, recoverType)

	metricLabel := strings.ToUpper(metric)

	if value >= critPct {
		if lastAlert == nil || lastAlert.AlertType != critType {
			e.fireAlert(clientID, critType, models.SeverityCritical,
				fmt.Sprintf("%s at %.1f%% on '%s' (critical threshold: %.1f%%)",
					metricLabel, value, hostname, critPct))
		}
	} else if value >= warnPct {
		if lastAlert == nil || lastAlert.AlertType != warnType {
			e.fireAlert(clientID, warnType, models.SeverityWarning,
				fmt.Sprintf("%s at %.1f%% on '%s' (warning threshold: %.1f%%)",
					metricLabel, value, hostname, warnPct))
		}
	} else if lastAlert != nil && (lastAlert.AlertType == critType || lastAlert.AlertType == warnType) {
		e.fireAlert(clientID, recoverType, models.SeverityInfo,
			fmt.Sprintf("%s recovered to %.1f%% on '%s'",
				metricLabel, value, hostname))
	}
}

func (e *Engine) checkProcesses(clientID, hostname string) {
	current, err := e.store.GetLatestProcessSnapshots(clientID)
	if err != nil || len(current) == 0 {
		return
	}
	previous, err := e.store.GetPreviousProcessSnapshots(clientID)
	if err != nil || len(previous) == 0 {
		return // No previous data to compare
	}

	prevMap := make(map[string]models.ProcessSnapshot)
	for _, p := range previous {
		prevMap[p.FriendlyName] = p
	}

	for _, curr := range current {
		prev, exists := prevMap[curr.FriendlyName]
		if !exists {
			continue
		}

		if prev.IsRunning && !curr.IsRunning {
			e.fireAlert(clientID, models.AlertTypeProcessDied, models.SeverityCritical,
				fmt.Sprintf("Process '%s' has stopped on '%s'", curr.FriendlyName, hostname))
		} else if prev.IsRunning && curr.IsRunning && prev.PID != nil && curr.PID != nil && *prev.PID != *curr.PID {
			e.fireAlert(clientID, models.AlertTypePIDChange, models.SeverityWarning,
				fmt.Sprintf("Process '%s' PID changed: %d -> %d on '%s'",
					curr.FriendlyName, *prev.PID, *curr.PID, hostname))
		}
	}
}

func (e *Engine) checkChecks(clientID, hostname string) {
	current, err := e.store.GetLatestCheckSnapshots(clientID)
	if err != nil || len(current) == 0 {
		return
	}
	previous, err := e.store.GetPreviousCheckSnapshots(clientID)
	if err != nil {
		return
	}

	prevMap := make(map[string]models.CheckSnapshot)
	for _, cs := range previous {
		prevMap[cs.FriendlyName] = cs
	}

	for _, curr := range current {
		prev, exists := prevMap[curr.FriendlyName]

		if !curr.Healthy {
			// Only alert if it was previously healthy or is first time failing
			if !exists || prev.Healthy {
				msg := fmt.Sprintf("Check '%s' (%s) failed on '%s'",
					curr.FriendlyName, curr.CheckType, hostname)
				if curr.Message != "" {
					msg += ": " + curr.Message
				}
				e.fireAlert(clientID, models.AlertTypeCheckFailed, models.SeverityCritical, msg)
			}
		} else if exists && !prev.Healthy {
			// Was failing, now healthy
			e.fireAlert(clientID, models.AlertTypeCheckRecovered, models.SeverityInfo,
				fmt.Sprintf("Check '%s' (%s) recovered on '%s'",
					curr.FriendlyName, curr.CheckType, hostname))
		}
	}
}

func (e *Engine) fireAlert(clientID, alertType, severity, message string) {
	alert := &models.Alert{
		ClientID:  clientID,
		AlertType: alertType,
		Severity:  severity,
		Message:   message,
		FiredAt:   time.Now().UTC(),
	}

	if err := e.store.InsertAlert(alert); err != nil {
		e.logger.Error("failed to insert alert", "err", err)
		return
	}

	e.logger.Info("alert fired",
		"client_id", clientID,
		"type", alertType,
		"severity", severity,
		"message", message)

	if err := e.dispatcher.Dispatch(alert); err != nil {
		e.logger.Error("failed to dispatch alert", "err", err)
	}
}

func (e *Engine) cleanupOldData() {
	metricsRetentionDays := 14 // default
	if v, _ := e.store.GetSetting("metrics_retention_days"); v != "" {
		if days, err := strconv.Atoi(strings.TrimSpace(v)); err == nil && days > 0 {
			metricsRetentionDays = days
		}
	}
	metricsRetention := time.Duration(metricsRetentionDays) * 24 * time.Hour
	alertsRetention := 90 * 24 * time.Hour // 90 days

	deleted, err := e.store.PruneOldData(metricsRetention, alertsRetention)
	if err != nil {
		e.logger.Error("failed to prune old data", "err", err)
		return
	}
	if deleted > 0 {
		e.logger.Info("pruned old data", "rows_deleted", deleted, "metrics_retention_days", metricsRetentionDays)
	}
}
