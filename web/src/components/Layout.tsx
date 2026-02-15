import { useEffect, useState } from 'react';
import { Outlet, Link, useLocation } from 'react-router-dom';
import { clearAuth } from '../api/client';
import { Monitor, Bell, Settings, LogOut, Menu, X, Download } from 'lucide-react';

export default function Layout({ onLogout }: { onLogout: () => void }) {
  const location = useLocation();
  const [mobileMenuOpen, setMobileMenuOpen] = useState(false);
  const [installHelpOpen, setInstallHelpOpen] = useState(false);
  const [copyStatus, setCopyStatus] = useState('');

  const basePath = (window as any).__BASE_PATH__ || '';
  const downloadBase = `${window.location.origin}${basePath}/download`;
  const installScriptURL = `${downloadBase}/install.sh`;
  const installCommand = `curl -sSL ${installScriptURL} | sh`;
  const installCommandInsecure = `curl -sSL --insecure ${installScriptURL} | sh -s -- --insecure`;

  const navItems = [
    { path: '/', label: 'Dashboard', icon: Monitor },
    { path: '/alerts', label: 'Alerts', icon: Bell },
    { path: '/settings', label: 'Settings', icon: Settings },
  ];

  useEffect(() => {
    setMobileMenuOpen(false);
  }, [location.pathname]);

  useEffect(() => {
    if (!installHelpOpen) return;
    const onKeyDown = (e: KeyboardEvent) => {
      if (e.key === 'Escape') setInstallHelpOpen(false);
    };
    window.addEventListener('keydown', onKeyDown);
    return () => window.removeEventListener('keydown', onKeyDown);
  }, [installHelpOpen]);

  useEffect(() => {
    if (!copyStatus) return;
    const t = window.setTimeout(() => setCopyStatus(''), 1600);
    return () => window.clearTimeout(t);
  }, [copyStatus]);

  const copyText = async (value: string, label: string) => {
    try {
      await navigator.clipboard.writeText(value);
      setCopyStatus(`${label} copied`);
    } catch {
      setCopyStatus('Copy failed');
    }
  };

  return (
    <div className="min-h-screen bg-gray-50 md:flex">
      <header className="md:hidden sticky top-0 z-40 bg-gray-900 text-white border-b border-gray-700">
        <div className="h-14 px-4 flex items-center justify-between">
          <h1 className="text-base font-bold tracking-tight">MachineMon</h1>
          <button
            onClick={() => setMobileMenuOpen(v => !v)}
            className="p-2 rounded-md hover:bg-gray-800"
            aria-label={mobileMenuOpen ? 'Close menu' : 'Open menu'}
          >
            {mobileMenuOpen ? <X size={18} /> : <Menu size={18} />}
          </button>
        </div>
      </header>

      {mobileMenuOpen && (
        <button
          className="md:hidden fixed inset-0 z-30 bg-black/40"
          onClick={() => setMobileMenuOpen(false)}
          aria-label="Close menu overlay"
        />
      )}

      <nav className={`w-64 bg-gray-900 text-white flex flex-col fixed z-40 top-0 bottom-0 transform transition-transform duration-200 md:translate-x-0 ${
        mobileMenuOpen ? 'translate-x-0' : '-translate-x-full'
      } md:w-56 md:min-h-screen`}>
        <div className="p-4 border-b border-gray-700">
          <h1 className="text-lg font-bold tracking-tight">MachineMon</h1>
        </div>
        <div className="flex-1 py-4">
          {navItems.map(({ path, label, icon: Icon }) => (
            <Link
              key={path}
              to={path}
              className={`flex items-center gap-3 px-4 py-2.5 text-sm transition-colors ${
                location.pathname === path
                  ? 'bg-gray-700 text-white'
                  : 'text-gray-300 hover:bg-gray-800 hover:text-white'
              }`}
            >
              <Icon size={18} />
              {label}
            </Link>
          ))}
        </div>
        <div className="p-4 border-t border-gray-700">
          <button
            onClick={() => setInstallHelpOpen(true)}
            className="flex items-center gap-2 text-sm text-gray-300 hover:text-white transition-colors w-full mb-3"
          >
            <Download size={16} />
            How to install on a client
          </button>
          <button
            onClick={() => { clearAuth(); onLogout(); }}
            className="flex items-center gap-2 text-sm text-gray-400 hover:text-white transition-colors w-full"
          >
            <LogOut size={16} />
            Sign out
          </button>
        </div>
      </nav>
      <main className="flex-1 md:ml-56 p-4 md:p-6">
        <Outlet />
      </main>

      {installHelpOpen && (
        <div className="fixed inset-0 z-50 flex items-center justify-center px-4">
          <div className="absolute inset-0 bg-black/50" onClick={() => setInstallHelpOpen(false)} />
          <div className="relative w-full max-w-2xl bg-white rounded-xl shadow-xl border">
            <div className="flex items-center justify-between px-4 py-3 border-b">
              <h3 className="font-semibold text-gray-800">Install Client</h3>
              <button
                onClick={() => setInstallHelpOpen(false)}
                className="p-1.5 rounded hover:bg-gray-100 text-gray-500"
                aria-label="Close install help"
              >
                <X size={16} />
              </button>
            </div>

            <div className="p-4 space-y-4">
              <p className="text-sm text-gray-600">
                Assumes client archives are already uploaded to your server binaries directory and exposed under <code>/download</code>.
              </p>

              <div>
                <div className="text-sm font-medium text-gray-700 mb-1">One-line install</div>
                <div className="bg-gray-900 text-gray-100 rounded-md p-3 text-xs font-mono break-all">{installCommand}</div>
                <button
                  onClick={() => copyText(installCommand, 'Install command')}
                  className="mt-1 text-xs text-blue-600 hover:text-blue-700"
                >
                  Copy command
                </button>
              </div>

              <div>
                <div className="text-sm font-medium text-gray-700 mb-1">If using self-signed TLS</div>
                <div className="bg-gray-900 text-gray-100 rounded-md p-3 text-xs font-mono break-all">{installCommandInsecure}</div>
                <button
                  onClick={() => copyText(installCommandInsecure, 'Insecure install command')}
                  className="mt-1 text-xs text-blue-600 hover:text-blue-700"
                >
                  Copy command
                </button>
              </div>

              <div>
                <div className="text-sm font-medium text-gray-700 mb-1">Download links</div>
                <div className="text-sm text-blue-600 space-y-1">
                  <div><a href={`${downloadBase}/`} target="_blank" rel="noreferrer" className="hover:underline">{downloadBase}/</a></div>
                  <div><a href={`${downloadBase}/install.sh`} target="_blank" rel="noreferrer" className="hover:underline">{downloadBase}/install.sh</a></div>
                  <div><a href={`${downloadBase}/machinemon-client-linux-amd64.tar.gz`} target="_blank" rel="noreferrer" className="hover:underline">{downloadBase}/machinemon-client-linux-amd64.tar.gz</a></div>
                  <div><a href={`${downloadBase}/machinemon-client-linux-arm64.tar.gz`} target="_blank" rel="noreferrer" className="hover:underline">{downloadBase}/machinemon-client-linux-arm64.tar.gz</a></div>
                  <div><a href={`${downloadBase}/machinemon-client-darwin-arm64.tar.gz`} target="_blank" rel="noreferrer" className="hover:underline">{downloadBase}/machinemon-client-darwin-arm64.tar.gz</a></div>
                </div>
              </div>

              {copyStatus && (
                <div className="text-xs text-green-700 bg-green-50 rounded px-2 py-1 inline-block">{copyStatus}</div>
              )}
            </div>
          </div>
        </div>
      )}
    </div>
  );
}
