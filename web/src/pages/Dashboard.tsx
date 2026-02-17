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

function formatFriendlyDuration(dateStr: string): string {
  const since = new Date(dateStr).getTime();
  if (Number.isNaN(since)) return '-';
  let seconds = Math.max(0, Math.floor((Date.now() - since) / 1000));
  const units = [
    { label: 'yr', secs: 365 * 24 * 60 * 60 },
    { label: 'd', secs: 24 * 60 * 60 },
    { label: 'hr', secs: 60 * 60 },
    { label: 'min', secs: 60 },
  ];
  const parts: string[] = [];
  for (const unit of units) {
    if (parts.length >= 2) break;
    if (seconds < unit.secs) continue;
    const count = Math.floor(seconds / unit.secs);
    seconds -= count * unit.secs;
    parts.push(`${count}${unit.label}`);
  }
  if (parts.length === 0) return '<1min';
  if (parts.length === 1) return parts[0];
  return `${parts[0]} ${parts[1]}`;
}

function isoTooltip(dateStr: string): string {
  const parsed = new Date(dateStr);
  if (Number.isNaN(parsed.getTime())) return dateStr;
  return parsed.toISOString();
}

function clientLabel(client: ClientWithMetrics): string {
  return client.custom_name?.trim() || client.hostname;
}

function normalizePublicIP(ip?: string): string {
  return (ip || '').trim();
}

function ipv4SortValue(ip: string): number | null {
  const parts = ip.split('.');
  if (parts.length !== 4) return null;
  let value = 0;
  for (const part of parts) {
    if (!/^\d+$/.test(part)) return null;
    const octet = Number(part);
    if (octet < 0 || octet > 255) return null;
    value = value * 256 + octet;
  }
  return value;
}

function comparePublicIP(aRaw?: string, bRaw?: string): number {
  const a = normalizePublicIP(aRaw);
  const b = normalizePublicIP(bRaw);
  if (!a && !b) return 0;
  if (!a) return 1;
  if (!b) return -1;

  const aV4 = ipv4SortValue(a);
  const bV4 = ipv4SortValue(b);
  if (aV4 !== null && bV4 !== null) return aV4 - bV4;
  if (aV4 !== null) return -1;
  if (bV4 !== null) return 1;
  return a.localeCompare(b);
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
  const groupedClients = (() => {
    const sorted = [...clients].sort((a, b) => {
      const ipCmp = comparePublicIP(a.public_ip, b.public_ip);
      if (ipCmp !== 0) return ipCmp;
      return clientLabel(a).localeCompare(clientLabel(b));
    });

    const groupMap = new Map<string, ClientWithMetrics[]>();
    for (const client of sorted) {
      const ip = normalizePublicIP(client.public_ip) || '(no public IP)';
      const existing = groupMap.get(ip);
      if (existing) {
        existing.push(client);
        continue;
      }
      groupMap.set(ip, [client]);
    }

    return Array.from(groupMap.entries()).map(([ip, grouped]) => ({
      ip,
      clients: grouped,
    }));
  })();

  return (
    <div>
      <div className="flex items-center justify-between mb-6">
        <h1 className="text-2xl font-bold text-gray-900">Dashboard</h1>
        <div className="text-sm text-gray-500">
          {onlineCount}/{clients.length} online
        </div>
      </div>
      <div className="space-y-4">
        {groupedClients.map(group => (
          <section key={group.ip} className="bg-white rounded-lg border p-3 sm:p-4">
            <div className="flex items-center justify-between mb-3">
              <div className="text-sm font-semibold text-gray-700">
                Public IP: <span className="font-mono">{group.ip}</span>
              </div>
              <div className="text-xs text-gray-500">
                {group.clients.length} client{group.clients.length !== 1 ? 's' : ''}
              </div>
            </div>
            <div className="grid grid-cols-1 md:grid-cols-2 xl:grid-cols-3 gap-4">
              {group.clients.map(client => (
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
                  <div className="mt-3 grid grid-cols-[auto,1fr,auto] items-center gap-2 text-xs text-gray-400">
                    <span className="flex items-center gap-1">
                      <Clock size={12} /> {timeAgo(client.last_seen_at)}
                    </span>
                    <span className="justify-self-center" title={isoTooltip(client.session_started_at)}>
                      Uptime: {formatFriendlyDuration(client.session_started_at)}
                    </span>
                    <span className="justify-self-end">
                      {client.process_count} process{client.process_count !== 1 ? 'es' : ''}
                    </span>
                  </div>
                </Link>
              ))}
            </div>
          </section>
        ))}
      </div>
    </div>
  );
}
