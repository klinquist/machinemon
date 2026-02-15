package store

import (
	"time"

	"github.com/machinemon/machinemon/internal/models"
)

// Store defines the data access interface for MachineMon.
type Store interface {
	Close() error

	// Client operations
	UpsertClient(req models.CheckInRequest) (clientID string, wasOffline bool, err error)
	GetClient(id string) (*models.Client, error)
	ListClients() ([]models.ClientWithMetrics, error)
	DeleteClient(id string) error
	SetClientOnline(id string, online bool) error
	GetOnlineClients() ([]models.Client, error)
	SetClientThresholds(id string, t *models.Thresholds) error
	SetClientMute(id string, muted bool, until *time.Time, reason string) error

	// Metrics
	InsertMetrics(clientID string, m models.MetricsPayload) error
	GetLatestMetrics(clientID string) (*models.Metric, error)
	GetMetrics(clientID string, from, to time.Time, limit int) ([]models.Metric, error)

	// Process tracking
	UpsertWatchedProcesses(clientID string, procs []models.ProcessPayload) error
	InsertProcessSnapshots(clientID string, procs []models.ProcessPayload) error
	GetLatestProcessSnapshots(clientID string) ([]models.ProcessSnapshot, error)
	GetPreviousProcessSnapshots(clientID string) ([]models.ProcessSnapshot, error)
	GetWatchedProcesses(clientID string) ([]models.WatchedProcess, error)

	// Checks (extensible typed check system: script, http, file_touch, ...)
	InsertCheckSnapshots(clientID string, checks []models.CheckPayload) error
	GetLatestCheckSnapshots(clientID string) ([]models.CheckSnapshot, error)
	GetPreviousCheckSnapshots(clientID string) ([]models.CheckSnapshot, error)

	// Alerts
	InsertAlert(a *models.Alert) error
	MarkAlertNotified(id int64) error
	GetUnnotifiedAlerts() ([]models.Alert, error)
	ListAlerts(clientID string, severity string, limit, offset int) ([]models.Alert, int, error)
	GetLastAlertByTypes(clientID string, types ...string) (*models.Alert, error)

	// Alert providers
	ListProviders() ([]models.AlertProvider, error)
	GetProvider(id int64) (*models.AlertProvider, error)
	CreateProvider(p *models.AlertProvider) error
	UpdateProvider(p *models.AlertProvider) error
	DeleteProvider(id int64) error
	GetEnabledProviders() ([]models.AlertProvider, error)

	// Settings
	GetSetting(key string) (string, error)
	SetSetting(key, value string) error
	GetAllSettings() (map[string]string, error)

	// Maintenance
	PruneOldData(metricsRetention, alertsRetention time.Duration) (int64, error)
}
