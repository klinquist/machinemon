import { BrowserRouter, Routes, Route, Navigate } from 'react-router-dom';
import { useState, useEffect } from 'react';
import { isAuthenticated } from './api/client';
import Layout from './components/Layout';
import Login from './pages/Login';
import Dashboard from './pages/Dashboard';
import ClientDetail from './pages/ClientDetail';
import Alerts from './pages/Alerts';
import Settings from './pages/Settings';

// Read base path injected by server (for subpath deployments behind reverse proxy)
const basePath = (window as any).__BASE_PATH__ || '';

function App() {
  const [authed, setAuthed] = useState(isAuthenticated());

  useEffect(() => {
    const check = () => setAuthed(isAuthenticated());
    window.addEventListener('storage', check);
    return () => window.removeEventListener('storage', check);
  }, []);

  if (!authed) {
    return <Login onLogin={() => setAuthed(true)} />;
  }

  return (
    <BrowserRouter basename={basePath}>
      <Routes>
        <Route element={<Layout onLogout={() => setAuthed(false)} />}>
          <Route path="/" element={<Dashboard />} />
          <Route path="/clients/:id" element={<ClientDetail />} />
          <Route path="/alerts" element={<Alerts />} />
          <Route path="/settings" element={<Settings />} />
          <Route path="*" element={<Navigate to="/" />} />
        </Route>
      </Routes>
    </BrowserRouter>
  );
}

export default App;
