import { useState, useEffect } from 'react';
import { useParams, useNavigate } from 'react-router-dom';
import { fetchClient, deleteClient, setMute, fetchMetrics, fetchAlerts } from '../api/client';
import type { Client, Metrics, ProcessSnapshot, CheckSnapshot, Alert } from '../types';
import MetricGauge from '../components/MetricGauge';
import StatusDot from '../components/StatusDot';
import { AreaChart, Area, XAxis, YAxis, CartesianGrid, Tooltip, ResponsiveContainer } from 'recharts';
import { Trash2, VolumeX, Volume2, ArrowLeft, RefreshCw } from 'lucide-react';

function formatBytes(bytes: number): string {
  if (bytes === 0) return '0 B';
  const k = 1024;
  const sizes = ['B', 'KB', 'MB', 'GB', 'TB'];
  const i = Math.floor(Math.log(bytes) / Math.log(k));
  return `${(bytes / Math.pow(k, i)).toFixed(1)} ${sizes[i]}`;
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
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState('');
  const [range, setRange] = useState('24h');

  const loadData = async () => {
    if (!id) return;
    setError('');
    try {
      const data = await fetchClient(id);
      setClient(data.client);
      setMetrics(data.metrics);
      setProcesses(data.processes || []);
      setChecks(data.checks || []);

      const rangeHours = range === '1h' ? 1 : range === '6h' ? 6 : range === '7d' ? 168 : 24;
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

      <div className="flex items-center justify-between mb-6">
        <div className="flex items-center gap-3">
          <StatusDot online={client.is_online} muted={client.alerts_muted} />
          <h1 className="text-2xl font-bold text-gray-900">{client.hostname}</h1>
          <span className="text-xs text-gray-400 bg-gray-100 px-2 py-1 rounded">{client.os}/{client.arch}</span>
        </div>
        <div className="flex gap-2">
          <button onClick={() => loadData()} className="p-2 text-gray-400 hover:text-gray-600 rounded-md hover:bg-gray-100" title="Refresh">
            <RefreshCw size={18} />
          </button>
          <button onClick={handleToggleMute} className="p-2 text-gray-400 hover:text-gray-600 rounded-md hover:bg-gray-100" title={client.alerts_muted ? 'Unmute' : 'Mute'}>
            {client.alerts_muted ? <Volume2 size={18} /> : <VolumeX size={18} />}
          </button>
          <button onClick={handleDelete} className="p-2 text-red-400 hover:text-red-600 rounded-md hover:bg-red-50" title="Delete">
            <Trash2 size={18} />
          </button>
        </div>
      </div>

      {/* Metrics */}
      {metrics && (
        <div className="grid grid-cols-3 gap-4 mb-6">
          <MetricGauge label="CPU" value={metrics.cpu_pct} size="lg" />
          <MetricGauge label="Memory" value={metrics.mem_pct} size="lg" unit={`% (${formatBytes(metrics.mem_used_bytes)}/${formatBytes(metrics.mem_total_bytes)})`} />
          <MetricGauge label="Disk" value={metrics.disk_pct} size="lg" unit={`% (${formatBytes(metrics.disk_used_bytes)}/${formatBytes(metrics.disk_total_bytes)})`} />
        </div>
      )}

      {/* Chart */}
      {chartData.length > 0 && (
        <div className="bg-white rounded-lg border p-4 mb-6">
          <div className="flex items-center justify-between mb-3">
            <h2 className="font-semibold text-gray-700">History</h2>
            <div className="flex gap-1">
              {['1h', '6h', '24h', '7d'].map(r => (
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
        </div>
      )}

      {/* Processes */}
      {processes.length > 0 && (
        <div className="bg-white rounded-lg border p-4 mb-6">
          <h2 className="font-semibold text-gray-700 mb-3">Watched Processes</h2>
          <table className="w-full text-sm">
            <thead>
              <tr className="text-left text-gray-500 border-b">
                <th className="pb-2">Name</th>
                <th className="pb-2">Status</th>
                <th className="pb-2">PID</th>
                <th className="pb-2">CPU</th>
                <th className="pb-2">Memory</th>
              </tr>
            </thead>
            <tbody>
              {processes.map(p => (
                <tr key={p.friendly_name} className="border-b last:border-0">
                  <td className="py-2 font-medium">{p.friendly_name}</td>
                  <td className="py-2">
                    <span className={`inline-flex items-center gap-1 px-2 py-0.5 rounded text-xs ${p.is_running ? 'bg-green-100 text-green-700' : 'bg-red-100 text-red-700'}`}>
                      {p.is_running ? 'Running' : 'Stopped'}
                    </span>
                  </td>
                  <td className="py-2 text-gray-500 font-mono">{p.pid || '-'}</td>
                  <td className="py-2 text-gray-500">{p.cpu_pct?.toFixed(1)}%</td>
                  <td className="py-2 text-gray-500">{p.mem_pct?.toFixed(1)}%</td>
                </tr>
              ))}
            </tbody>
          </table>
        </div>
      )}

      {/* Checks */}
      {checks.length > 0 && (
        <div className="bg-white rounded-lg border p-4 mb-6">
          <h2 className="font-semibold text-gray-700 mb-3">Checks</h2>
          <table className="w-full text-sm">
            <thead>
              <tr className="text-left text-gray-500 border-b">
                <th className="pb-2">Name</th>
                <th className="pb-2">Type</th>
                <th className="pb-2">Status</th>
                <th className="pb-2">Message</th>
              </tr>
            </thead>
            <tbody>
              {checks.map(c => (
                <tr key={c.friendly_name} className="border-b last:border-0">
                  <td className="py-2 font-medium">{c.friendly_name}</td>
                  <td className="py-2 text-gray-500">{c.check_type}</td>
                  <td className="py-2">
                    <span className={`inline-flex items-center gap-1 px-2 py-0.5 rounded text-xs ${c.healthy ? 'bg-green-100 text-green-700' : 'bg-red-100 text-red-700'}`}>
                      {c.healthy ? 'Healthy' : 'Unhealthy'}
                    </span>
                  </td>
                  <td className="py-2 text-gray-500 truncate max-w-xs" title={c.message}>{c.message || '-'}</td>
                </tr>
              ))}
            </tbody>
          </table>
        </div>
      )}

      {/* Recent Alerts */}
      {alerts.length > 0 && (
        <div className="bg-white rounded-lg border p-4">
          <h2 className="font-semibold text-gray-700 mb-3">Recent Alerts</h2>
          <div className="space-y-2">
            {alerts.map(a => (
              <div key={a.id} className="flex items-center gap-3 text-sm py-1.5 border-b last:border-0">
                <span className={`w-2 h-2 rounded-full flex-shrink-0 ${
                  a.severity === 'critical' ? 'bg-red-500' : a.severity === 'warning' ? 'bg-amber-400' : 'bg-blue-400'
                }`} />
                <span className="text-gray-500 text-xs w-32 flex-shrink-0">
                  {new Date(a.fired_at).toLocaleString([], { month: 'short', day: 'numeric', hour: '2-digit', minute: '2-digit' })}
                </span>
                <span className="text-gray-700">{a.message}</span>
              </div>
            ))}
          </div>
        </div>
      )}
    </div>
  );
}
