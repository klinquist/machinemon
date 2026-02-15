import { useState, useEffect } from 'react';
import { fetchAlerts } from '../api/client';
import type { Alert } from '../types';

const severityColors: Record<string, string> = {
  critical: 'bg-red-100 text-red-700',
  warning: 'bg-amber-100 text-amber-700',
  info: 'bg-blue-100 text-blue-700',
};

const alertTypeLabels: Record<string, string> = {
  offline: 'Offline',
  online: 'Online',
  pid_change: 'PID Change',
  process_died: 'Process Died',
  check_failed: 'Check Failed',
  check_recovered: 'Check Recovered',
  cpu_warn: 'CPU Warning',
  cpu_crit: 'CPU Critical',
  cpu_recover: 'CPU Recovered',
  mem_warn: 'Memory Warning',
  mem_crit: 'Memory Critical',
  mem_recover: 'Memory Recovered',
  disk_warn: 'Disk Warning',
  disk_crit: 'Disk Critical',
  disk_recover: 'Disk Recovered',
};

export default function Alerts() {
  const [alerts, setAlerts] = useState<Alert[]>([]);
  const [total, setTotal] = useState(0);
  const [offset, setOffset] = useState(0);
  const [severity, setSeverity] = useState('');
  const [loading, setLoading] = useState(true);
  const limit = 50;

  const load = async () => {
    setLoading(true);
    try {
      const data = await fetchAlerts(undefined, severity, limit, offset);
      setAlerts(data.alerts);
      setTotal(data.total);
    } catch {
      // ignore
    } finally {
      setLoading(false);
    }
  };

  useEffect(() => { load(); }, [severity, offset]);

  return (
    <div>
      <div className="flex items-center justify-between mb-6">
        <h1 className="text-2xl font-bold text-gray-900">Alerts</h1>
        <div className="flex gap-2">
          {['', 'critical', 'warning', 'info'].map(s => (
            <button
              key={s}
              onClick={() => { setSeverity(s); setOffset(0); }}
              className={`px-3 py-1 text-sm rounded ${severity === s ? 'bg-blue-100 text-blue-700' : 'text-gray-500 hover:bg-gray-100'}`}
            >
              {s || 'All'}
            </button>
          ))}
        </div>
      </div>

      {loading ? (
        <div className="text-gray-500">Loading...</div>
      ) : alerts.length === 0 ? (
        <div className="text-center py-16 text-gray-400">No alerts found</div>
      ) : (
        <>
          <div className="bg-white rounded-lg border overflow-hidden">
            <table className="w-full text-sm">
              <thead>
                <tr className="text-left text-gray-500 bg-gray-50 border-b">
                  <th className="px-4 py-3">Time</th>
                  <th className="px-4 py-3">Severity</th>
                  <th className="px-4 py-3">Type</th>
                  <th className="px-4 py-3">Message</th>
                </tr>
              </thead>
              <tbody>
                {alerts.map(a => (
                  <tr key={a.id} className="border-b last:border-0 hover:bg-gray-50">
                    <td className="px-4 py-3 text-gray-500 whitespace-nowrap">
                      {new Date(a.fired_at).toLocaleString([], { month: 'short', day: 'numeric', hour: '2-digit', minute: '2-digit', second: '2-digit' })}
                    </td>
                    <td className="px-4 py-3">
                      <span className={`px-2 py-0.5 rounded text-xs font-medium ${severityColors[a.severity] || 'bg-gray-100'}`}>
                        {a.severity}
                      </span>
                    </td>
                    <td className="px-4 py-3 text-gray-600 whitespace-nowrap">
                      {alertTypeLabels[a.alert_type] || a.alert_type}
                    </td>
                    <td className="px-4 py-3 text-gray-700">{a.message}</td>
                  </tr>
                ))}
              </tbody>
            </table>
          </div>

          {total > limit && (
            <div className="flex items-center justify-between mt-4 text-sm text-gray-500">
              <span>Showing {offset + 1}-{Math.min(offset + limit, total)} of {total}</span>
              <div className="flex gap-2">
                <button
                  disabled={offset === 0}
                  onClick={() => setOffset(Math.max(0, offset - limit))}
                  className="px-3 py-1 border rounded disabled:opacity-50 hover:bg-gray-50"
                >
                  Previous
                </button>
                <button
                  disabled={offset + limit >= total}
                  onClick={() => setOffset(offset + limit)}
                  className="px-3 py-1 border rounded disabled:opacity-50 hover:bg-gray-50"
                >
                  Next
                </button>
              </div>
            </div>
          )}
        </>
      )}
    </div>
  );
}
