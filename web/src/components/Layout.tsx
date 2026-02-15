import { Outlet, Link, useLocation } from 'react-router-dom';
import { clearAuth } from '../api/client';
import { Monitor, Bell, Settings, LogOut } from 'lucide-react';

export default function Layout({ onLogout }: { onLogout: () => void }) {
  const location = useLocation();

  const navItems = [
    { path: '/', label: 'Dashboard', icon: Monitor },
    { path: '/alerts', label: 'Alerts', icon: Bell },
    { path: '/settings', label: 'Settings', icon: Settings },
  ];

  return (
    <div className="min-h-screen bg-gray-50 flex">
      <nav className="w-56 bg-gray-900 text-white flex flex-col min-h-screen fixed">
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
      <main className="flex-1 ml-56 p-6">
        <Outlet />
      </main>
    </div>
  );
}
