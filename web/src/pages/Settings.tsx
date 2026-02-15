import { useState, useEffect } from 'react';
import { fetchProviders, createProvider, updateProvider, deleteProvider, testProvider, changePassword, fetchSettings, updateSettings } from '../api/client';
import type { AlertProvider } from '../types';
import { Trash2, Send, Plus, Pencil } from 'lucide-react';

export default function Settings() {
  const [providers, setProviders] = useState<AlertProvider[]>([]);
  const [showAddProvider, setShowAddProvider] = useState(false);
  const [providerType, setProviderType] = useState<'twilio' | 'pushover' | 'smtp'>('pushover');
  const [providerName, setProviderName] = useState('');
  const [providerConfig, setProviderConfig] = useState('{}');
  const [editingProviderId, setEditingProviderId] = useState<number | null>(null);
  const [editProviderName, setEditProviderName] = useState('');
  const [editProviderConfig, setEditProviderConfig] = useState('{}');
  const [editProviderEnabled, setEditProviderEnabled] = useState(true);
  const [settings, setSettings] = useState<Record<string, string>>({});
  const [adminPw, setAdminPw] = useState('');
  const [clientPw, setClientPw] = useState('');
  const [message, setMessage] = useState('');

  const loadData = async () => {
    try {
      const [p, s] = await Promise.all([fetchProviders(), fetchSettings()]);
      setProviders(p);
      setSettings(s);
    } catch {
      // ignore
    }
  };

  useEffect(() => { loadData(); }, []);

  const handleAddProvider = async (e: React.FormEvent) => {
    e.preventDefault();
    try {
      await createProvider({ type: providerType, name: providerName, enabled: true, config: providerConfig });
      setShowAddProvider(false);
      setProviderName('');
      setProviderConfig('{}');
      loadData();
      setMessage('Provider added');
    } catch (err: any) {
      setMessage(`Error: ${err.message}`);
    }
  };

  const handleDeleteProvider = async (id: number) => {
    if (!confirm('Delete this alert provider?')) return;
    await deleteProvider(id);
    if (editingProviderId === id) {
      setEditingProviderId(null);
    }
    loadData();
  };

  const startEditProvider = (provider: AlertProvider) => {
    setShowAddProvider(false);
    setEditingProviderId(provider.id);
    setEditProviderName(provider.name);
    setEditProviderConfig(provider.config || '{}');
    setEditProviderEnabled(provider.enabled);
  };

  const cancelEditProvider = () => {
    setEditingProviderId(null);
    setEditProviderName('');
    setEditProviderConfig('{}');
    setEditProviderEnabled(true);
  };

  const handleSaveProviderEdit = async (provider: AlertProvider) => {
    try {
      await updateProvider(provider.id, {
        type: provider.type,
        name: editProviderName,
        enabled: editProviderEnabled,
        config: editProviderConfig,
      });
      setMessage('Provider updated');
      cancelEditProvider();
      loadData();
    } catch (err: any) {
      setMessage(`Error: ${err.message}`);
    }
  };

  const handleTestProvider = async (id: number) => {
    try {
      const res = await testProvider(id);
      if (res.result?.api_response) {
        setMessage(`${res.result.message}. API: ${res.result.api_response}`);
      } else if (res.result?.message) {
        setMessage(res.result.message);
      } else {
        setMessage(res.status || 'Test alert sent');
      }
    } catch (err: any) {
      setMessage(`Test failed: ${err.message}`);
    }
  };

  const handleSaveThresholds = async () => {
    try {
      await updateSettings(settings);
      setMessage('Thresholds saved');
    } catch (err: any) {
      setMessage(`Error: ${err.message}`);
    }
  };

  const handleChangePassword = async (type: 'admin' | 'client') => {
    const pw = type === 'admin' ? adminPw : clientPw;
    if (!pw) return;
    try {
      await changePassword(type, pw);
      setMessage(`${type} password updated`);
      if (type === 'admin') setAdminPw('');
      else setClientPw('');
    } catch (err: any) {
      setMessage(`Error: ${err.message}`);
    }
  };

  const configTemplates: Record<string, string> = {
    pushover: JSON.stringify({ app_token: '', user_key: '' }, null, 2),
    twilio: JSON.stringify({ account_sid: '', auth_token: '', from_number: '', to_number: '' }, null, 2),
    smtp: JSON.stringify({ host: '', port: 587, username: '', password: '', from: '', to: '', use_tls: false }, null, 2),
  };

  return (
    <div className="max-w-2xl">
      <h1 className="text-2xl font-bold text-gray-900 mb-6">Settings</h1>

      {message && (
        <div className="mb-4 px-4 py-2 bg-blue-50 text-blue-700 rounded text-sm">
          {message}
          <button onClick={() => setMessage('')} className="ml-2 text-blue-500">&times;</button>
        </div>
      )}

      {/* Default Thresholds */}
      <section className="bg-white rounded-lg border p-4 mb-6">
        <h2 className="font-semibold text-gray-700 mb-4">Default Thresholds</h2>
        <div className="grid grid-cols-2 gap-4">
          {[
            ['cpu_warn_pct_default', 'CPU Warning %'],
            ['cpu_crit_pct_default', 'CPU Critical %'],
            ['mem_warn_pct_default', 'Memory Warning %'],
            ['mem_crit_pct_default', 'Memory Critical %'],
            ['disk_warn_pct_default', 'Disk Warning %'],
            ['disk_crit_pct_default', 'Disk Critical %'],
          ].map(([key, label]) => (
            <div key={key}>
              <label className="block text-sm text-gray-600 mb-1">{label}</label>
              <input
                type="number"
                value={settings[key] || ''}
                onChange={e => setSettings({ ...settings, [key]: e.target.value })}
                className="w-full px-3 py-1.5 border rounded text-sm"
                placeholder="e.g. 80"
              />
            </div>
          ))}
        </div>
        <div className="mt-4">
          <label className="block text-sm text-gray-600 mb-1">Metric Retention (days)</label>
          <input
            type="number"
            min={1}
            value={settings['metrics_retention_days'] || '14'}
            onChange={e => setSettings({ ...settings, metrics_retention_days: e.target.value })}
            className="w-full max-w-xs px-3 py-1.5 border rounded text-sm"
            placeholder="14"
          />
          <p className="text-xs text-gray-500 mt-1">
            CPU, memory, disk, process, and check history older than this is automatically deleted daily.
          </p>
        </div>
        <button onClick={handleSaveThresholds} className="mt-4 px-4 py-2 bg-blue-600 text-white rounded text-sm hover:bg-blue-700">
          Save Thresholds
        </button>
      </section>

      {/* Alert Providers */}
      <section className="bg-white rounded-lg border p-4 mb-6">
        <div className="flex items-center justify-between mb-4">
          <h2 className="font-semibold text-gray-700">Alert Providers</h2>
          <button onClick={() => { setShowAddProvider(true); setProviderConfig(configTemplates[providerType]); }} className="flex items-center gap-1 text-sm text-blue-600 hover:text-blue-700">
            <Plus size={16} /> Add Provider
          </button>
        </div>

        {providers.length === 0 && !showAddProvider && (
          <p className="text-sm text-gray-400">No alert providers configured. Add one to receive notifications.</p>
        )}

        {providers.map(p => (
          <div key={p.id} className="py-2 border-b last:border-0">
            {editingProviderId === p.id ? (
              <div className="p-3 bg-gray-50 rounded border">
                <div className="grid grid-cols-2 gap-3 mb-3">
                  <div>
                    <label className="block text-sm text-gray-600 mb-1">Name</label>
                    <input
                      value={editProviderName}
                      onChange={e => setEditProviderName(e.target.value)}
                      className="w-full px-3 py-1.5 border rounded text-sm"
                      required
                    />
                  </div>
                  <div className="flex items-end justify-between">
                    <div>
                      <span className="text-xs text-gray-400 bg-gray-100 px-2 py-0.5 rounded">{p.type}</span>
                    </div>
                    <label className="text-sm text-gray-600 flex items-center gap-2">
                      <input
                        type="checkbox"
                        checked={editProviderEnabled}
                        onChange={e => setEditProviderEnabled(e.target.checked)}
                      />
                      Enabled
                    </label>
                  </div>
                </div>
                <div>
                  <label className="block text-sm text-gray-600 mb-1">Config (JSON)</label>
                  <textarea
                    value={editProviderConfig}
                    onChange={e => setEditProviderConfig(e.target.value)}
                    className="w-full px-3 py-1.5 border rounded text-sm font-mono"
                    rows={5}
                  />
                </div>
                <div className="flex gap-2 mt-3">
                  <button type="button" onClick={() => handleSaveProviderEdit(p)} className="px-4 py-2 bg-blue-600 text-white rounded text-sm hover:bg-blue-700">
                    Save
                  </button>
                  <button type="button" onClick={cancelEditProvider} className="px-4 py-2 border rounded text-sm hover:bg-gray-50">
                    Cancel
                  </button>
                </div>
              </div>
            ) : (
              <div className="flex items-center justify-between">
                <div>
                  <span className="font-medium text-gray-700">{p.name}</span>
                  <span className="ml-2 text-xs text-gray-400 bg-gray-100 px-2 py-0.5 rounded">{p.type}</span>
                  {!p.enabled && <span className="ml-2 text-xs text-red-400">disabled</span>}
                </div>
                <div className="flex gap-1">
                  <button onClick={() => startEditProvider(p)} className="p-1.5 text-gray-400 hover:text-amber-600" title="Edit provider">
                    <Pencil size={14} />
                  </button>
                  <button onClick={() => handleTestProvider(p.id)} className="p-1.5 text-gray-400 hover:text-blue-500" title="Send test alert">
                    <Send size={14} />
                  </button>
                  <button onClick={() => handleDeleteProvider(p.id)} className="p-1.5 text-gray-400 hover:text-red-500" title="Delete">
                    <Trash2 size={14} />
                  </button>
                </div>
              </div>
            )}
          </div>
        ))}

        {showAddProvider && (
          <form onSubmit={handleAddProvider} className="mt-4 p-4 bg-gray-50 rounded">
            <div className="grid grid-cols-2 gap-3 mb-3">
              <div>
                <label className="block text-sm text-gray-600 mb-1">Name</label>
                <input
                  value={providerName}
                  onChange={e => setProviderName(e.target.value)}
                  className="w-full px-3 py-1.5 border rounded text-sm"
                  placeholder="My Pushover"
                  required
                />
              </div>
              <div>
                <label className="block text-sm text-gray-600 mb-1">Type</label>
                <select
                  value={providerType}
                  onChange={e => { setProviderType(e.target.value as any); setProviderConfig(configTemplates[e.target.value]); }}
                  className="w-full px-3 py-1.5 border rounded text-sm"
                >
                  <option value="pushover">Pushover</option>
                  <option value="twilio">Twilio SMS</option>
                  <option value="smtp">SMTP Email</option>
                </select>
              </div>
            </div>
            <div>
              <label className="block text-sm text-gray-600 mb-1">Config (JSON)</label>
              <textarea
                value={providerConfig}
                onChange={e => setProviderConfig(e.target.value)}
                className="w-full px-3 py-1.5 border rounded text-sm font-mono"
                rows={5}
              />
            </div>
            <div className="flex gap-2 mt-3">
              <button type="submit" className="px-4 py-2 bg-blue-600 text-white rounded text-sm hover:bg-blue-700">Add</button>
              <button type="button" onClick={() => setShowAddProvider(false)} className="px-4 py-2 border rounded text-sm hover:bg-gray-50">Cancel</button>
            </div>
          </form>
        )}
      </section>

      {/* Passwords */}
      <section className="bg-white rounded-lg border p-4">
        <h2 className="font-semibold text-gray-700 mb-4">Passwords</h2>
        <div className="space-y-4">
          <div className="flex items-end gap-3">
            <div className="flex-1">
              <label className="block text-sm text-gray-600 mb-1">Admin Password</label>
              <input
                type="password"
                value={adminPw}
                onChange={e => setAdminPw(e.target.value)}
                className="w-full px-3 py-1.5 border rounded text-sm"
                placeholder="New admin password"
              />
            </div>
            <button onClick={() => handleChangePassword('admin')} disabled={!adminPw} className="px-4 py-1.5 bg-gray-800 text-white rounded text-sm hover:bg-gray-900 disabled:opacity-50">
              Update
            </button>
          </div>
          <div className="flex items-end gap-3">
            <div className="flex-1">
              <label className="block text-sm text-gray-600 mb-1">Client Password</label>
              <input
                type="password"
                value={clientPw}
                onChange={e => setClientPw(e.target.value)}
                className="w-full px-3 py-1.5 border rounded text-sm"
                placeholder="New client password"
              />
            </div>
            <button onClick={() => handleChangePassword('client')} disabled={!clientPw} className="px-4 py-1.5 bg-gray-800 text-white rounded text-sm hover:bg-gray-900 disabled:opacity-50">
              Update
            </button>
          </div>
        </div>
      </section>
    </div>
  );
}
