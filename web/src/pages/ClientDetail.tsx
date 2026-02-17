import { useState, useEffect } from 'react';
import { useParams, useNavigate } from 'react-router-dom';
import { fetchClient, deleteClient, deleteWatchedProcess, deleteCheckSnapshot, setMute, setScopedMute, fetchMetrics, fetchAlerts, setThresholds, clearThresholds, setClientName, fetchSettings } from '../api/client';
import type { Client, Metrics, ProcessSnapshot, CheckSnapshot, ClientAlertMute, Alert, Thresholds } from '../types';
import MetricGauge from '../components/MetricGauge';
import StatusDot from '../components/StatusDot';
import { AreaChart, Area, XAxis, YAxis, CartesianGrid, Tooltip, ResponsiveContainer } from 'recharts';
import { Trash2, VolumeX, Volume2, ArrowLeft, RefreshCw, Pencil, ChevronRight, ChevronDown } from 'lucide-react';

function formatBytes(bytes: number): string {
  if (bytes === 0) return '0 B';
  const k = 1024;
  const sizes = ['B', 'KB', 'MB', 'GB', 'TB'];
  const i = Math.floor(Math.log(bytes) / Math.log(k));
  return `${(bytes / Math.pow(k, i)).toFixed(1)} ${sizes[i]}`;
}

function formatFriendlyDuration(dateStr: string): string {
  const since = new Date(dateStr).getTime();
  if (Number.isNaN(since)) return '-';
  let seconds = Math.max(0, Math.floor((Date.now() - since) / 1000));
  const units = [
    { label: 'year', secs: 365 * 24 * 60 * 60 },
    { label: 'day', secs: 24 * 60 * 60 },
    { label: 'hour', secs: 60 * 60 },
    { label: 'minute', secs: 60 },
    { label: 'second', secs: 1 },
  ];
  const parts: string[] = [];
  for (const unit of units) {
    if (parts.length >= 2) break;
    if (seconds < unit.secs) continue;
    const count = Math.floor(seconds / unit.secs);
    seconds -= count * unit.secs;
    parts.push(`${count} ${unit.label}${count === 1 ? '' : 's'}`);
  }
  if (parts.length === 0) return '0 seconds';
  if (parts.length === 1) return parts[0];
  return `${parts[0]} and ${parts[1]}`;
}

function isoTooltip(dateStr: string): string {
  const parsed = new Date(dateStr);
  if (Number.isNaN(parsed.getTime())) return dateStr;
  return parsed.toISOString();
}

const FALLBACK_THRESHOLDS: Thresholds = {
  cpu_warn_pct: 80,
  cpu_crit_pct: 95,
  mem_warn_pct: 85,
  mem_crit_pct: 95,
  disk_warn_pct: 80,
  disk_crit_pct: 90,
  offline_threshold_minutes: 4,
};

function displayName(client: Client): string {
  return client.custom_name?.trim() || client.hostname;
}

export default function ClientDetail() {
  const { id } = useParams<{ id: string }>();
  const navigate = useNavigate();
  const [client, setClient] = useState<Client | null>(null);
  const [metrics, setMetrics] = useState<Metrics | null>(null);
  const [processes, setProcesses] = useState<ProcessSnapshot[]>([]);
  const [checks, setChecks] = useState<CheckSnapshot[]>([]);
  const [history, setHistory] = useState<Metrics[]>([]);
  const [alerts, setAlerts] = useState<Alert[]>([]);
  const [alertMutes, setAlertMutes] = useState<ClientAlertMute[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState('');
  const [status, setStatus] = useState('');
  const [range, setRange] = useState('24h');
  const [nameInput, setNameInput] = useState('');
  const [thresholdsForm, setThresholdsForm] = useState<Thresholds>(FALLBACK_THRESHOLDS);
  const [renameOpen, setRenameOpen] = useState(false);
  const [deleteTarget, setDeleteTarget] = useState<{ kind: 'process' | 'check'; friendlyName: string; checkType?: string } | null>(null);
  const [deleteBusy, setDeleteBusy] = useState(false);
  const [showThresholds, setShowThresholds] = useState(false);
  const [showAlerts, setShowAlerts] = useState(false);

  const parseSettingNumber = (settings: Record<string, string>, key: string, fallback: number): number => {
    const raw = settings[key];
    if (!raw) return fallback;
    const parsed = Number(raw);
    return Number.isFinite(parsed) ? parsed : fallback;
  };

  const loadData = async () => {
    if (!id) return;
    setError('');
    try {
      const [data, settings] = await Promise.all([fetchClient(id), fetchSettings()]);
      setClient(data.client);
      setMetrics(data.metrics);
      setProcesses(data.processes || []);
      setChecks(data.checks || []);
      setAlertMutes(data.alert_mutes || []);
      setNameInput(data.client.custom_name || '');

      const defaults: Thresholds = {
        cpu_warn_pct: parseSettingNumber(settings, 'cpu_warn_pct_default', FALLBACK_THRESHOLDS.cpu_warn_pct),
        cpu_crit_pct: parseSettingNumber(settings, 'cpu_crit_pct_default', FALLBACK_THRESHOLDS.cpu_crit_pct),
        mem_warn_pct: parseSettingNumber(settings, 'mem_warn_pct_default', FALLBACK_THRESHOLDS.mem_warn_pct),
        mem_crit_pct: parseSettingNumber(settings, 'mem_crit_pct_default', FALLBACK_THRESHOLDS.mem_crit_pct),
        disk_warn_pct: parseSettingNumber(settings, 'disk_warn_pct_default', FALLBACK_THRESHOLDS.disk_warn_pct),
        disk_crit_pct: parseSettingNumber(settings, 'disk_crit_pct_default', FALLBACK_THRESHOLDS.disk_crit_pct),
        offline_threshold_minutes: Math.max(1, Math.round(parseSettingNumber(settings, 'offline_threshold_seconds', 240) / 60)),
      };
      setThresholdsForm({
        cpu_warn_pct: data.client.cpu_warn_pct ?? defaults.cpu_warn_pct,
        cpu_crit_pct: data.client.cpu_crit_pct ?? defaults.cpu_crit_pct,
        mem_warn_pct: data.client.mem_warn_pct ?? defaults.mem_warn_pct,
        mem_crit_pct: data.client.mem_crit_pct ?? defaults.mem_crit_pct,
        disk_warn_pct: data.client.disk_warn_pct ?? defaults.disk_warn_pct,
        disk_crit_pct: data.client.disk_crit_pct ?? defaults.disk_crit_pct,
        offline_threshold_minutes: data.client.offline_threshold_seconds
          ? Math.max(1, Math.round(data.client.offline_threshold_seconds / 60))
          : defaults.offline_threshold_minutes,
      });

      const rangeHours = range === '1h' ? 1 : range === '6h' ? 6 : range === '7d' ? 168 : range === '14d' ? 336 : 24;
      const from = new Date(Date.now() - rangeHours * 3600000).toISOString();
      const [historyData, alertsData] = await Promise.all([
        fetchMetrics(id, from),
        fetchAlerts(id, undefined, 20),
      ]);
      setHistory(historyData);
      setAlerts(alertsData.alerts);
    } catch (err) {
      if (err instanceof Error) {
        setError(err.message);
      } else {
        setError('Failed to load client details');
      }
    } finally {
      setLoading(false);
    }
  };

  useEffect(() => { loadData(); }, [id, range]);

  const handleDelete = async () => {
    if (!id || !confirm('Delete this client? It will reappear if it checks in again.')) return;
    await deleteClient(id);
    navigate('/');
  };

  const handleToggleMute = async () => {
    if (!id || !client) return;
    await setMute(id, !client.alerts_muted, 0, '');
    loadData();
  };

  const checkMuteTarget = (friendlyName: string, checkType: string): string => `${friendlyName.trim()}::${checkType.trim()}`;

  const isScopedMuted = (scope: ClientAlertMute['scope'], target = ''): boolean =>
    alertMutes.some(m => m.scope === scope && (m.target || '') === target);

  const handleToggleScopedMute = async (scope: ClientAlertMute['scope'], target = '') => {
    if (!id) return;
    const muted = !isScopedMuted(scope, target);
    try {
      await setScopedMute(id, scope, target, muted);
      await loadData();
    } catch (err: any) {
      setStatus(`Error: ${err.message}`);
    }
  };

  const handleRename = async () => {
    if (!id) return;
    try {
      await setClientName(id, nameInput.trim());
      setStatus('Client name updated');
      setRenameOpen(false);
      loadData();
    } catch (err: any) {
      setStatus(`Error: ${err.message}`);
    }
  };

  const handleSaveThresholds = async () => {
    if (!id) return;
    try {
      await setThresholds(id, thresholdsForm);
      setStatus('Per-client thresholds updated');
      loadData();
    } catch (err: any) {
      setStatus(`Error: ${err.message}`);
    }
  };

  const handleResetThresholds = async () => {
    if (!id) return;
    try {
      await clearThresholds(id);
      setStatus('Per-client thresholds reset to global defaults');
      loadData();
    } catch (err: any) {
      setStatus(`Error: ${err.message}`);
    }
  };

  const handleDeleteProcess = (friendlyName: string) => {
    setDeleteTarget({ kind: 'process', friendlyName });
  };

  const handleDeleteCheck = (friendlyName: string, checkType: string) => {
    setDeleteTarget({ kind: 'check', friendlyName, checkType });
  };

  const handleConfirmDeleteTarget = async () => {
    if (!id || !deleteTarget) return;
    setDeleteBusy(true);
    try {
      if (deleteTarget.kind === 'process') {
        await deleteWatchedProcess(id, deleteTarget.friendlyName);
        setStatus(`Deleted watched process: ${deleteTarget.friendlyName}`);
      } else {
        await deleteCheckSnapshot(id, deleteTarget.friendlyName, deleteTarget.checkType || '');
        setStatus(`Deleted check: ${deleteTarget.friendlyName}`);
      }
      setDeleteTarget(null);
      await loadData();
    } catch (err: any) {
      setStatus(`Error: ${err.message}`);
    } finally {
      setDeleteBusy(false);
    }
  };

  if (loading) return <div className="text-gray-500">Loading...</div>;
  if (error) return <div className="text-red-500">{error}</div>;
  if (!client) return <div className="text-red-500">Client not found</div>;

  const chartData = history.map(m => ({
    time: new Date(m.recorded_at).toLocaleTimeString([], { hour: '2-digit', minute: '2-digit' }),
    cpu: m.cpu_pct,
    mem: m.mem_pct,
    disk: m.disk_pct,
  }));

  return (
    <div>
      <button onClick={() => navigate('/')} className="flex items-center gap-1 text-sm text-gray-500 hover:text-gray-700 mb-4">
        <ArrowLeft size={16} /> Back
      </button>

      <div className="flex flex-col gap-3 sm:flex-row sm:items-start sm:justify-between mb-6">
        <div className="min-w-0">
          <div className="flex items-center gap-3 min-w-0">
            <StatusDot online={client.is_online} muted={client.alerts_muted} />
            <h1 className="text-xl sm:text-2xl font-bold text-gray-900 truncate">{displayName(client)}</h1>
            <button
              onClick={() => { setNameInput(client.custom_name || ''); setRenameOpen(true); }}
              className="p-1.5 text-gray-400 hover:text-gray-600 rounded hover:bg-gray-100"
              title="Rename client"
            >
              <Pencil size={14} />
            </button>
          </div>
          <div className="mt-2 flex flex-wrap gap-2">
            <span className="text-xs text-gray-400 bg-gray-100 px-2 py-1 rounded">{client.os}/{client.arch}</span>
            {client.public_ip && (
              <span className="text-xs text-gray-500 bg-gray-100 px-2 py-1 rounded font-mono">public {client.public_ip}</span>
            )}
          </div>
          {client.interface_ips && client.interface_ips.length > 0 && (
            <div className="mt-2">
              <div className="text-[11px] uppercase tracking-wide text-gray-400 mb-1">Interface IPs</div>
              <div className="flex flex-wrap gap-1.5">
                {client.interface_ips.map(ip => (
                  <span key={ip} className="text-xs text-gray-600 bg-gray-100 px-2 py-1 rounded font-mono">
                    {ip}
                  </span>
                ))}
              </div>
            </div>
          )}
        </div>
        <div className="flex gap-2 self-start sm:self-auto">
          <button onClick={() => loadData()} className="p-2 text-gray-400 hover:text-gray-600 rounded-md hover:bg-gray-100" title="Refresh">
            <RefreshCw size={18} />
          </button>
          <button
            onClick={handleToggleMute}
            className={`p-2 rounded-md ${client.alerts_muted ? 'text-red-600 hover:text-red-700 bg-red-50 hover:bg-red-100' : 'text-gray-400 hover:text-gray-600 hover:bg-gray-100'}`}
            title={client.alerts_muted ? 'Unmute' : 'Mute'}
          >
            {client.alerts_muted ? <Volume2 size={18} /> : <VolumeX size={18} />}
          </button>
          <button onClick={handleDelete} className="p-2 text-red-400 hover:text-red-600 rounded-md hover:bg-red-50" title="Delete">
            <Trash2 size={18} />
          </button>
        </div>
      </div>

      {status && (
        <div className="mb-4 px-4 py-2 bg-blue-50 text-blue-700 rounded text-sm">
          {status}
          <button onClick={() => setStatus('')} className="ml-2 text-blue-500">&times;</button>
        </div>
      )}

      {/* Metrics (top section) */}
      {metrics && (
        <div className="grid grid-cols-1 sm:grid-cols-3 gap-4 mb-6">
          {[
            {
              scope: 'cpu' as const,
              label: 'CPU',
              gauge: <MetricGauge label="CPU" value={metrics.cpu_pct} size="lg" />,
            },
            {
              scope: 'memory' as const,
              label: 'Memory',
              gauge: <MetricGauge label="Memory" value={metrics.mem_pct} size="lg" unit={`% (${formatBytes(metrics.mem_used_bytes)}/${formatBytes(metrics.mem_total_bytes)})`} />,
            },
            {
              scope: 'disk' as const,
              label: 'Disk',
              gauge: <MetricGauge label="Disk" value={metrics.disk_pct} size="lg" unit={`% (${formatBytes(metrics.disk_used_bytes)}/${formatBytes(metrics.disk_total_bytes)})`} />,
            },
          ].map(item => {
            const muted = isScopedMuted(item.scope);
            return (
              <div key={item.scope} className="relative">
                {item.gauge}
                <button
                  onClick={() => handleToggleScopedMute(item.scope)}
                  className={`absolute top-2 right-2 p-1 rounded border ${
                    muted
                      ? 'bg-red-50 border-red-200 text-red-600 hover:bg-red-100'
                      : 'bg-white/90 border-gray-200 text-gray-500 hover:text-gray-700 hover:bg-gray-50'
                  }`}
                  title={muted ? `Unmute ${item.label} alerts` : `Mute ${item.label} alerts`}
                >
                  {muted ? <Volume2 size={14} /> : <VolumeX size={14} />}
                </button>
              </div>
            );
          })}
        </div>
      )}

      {/* Watched Processes + Checks (second section) */}
      {(processes.length > 0 || checks.length > 0) && (
        <div className="bg-white rounded-lg border p-4 mb-6">
          <h2 className="font-semibold text-gray-700 mb-3">Watched Processes &amp; Checks</h2>
          <div className="overflow-x-auto -mx-1 px-1">
            <table className="w-full min-w-[860px] text-sm">
              <thead>
                <tr className="text-left text-gray-500 border-b">
                  <th className="pb-2">Name</th>
                  <th className="pb-2">Kind</th>
                  <th className="pb-2">Status</th>
                  <th className="pb-2">PID</th>
                  <th className="pb-2">CPU</th>
                  <th className="pb-2">Memory</th>
                  <th className="pb-2">Uptime</th>
                  <th className="pb-2 text-right">Actions</th>
                </tr>
              </thead>
              <tbody>
                {processes.map(p => (
                  <tr key={`proc:${p.friendly_name}`} className="border-b">
                    <td className="py-2 font-medium">{p.friendly_name}</td>
                    <td className="py-2 text-gray-500">process</td>
                    <td className="py-2">
                      <span className={`inline-flex items-center gap-1 px-2 py-0.5 rounded text-xs ${p.is_running ? 'bg-green-100 text-green-700' : 'bg-red-100 text-red-700'}`}>
                        {p.is_running ? 'Running' : 'Stopped'}
                      </span>
                    </td>
                    <td className="py-2 text-gray-500 font-mono">{p.pid || '-'}</td>
                    <td className="py-2 text-gray-500">{p.cpu_pct?.toFixed(1)}%</td>
                    <td className="py-2 text-gray-500">{p.mem_pct?.toFixed(1)}%</td>
                    <td className="py-2 text-gray-500" title={isoTooltip(p.uptime_since_at)}>
                      {formatFriendlyDuration(p.uptime_since_at)}
                    </td>
                    <td className="py-2 text-right">
                      <button
                        onClick={() => handleToggleScopedMute('process', p.friendly_name)}
                        className={`inline-flex items-center gap-1 px-2 py-1 text-xs rounded mr-1 ${
                          isScopedMuted('process', p.friendly_name)
                            ? 'text-red-600 hover:bg-red-50'
                            : 'text-gray-600 hover:bg-gray-50'
                        }`}
                        title={isScopedMuted('process', p.friendly_name) ? 'Unmute alerts' : 'Mute alerts'}
                      >
                        {isScopedMuted('process', p.friendly_name) ? <Volume2 size={12} /> : <VolumeX size={12} />} {isScopedMuted('process', p.friendly_name) ? 'Unmute' : 'Mute'}
                      </button>
                      <button
                        onClick={() => handleDeleteProcess(p.friendly_name)}
                        className="inline-flex items-center gap-1 px-2 py-1 text-xs text-red-600 hover:bg-red-50 rounded"
                        title="Delete from server"
                      >
                        <Trash2 size={12} /> Delete
                      </button>
                    </td>
                  </tr>
                ))}
                {checks.map(c => (
                  <tr key={`check:${c.friendly_name}:${c.check_type}`} className="border-b last:border-0">
                    <td className="py-2 font-medium">{c.friendly_name}</td>
                    <td className="py-2 text-gray-500">check ({c.check_type})</td>
                    <td className="py-2">
                      <span className={`inline-flex items-center gap-1 px-2 py-0.5 rounded text-xs ${c.healthy ? 'bg-green-100 text-green-700' : 'bg-red-100 text-red-700'}`}>
                        {c.healthy ? 'Healthy' : 'Unhealthy'}
                      </span>
                    </td>
                    <td className="py-2 text-gray-400">-</td>
                    <td className="py-2 text-gray-400">-</td>
                    <td className="py-2 text-gray-400">-</td>
                    <td className="py-2 text-gray-500" title={isoTooltip(c.uptime_since_at)}>
                      {formatFriendlyDuration(c.uptime_since_at)}
                    </td>
                    <td className="py-2 text-right">
                      <button
                        onClick={() => handleToggleScopedMute('check', checkMuteTarget(c.friendly_name, c.check_type))}
                        className={`inline-flex items-center gap-1 px-2 py-1 text-xs rounded mr-1 ${
                          isScopedMuted('check', checkMuteTarget(c.friendly_name, c.check_type))
                            ? 'text-red-600 hover:bg-red-50'
                            : 'text-gray-600 hover:bg-gray-50'
                        }`}
                        title={isScopedMuted('check', checkMuteTarget(c.friendly_name, c.check_type)) ? 'Unmute alerts' : 'Mute alerts'}
                      >
                        {isScopedMuted('check', checkMuteTarget(c.friendly_name, c.check_type)) ? <Volume2 size={12} /> : <VolumeX size={12} />} {isScopedMuted('check', checkMuteTarget(c.friendly_name, c.check_type)) ? 'Unmute' : 'Mute'}
                      </button>
                      <button
                        onClick={() => handleDeleteCheck(c.friendly_name, c.check_type)}
                        className="inline-flex items-center gap-1 px-2 py-1 text-xs text-red-600 hover:bg-red-50 rounded"
                        title="Delete from server"
                      >
                        <Trash2 size={12} /> Delete
                      </button>
                    </td>
                  </tr>
                ))}
              </tbody>
            </table>
          </div>
        </div>
      )}

      {/* Chart */}
      <div className="bg-white rounded-lg border p-4 mb-6">
        <div className="flex flex-col sm:flex-row sm:items-center sm:justify-between gap-2 mb-3">
          <h2 className="font-semibold text-gray-700">History</h2>
          <div className="flex flex-wrap gap-1">
            {['1h', '6h', '24h', '7d', '14d'].map(r => (
              <button
                key={r}
                onClick={() => setRange(r)}
                className={`px-2 py-1 text-xs rounded ${range === r ? 'bg-blue-100 text-blue-700' : 'text-gray-500 hover:bg-gray-100'}`}
              >
                {r}
              </button>
            ))}
          </div>
        </div>
        {chartData.length > 0 ? (
          <ResponsiveContainer width="100%" height={200}>
            <AreaChart data={chartData}>
              <CartesianGrid strokeDasharray="3 3" stroke="#f0f0f0" />
              <XAxis dataKey="time" tick={{ fontSize: 11 }} />
              <YAxis domain={[0, 100]} tick={{ fontSize: 11 }} />
              <Tooltip />
              <Area type="monotone" dataKey="cpu" stroke="#3b82f6" fill="#93c5fd" fillOpacity={0.3} name="CPU %" />
              <Area type="monotone" dataKey="mem" stroke="#10b981" fill="#6ee7b7" fillOpacity={0.3} name="Mem %" />
              <Area type="monotone" dataKey="disk" stroke="#f59e0b" fill="#fcd34d" fillOpacity={0.3} name="Disk %" />
            </AreaChart>
          </ResponsiveContainer>
        ) : (
          <p className="text-sm text-gray-400 py-6">No history yet for this range.</p>
        )}
      </div>

      {/* Thresholds (collapsed by default) */}
      <div className="bg-white rounded-lg border p-4 mb-6">
        <button
          onClick={() => setShowThresholds(!showThresholds)}
          className="w-full flex items-center justify-between"
        >
          <h2 className="font-semibold text-gray-700">Per-Client Alert Thresholds</h2>
          <span className="text-gray-400">{showThresholds ? <ChevronDown size={16} /> : <ChevronRight size={16} />}</span>
        </button>
        {showThresholds && (
          <div className="mt-4">
            <div className="grid grid-cols-1 md:grid-cols-3 gap-3">
              {[
                { title: 'CPU', warnKey: 'cpu_warn_pct', critKey: 'cpu_crit_pct' },
                { title: 'Memory', warnKey: 'mem_warn_pct', critKey: 'mem_crit_pct' },
                { title: 'Disk', warnKey: 'disk_warn_pct', critKey: 'disk_crit_pct' },
              ].map(group => (
                <div key={group.title} className="border rounded-lg p-3 bg-gray-50">
                  <h3 className="text-sm font-semibold text-gray-700 mb-2">{group.title}</h3>
                  <label className="block text-xs text-gray-600 mb-1">Warning %</label>
                  <input
                    type="number"
                    min={0}
                    max={100}
                    value={(thresholdsForm as any)[group.warnKey]}
                    onChange={e => setThresholdsForm({ ...thresholdsForm, [group.warnKey]: Number(e.target.value) } as Thresholds)}
                    className="w-full px-3 py-1.5 border rounded text-sm mb-2"
                  />
                  <label className="block text-xs text-gray-600 mb-1">Critical %</label>
                  <input
                    type="number"
                    min={0}
                    max={100}
                    value={(thresholdsForm as any)[group.critKey]}
                    onChange={e => setThresholdsForm({ ...thresholdsForm, [group.critKey]: Number(e.target.value) } as Thresholds)}
                    className="w-full px-3 py-1.5 border rounded text-sm"
                  />
                </div>
              ))}
            </div>
            <div className="mt-3 border rounded-lg p-3 bg-gray-50">
              <h3 className="text-sm font-semibold text-gray-700 mb-2">Offline Alert Delay</h3>
              <label className="block text-xs text-gray-600 mb-1">Minutes before offline alert</label>
              <input
                type="number"
                min={1}
                value={thresholdsForm.offline_threshold_minutes}
                onChange={e => setThresholdsForm({
                  ...thresholdsForm,
                  offline_threshold_minutes: Math.max(1, Number(e.target.value) || 1),
                })}
                className="w-full max-w-xs px-3 py-1.5 border rounded text-sm"
              />
              <p className="text-xs text-gray-500 mt-1">
                Per-client override. Use &quot;Use Global Defaults&quot; to inherit the global delay.
              </p>
            </div>
            <div className="flex flex-col sm:flex-row gap-2 mt-4">
              <button onClick={handleSaveThresholds} className="px-4 py-2 bg-blue-600 text-white rounded text-sm hover:bg-blue-700">
                Save Thresholds
              </button>
              <button onClick={handleResetThresholds} className="px-4 py-2 border rounded text-sm hover:bg-gray-50">
                Use Global Defaults
              </button>
            </div>
          </div>
        )}
      </div>

      {/* Recent Alerts (collapsed by default) */}
      <div className="bg-white rounded-lg border p-4 mb-6">
        <button
          onClick={() => setShowAlerts(!showAlerts)}
          className="w-full flex items-center justify-between"
        >
          <h2 className="font-semibold text-gray-700">Recent Alerts</h2>
          <span className="text-gray-400">{showAlerts ? <ChevronDown size={16} /> : <ChevronRight size={16} />}</span>
        </button>
        {showAlerts && (
          <div className="mt-4">
            {alerts.length === 0 && <p className="text-sm text-gray-400">No recent alerts.</p>}
            {alerts.length > 0 && (
              <div className="space-y-2">
                {alerts.map(a => (
                  <div key={a.id} className="flex flex-col sm:flex-row sm:items-center gap-1 sm:gap-3 text-sm py-1.5 border-b last:border-0">
                    <span className={`w-2 h-2 rounded-full flex-shrink-0 ${
                      a.severity === 'critical' ? 'bg-red-500' : a.severity === 'warning' ? 'bg-amber-400' : 'bg-blue-400'
                    }`} />
                    <span className="text-gray-500 text-xs sm:w-32 sm:flex-shrink-0">
                      {new Date(a.fired_at).toLocaleString([], { month: 'short', day: 'numeric', hour: '2-digit', minute: '2-digit' })}
                    </span>
                    <span className="text-gray-700">{a.message}</span>
                  </div>
                ))}
              </div>
            )}
          </div>
        )}
      </div>

      {/* Rename Modal */}
      {renameOpen && (
        <div className="fixed inset-0 z-50 bg-black/40 flex items-center justify-center px-4">
          <div className="w-full max-w-md bg-white rounded-lg border shadow-lg p-4">
            <h2 className="font-semibold text-gray-800 mb-2">Rename Client</h2>
            <p className="text-sm text-gray-500 mb-3">Hostname: <span className="font-mono">{client.hostname}</span></p>
            <input
              value={nameInput}
              onChange={e => setNameInput(e.target.value)}
              className="w-full px-3 py-2 border rounded text-sm"
              placeholder="Optional display name (blank = use hostname)"
            />
            <div className="flex justify-end gap-2 mt-4">
              <button onClick={() => setRenameOpen(false)} className="px-4 py-2 border rounded text-sm hover:bg-gray-50">
                Cancel
              </button>
              <button onClick={handleRename} className="px-4 py-2 bg-blue-600 text-white rounded text-sm hover:bg-blue-700">
                Save
              </button>
            </div>
          </div>
        </div>
      )}

      {/* Delete Process/Check Modal */}
      {deleteTarget && (
        <div className="fixed inset-0 z-50 bg-black/40 flex items-center justify-center px-4">
          <div className="w-full max-w-md bg-white rounded-lg border shadow-lg p-4">
            <h2 className="font-semibold text-gray-800 mb-2">
              Delete {deleteTarget.kind === 'process' ? 'Process' : 'Check'}?
            </h2>
            <p className="text-sm text-gray-600 mb-1">
              <span className="font-medium">{deleteTarget.friendlyName}</span>
              {deleteTarget.kind === 'check' && deleteTarget.checkType ? (
                <span className="text-gray-500"> ({deleteTarget.checkType})</span>
              ) : null}
            </p>
            <p className="text-sm text-gray-500 mb-4">
              This removes it from the server now. It will automatically return if the client sends it again on a future check-in.
            </p>
            <div className="flex justify-end gap-2">
              <button
                onClick={() => setDeleteTarget(null)}
                className="px-4 py-2 border rounded text-sm hover:bg-gray-50"
                disabled={deleteBusy}
              >
                Cancel
              </button>
              <button
                onClick={handleConfirmDeleteTarget}
                className="px-4 py-2 bg-red-600 text-white rounded text-sm hover:bg-red-700 disabled:opacity-60"
                disabled={deleteBusy}
              >
                {deleteBusy ? 'Deleting...' : 'Delete'}
              </button>
            </div>
          </div>
        </div>
      )}
    </div>
  );
}
