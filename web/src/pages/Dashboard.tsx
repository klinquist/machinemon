import { useState, useEffect } from 'react';
import { Link } from 'react-router-dom';
import { fetchClients, AuthError } from '../api/client';
import type { ClientWithMetrics } from '../types';
import StatusDot from '../components/StatusDot';
import MetricGauge from '../components/MetricGauge';
import { Server, Clock } from 'lucide-react';

function timeAgo(dateStr: string): string {
  const seconds = Math.floor((Date.now() - new Date(dateStr).getTime()) / 1000);
  if (seconds < 60) return `${seconds}s ago`;
  if (seconds < 3600) return `${Math.floor(seconds / 60)}m ago`;
  if (seconds < 86400) return `${Math.floor(seconds / 3600)}h ago`;
  return `${Math.floor(seconds / 86400)}d ago`;
}

function clientLabel(client: ClientWithMetrics): string {
  return client.custom_name?.trim() || client.hostname;
}

export default function Dashboard() {
  const [clients, setClients] = useState<ClientWithMetrics[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState('');

  const loadClients = async () => {
    try {
      const data = await fetchClients();
      setClients(data);
      setError('');
    } catch (err) {
      if (err instanceof AuthError) return;
      setError('Failed to load clients');
    } finally {
      setLoading(false);
    }
  };

  useEffect(() => {
    loadClients();
    const interval = setInterval(loadClients, 30000);
    return () => clearInterval(interval);
  }, []);

  if (loading) {
    return <div className="text-gray-500">Loading...</div>;
  }

  if (error) {
    return <div className="text-red-500">{error}</div>;
  }

  if (clients.length === 0) {
    return (
      <div className="text-center py-16">
        <Server className="mx-auto text-gray-300 mb-4" size={48} />
        <h2 className="text-xl font-semibold text-gray-700">No clients yet</h2>
        <p className="text-gray-500 mt-2">
          Install the MachineMon client on a machine and point it at this server to get started.
        </p>
      </div>
    );
  }

  const onlineCount = clients.filter(c => c.is_online).length;

  return (
    <div>
      <div className="flex items-center justify-between mb-6">
        <h1 className="text-2xl font-bold text-gray-900">Dashboard</h1>
        <div className="text-sm text-gray-500">
          {onlineCount}/{clients.length} online
        </div>
      </div>
      <div className="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-3 gap-4">
        {clients.map(client => (
          <Link
            key={client.id}
            to={`/clients/${client.id}`}
            className="bg-white rounded-lg border hover:shadow-md transition-shadow p-4 block"
          >
            <div className="flex items-center justify-between mb-3">
              <div className="flex items-center gap-2">
                <StatusDot online={client.is_online} muted={client.alerts_muted} />
                <span className="font-semibold text-gray-900">{clientLabel(client)}</span>
              </div>
              <span className="text-xs text-gray-400 bg-gray-100 px-2 py-0.5 rounded">
                {client.os}/{client.arch}
              </span>
            </div>
            {client.latest_metrics ? (
              <div className="flex gap-4">
                <MetricGauge label="CPU" value={client.latest_metrics.cpu_pct} />
                <MetricGauge label="Memory" value={client.latest_metrics.mem_pct} />
                <MetricGauge label="Disk" value={client.latest_metrics.disk_pct} />
              </div>
            ) : (
              <div className="text-sm text-gray-400">No metrics yet</div>
            )}
            <div className="mt-3 flex items-center justify-between text-xs text-gray-400">
              <span className="flex items-center gap-1">
                <Clock size={12} /> {timeAgo(client.last_seen_at)}
              </span>
              {client.process_count > 0 && (
                <span>{client.process_count} process{client.process_count !== 1 ? 'es' : ''}</span>
              )}
            </div>
          </Link>
        ))}
      </div>
    </div>
  );
}
