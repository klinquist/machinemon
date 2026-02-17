package models

import "time"

// CheckInRequest is sent by the client to the server every check-in interval.
type CheckInRequest struct {
	Hostname      string           `json:"hostname"`
	OS            string           `json:"os"`
	Arch          string           `json:"arch"`
	ClientVersion string           `json:"client_version"`
	ClientID      string           `json:"client_id,omitempty"`
	SessionID     string           `json:"session_id,omitempty"`
	InterfaceIPs  []string         `json:"interface_ips,omitempty"`
	Metrics       MetricsPayload   `json:"metrics"`
	Processes     []ProcessPayload `json:"processes"`
	Checks        []CheckPayload   `json:"checks,omitempty"`
}

// CheckPayload reports the result of a client-side check.
// The check_type field identifies what kind of check this is (script, http,
// file_touch, etc). The State field carries type-specific details as JSON.
// New check types can be added to the client without server changes -- the
// server alerts purely on healthy/unhealthy transitions.
type CheckPayload struct {
	FriendlyName string `json:"friendly_name"`
	CheckType    string `json:"check_type"` // "script", "http", "file_touch", ...
	Healthy      bool   `json:"healthy"`
	Message      string `json:"message,omitempty"` // human-readable status summary
	State        string `json:"state,omitempty"`   // JSON blob with type-specific details
}

// Well-known check types. New types can be added without changing the server.
const (
	CheckTypeScript    = "script"
	CheckTypeHTTP      = "http"
	CheckTypeFileTouch = "file_touch"
)

// ScriptCheckState is the state blob for CheckTypeScript checks.
type ScriptCheckState struct {
	ScriptPath string `json:"script_path"`
	RunAsUser  string `json:"run_as_user,omitempty"`
	ExitCode   int    `json:"exit_code"`
	Output     string `json:"output,omitempty"`
}

// HTTPCheckState is the state blob for CheckTypeHTTP checks (future).
type HTTPCheckState struct {
	URL            string `json:"url"`
	ExpectedStatus int    `json:"expected_status,omitempty"`
	ActualStatus   int    `json:"actual_status,omitempty"`
	ResponseTimeMs int64  `json:"response_time_ms,omitempty"`
	Error          string `json:"error,omitempty"`
}

// FileTouchCheckState is the state blob for CheckTypeFileTouch checks (future).
type FileTouchCheckState struct {
	FilePath     string `json:"file_path"`
	MaxAgeSecs   int    `json:"max_age_secs"`
	LastModified string `json:"last_modified,omitempty"`
	AgeSecs      int    `json:"age_secs,omitempty"`
}

type MetricsPayload struct {
	CPUPercent     float64 `json:"cpu_pct"`
	MemPercent     float64 `json:"mem_pct"`
	MemTotalBytes  uint64  `json:"mem_total_bytes"`
	MemUsedBytes   uint64  `json:"mem_used_bytes"`
	DiskPercent    float64 `json:"disk_pct"`
	DiskTotalBytes uint64  `json:"disk_total_bytes"`
	DiskUsedBytes  uint64  `json:"disk_used_bytes"`
}

type ProcessPayload struct {
	FriendlyName string  `json:"friendly_name"`
	MatchPattern string  `json:"match_pattern"`
	IsRunning    bool    `json:"is_running"`
	PID          int32   `json:"pid,omitempty"`
	CPUPercent   float64 `json:"cpu_pct,omitempty"`
	MemPercent   float64 `json:"mem_pct,omitempty"`
	Cmdline      string  `json:"cmdline,omitempty"`
}

// CheckInResponse is returned to the client after a successful check-in.
type CheckInResponse struct {
	ClientID           string    `json:"client_id"`
	NextCheckInSeconds int       `json:"next_checkin_seconds"`
	ServerTime         time.Time `json:"server_time"`
}

// ClientAlertMute stores per-client scoped alert mute rules.
// Scope values: "cpu", "memory", "disk", "process", "check".
type ClientAlertMute struct {
	ID        int64     `json:"id,omitempty"`
	ClientID  string    `json:"client_id,omitempty"`
	Scope     string    `json:"scope"`
	Target    string    `json:"target,omitempty"`
	CreatedAt time.Time `json:"created_at,omitempty"`
}

// Client represents a monitored machine.
type Client struct {
	ID               string    `json:"id"`
	Hostname         string    `json:"hostname"`
	CustomName       string    `json:"custom_name,omitempty"`
	PublicIP         string    `json:"public_ip,omitempty"`
	InterfaceIPs     []string  `json:"interface_ips,omitempty"`
	OS               string    `json:"os"`
	Arch             string    `json:"arch"`
	ClientVersion    string    `json:"client_version"`
	FirstSeenAt      time.Time `json:"first_seen_at"`
	LastSeenAt       time.Time `json:"last_seen_at"`
	SessionStartedAt time.Time `json:"session_started_at"`
	IsOnline         bool      `json:"is_online"`
	IsDeleted        bool      `json:"is_deleted,omitempty"`

	CPUWarnPct  *float64 `json:"cpu_warn_pct,omitempty"`
	CPUCritPct  *float64 `json:"cpu_crit_pct,omitempty"`
	MemWarnPct  *float64 `json:"mem_warn_pct,omitempty"`
	MemCritPct  *float64 `json:"mem_crit_pct,omitempty"`
	DiskWarnPct *float64 `json:"disk_warn_pct,omitempty"`
	DiskCritPct *float64 `json:"disk_crit_pct,omitempty"`
	// Per-client offline alert delay override (seconds). Nil means use global default.
	OfflineThresholdSeconds *int `json:"offline_threshold_seconds,omitempty"`
	// Optional per-client override for metric alert streak length.
	// Nil means use global default.
	MetricConsecutiveCheckins *int `json:"metric_consecutive_checkins,omitempty"`

	AlertsMuted bool       `json:"alerts_muted"`
	MutedUntil  *time.Time `json:"muted_until,omitempty"`
	MuteReason  string     `json:"mute_reason,omitempty"`
}

// ClientWithMetrics is a client with its most recent metrics attached.
type ClientWithMetrics struct {
	Client
	LatestMetrics *Metric `json:"latest_metrics,omitempty"`
	ProcessCount  int     `json:"process_count"`
}

// Metric is a single point-in-time metric reading.
type Metric struct {
	ID             int64     `json:"id,omitempty"`
	ClientID       string    `json:"client_id,omitempty"`
	RecordedAt     time.Time `json:"recorded_at"`
	CPUPercent     float64   `json:"cpu_pct"`
	MemPercent     float64   `json:"mem_pct"`
	DiskPercent    float64   `json:"disk_pct"`
	MemTotalBytes  uint64    `json:"mem_total_bytes"`
	MemUsedBytes   uint64    `json:"mem_used_bytes"`
	DiskTotalBytes uint64    `json:"disk_total_bytes"`
	DiskUsedBytes  uint64    `json:"disk_used_bytes"`
}

// WatchedProcess is a process definition configured for monitoring.
type WatchedProcess struct {
	ID           int64  `json:"id,omitempty"`
	ClientID     string `json:"client_id,omitempty"`
	FriendlyName string `json:"friendly_name"`
	MatchPattern string `json:"match_pattern"`
	MatchType    string `json:"match_type"` // "substring" or "regex"
}

// ProcessSnapshot is a point-in-time status of a watched process.
type ProcessSnapshot struct {
	ID            int64     `json:"id,omitempty"`
	ClientID      string    `json:"client_id,omitempty"`
	FriendlyName  string    `json:"friendly_name"`
	RecordedAt    time.Time `json:"recorded_at"`
	UptimeSinceAt time.Time `json:"uptime_since_at"`
	IsRunning     bool      `json:"is_running"`
	PID           *int32    `json:"pid,omitempty"`
	CPUPercent    float64   `json:"cpu_pct,omitempty"`
	MemPercent    float64   `json:"mem_pct,omitempty"`
	Cmdline       string    `json:"cmdline,omitempty"`
}

// CheckSnapshot is a point-in-time result of any typed client check.
// The CheckType + State fields make this extensible to new check types
// without schema changes. The server only needs to look at Healthy for
// alerting -- State is preserved for display/debugging.
type CheckSnapshot struct {
	ID            int64     `json:"id,omitempty"`
	ClientID      string    `json:"client_id,omitempty"`
	FriendlyName  string    `json:"friendly_name"`
	CheckType     string    `json:"check_type"`
	RecordedAt    time.Time `json:"recorded_at"`
	UptimeSinceAt time.Time `json:"uptime_since_at"`
	Healthy       bool      `json:"healthy"`
	Message       string    `json:"message,omitempty"`
	State         string    `json:"state,omitempty"` // JSON blob, type-specific
}

// Alert types.
const (
	AlertTypeOffline         = "offline"
	AlertTypeOnline          = "online"
	AlertTypePIDChange       = "pid_change"
	AlertTypeProcessDied     = "process_died"
	AlertTypeCheckFailed     = "check_failed"
	AlertTypeCheckRecovered  = "check_recovered"
	AlertTypeClientRestarted = "client_restarted"
	AlertTypeCPUWarn         = "cpu_warn"
	AlertTypeCPUCrit         = "cpu_crit"
	AlertTypeCPURecover      = "cpu_recover"
	AlertTypeMemWarn         = "mem_warn"
	AlertTypeMemCrit         = "mem_crit"
	AlertTypeMemRecover      = "mem_recover"
	AlertTypeDiskWarn        = "disk_warn"
	AlertTypeDiskCrit        = "disk_crit"
	AlertTypeDiskRecover     = "disk_recover"
)

// Alert severities.
const (
	SeverityInfo     = "info"
	SeverityWarning  = "warning"
	SeverityCritical = "critical"
)

// Alert represents a fired alert event.
type Alert struct {
	ID         int64      `json:"id"`
	ClientID   string     `json:"client_id"`
	AlertType  string     `json:"alert_type"`
	Severity   string     `json:"severity"`
	Message    string     `json:"message"`
	Details    string     `json:"details,omitempty"`
	FiredAt    time.Time  `json:"fired_at"`
	Notified   bool       `json:"notified"`
	NotifiedAt *time.Time `json:"notified_at,omitempty"`
}

// AlertProvider represents a configured notification channel.
type AlertProvider struct {
	ID        int64     `json:"id"`
	Type      string    `json:"type"` // "twilio", "pushover", "smtp"
	Name      string    `json:"name"`
	Enabled   bool      `json:"enabled"`
	Config    string    `json:"config"` // JSON blob
	CreatedAt time.Time `json:"created_at"`
}

// TestAlertResult carries delivery details for a provider test-send request.
type TestAlertResult struct {
	Provider      string `json:"provider"`
	Message       string `json:"message"`
	APIStatusCode int    `json:"api_status_code,omitempty"`
	APIResponse   string `json:"api_response,omitempty"`
}

// Thresholds holds warn/crit thresholds for a client.
type Thresholds struct {
	CPUWarnPct  float64 `json:"cpu_warn_pct"`
	CPUCritPct  float64 `json:"cpu_crit_pct"`
	MemWarnPct  float64 `json:"mem_warn_pct"`
	MemCritPct  float64 `json:"mem_crit_pct"`
	DiskWarnPct float64 `json:"disk_warn_pct"`
	DiskCritPct float64 `json:"disk_crit_pct"`
	// Optional per-client offline alert delay override in minutes.
	// Nil means keep current value; use clear-thresholds endpoint to reset to global.
	OfflineThresholdMinutes *int `json:"offline_threshold_minutes,omitempty"`
	// Optional per-client override: number of consecutive check-ins above threshold
	// required before metric alerts fire.
	MetricConsecutiveCheckins *int `json:"metric_consecutive_checkins,omitempty"`
}

// Default thresholds if nothing else is configured.
var DefaultThresholds = Thresholds{
	CPUWarnPct:  80,
	CPUCritPct:  95,
	MemWarnPct:  85,
	MemCritPct:  95,
	DiskWarnPct: 80,
	DiskCritPct: 90,
}
