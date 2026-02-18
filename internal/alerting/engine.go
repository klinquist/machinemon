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

type scopedMuteState struct {
	metrics   map[string]bool
	processes map[string]bool
	checks    map[string]bool
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

// NotifyRestart fires an alert when a client session_id changes.
func (e *Engine) NotifyRestart(clientID, hostname string) {
	client, err := e.store.GetClient(clientID)
	if err == nil && client != nil {
		hostname = clientLabel(client)
	}
	e.fireAlert(clientID, models.AlertTypeClientRestarted, models.SeverityWarning,
		fmt.Sprintf("Client '%s' has a new session (session change detected)", hostname))
}

// Run starts the alert engine background loop.
func (e *Engine) Run(ctx context.Context) {
	offlineTicker := time.NewTicker(30 * time.Second)
	cleanupTicker := time.NewTicker(24 * time.Hour)
	defer offlineTicker.Stop()
	defer cleanupTicker.Stop()

	e.logger.Info("alert engine started")
	// Run cleanup once at startup so stale data is pruned immediately.
	e.cleanupOldData()

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
	globalThresholdSecs := e.globalOfflineThresholdSeconds()
	clients, err := e.store.GetOnlineClients()
	if err != nil {
		e.logger.Error("failed to get online clients", "err", err)
		return
	}

	now := time.Now().UTC()
	for _, c := range clients {
		thresholdSecs := globalThresholdSecs
		if c.OfflineThresholdSeconds != nil && *c.OfflineThresholdSeconds > 0 {
			thresholdSecs = *c.OfflineThresholdSeconds
		}
		if now.Sub(c.LastSeenAt) < time.Duration(thresholdSecs)*time.Second {
			continue
		}

		hostLabel := clientLabel(&c)
		// Resolve label from the latest full client record so alert messages
		// always prefer custom_name when set.
		if latest, err := e.store.GetClient(c.ID); err == nil && latest != nil {
			hostLabel = clientLabel(latest)
		}
		e.logger.Warn("client went offline", "client_id", c.ID, "hostname", hostLabel,
			"last_seen", c.LastSeenAt, "threshold_seconds", thresholdSecs)
		e.store.SetClientOnline(c.ID, false)
		e.fireAlert(c.ID, models.AlertTypeOffline, models.SeverityCritical,
			fmt.Sprintf("Client '%s' has gone offline (no check-in for %d+ seconds)",
				hostLabel, thresholdSecs))
	}
}

func (e *Engine) globalOfflineThresholdSeconds() int {
	thresholdSecs := 240 // default 4 minutes
	if raw, _ := e.store.GetSetting("offline_threshold_seconds"); raw != "" {
		if parsed, err := strconv.Atoi(strings.TrimSpace(raw)); err == nil && parsed > 0 {
			thresholdSecs = parsed
		}
	}
	return thresholdSecs
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

	scopedMutes := e.loadScopedMutes(clientID)

	// 1. Was this client previously offline? Fire "online" alert
	// The UpsertClient already set is_online=true and returned wasOffline.
	// We check by looking for a recent offline alert without a corresponding online alert.
	hostLabel := clientLabel(client)
	lastOfflineAlert, _ := e.store.GetLastAlertByTypes(clientID, models.AlertTypeOffline, models.AlertTypeOnline)
	if lastOfflineAlert != nil && lastOfflineAlert.AlertType == models.AlertTypeOffline {
		e.fireAlert(clientID, models.AlertTypeOnline, models.SeverityInfo,
			fmt.Sprintf("Client '%s' is back online", hostLabel))
	}

	// 2. Threshold checks
	consecutiveRequired := e.resolveMetricConsecutiveCheckins(client)
	recentMetrics, err := e.store.GetRecentMetrics(clientID, consecutiveRequired)
	if err != nil || len(recentMetrics) == 0 {
		return
	}
	latest := recentMetrics[0]

	thresholds := e.resolveThresholds(client)
	if !scopedMutes.metrics["cpu"] {
		e.checkThreshold(clientID, hostLabel, "cpu", latest.CPUPercent, thresholds.CPUWarnPct, thresholds.CPUCritPct, recentMetrics, consecutiveRequired)
	}
	if !scopedMutes.metrics["mem"] {
		e.checkThreshold(clientID, hostLabel, "mem", latest.MemPercent, thresholds.MemWarnPct, thresholds.MemCritPct, recentMetrics, consecutiveRequired)
	}
	if !scopedMutes.metrics["disk"] {
		e.checkThreshold(clientID, hostLabel, "disk", latest.DiskPercent, thresholds.DiskWarnPct, thresholds.DiskCritPct, recentMetrics, consecutiveRequired)
	}

	// 3. Process checks
	e.checkProcesses(clientID, hostLabel, scopedMutes)

	// 4. Check results (script, http, file_touch, ...)
	e.checkChecks(clientID, hostLabel, scopedMutes)
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

func (e *Engine) resolveMetricConsecutiveCheckins(client *models.Client) int {
	required := 1
	if raw, _ := e.store.GetSetting("metric_consecutive_checkins_default"); raw != "" {
		if parsed, err := strconv.Atoi(strings.TrimSpace(raw)); err == nil && parsed > 0 {
			required = parsed
		}
	}
	if client.MetricConsecutiveCheckins != nil && *client.MetricConsecutiveCheckins > 0 {
		required = *client.MetricConsecutiveCheckins
	}
	return required
}

func (e *Engine) checkThreshold(clientID, hostname, metric string, value, warnPct, critPct float64, recent []models.Metric, consecutiveRequired int) {
	warnType := metric + "_warn"
	critType := metric + "_crit"
	recoverType := metric + "_recover"

	lastAlert, _ := e.store.GetLastAlertByTypes(clientID, warnType, critType, recoverType)

	metricLabel := strings.ToUpper(metric)
	critStreak := consecutiveThresholdStreak(recent, metric, critPct)
	warnStreak := consecutiveThresholdStreak(recent, metric, warnPct)

	if value >= critPct {
		if critStreak >= consecutiveRequired && (lastAlert == nil || lastAlert.AlertType != critType) {
			e.fireAlert(clientID, critType, models.SeverityCritical,
				fmt.Sprintf("%s at %.1f%% on '%s' (critical threshold: %.1f%%)",
					metricLabel, value, hostname, critPct))
		}
	} else if value >= warnPct {
		if warnStreak >= consecutiveRequired && (lastAlert == nil || lastAlert.AlertType != warnType) {
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

func consecutiveThresholdStreak(recent []models.Metric, metric string, threshold float64) int {
	streak := 0
	for _, m := range recent {
		value := metricValue(m, metric)
		if value < threshold {
			break
		}
		streak++
	}
	return streak
}

func metricValue(m models.Metric, metric string) float64 {
	switch metric {
	case "cpu":
		return m.CPUPercent
	case "mem":
		return m.MemPercent
	case "disk":
		return m.DiskPercent
	default:
		return 0
	}
}

func (e *Engine) checkProcesses(clientID, hostname string, mutes scopedMuteState) {
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
		if mutes.processes[curr.FriendlyName] {
			continue
		}
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

func (e *Engine) checkChecks(clientID, hostname string, mutes scopedMuteState) {
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
		prevMap[checkMuteTarget(cs.FriendlyName, cs.CheckType)] = cs
	}

	for _, curr := range current {
		if mutes.checks[checkMuteTarget(curr.FriendlyName, curr.CheckType)] {
			continue
		}
		prev, exists := prevMap[checkMuteTarget(curr.FriendlyName, curr.CheckType)]

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

func (e *Engine) loadScopedMutes(clientID string) scopedMuteState {
	out := scopedMuteState{
		metrics:   map[string]bool{},
		processes: map[string]bool{},
		checks:    map[string]bool{},
	}
	mutes, err := e.store.ListClientAlertMutes(clientID)
	if err != nil {
		e.logger.Error("failed to load scoped mutes", "client_id", clientID, "err", err)
		return out
	}
	for _, m := range mutes {
		switch m.Scope {
		case "cpu":
			out.metrics["cpu"] = true
		case "memory":
			out.metrics["mem"] = true
		case "disk":
			out.metrics["disk"] = true
		case "process":
			if m.Target != "" {
				out.processes[m.Target] = true
			}
		case "check":
			if m.Target != "" {
				out.checks[m.Target] = true
			}
		}
	}
	return out
}

func checkMuteTarget(friendlyName, checkType string) string {
	return strings.TrimSpace(friendlyName) + "::" + strings.TrimSpace(checkType)
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
	alertsRetentionDays := metricsRetentionDays // default: follow global data retention
	if v, _ := e.store.GetSetting("alerts_retention_days"); v != "" {
		if days, err := strconv.Atoi(strings.TrimSpace(v)); err == nil && days > 0 {
			alertsRetentionDays = days
		}
	}
	metricsRetention := time.Duration(metricsRetentionDays) * 24 * time.Hour
	alertsRetention := time.Duration(alertsRetentionDays) * 24 * time.Hour

	deleted, err := e.store.PruneOldData(metricsRetention, alertsRetention)
	if err != nil {
		e.logger.Error("failed to prune old data", "err", err)
		return
	}
	if deleted > 0 {
		e.logger.Info("pruned old data",
			"rows_deleted", deleted,
			"metrics_retention_days", metricsRetentionDays,
			"alerts_retention_days", alertsRetentionDays)
	}
}

func clientLabel(c *models.Client) string {
	if c == nil {
		return ""
	}
	if strings.TrimSpace(c.CustomName) != "" {
		return c.CustomName
	}
	return c.Hostname
}
