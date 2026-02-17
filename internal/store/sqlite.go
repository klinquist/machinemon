package store

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/machinemon/machinemon/internal/models"
	_ "modernc.org/sqlite"
)

type SQLiteStore struct {
	db *sql.DB
}

func encodeInterfaceIPs(ips []string) string {
	if len(ips) == 0 {
		return "[]"
	}
	seen := make(map[string]struct{}, len(ips))
	cleaned := make([]string, 0, len(ips))
	for _, ip := range ips {
		ip = strings.TrimSpace(ip)
		if ip == "" {
			continue
		}
		if _, ok := seen[ip]; ok {
			continue
		}
		seen[ip] = struct{}{}
		cleaned = append(cleaned, ip)
	}
	sort.Strings(cleaned)
	if len(cleaned) == 0 {
		return "[]"
	}
	b, err := json.Marshal(cleaned)
	if err != nil {
		return "[]"
	}
	return string(b)
}

func decodeInterfaceIPs(raw string) []string {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return nil
	}
	var ips []string
	if err := json.Unmarshal([]byte(raw), &ips); err != nil {
		return nil
	}
	return ips
}

func NewSQLiteStore(dbPath string) (*SQLiteStore, error) {
	db, err := sql.Open("sqlite", dbPath+"?_pragma=journal_mode(wal)&_pragma=foreign_keys(1)&_pragma=busy_timeout(5000)")
	if err != nil {
		return nil, fmt.Errorf("open database: %w", err)
	}
	if err := db.Ping(); err != nil {
		return nil, fmt.Errorf("ping database: %w", err)
	}
	s := &SQLiteStore{db: db}
	if err := s.migrate(); err != nil {
		return nil, fmt.Errorf("migrate: %w", err)
	}
	return s, nil
}

func (s *SQLiteStore) Close() error {
	return s.db.Close()
}

func (s *SQLiteStore) getUserVersion() int {
	var v int
	s.db.QueryRow("PRAGMA user_version").Scan(&v)
	return v
}

func (s *SQLiteStore) migrate() error {
	current := s.getUserVersion()
	for i := current; i < len(migrations); i++ {
		tx, err := s.db.Begin()
		if err != nil {
			return fmt.Errorf("begin tx for migration v%d: %w", i+1, err)
		}
		if err := migrations[i](tx); err != nil {
			tx.Rollback()
			return fmt.Errorf("migration v%d: %w", i+1, err)
		}
		if _, err := tx.Exec(fmt.Sprintf("PRAGMA user_version = %d", i+1)); err != nil {
			tx.Rollback()
			return fmt.Errorf("set user_version %d: %w", i+1, err)
		}
		if err := tx.Commit(); err != nil {
			return fmt.Errorf("commit migration v%d: %w", i+1, err)
		}
	}
	return nil
}

// --- Client operations ---

func (s *SQLiteStore) UpsertClient(req models.CheckInRequest, publicIP string) (string, bool, bool, error) {
	now := time.Now().UTC()
	interfaceIPsJSON := encodeInterfaceIPs(req.InterfaceIPs)

	// If client has an ID, try to update it
	if req.ClientID != "" {
		var isOnline bool
		var isDeleted bool
		var oldSessionID sql.NullString
		var oldSessionStartedAt sql.NullTime
		err := s.db.QueryRow("SELECT is_online, is_deleted, session_id, session_started_at FROM clients WHERE id = ?", req.ClientID).
			Scan(&isOnline, &isDeleted, &oldSessionID, &oldSessionStartedAt)
		if err == nil {
			// Client exists - update it
			wasOffline := !isOnline
			sessionChanged := req.SessionID != "" && oldSessionID.Valid && oldSessionID.String != "" && oldSessionID.String != req.SessionID
			_, err := s.db.Exec(`UPDATE clients SET hostname = ?, os = ?, arch = ?, client_version = ?,
				last_seen_at = ?, is_online = 1, is_deleted = 0, session_id = ?, public_ip = ?, interface_ips = ?,
				session_started_at = CASE WHEN ? THEN ? ELSE COALESCE(session_started_at, ?) END
				WHERE id = ?`,
				req.Hostname, req.OS, req.Arch, req.ClientVersion, now, req.SessionID, publicIP, interfaceIPsJSON,
				sessionChanged, now, now, req.ClientID)
			if err != nil {
				return "", false, false, fmt.Errorf("update client: %w", err)
			}
			return req.ClientID, wasOffline, sessionChanged, nil
		}
		// If not found, fall through to create
	}

	// Create new client
	id := uuid.New().String()
	_, err := s.db.Exec(`INSERT INTO clients (id, hostname, os, arch, client_version, first_seen_at, last_seen_at, session_started_at, is_online, session_id, public_ip, interface_ips)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, 1, ?, ?, ?)`,
		id, req.Hostname, req.OS, req.Arch, req.ClientVersion, now, now, now, req.SessionID, publicIP, interfaceIPsJSON)
	if err != nil {
		return "", false, false, fmt.Errorf("insert client: %w", err)
	}
	return id, false, false, nil
}

func (s *SQLiteStore) GetClient(id string) (*models.Client, error) {
	c := &models.Client{}
	var mutedUntil sql.NullTime
	var sessionStartedAt sql.NullTime
	var muteReason sql.NullString
	var offlineThresholdSecs sql.NullInt64
	var metricConsecutiveCheckins sql.NullInt64
	var interfaceIPsJSON string
	err := s.db.QueryRow(`SELECT id, hostname, custom_name, public_ip, interface_ips, os, arch, client_version, first_seen_at, last_seen_at, session_started_at,
		is_online, is_deleted, cpu_warn_pct, cpu_crit_pct, mem_warn_pct, mem_crit_pct,
		disk_warn_pct, disk_crit_pct, offline_threshold_seconds, metric_consecutive_checkins, alerts_muted, muted_until, mute_reason
		FROM clients WHERE id = ?`, id).Scan(
		&c.ID, &c.Hostname, &c.CustomName, &c.PublicIP, &interfaceIPsJSON, &c.OS, &c.Arch, &c.ClientVersion,
		&c.FirstSeenAt, &c.LastSeenAt, &sessionStartedAt, &c.IsOnline, &c.IsDeleted,
		&c.CPUWarnPct, &c.CPUCritPct, &c.MemWarnPct, &c.MemCritPct,
		&c.DiskWarnPct, &c.DiskCritPct, &offlineThresholdSecs, &metricConsecutiveCheckins, &c.AlertsMuted, &mutedUntil, &muteReason)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("get client: %w", err)
	}
	if mutedUntil.Valid {
		c.MutedUntil = &mutedUntil.Time
	}
	if sessionStartedAt.Valid {
		c.SessionStartedAt = sessionStartedAt.Time
	} else {
		c.SessionStartedAt = time.Now().UTC()
	}
	if muteReason.Valid {
		c.MuteReason = muteReason.String
	}
	if offlineThresholdSecs.Valid {
		v := int(offlineThresholdSecs.Int64)
		c.OfflineThresholdSeconds = &v
	}
	if metricConsecutiveCheckins.Valid {
		v := int(metricConsecutiveCheckins.Int64)
		c.MetricConsecutiveCheckins = &v
	}
	c.InterfaceIPs = decodeInterfaceIPs(interfaceIPsJSON)
	return c, nil
}

func (s *SQLiteStore) ListClients() ([]models.ClientWithMetrics, error) {
	rows, err := s.db.Query(`SELECT c.id, c.hostname, c.custom_name, c.public_ip, c.interface_ips, c.os, c.arch, c.client_version,
		c.first_seen_at, c.last_seen_at, c.session_started_at, c.is_online, c.alerts_muted, c.muted_until,
		c.cpu_warn_pct, c.cpu_crit_pct, c.mem_warn_pct, c.mem_crit_pct,
		c.disk_warn_pct, c.disk_crit_pct, c.offline_threshold_seconds, c.metric_consecutive_checkins,
		m.cpu_pct, m.mem_pct, m.disk_pct, m.mem_total_bytes, m.mem_used_bytes,
		m.disk_total_bytes, m.disk_used_bytes, m.recorded_at,
		(SELECT COUNT(*) FROM watched_processes wp WHERE wp.client_id = c.id) as proc_count
		FROM clients c
		LEFT JOIN metrics m ON m.client_id = c.id AND m.id = (
			SELECT id FROM metrics WHERE client_id = c.id ORDER BY recorded_at DESC LIMIT 1
		)
		WHERE c.is_deleted = 0
		ORDER BY COALESCE(NULLIF(c.custom_name, ''), c.hostname)`)
	if err != nil {
		return nil, fmt.Errorf("list clients: %w", err)
	}
	defer rows.Close()

	var result []models.ClientWithMetrics
	for rows.Next() {
		var cwm models.ClientWithMetrics
		var mutedUntil sql.NullTime
		var sessionStartedAt sql.NullTime
		var cpuPct, memPct, diskPct sql.NullFloat64
		var memTotal, memUsed, diskTotal, diskUsed sql.NullInt64
		var recordedAt sql.NullTime
		var offlineThresholdSecs sql.NullInt64
		var metricConsecutiveCheckins sql.NullInt64
		var interfaceIPsJSON string

		err := rows.Scan(
			&cwm.ID, &cwm.Hostname, &cwm.CustomName, &cwm.PublicIP, &interfaceIPsJSON, &cwm.OS, &cwm.Arch, &cwm.ClientVersion,
			&cwm.FirstSeenAt, &cwm.LastSeenAt, &sessionStartedAt, &cwm.IsOnline, &cwm.AlertsMuted, &mutedUntil,
			&cwm.CPUWarnPct, &cwm.CPUCritPct, &cwm.MemWarnPct, &cwm.MemCritPct,
			&cwm.DiskWarnPct, &cwm.DiskCritPct, &offlineThresholdSecs, &metricConsecutiveCheckins,
			&cpuPct, &memPct, &diskPct, &memTotal, &memUsed,
			&diskTotal, &diskUsed, &recordedAt,
			&cwm.ProcessCount,
		)
		if err != nil {
			return nil, fmt.Errorf("scan client row: %w", err)
		}
		if mutedUntil.Valid {
			cwm.MutedUntil = &mutedUntil.Time
		}
		if sessionStartedAt.Valid {
			cwm.SessionStartedAt = sessionStartedAt.Time
		} else {
			cwm.SessionStartedAt = time.Now().UTC()
		}
		if offlineThresholdSecs.Valid {
			v := int(offlineThresholdSecs.Int64)
			cwm.OfflineThresholdSeconds = &v
		}
		if metricConsecutiveCheckins.Valid {
			v := int(metricConsecutiveCheckins.Int64)
			cwm.MetricConsecutiveCheckins = &v
		}
		cwm.InterfaceIPs = decodeInterfaceIPs(interfaceIPsJSON)
		if cpuPct.Valid {
			cwm.LatestMetrics = &models.Metric{
				CPUPercent:     cpuPct.Float64,
				MemPercent:     memPct.Float64,
				DiskPercent:    diskPct.Float64,
				MemTotalBytes:  uint64(memTotal.Int64),
				MemUsedBytes:   uint64(memUsed.Int64),
				DiskTotalBytes: uint64(diskTotal.Int64),
				DiskUsedBytes:  uint64(diskUsed.Int64),
				RecordedAt:     recordedAt.Time,
			}
		}
		result = append(result, cwm)
	}
	return result, rows.Err()
}

func (s *SQLiteStore) DeleteClient(id string) error {
	_, err := s.db.Exec("UPDATE clients SET is_deleted = 1 WHERE id = ?", id)
	return err
}

func (s *SQLiteStore) SetClientOnline(id string, online bool) error {
	_, err := s.db.Exec("UPDATE clients SET is_online = ? WHERE id = ?", online, id)
	return err
}

func (s *SQLiteStore) GetOnlineClients() ([]models.Client, error) {
	rows, err := s.db.Query(`SELECT id, hostname, custom_name, public_ip, os, arch, last_seen_at, is_online,
		alerts_muted, muted_until, mute_reason, offline_threshold_seconds, metric_consecutive_checkins
		FROM clients WHERE is_online = 1 AND is_deleted = 0`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var clients []models.Client
	for rows.Next() {
		var c models.Client
		var mutedUntil sql.NullTime
		var muteReason sql.NullString
		var offlineThresholdSecs sql.NullInt64
		var metricConsecutiveCheckins sql.NullInt64
		err := rows.Scan(&c.ID, &c.Hostname, &c.CustomName, &c.PublicIP, &c.OS, &c.Arch, &c.LastSeenAt, &c.IsOnline,
			&c.AlertsMuted, &mutedUntil, &muteReason, &offlineThresholdSecs, &metricConsecutiveCheckins)
		if err != nil {
			return nil, err
		}
		if mutedUntil.Valid {
			c.MutedUntil = &mutedUntil.Time
		}
		if muteReason.Valid {
			c.MuteReason = muteReason.String
		}
		if offlineThresholdSecs.Valid {
			v := int(offlineThresholdSecs.Int64)
			c.OfflineThresholdSeconds = &v
		}
		if metricConsecutiveCheckins.Valid {
			v := int(metricConsecutiveCheckins.Int64)
			c.MetricConsecutiveCheckins = &v
		}
		clients = append(clients, c)
	}
	return clients, rows.Err()
}

// GetStaleOnlineClients returns clients marked online whose last_seen_at
// is older than thresholdSeconds. The comparison uses SQLite's datetime('now')
// to avoid Go/SQLite timezone mismatches.
func (s *SQLiteStore) GetStaleOnlineClients(thresholdSeconds int) ([]models.Client, error) {
	rows, err := s.db.Query(`SELECT id, hostname, custom_name, public_ip, os, arch, last_seen_at, is_online,
		alerts_muted, muted_until, mute_reason, offline_threshold_seconds, metric_consecutive_checkins
		FROM clients
		WHERE is_online = 1 AND is_deleted = 0
		AND last_seen_at < datetime('now', ? || ' seconds')`,
		fmt.Sprintf("-%d", thresholdSeconds))
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var clients []models.Client
	for rows.Next() {
		var c models.Client
		var mutedUntil sql.NullTime
		var muteReason sql.NullString
		var offlineThresholdSecs sql.NullInt64
		var metricConsecutiveCheckins sql.NullInt64
		err := rows.Scan(&c.ID, &c.Hostname, &c.CustomName, &c.PublicIP, &c.OS, &c.Arch, &c.LastSeenAt, &c.IsOnline,
			&c.AlertsMuted, &mutedUntil, &muteReason, &offlineThresholdSecs, &metricConsecutiveCheckins)
		if err != nil {
			return nil, err
		}
		if mutedUntil.Valid {
			c.MutedUntil = &mutedUntil.Time
		}
		if muteReason.Valid {
			c.MuteReason = muteReason.String
		}
		if offlineThresholdSecs.Valid {
			v := int(offlineThresholdSecs.Int64)
			c.OfflineThresholdSeconds = &v
		}
		if metricConsecutiveCheckins.Valid {
			v := int(metricConsecutiveCheckins.Int64)
			c.MetricConsecutiveCheckins = &v
		}
		clients = append(clients, c)
	}
	return clients, rows.Err()
}

func (s *SQLiteStore) SetClientThresholds(id string, t *models.Thresholds) error {
	if t == nil {
		_, err := s.db.Exec(`UPDATE clients SET cpu_warn_pct = NULL, cpu_crit_pct = NULL,
			mem_warn_pct = NULL, mem_crit_pct = NULL, disk_warn_pct = NULL, disk_crit_pct = NULL,
			offline_threshold_seconds = NULL, metric_consecutive_checkins = NULL
			WHERE id = ?`, id)
		return err
	}
	offlineOverrideProvided := t.OfflineThresholdMinutes != nil
	offlineThresholdSecs := 0
	if offlineOverrideProvided && *t.OfflineThresholdMinutes > 0 {
		offlineThresholdSecs = *t.OfflineThresholdMinutes * 60
	}
	consecutiveOverrideProvided := t.MetricConsecutiveCheckins != nil
	consecutiveThreshold := 0
	if consecutiveOverrideProvided && *t.MetricConsecutiveCheckins > 0 {
		consecutiveThreshold = *t.MetricConsecutiveCheckins
	}
	_, err := s.db.Exec(`UPDATE clients SET cpu_warn_pct = ?, cpu_crit_pct = ?,
		mem_warn_pct = ?, mem_crit_pct = ?, disk_warn_pct = ?, disk_crit_pct = ?,
		offline_threshold_seconds = CASE
			WHEN ? THEN NULLIF(?, 0)
			ELSE offline_threshold_seconds
		END,
		metric_consecutive_checkins = CASE
			WHEN ? THEN NULLIF(?, 0)
			ELSE metric_consecutive_checkins
		END
		WHERE id = ?`,
		t.CPUWarnPct, t.CPUCritPct, t.MemWarnPct, t.MemCritPct, t.DiskWarnPct, t.DiskCritPct,
		offlineOverrideProvided, offlineThresholdSecs,
		consecutiveOverrideProvided, consecutiveThreshold,
		id)
	return err
}

func (s *SQLiteStore) SetClientCustomName(id, customName string) error {
	_, err := s.db.Exec(`UPDATE clients SET custom_name = ? WHERE id = ?`, strings.TrimSpace(customName), id)
	return err
}

func (s *SQLiteStore) SetClientMute(id string, muted bool, until *time.Time, reason string) error {
	var mutedUntil interface{}
	if until != nil {
		mutedUntil = *until
	}
	_, err := s.db.Exec(`UPDATE clients SET alerts_muted = ?, muted_until = ?, mute_reason = ? WHERE id = ?`,
		muted, mutedUntil, reason, id)
	return err
}

func (s *SQLiteStore) ListClientAlertMutes(clientID string) ([]models.ClientAlertMute, error) {
	rows, err := s.db.Query(`SELECT id, client_id, scope, target, created_at
		FROM client_alert_mutes
		WHERE client_id = ?
		ORDER BY scope, target`, clientID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []models.ClientAlertMute
	for rows.Next() {
		var m models.ClientAlertMute
		if err := rows.Scan(&m.ID, &m.ClientID, &m.Scope, &m.Target, &m.CreatedAt); err != nil {
			return nil, err
		}
		out = append(out, m)
	}
	return out, rows.Err()
}

func (s *SQLiteStore) SetClientAlertMute(clientID, scope, target string, muted bool) error {
	scope = strings.TrimSpace(scope)
	target = strings.TrimSpace(target)
	if muted {
		_, err := s.db.Exec(`INSERT INTO client_alert_mutes (client_id, scope, target)
			VALUES (?, ?, ?)
			ON CONFLICT(client_id, scope, target) DO NOTHING`, clientID, scope, target)
		return err
	}
	_, err := s.db.Exec(`DELETE FROM client_alert_mutes WHERE client_id = ? AND scope = ? AND target = ?`,
		clientID, scope, target)
	return err
}

// --- Metrics ---

func (s *SQLiteStore) InsertMetrics(clientID string, m models.MetricsPayload) error {
	_, err := s.db.Exec(`INSERT INTO metrics (client_id, cpu_pct, mem_pct, disk_pct,
		mem_total_bytes, mem_used_bytes, disk_total_bytes, disk_used_bytes)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?)`,
		clientID, m.CPUPercent, m.MemPercent, m.DiskPercent,
		m.MemTotalBytes, m.MemUsedBytes, m.DiskTotalBytes, m.DiskUsedBytes)
	return err
}

func (s *SQLiteStore) GetLatestMetrics(clientID string) (*models.Metric, error) {
	m := &models.Metric{}
	err := s.db.QueryRow(`SELECT id, client_id, recorded_at, cpu_pct, mem_pct, disk_pct,
		mem_total_bytes, mem_used_bytes, disk_total_bytes, disk_used_bytes
		FROM metrics WHERE client_id = ? ORDER BY recorded_at DESC LIMIT 1`, clientID).Scan(
		&m.ID, &m.ClientID, &m.RecordedAt, &m.CPUPercent, &m.MemPercent, &m.DiskPercent,
		&m.MemTotalBytes, &m.MemUsedBytes, &m.DiskTotalBytes, &m.DiskUsedBytes)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return m, nil
}

func (s *SQLiteStore) GetMetrics(clientID string, from, to time.Time, limit int) ([]models.Metric, error) {
	if limit <= 0 {
		limit = 500
	}
	fromUTC := from.UTC().Format("2006-01-02 15:04:05")
	toUTC := to.UTC().Format("2006-01-02 15:04:05")
	rows, err := s.db.Query(`SELECT id, client_id, recorded_at, cpu_pct, mem_pct, disk_pct,
		mem_total_bytes, mem_used_bytes, disk_total_bytes, disk_used_bytes
		FROM metrics
		WHERE client_id = ?
			AND datetime(recorded_at) >= datetime(?)
			AND datetime(recorded_at) <= datetime(?)
		ORDER BY recorded_at ASC LIMIT ?`, clientID, fromUTC, toUTC, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var metrics []models.Metric
	for rows.Next() {
		var m models.Metric
		err := rows.Scan(&m.ID, &m.ClientID, &m.RecordedAt, &m.CPUPercent, &m.MemPercent, &m.DiskPercent,
			&m.MemTotalBytes, &m.MemUsedBytes, &m.DiskTotalBytes, &m.DiskUsedBytes)
		if err != nil {
			return nil, err
		}
		metrics = append(metrics, m)
	}
	return metrics, rows.Err()
}

func (s *SQLiteStore) GetRecentMetrics(clientID string, limit int) ([]models.Metric, error) {
	if limit <= 0 {
		return []models.Metric{}, nil
	}
	rows, err := s.db.Query(`SELECT id, client_id, recorded_at, cpu_pct, mem_pct, disk_pct,
		mem_total_bytes, mem_used_bytes, disk_total_bytes, disk_used_bytes
		FROM metrics
		WHERE client_id = ?
		ORDER BY recorded_at DESC
		LIMIT ?`, clientID, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var metrics []models.Metric
	for rows.Next() {
		var m models.Metric
		err := rows.Scan(&m.ID, &m.ClientID, &m.RecordedAt, &m.CPUPercent, &m.MemPercent, &m.DiskPercent,
			&m.MemTotalBytes, &m.MemUsedBytes, &m.DiskTotalBytes, &m.DiskUsedBytes)
		if err != nil {
			return nil, err
		}
		metrics = append(metrics, m)
	}
	return metrics, rows.Err()
}

// --- Process tracking ---

func (s *SQLiteStore) UpsertWatchedProcesses(clientID string, procs []models.ProcessPayload) error {
	// If the client sends no watched processes, clear them all.
	if len(procs) == 0 {
		tx, err := s.db.Begin()
		if err != nil {
			return err
		}
		defer tx.Rollback()
		if _, err := tx.Exec(`DELETE FROM watched_processes WHERE client_id = ?`, clientID); err != nil {
			return err
		}
		if _, err := tx.Exec(`DELETE FROM process_snapshots WHERE client_id = ?`, clientID); err != nil {
			return err
		}
		return tx.Commit()
	}

	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	// Remove processes no longer configured by the client.
	placeholders := make([]string, len(procs))
	args := make([]interface{}, 0, len(procs)+1)
	args = append(args, clientID)
	for i, p := range procs {
		placeholders[i] = "?"
		args = append(args, p.FriendlyName)
	}
	deleteSQL := fmt.Sprintf(`DELETE FROM watched_processes
		WHERE client_id = ? AND friendly_name NOT IN (%s)`, strings.Join(placeholders, ","))
	if _, err := tx.Exec(deleteSQL, args...); err != nil {
		return fmt.Errorf("delete stale watched processes: %w", err)
	}
	deleteSnapshotsSQL := fmt.Sprintf(`DELETE FROM process_snapshots
		WHERE client_id = ? AND friendly_name NOT IN (%s)`, strings.Join(placeholders, ","))
	if _, err := tx.Exec(deleteSnapshotsSQL, args...); err != nil {
		return fmt.Errorf("delete stale process snapshots: %w", err)
	}

	for _, p := range procs {
		_, err := tx.Exec(`INSERT INTO watched_processes (client_id, friendly_name, match_pattern, match_type)
			VALUES (?, ?, ?, 'substring')
			ON CONFLICT(client_id, friendly_name) DO UPDATE SET match_pattern = excluded.match_pattern`,
			clientID, p.FriendlyName, p.MatchPattern)
		if err != nil {
			return fmt.Errorf("upsert watched process %q: %w", p.FriendlyName, err)
		}
	}
	return tx.Commit()
}

func (s *SQLiteStore) DeleteWatchedProcess(clientID, friendlyName string) error {
	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	if _, err := tx.Exec(`DELETE FROM watched_processes WHERE client_id = ? AND friendly_name = ?`, clientID, friendlyName); err != nil {
		return err
	}
	if _, err := tx.Exec(`DELETE FROM process_snapshots WHERE client_id = ? AND friendly_name = ?`, clientID, friendlyName); err != nil {
		return err
	}
	return tx.Commit()
}

func (s *SQLiteStore) DeleteCheckSnapshots(clientID, friendlyName, checkType string) error {
	if strings.TrimSpace(checkType) == "" {
		_, err := s.db.Exec(`DELETE FROM check_snapshots WHERE client_id = ? AND friendly_name = ?`, clientID, friendlyName)
		return err
	}
	_, err := s.db.Exec(`DELETE FROM check_snapshots WHERE client_id = ? AND friendly_name = ? AND check_type = ?`, clientID, friendlyName, checkType)
	return err
}

func (s *SQLiteStore) InsertProcessSnapshots(clientID string, procs []models.ProcessPayload) error {
	if len(procs) == 0 {
		return nil
	}
	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	previous, err := getLatestProcessSnapshotStatesTx(tx, clientID)
	if err != nil {
		return err
	}

	stmt, err := tx.Prepare(`INSERT INTO process_snapshots (client_id, friendly_name, is_running, pid, cpu_pct, mem_pct, cmdline, uptime_since_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?)`)
	if err != nil {
		return err
	}
	defer stmt.Close()

	now := time.Now().UTC()
	for _, p := range procs {
		pidPtr := pidPointer(p.PID)
		uptimeSince := now
		if prev, ok := previous[p.FriendlyName]; ok {
			if prev.IsRunning == p.IsRunning && pidEqual(prev.PID, pidPtr) && prev.UptimeSinceAt.Valid {
				uptimeSince = prev.UptimeSinceAt.Time.UTC()
			}
		}

		var pid interface{}
		if pidPtr != nil {
			pid = *pidPtr
		}
		_, err := stmt.Exec(clientID, p.FriendlyName, p.IsRunning, pid, p.CPUPercent, p.MemPercent, p.Cmdline, uptimeSince)
		if err != nil {
			return err
		}
	}
	return tx.Commit()
}

func (s *SQLiteStore) GetLatestProcessSnapshots(clientID string) ([]models.ProcessSnapshot, error) {
	rows, err := s.db.Query(`SELECT ps.id, ps.client_id, ps.friendly_name, ps.recorded_at,
		ps.uptime_since_at, ps.is_running, ps.pid, ps.cpu_pct, ps.mem_pct, ps.cmdline
		FROM process_snapshots ps
		INNER JOIN watched_processes wp ON wp.client_id = ps.client_id AND wp.friendly_name = ps.friendly_name
		INNER JOIN (
			SELECT ps2.friendly_name, MAX(ps2.recorded_at) as max_time
			FROM process_snapshots ps2
			INNER JOIN watched_processes wp2 ON wp2.client_id = ps2.client_id AND wp2.friendly_name = ps2.friendly_name
			WHERE ps2.client_id = ?
			GROUP BY ps2.friendly_name
		) latest ON ps.friendly_name = latest.friendly_name AND ps.recorded_at = latest.max_time
		WHERE ps.client_id = ?`, clientID, clientID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanProcessSnapshots(rows)
}

func (s *SQLiteStore) GetPreviousProcessSnapshots(clientID string) ([]models.ProcessSnapshot, error) {
	// Get the second-most-recent snapshot for each process
	rows, err := s.db.Query(`SELECT ps.id, ps.client_id, ps.friendly_name, ps.recorded_at,
		ps.uptime_since_at, ps.is_running, ps.pid, ps.cpu_pct, ps.mem_pct, ps.cmdline
		FROM process_snapshots ps
		INNER JOIN watched_processes wp ON wp.client_id = ps.client_id AND wp.friendly_name = ps.friendly_name
		INNER JOIN (
			SELECT ps2.friendly_name, MAX(ps2.recorded_at) as max_time
			FROM process_snapshots ps2
			INNER JOIN watched_processes wp2 ON wp2.client_id = ps2.client_id AND wp2.friendly_name = ps2.friendly_name
			WHERE ps2.client_id = ? AND ps2.recorded_at < (
				SELECT MAX(ps3.recorded_at) FROM process_snapshots ps3
				INNER JOIN watched_processes wp3 ON wp3.client_id = ps3.client_id AND wp3.friendly_name = ps3.friendly_name
				WHERE ps3.client_id = ?
			)
			GROUP BY ps2.friendly_name
		) prev ON ps.friendly_name = prev.friendly_name AND ps.recorded_at = prev.max_time
		WHERE ps.client_id = ?`, clientID, clientID, clientID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanProcessSnapshots(rows)
}

func (s *SQLiteStore) GetWatchedProcesses(clientID string) ([]models.WatchedProcess, error) {
	rows, err := s.db.Query(`SELECT id, client_id, friendly_name, match_pattern, match_type
		FROM watched_processes WHERE client_id = ?`, clientID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var procs []models.WatchedProcess
	for rows.Next() {
		var p models.WatchedProcess
		if err := rows.Scan(&p.ID, &p.ClientID, &p.FriendlyName, &p.MatchPattern, &p.MatchType); err != nil {
			return nil, err
		}
		procs = append(procs, p)
	}
	return procs, rows.Err()
}

func scanProcessSnapshots(rows *sql.Rows) ([]models.ProcessSnapshot, error) {
	var snaps []models.ProcessSnapshot
	for rows.Next() {
		var ps models.ProcessSnapshot
		var pid sql.NullInt32
		var uptimeSince sql.NullTime
		var cpuPct, memPct sql.NullFloat64
		var cmdline sql.NullString
		err := rows.Scan(&ps.ID, &ps.ClientID, &ps.FriendlyName, &ps.RecordedAt,
			&uptimeSince, &ps.IsRunning, &pid, &cpuPct, &memPct, &cmdline)
		if err != nil {
			return nil, err
		}
		if uptimeSince.Valid {
			ps.UptimeSinceAt = uptimeSince.Time
		} else {
			ps.UptimeSinceAt = ps.RecordedAt
		}
		if pid.Valid {
			v := pid.Int32
			ps.PID = &v
		}
		ps.CPUPercent = cpuPct.Float64
		ps.MemPercent = memPct.Float64
		ps.Cmdline = cmdline.String
		snaps = append(snaps, ps)
	}
	return snaps, rows.Err()
}

type processSnapshotState struct {
	IsRunning     bool
	PID           *int32
	UptimeSinceAt sql.NullTime
}

func getLatestProcessSnapshotStatesTx(tx *sql.Tx, clientID string) (map[string]processSnapshotState, error) {
	rows, err := tx.Query(`SELECT ps.friendly_name, ps.is_running, ps.pid, ps.uptime_since_at
		FROM process_snapshots ps
		INNER JOIN (
			SELECT friendly_name, MAX(recorded_at) as max_time
			FROM process_snapshots
			WHERE client_id = ?
			GROUP BY friendly_name
		) latest ON ps.friendly_name = latest.friendly_name AND ps.recorded_at = latest.max_time
		WHERE ps.client_id = ?`, clientID, clientID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	states := make(map[string]processSnapshotState)
	for rows.Next() {
		var name string
		var state processSnapshotState
		var pid sql.NullInt32
		if err := rows.Scan(&name, &state.IsRunning, &pid, &state.UptimeSinceAt); err != nil {
			return nil, err
		}
		if pid.Valid {
			v := pid.Int32
			state.PID = &v
		}
		states[name] = state
	}
	return states, rows.Err()
}

func pidPointer(pid int32) *int32 {
	if pid <= 0 {
		return nil
	}
	v := pid
	return &v
}

func pidEqual(a, b *int32) bool {
	if a == nil || b == nil {
		return a == nil && b == nil
	}
	return *a == *b
}

// --- Checks (extensible typed check system) ---

func (s *SQLiteStore) InsertCheckSnapshots(clientID string, checks []models.CheckPayload) error {
	if len(checks) == 0 {
		return nil
	}
	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	previous, err := getLatestCheckSnapshotStatesTx(tx, clientID)
	if err != nil {
		return err
	}

	stmt, err := tx.Prepare(`INSERT INTO check_snapshots (client_id, friendly_name, check_type, healthy, message, state, uptime_since_at)
		VALUES (?, ?, ?, ?, ?, ?, ?)`)
	if err != nil {
		return err
	}
	defer stmt.Close()

	now := time.Now().UTC()
	for _, c := range checks {
		uptimeSince := now
		key := checkSnapshotKey(c.FriendlyName, c.CheckType)
		if prev, ok := previous[key]; ok {
			if prev.Healthy == c.Healthy && prev.UptimeSinceAt.Valid {
				uptimeSince = prev.UptimeSinceAt.Time.UTC()
			}
		}
		_, err := stmt.Exec(clientID, c.FriendlyName, c.CheckType, c.Healthy, c.Message, c.State, uptimeSince)
		if err != nil {
			return err
		}
	}
	return tx.Commit()
}

func (s *SQLiteStore) GetLatestCheckSnapshots(clientID string) ([]models.CheckSnapshot, error) {
	rows, err := s.db.Query(`SELECT cs.id, cs.client_id, cs.friendly_name, cs.check_type,
		cs.recorded_at, cs.uptime_since_at, cs.healthy, cs.message, cs.state
		FROM check_snapshots cs
		INNER JOIN (
			SELECT friendly_name, check_type, MAX(recorded_at) as max_time
			FROM check_snapshots WHERE client_id = ?
			GROUP BY friendly_name, check_type
		) latest ON cs.friendly_name = latest.friendly_name AND cs.check_type = latest.check_type AND cs.recorded_at = latest.max_time
		WHERE cs.client_id = ?`, clientID, clientID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanCheckSnapshots(rows)
}

func (s *SQLiteStore) GetPreviousCheckSnapshots(clientID string) ([]models.CheckSnapshot, error) {
	rows, err := s.db.Query(`SELECT cs.id, cs.client_id, cs.friendly_name, cs.check_type,
		cs.recorded_at, cs.uptime_since_at, cs.healthy, cs.message, cs.state
		FROM check_snapshots cs
		INNER JOIN (
			SELECT friendly_name, check_type, MAX(recorded_at) as max_time
			FROM check_snapshots
			WHERE client_id = ? AND recorded_at < (
				SELECT MAX(recorded_at) FROM check_snapshots WHERE client_id = ?
			)
			GROUP BY friendly_name, check_type
		) prev ON cs.friendly_name = prev.friendly_name AND cs.check_type = prev.check_type AND cs.recorded_at = prev.max_time
		WHERE cs.client_id = ?`, clientID, clientID, clientID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanCheckSnapshots(rows)
}

func scanCheckSnapshots(rows *sql.Rows) ([]models.CheckSnapshot, error) {
	var snaps []models.CheckSnapshot
	for rows.Next() {
		var cs models.CheckSnapshot
		var uptimeSince sql.NullTime
		var message, state sql.NullString
		err := rows.Scan(&cs.ID, &cs.ClientID, &cs.FriendlyName, &cs.CheckType,
			&cs.RecordedAt, &uptimeSince, &cs.Healthy, &message, &state)
		if err != nil {
			return nil, err
		}
		if uptimeSince.Valid {
			cs.UptimeSinceAt = uptimeSince.Time
		} else {
			cs.UptimeSinceAt = cs.RecordedAt
		}
		cs.Message = message.String
		cs.State = state.String
		snaps = append(snaps, cs)
	}
	return snaps, rows.Err()
}

type checkSnapshotState struct {
	Healthy       bool
	UptimeSinceAt sql.NullTime
}

func getLatestCheckSnapshotStatesTx(tx *sql.Tx, clientID string) (map[string]checkSnapshotState, error) {
	rows, err := tx.Query(`SELECT cs.friendly_name, cs.check_type, cs.healthy, cs.uptime_since_at
		FROM check_snapshots cs
		INNER JOIN (
			SELECT friendly_name, check_type, MAX(recorded_at) as max_time
			FROM check_snapshots
			WHERE client_id = ?
			GROUP BY friendly_name, check_type
		) latest ON cs.friendly_name = latest.friendly_name AND cs.check_type = latest.check_type AND cs.recorded_at = latest.max_time
		WHERE cs.client_id = ?`, clientID, clientID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	states := make(map[string]checkSnapshotState)
	for rows.Next() {
		var friendlyName, checkType string
		var state checkSnapshotState
		if err := rows.Scan(&friendlyName, &checkType, &state.Healthy, &state.UptimeSinceAt); err != nil {
			return nil, err
		}
		states[checkSnapshotKey(friendlyName, checkType)] = state
	}
	return states, rows.Err()
}

func checkSnapshotKey(friendlyName, checkType string) string {
	return strings.TrimSpace(friendlyName) + "::" + strings.TrimSpace(checkType)
}

// --- Alerts ---

func (s *SQLiteStore) InsertAlert(a *models.Alert) error {
	result, err := s.db.Exec(`INSERT INTO alerts (client_id, alert_type, severity, message, details)
		VALUES (?, ?, ?, ?, ?)`,
		a.ClientID, a.AlertType, a.Severity, a.Message, a.Details)
	if err != nil {
		return err
	}
	id, _ := result.LastInsertId()
	a.ID = id
	return nil
}

func (s *SQLiteStore) MarkAlertNotified(id int64) error {
	_, err := s.db.Exec("UPDATE alerts SET notified = 1, notified_at = datetime('now') WHERE id = ?", id)
	return err
}

func (s *SQLiteStore) GetUnnotifiedAlerts() ([]models.Alert, error) {
	rows, err := s.db.Query(`SELECT id, client_id, alert_type, severity, message, details, fired_at
		FROM alerts WHERE notified = 0 ORDER BY fired_at ASC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanAlerts(rows)
}

func (s *SQLiteStore) ListAlerts(clientID string, severity string, limit, offset int) ([]models.Alert, int, error) {
	if limit <= 0 {
		limit = 100
	}
	var conditions []string
	var args []interface{}

	if clientID != "" {
		conditions = append(conditions, "client_id = ?")
		args = append(args, clientID)
	}
	if severity != "" {
		conditions = append(conditions, "severity = ?")
		args = append(args, severity)
	}

	where := ""
	if len(conditions) > 0 {
		where = "WHERE " + strings.Join(conditions, " AND ")
	}

	var total int
	err := s.db.QueryRow("SELECT COUNT(*) FROM alerts "+where, args...).Scan(&total)
	if err != nil {
		return nil, 0, err
	}

	queryArgs := append(args, limit, offset)
	rows, err := s.db.Query(fmt.Sprintf(`SELECT id, client_id, alert_type, severity, message, details, fired_at
		FROM alerts %s ORDER BY fired_at DESC LIMIT ? OFFSET ?`, where), queryArgs...)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	alerts, err := scanAlerts(rows)
	return alerts, total, err
}

func (s *SQLiteStore) GetLastAlertByTypes(clientID string, types ...string) (*models.Alert, error) {
	if len(types) == 0 {
		return nil, nil
	}
	placeholders := make([]string, len(types))
	args := []interface{}{clientID}
	for i, t := range types {
		placeholders[i] = "?"
		args = append(args, t)
	}
	a := &models.Alert{}
	var details sql.NullString
	err := s.db.QueryRow(fmt.Sprintf(`SELECT id, client_id, alert_type, severity, message, details, fired_at
		FROM alerts WHERE client_id = ? AND alert_type IN (%s)
		ORDER BY fired_at DESC LIMIT 1`, strings.Join(placeholders, ",")), args...).Scan(
		&a.ID, &a.ClientID, &a.AlertType, &a.Severity, &a.Message, &details, &a.FiredAt)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	a.Details = details.String
	return a, nil
}

func scanAlerts(rows *sql.Rows) ([]models.Alert, error) {
	var alerts []models.Alert
	for rows.Next() {
		var a models.Alert
		var details sql.NullString
		err := rows.Scan(&a.ID, &a.ClientID, &a.AlertType, &a.Severity, &a.Message, &details, &a.FiredAt)
		if err != nil {
			return nil, err
		}
		a.Details = details.String
		alerts = append(alerts, a)
	}
	return alerts, rows.Err()
}

// --- Alert providers ---

func (s *SQLiteStore) ListProviders() ([]models.AlertProvider, error) {
	rows, err := s.db.Query("SELECT id, type, name, enabled, config, created_at FROM alert_providers ORDER BY name")
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanProviders(rows)
}

func (s *SQLiteStore) GetProvider(id int64) (*models.AlertProvider, error) {
	p := &models.AlertProvider{}
	err := s.db.QueryRow("SELECT id, type, name, enabled, config, created_at FROM alert_providers WHERE id = ?", id).
		Scan(&p.ID, &p.Type, &p.Name, &p.Enabled, &p.Config, &p.CreatedAt)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return p, nil
}

func (s *SQLiteStore) CreateProvider(p *models.AlertProvider) error {
	result, err := s.db.Exec("INSERT INTO alert_providers (type, name, enabled, config) VALUES (?, ?, ?, ?)",
		p.Type, p.Name, p.Enabled, p.Config)
	if err != nil {
		return err
	}
	id, _ := result.LastInsertId()
	p.ID = id
	return nil
}

func (s *SQLiteStore) UpdateProvider(p *models.AlertProvider) error {
	_, err := s.db.Exec("UPDATE alert_providers SET type = ?, name = ?, enabled = ?, config = ? WHERE id = ?",
		p.Type, p.Name, p.Enabled, p.Config, p.ID)
	return err
}

func (s *SQLiteStore) DeleteProvider(id int64) error {
	_, err := s.db.Exec("DELETE FROM alert_providers WHERE id = ?", id)
	return err
}

func (s *SQLiteStore) GetEnabledProviders() ([]models.AlertProvider, error) {
	rows, err := s.db.Query("SELECT id, type, name, enabled, config, created_at FROM alert_providers WHERE enabled = 1")
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanProviders(rows)
}

func scanProviders(rows *sql.Rows) ([]models.AlertProvider, error) {
	var providers []models.AlertProvider
	for rows.Next() {
		var p models.AlertProvider
		if err := rows.Scan(&p.ID, &p.Type, &p.Name, &p.Enabled, &p.Config, &p.CreatedAt); err != nil {
			return nil, err
		}
		providers = append(providers, p)
	}
	return providers, rows.Err()
}

// --- Settings ---

func (s *SQLiteStore) GetSetting(key string) (string, error) {
	var value string
	err := s.db.QueryRow("SELECT value FROM global_settings WHERE key = ?", key).Scan(&value)
	if err == sql.ErrNoRows {
		return "", nil
	}
	return value, err
}

func (s *SQLiteStore) SetSetting(key, value string) error {
	_, err := s.db.Exec(`INSERT INTO global_settings (key, value) VALUES (?, ?)
		ON CONFLICT(key) DO UPDATE SET value = excluded.value`, key, value)
	return err
}

func (s *SQLiteStore) GetAllSettings() (map[string]string, error) {
	rows, err := s.db.Query("SELECT key, value FROM global_settings")
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	settings := make(map[string]string)
	for rows.Next() {
		var k, v string
		if err := rows.Scan(&k, &v); err != nil {
			return nil, err
		}
		settings[k] = v
	}
	return settings, rows.Err()
}

// --- Maintenance ---

func (s *SQLiteStore) PruneOldData(metricsRetention, alertsRetention time.Duration) (int64, error) {
	var totalDeleted int64

	metricsCutoff := time.Now().Add(-metricsRetention)
	result, err := s.db.Exec("DELETE FROM metrics WHERE recorded_at < ?", metricsCutoff)
	if err != nil {
		return 0, fmt.Errorf("prune metrics: %w", err)
	}
	n, _ := result.RowsAffected()
	totalDeleted += n

	result, err = s.db.Exec("DELETE FROM process_snapshots WHERE recorded_at < ?", metricsCutoff)
	if err != nil {
		return totalDeleted, fmt.Errorf("prune process snapshots: %w", err)
	}
	n, _ = result.RowsAffected()
	totalDeleted += n

	result, err = s.db.Exec("DELETE FROM check_snapshots WHERE recorded_at < ?", metricsCutoff)
	if err != nil {
		return totalDeleted, fmt.Errorf("prune check snapshots: %w", err)
	}
	n, _ = result.RowsAffected()
	totalDeleted += n

	alertsCutoff := time.Now().Add(-alertsRetention)
	result, err = s.db.Exec("DELETE FROM alerts WHERE fired_at < ?", alertsCutoff)
	if err != nil {
		return totalDeleted, fmt.Errorf("prune alerts: %w", err)
	}
	n, _ = result.RowsAffected()
	totalDeleted += n

	return totalDeleted, nil
}
