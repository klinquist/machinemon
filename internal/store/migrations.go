package store

import "database/sql"

var migrations = []func(tx *sql.Tx) error{
	migrateV1,
	migrateV2,
	migrateV3,
	migrateV4,
	migrateV5,
	migrateV6,
}

func migrateV1(tx *sql.Tx) error {
	stmts := []string{
		`CREATE TABLE IF NOT EXISTS clients (
			id              TEXT PRIMARY KEY,
			hostname        TEXT NOT NULL,
			os              TEXT NOT NULL DEFAULT '',
			arch            TEXT NOT NULL DEFAULT '',
			client_version  TEXT NOT NULL DEFAULT '',
			first_seen_at   DATETIME NOT NULL DEFAULT (datetime('now')),
			last_seen_at    DATETIME NOT NULL DEFAULT (datetime('now')),
			is_online       BOOLEAN NOT NULL DEFAULT 1,
			is_deleted      BOOLEAN NOT NULL DEFAULT 0,
			cpu_warn_pct    REAL,
			cpu_crit_pct    REAL,
			mem_warn_pct    REAL,
			mem_crit_pct    REAL,
			disk_warn_pct   REAL,
			disk_crit_pct   REAL,
			alerts_muted    BOOLEAN NOT NULL DEFAULT 0,
			muted_until     DATETIME,
			mute_reason     TEXT
		)`,
		`CREATE TABLE IF NOT EXISTS metrics (
			id              INTEGER PRIMARY KEY AUTOINCREMENT,
			client_id       TEXT NOT NULL REFERENCES clients(id) ON DELETE CASCADE,
			recorded_at     DATETIME NOT NULL DEFAULT (datetime('now')),
			cpu_pct         REAL NOT NULL,
			mem_pct         REAL NOT NULL,
			disk_pct        REAL NOT NULL,
			mem_total_bytes INTEGER NOT NULL DEFAULT 0,
			mem_used_bytes  INTEGER NOT NULL DEFAULT 0,
			disk_total_bytes INTEGER NOT NULL DEFAULT 0,
			disk_used_bytes  INTEGER NOT NULL DEFAULT 0
		)`,
		`CREATE INDEX IF NOT EXISTS idx_metrics_client_time ON metrics(client_id, recorded_at)`,
		`CREATE TABLE IF NOT EXISTS watched_processes (
			id              INTEGER PRIMARY KEY AUTOINCREMENT,
			client_id       TEXT NOT NULL REFERENCES clients(id) ON DELETE CASCADE,
			friendly_name   TEXT NOT NULL,
			match_pattern   TEXT NOT NULL,
			match_type      TEXT NOT NULL DEFAULT 'substring',
			UNIQUE(client_id, friendly_name)
		)`,
		`CREATE TABLE IF NOT EXISTS process_snapshots (
			id              INTEGER PRIMARY KEY AUTOINCREMENT,
			client_id       TEXT NOT NULL REFERENCES clients(id) ON DELETE CASCADE,
			friendly_name   TEXT NOT NULL,
			recorded_at     DATETIME NOT NULL DEFAULT (datetime('now')),
			is_running      BOOLEAN NOT NULL,
			pid             INTEGER,
			cpu_pct         REAL,
			mem_pct         REAL,
			cmdline         TEXT
		)`,
		`CREATE INDEX IF NOT EXISTS idx_process_snap_client_time ON process_snapshots(client_id, recorded_at)`,
		`CREATE TABLE IF NOT EXISTS alerts (
			id              INTEGER PRIMARY KEY AUTOINCREMENT,
			client_id       TEXT NOT NULL REFERENCES clients(id) ON DELETE CASCADE,
			alert_type      TEXT NOT NULL,
			severity        TEXT NOT NULL,
			message         TEXT NOT NULL,
			details         TEXT,
			fired_at        DATETIME NOT NULL DEFAULT (datetime('now')),
			notified        BOOLEAN NOT NULL DEFAULT 0,
			notified_at     DATETIME
		)`,
		`CREATE INDEX IF NOT EXISTS idx_alerts_client_time ON alerts(client_id, fired_at)`,
		`CREATE INDEX IF NOT EXISTS idx_alerts_unnotified ON alerts(notified) WHERE notified = 0`,
		`CREATE TABLE IF NOT EXISTS alert_providers (
			id              INTEGER PRIMARY KEY AUTOINCREMENT,
			type            TEXT NOT NULL,
			name            TEXT NOT NULL UNIQUE,
			enabled         BOOLEAN NOT NULL DEFAULT 1,
			config          TEXT NOT NULL DEFAULT '{}',
			created_at      DATETIME NOT NULL DEFAULT (datetime('now'))
		)`,
		`CREATE TABLE IF NOT EXISTS check_snapshots (
			id              INTEGER PRIMARY KEY AUTOINCREMENT,
			client_id       TEXT NOT NULL REFERENCES clients(id) ON DELETE CASCADE,
			friendly_name   TEXT NOT NULL,
			check_type      TEXT NOT NULL DEFAULT 'script',
			recorded_at     DATETIME NOT NULL DEFAULT (datetime('now')),
			healthy         BOOLEAN NOT NULL,
			message         TEXT,
			state           TEXT
		)`,
		`CREATE INDEX IF NOT EXISTS idx_check_snap_client_time ON check_snapshots(client_id, recorded_at)`,
		`CREATE TABLE IF NOT EXISTS global_settings (
			key             TEXT PRIMARY KEY,
			value           TEXT NOT NULL
		)`,
	}

	for _, stmt := range stmts {
		if _, err := tx.Exec(stmt); err != nil {
			return err
		}
	}
	return nil
}

func migrateV2(tx *sql.Tx) error {
	_, err := tx.Exec(`ALTER TABLE clients ADD COLUMN session_id TEXT`)
	return err
}

func migrateV3(tx *sql.Tx) error {
	_, err := tx.Exec(`ALTER TABLE clients ADD COLUMN custom_name TEXT NOT NULL DEFAULT ''`)
	return err
}

func migrateV4(tx *sql.Tx) error {
	if _, err := tx.Exec(`ALTER TABLE clients ADD COLUMN public_ip TEXT NOT NULL DEFAULT ''`); err != nil {
		return err
	}
	_, err := tx.Exec(`ALTER TABLE clients ADD COLUMN interface_ips TEXT NOT NULL DEFAULT '[]'`)
	return err
}

func migrateV5(tx *sql.Tx) error {
	stmts := []string{
		`CREATE TABLE IF NOT EXISTS client_alert_mutes (
			id          INTEGER PRIMARY KEY AUTOINCREMENT,
			client_id   TEXT NOT NULL REFERENCES clients(id) ON DELETE CASCADE,
			scope       TEXT NOT NULL,
			target      TEXT NOT NULL DEFAULT '',
			created_at  DATETIME NOT NULL DEFAULT (datetime('now')),
			UNIQUE(client_id, scope, target)
		)`,
		`CREATE INDEX IF NOT EXISTS idx_client_alert_mutes_client ON client_alert_mutes(client_id)`,
		`CREATE INDEX IF NOT EXISTS idx_client_alert_mutes_scope ON client_alert_mutes(client_id, scope)`,
	}
	for _, stmt := range stmts {
		if _, err := tx.Exec(stmt); err != nil {
			return err
		}
	}
	return nil
}

func migrateV6(tx *sql.Tx) error {
	_, err := tx.Exec(`ALTER TABLE clients ADD COLUMN offline_threshold_seconds INTEGER`)
	return err
}
