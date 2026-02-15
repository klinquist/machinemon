import type { ClientWithMetrics, Client, Metrics, ProcessSnapshot, CheckSnapshot, Alert, Thresholds, AlertProvider, TestAlertResult } from '../types';

function normalizeBasePath(path: string): string {
  if (!path) return '';
  const trimmed = path.trim().replace(/^\/+|\/+$/g, '');
  return trimmed ? `/${trimmed}` : '';
}

// Anchor API calls to the server-injected base path (if any), so calls work from nested routes too.
const BASE_PATH = normalizeBasePath((window as any).__BASE_PATH__ || '');
const API_BASE = `${BASE_PATH}/api/v1/admin`;

function getAuthHeaders(): HeadersInit {
  const creds = sessionStorage.getItem('machinemon_auth');
  if (!creds) throw new AuthError();
  return {
    'Authorization': `Basic ${creds}`,
    'Content-Type': 'application/json',
  };
}

export class AuthError extends Error {
  constructor() {
    super('Not authenticated');
    this.name = 'AuthError';
  }
}

async function fetchJSON<T>(path: string, init?: RequestInit): Promise<T> {
  const res = await fetch(`${API_BASE}${path}`, {
    ...init,
    headers: { ...getAuthHeaders(), ...init?.headers },
  });
  if (res.status === 401) throw new AuthError();
  if (!res.ok) {
    const body = await res.text();
    throw new Error(`API error ${res.status}: ${body}`);
  }
  return res.json();
}

export function setAuth(username: string, password: string) {
  sessionStorage.setItem('machinemon_auth', btoa(`${username}:${password}`));
}

export function clearAuth() {
  sessionStorage.removeItem('machinemon_auth');
}

export function isAuthenticated(): boolean {
  return !!sessionStorage.getItem('machinemon_auth');
}

// Clients
export async function fetchClients(): Promise<ClientWithMetrics[]> {
  const data = await fetchJSON<{ clients: ClientWithMetrics[] }>('/clients');
  return data.clients;
}

export async function fetchClient(id: string): Promise<{ client: Client; metrics: Metrics | null; processes: ProcessSnapshot[]; checks?: CheckSnapshot[] }> {
  return fetchJSON(`/clients/${id}`);
}

export async function deleteClient(id: string): Promise<void> {
  await fetchJSON(`/clients/${id}`, { method: 'DELETE' });
}

export async function setThresholds(id: string, thresholds: Thresholds): Promise<void> {
  await fetchJSON(`/clients/${id}/thresholds`, {
    method: 'PUT',
    body: JSON.stringify(thresholds),
  });
}

export async function clearThresholds(id: string): Promise<void> {
  await fetchJSON(`/clients/${id}/thresholds`, {
    method: 'DELETE',
  });
}

export async function setClientName(id: string, name: string): Promise<void> {
  await fetchJSON(`/clients/${id}/name`, {
    method: 'PUT',
    body: JSON.stringify({ name }),
  });
}

export async function setMute(id: string, muted: boolean, durationMinutes?: number, reason?: string): Promise<void> {
  await fetchJSON(`/clients/${id}/mute`, {
    method: 'PUT',
    body: JSON.stringify({ muted, duration_minutes: durationMinutes || 0, reason: reason || '' }),
  });
}

export async function fetchMetrics(id: string, from?: string, to?: string): Promise<Metrics[]> {
  const params = new URLSearchParams();
  if (from) params.set('from', from);
  if (to) params.set('to', to);
  const data = await fetchJSON<{ metrics: Metrics[] }>(`/clients/${id}/metrics?${params}`);
  return data.metrics;
}

export async function fetchProcesses(id: string): Promise<{ watched: ProcessSnapshot[]; snapshots: ProcessSnapshot[] }> {
  return fetchJSON(`/clients/${id}/processes`);
}

export async function deleteWatchedProcess(id: string, friendlyName: string): Promise<void> {
  const params = new URLSearchParams({ friendly_name: friendlyName });
  await fetchJSON(`/clients/${id}/processes?${params.toString()}`, { method: 'DELETE' });
}

// Alerts
export async function fetchAlerts(clientId?: string, severity?: string, limit = 100, offset = 0): Promise<{ alerts: Alert[]; total: number }> {
  const params = new URLSearchParams({ limit: String(limit), offset: String(offset) });
  if (clientId) params.set('client_id', clientId);
  if (severity) params.set('severity', severity);
  return fetchJSON(`/alerts?${params}`);
}

// Providers
export async function fetchProviders(): Promise<AlertProvider[]> {
  const data = await fetchJSON<{ providers: AlertProvider[] }>('/providers');
  return data.providers;
}

export async function createProvider(provider: Partial<AlertProvider>): Promise<AlertProvider> {
  return fetchJSON('/providers', { method: 'POST', body: JSON.stringify(provider) });
}

export async function updateProvider(id: number, provider: Partial<AlertProvider>): Promise<void> {
  await fetchJSON(`/providers/${id}`, { method: 'PUT', body: JSON.stringify(provider) });
}

export async function deleteProvider(id: number): Promise<void> {
  await fetchJSON(`/providers/${id}`, { method: 'DELETE' });
}

export async function testProvider(id: number): Promise<{ status: string; result?: TestAlertResult }> {
  return fetchJSON(`/providers/${id}/test`, { method: 'POST' });
}

// Settings
export async function fetchSettings(): Promise<Record<string, string>> {
  return fetchJSON('/settings');
}

export async function updateSettings(settings: Record<string, string>): Promise<void> {
  await fetchJSON('/settings', { method: 'PUT', body: JSON.stringify(settings) });
}

export async function changePassword(type: 'admin' | 'client', password: string): Promise<void> {
  await fetchJSON('/password', { method: 'PUT', body: JSON.stringify({ type, password }) });
}
