package store

import (
	"database/sql"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/machinemon/machinemon/internal/models"
	_ "modernc.org/sqlite"
)

type SQLiteStore struct {
	db *sql.DB
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

func (s *SQLiteStore) UpsertClient(req models.CheckInRequest) (string, bool, bool, error) {
	now := time.Now().UTC()

	// If client has an ID, try to update it
	if req.ClientID != "" {
		var isOnline bool
		var isDeleted bool
		var oldSessionID sql.NullString
		err := s.db.QueryRow("SELECT is_online, is_deleted, session_id FROM clients WHERE id = ?", req.ClientID).Scan(&isOnline, &isDeleted, &oldSessionID)
		if err == nil {
			// Client exists - update it
			wasOffline := !isOnline
			sessionChanged := req.SessionID != "" && oldSessionID.Valid && oldSessionID.String != "" && oldSessionID.String != req.SessionID
			_, err := s.db.Exec(`UPDATE clients SET hostname = ?, os = ?, arch = ?, client_version = ?,
				last_seen_at = ?, is_online = 1, is_deleted = 0, session_id = ? WHERE id = ?`,
				req.Hostname, req.OS, req.Arch, req.ClientVersion, now, req.SessionID, req.ClientID)
			if err != nil {
				return "", false, false, fmt.Errorf("update client: %w", err)
			}
			return req.ClientID, wasOffline, sessionChanged, nil
		}
		// If not found, fall through to create
	}

	// Create new client
	id := uuid.New().String()
	_, err := s.db.Exec(`INSERT INTO clients (id, hostname, os, arch, client_version, first_seen_at, last_seen_at, is_online, session_id)
		VALUES (?, ?, ?, ?, ?, ?, ?, 1, ?)`,
		id, req.Hostname, req.OS, req.Arch, req.ClientVersion, now, now, req.SessionID)
	if err != nil {
		return "", false, false, fmt.Errorf("insert client: %w", err)
	}
	return id, false, false, nil
}

func (s *SQLiteStore) GetClient(id string) (*models.Client, error) {
	c := &models.Client{}
	var mutedUntil sql.NullTime
	var muteReason sql.NullString
	err := s.db.QueryRow(`SELECT id, hostname, os, arch, client_version, first_seen_at, last_seen_at,
		is_online, is_deleted, cpu_warn_pct, cpu_crit_pct, mem_warn_pct, mem_crit_pct,
		disk_warn_pct, disk_crit_pct, alerts_muted, muted_until, mute_reason
		FROM clients WHERE id = ?`, id).Scan(
		&c.ID, &c.Hostname, &c.OS, &c.Arch, &c.ClientVersion,
		&c.FirstSeenAt, &c.LastSeenAt, &c.IsOnline, &c.IsDeleted,
		&c.CPUWarnPct, &c.CPUCritPct, &c.MemWarnPct, &c.MemCritPct,
		&c.DiskWarnPct, &c.DiskCritPct, &c.AlertsMuted, &mutedUntil, &muteReason)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("get client: %w", err)
	}
	if mutedUntil.Valid {
		c.MutedUntil = &mutedUntil.Time
	}
	if muteReason.Valid {
		c.MuteReason = muteReason.String
	}
	return c, nil
}

func (s *SQLiteStore) ListClients() ([]models.ClientWithMetrics, error) {
	rows, err := s.db.Query(`SELECT c.id, c.hostname, c.os, c.arch, c.client_version,
		c.first_seen_at, c.last_seen_at, c.is_online, c.alerts_muted, c.muted_until,
		c.cpu_warn_pct, c.cpu_crit_pct, c.mem_warn_pct, c.mem_crit_pct,
		c.disk_warn_pct, c.disk_crit_pct,
		m.cpu_pct, m.mem_pct, m.disk_pct, m.mem_total_bytes, m.mem_used_bytes,
		m.disk_total_bytes, m.disk_used_bytes, m.recorded_at,
		(SELECT COUNT(*) FROM watched_processes wp WHERE wp.client_id = c.id) as proc_count
		FROM clients c
		LEFT JOIN metrics m ON m.client_id = c.id AND m.id = (
			SELECT id FROM metrics WHERE client_id = c.id ORDER BY recorded_at DESC LIMIT 1
		)
		WHERE c.is_deleted = 0
		ORDER BY c.hostname`)
	if err != nil {
		return nil, fmt.Errorf("list clients: %w", err)
	}
	defer rows.Close()

	var result []models.ClientWithMetrics
	for rows.Next() {
		var cwm models.ClientWithMetrics
		var mutedUntil sql.NullTime
		var cpuPct, memPct, diskPct sql.NullFloat64
		var memTotal, memUsed, diskTotal, diskUsed sql.NullInt64
		var recordedAt sql.NullTime

		err := rows.Scan(
			&cwm.ID, &cwm.Hostname, &cwm.OS, &cwm.Arch, &cwm.ClientVersion,
			&cwm.FirstSeenAt, &cwm.LastSeenAt, &cwm.IsOnline, &cwm.AlertsMuted, &mutedUntil,
			&cwm.CPUWarnPct, &cwm.CPUCritPct, &cwm.MemWarnPct, &cwm.MemCritPct,
			&cwm.DiskWarnPct, &cwm.DiskCritPct,
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
	rows, err := s.db.Query(`SELECT id, hostname, os, arch, last_seen_at, is_online,
		alerts_muted, muted_until, mute_reason
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
		err := rows.Scan(&c.ID, &c.Hostname, &c.OS, &c.Arch, &c.LastSeenAt, &c.IsOnline,
			&c.AlertsMuted, &mutedUntil, &muteReason)
		if err != nil {
			return nil, err
		}
		if mutedUntil.Valid {
			c.MutedUntil = &mutedUntil.Time
		}
		if muteReason.Valid {
			c.MuteReason = muteReason.String
		}
		clients = append(clients, c)
	}
	return clients, rows.Err()
}

// GetStaleOnlineClients returns clients marked online whose last_seen_at
// is older than thresholdSeconds. The comparison uses SQLite's datetime('now')
// to avoid Go/SQLite timezone mismatches.
func (s *SQLiteStore) GetStaleOnlineClients(thresholdSeconds int) ([]models.Client, error) {
	rows, err := s.db.Query(`SELECT id, hostname, os, arch, last_seen_at, is_online,
		alerts_muted, muted_until, mute_reason
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
		err := rows.Scan(&c.ID, &c.Hostname, &c.OS, &c.Arch, &c.LastSeenAt, &c.IsOnline,
			&c.AlertsMuted, &mutedUntil, &muteReason)
		if err != nil {
			return nil, err
		}
		if mutedUntil.Valid {
			c.MutedUntil = &mutedUntil.Time
		}
		if muteReason.Valid {
			c.MuteReason = muteReason.String
		}
		clients = append(clients, c)
	}
	return clients, rows.Err()
}

func (s *SQLiteStore) SetClientThresholds(id string, t *models.Thresholds) error {
	if t == nil {
		_, err := s.db.Exec(`UPDATE clients SET cpu_warn_pct = NULL, cpu_crit_pct = NULL,
			mem_warn_pct = NULL, mem_crit_pct = NULL, disk_warn_pct = NULL, disk_crit_pct = NULL
			WHERE id = ?`, id)
		return err
	}
	_, err := s.db.Exec(`UPDATE clients SET cpu_warn_pct = ?, cpu_crit_pct = ?,
		mem_warn_pct = ?, mem_crit_pct = ?, disk_warn_pct = ?, disk_crit_pct = ?
		WHERE id = ?`,
		t.CPUWarnPct, t.CPUCritPct, t.MemWarnPct, t.MemCritPct, t.DiskWarnPct, t.DiskCritPct, id)
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
	rows, err := s.db.Query(`SELECT id, client_id, recorded_at, cpu_pct, mem_pct, disk_pct,
		mem_total_bytes, mem_used_bytes, disk_total_bytes, disk_used_bytes
		FROM metrics WHERE client_id = ? AND recorded_at >= ? AND recorded_at <= ?
		ORDER BY recorded_at ASC LIMIT ?`, clientID, from, to, limit)
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
	for _, p := range procs {
		_, err := s.db.Exec(`INSERT INTO watched_processes (client_id, friendly_name, match_pattern, match_type)
			VALUES (?, ?, ?, 'substring')
			ON CONFLICT(client_id, friendly_name) DO UPDATE SET match_pattern = excluded.match_pattern`,
			clientID, p.FriendlyName, p.MatchPattern)
		if err != nil {
			return fmt.Errorf("upsert watched process %q: %w", p.FriendlyName, err)
		}
	}
	return nil
}

func (s *SQLiteStore) InsertProcessSnapshots(clientID string, procs []models.ProcessPayload) error {
	if len(procs) == 0 {
		return nil
	}
	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	stmt, err := tx.Prepare(`INSERT INTO process_snapshots (client_id, friendly_name, is_running, pid, cpu_pct, mem_pct, cmdline)
		VALUES (?, ?, ?, ?, ?, ?, ?)`)
	if err != nil {
		tx.Rollback()
		return err
	}
	defer stmt.Close()

	for _, p := range procs {
		var pid interface{}
		if p.PID > 0 {
			pid = p.PID
		}
		_, err := stmt.Exec(clientID, p.FriendlyName, p.IsRunning, pid, p.CPUPercent, p.MemPercent, p.Cmdline)
		if err != nil {
			tx.Rollback()
			return err
		}
	}
	return tx.Commit()
}

func (s *SQLiteStore) GetLatestProcessSnapshots(clientID string) ([]models.ProcessSnapshot, error) {
	rows, err := s.db.Query(`SELECT ps.id, ps.client_id, ps.friendly_name, ps.recorded_at,
		ps.is_running, ps.pid, ps.cpu_pct, ps.mem_pct, ps.cmdline
		FROM process_snapshots ps
		INNER JOIN (
			SELECT friendly_name, MAX(recorded_at) as max_time
			FROM process_snapshots WHERE client_id = ?
			GROUP BY friendly_name
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
		ps.is_running, ps.pid, ps.cpu_pct, ps.mem_pct, ps.cmdline
		FROM process_snapshots ps
		INNER JOIN (
			SELECT friendly_name, MAX(recorded_at) as max_time
			FROM process_snapshots
			WHERE client_id = ? AND recorded_at < (
				SELECT MAX(recorded_at) FROM process_snapshots WHERE client_id = ?
			)
			GROUP BY friendly_name
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
		var cpuPct, memPct sql.NullFloat64
		var cmdline sql.NullString
		err := rows.Scan(&ps.ID, &ps.ClientID, &ps.FriendlyName, &ps.RecordedAt,
			&ps.IsRunning, &pid, &cpuPct, &memPct, &cmdline)
		if err != nil {
			return nil, err
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

// --- Checks (extensible typed check system) ---

func (s *SQLiteStore) InsertCheckSnapshots(clientID string, checks []models.CheckPayload) error {
	if len(checks) == 0 {
		return nil
	}
	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	stmt, err := tx.Prepare(`INSERT INTO check_snapshots (client_id, friendly_name, check_type, healthy, message, state)
		VALUES (?, ?, ?, ?, ?, ?)`)
	if err != nil {
		tx.Rollback()
		return err
	}
	defer stmt.Close()

	for _, c := range checks {
		_, err := stmt.Exec(clientID, c.FriendlyName, c.CheckType, c.Healthy, c.Message, c.State)
		if err != nil {
			tx.Rollback()
			return err
		}
	}
	return tx.Commit()
}

func (s *SQLiteStore) GetLatestCheckSnapshots(clientID string) ([]models.CheckSnapshot, error) {
	rows, err := s.db.Query(`SELECT cs.id, cs.client_id, cs.friendly_name, cs.check_type,
		cs.recorded_at, cs.healthy, cs.message, cs.state
		FROM check_snapshots cs
		INNER JOIN (
			SELECT friendly_name, MAX(recorded_at) as max_time
			FROM check_snapshots WHERE client_id = ?
			GROUP BY friendly_name
		) latest ON cs.friendly_name = latest.friendly_name AND cs.recorded_at = latest.max_time
		WHERE cs.client_id = ?`, clientID, clientID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanCheckSnapshots(rows)
}

func (s *SQLiteStore) GetPreviousCheckSnapshots(clientID string) ([]models.CheckSnapshot, error) {
	rows, err := s.db.Query(`SELECT cs.id, cs.client_id, cs.friendly_name, cs.check_type,
		cs.recorded_at, cs.healthy, cs.message, cs.state
		FROM check_snapshots cs
		INNER JOIN (
			SELECT friendly_name, MAX(recorded_at) as max_time
			FROM check_snapshots
			WHERE client_id = ? AND recorded_at < (
				SELECT MAX(recorded_at) FROM check_snapshots WHERE client_id = ?
			)
			GROUP BY friendly_name
		) prev ON cs.friendly_name = prev.friendly_name AND cs.recorded_at = prev.max_time
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
		var message, state sql.NullString
		err := rows.Scan(&cs.ID, &cs.ClientID, &cs.FriendlyName, &cs.CheckType,
			&cs.RecordedAt, &cs.Healthy, &message, &state)
		if err != nil {
			return nil, err
		}
		cs.Message = message.String
		cs.State = state.String
		snaps = append(snaps, cs)
	}
	return snaps, rows.Err()
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
