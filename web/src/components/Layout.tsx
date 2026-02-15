import { useEffect, useState } from 'react';
import { Outlet, Link, useLocation } from 'react-router-dom';
import { clearAuth } from '../api/client';
import { Monitor, Bell, Settings, LogOut, Menu, X } from 'lucide-react';

export default function Layout({ onLogout }: { onLogout: () => void }) {
  const location = useLocation();
  const [mobileMenuOpen, setMobileMenuOpen] = useState(false);

  const navItems = [
    { path: '/', label: 'Dashboard', icon: Monitor },
    { path: '/alerts', label: 'Alerts', icon: Bell },
    { path: '/settings', label: 'Settings', icon: Settings },
  ];

  useEffect(() => {
    setMobileMenuOpen(false);
  }, [location.pathname]);

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
    </div>
  );
}
