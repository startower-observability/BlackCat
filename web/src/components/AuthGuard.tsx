import { useState, useEffect } from 'react';
import { Navigate, Outlet } from 'react-router-dom';

type AuthState = 'checking' | 'authenticated' | 'unauthenticated';

export default function AuthGuard() {
  const [authState, setAuthState] = useState<AuthState>('checking');

  useEffect(() => {
    fetch('/dashboard/api/me')
      .then(res => {
        if (res.ok) setAuthState('authenticated');
        else setAuthState('unauthenticated');
      })
      .catch(() => setAuthState('unauthenticated'));
  }, []);

  if (authState === 'checking') {
    return (
      <div style={{ minHeight: '100vh', display: 'flex', alignItems: 'center', justifyContent: 'center', background: '#0d1117' }}>
        <span style={{ color: '#484f58', fontFamily: "'Press Start 2P', monospace", fontSize: '8px' }}>LOADING...</span>
      </div>
    );
  }

  if (authState === 'unauthenticated') {
    return <Navigate to="/login" replace />;
  }

  return <Outlet />;
}
