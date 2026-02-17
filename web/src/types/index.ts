export interface Client {
  id: string;
  hostname: string;
  custom_name?: string;
  public_ip?: string;
  interface_ips?: string[];
  os: string;
  arch: string;
  client_version: string;
  first_seen_at: string;
  last_seen_at: string;
  session_started_at: string;
  is_online: boolean;
  alerts_muted: boolean;
  muted_until: string | null;
  mute_reason: string;
  cpu_warn_pct: number | null;
  cpu_crit_pct: number | null;
  mem_warn_pct: number | null;
  mem_crit_pct: number | null;
  disk_warn_pct: number | null;
  disk_crit_pct: number | null;
  offline_threshold_seconds?: number | null;
  metric_consecutive_checkins?: number | null;
}

export interface ClientWithMetrics {
  id: string;
  hostname: string;
  custom_name?: string;
  public_ip?: string;
  interface_ips?: string[];
  os: string;
  arch: string;
  client_version: string;
  first_seen_at: string;
  last_seen_at: string;
  session_started_at: string;
  is_online: boolean;
  alerts_muted: boolean;
  muted_until: string | null;
  offline_threshold_seconds?: number | null;
  metric_consecutive_checkins?: number | null;
  latest_metrics: Metrics | null;
  process_count: number;
}

export interface Metrics {
  cpu_pct: number;
  mem_pct: number;
  disk_pct: number;
  mem_total_bytes: number;
  mem_used_bytes: number;
  disk_total_bytes: number;
  disk_used_bytes: number;
  recorded_at: string;
}

export interface ProcessSnapshot {
  friendly_name: string;
  is_running: boolean;
  pid: number | null;
  cpu_pct: number;
  mem_pct: number;
  cmdline: string;
  recorded_at: string;
  uptime_since_at: string;
}

export interface CheckSnapshot {
  friendly_name: string;
  check_type: string;
  healthy: boolean;
  message: string;
  state: string;  // JSON blob with type-specific details
  recorded_at: string;
  uptime_since_at: string;
}

export interface ClientAlertMute {
  scope: 'cpu' | 'memory' | 'disk' | 'process' | 'check';
  target: string;
}

export interface Alert {
  id: number;
  client_id: string;
  alert_type: string;
  severity: 'info' | 'warning' | 'critical';
  message: string;
  details: string;
  fired_at: string;
  notified: boolean;
}

export interface Thresholds {
  cpu_warn_pct: number;
  cpu_crit_pct: number;
  mem_warn_pct: number;
  mem_crit_pct: number;
  disk_warn_pct: number;
  disk_crit_pct: number;
  offline_threshold_minutes: number;
  metric_consecutive_checkins: number;
}

export interface AlertProvider {
  id: number;
  type: 'twilio' | 'pushover' | 'smtp';
  name: string;
  enabled: boolean;
  config: string;
  created_at: string;
}

export interface TestAlertResult {
  provider: string;
  message: string;
  api_status_code?: number;
  api_response?: string;
}

export type AlertType =
  | 'offline' | 'online'
  | 'pid_change' | 'process_died'
  | 'check_failed' | 'check_recovered'
  | 'cpu_warn' | 'cpu_crit' | 'cpu_recover'
  | 'mem_warn' | 'mem_crit' | 'mem_recover'
  | 'disk_warn' | 'disk_crit' | 'disk_recover';
