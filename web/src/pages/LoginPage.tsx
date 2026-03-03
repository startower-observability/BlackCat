import { useState, type FormEvent } from 'react';
import { useNavigate } from 'react-router-dom';

export default function LoginPage() {
  const [error, setError] = useState('');
  const [loading, setLoading] = useState(false);
  const navigate = useNavigate();

  const handleSubmit = async (e: FormEvent<HTMLFormElement>) => {
    e.preventDefault();
    setLoading(true);
    setError('');

    const form = e.currentTarget;
    const token = (form.elements.namedItem('token') as HTMLInputElement).value;

    try {
      const res = await fetch('/dashboard/login', {
        method: 'POST',
        headers: { 'Content-Type': 'application/x-www-form-urlencoded' },
        body: new URLSearchParams({ token, next: '/dashboard/' }),
        redirect: 'manual',
      });

      // opaqueredirect = success (302 redirect intercepted)
      if (res.type === 'opaqueredirect' || res.status === 302 || res.status === 303 || res.ok) {
        navigate('/');
      } else {
        setError('INVALID TOKEN');
      }
    } catch {
      setError('CONNECTION ERROR');
    } finally {
      setLoading(false);
    }
  };

  return (
    <div style={{ minHeight: '100vh', display: 'flex', alignItems: 'center', justifyContent: 'center', background: '#0d1117' }}>
      <div className="rpg-panel" style={{ width: '320px', padding: '24px' }}>
        <h1 style={{ fontSize: '10px', marginBottom: '24px', textAlign: 'center', letterSpacing: '2px' }}>
          BLACKCAT
        </h1>
        <p style={{ fontSize: '7px', color: '#484f58', textAlign: 'center', marginBottom: '24px' }}>
          ENTER ACCESS TOKEN
        </p>
        <form onSubmit={handleSubmit} style={{ display: 'flex', flexDirection: 'column', gap: '16px' }}>
          <input
            type="password"
            name="token"
            placeholder="••••••••"
            required
            style={{
              background: '#161b22',
              border: '2px solid #30363d',
              color: '#c9d1d9',
              padding: '8px 12px',
              fontFamily: "'Press Start 2P', monospace",
              fontSize: '8px',
              outline: 'none',
              width: '100%',
              boxSizing: 'border-box',
            }}
          />
          {error && (
            <div style={{ color: '#da3633', fontSize: '7px', textAlign: 'center' }}>
              {error}
            </div>
          )}
          <button
            type="submit"
            disabled={loading}
            style={{
              background: loading ? '#161b22' : '#1f6feb',
              border: '2px solid #388bfd',
              color: '#ffffff',
              padding: '10px',
              fontFamily: "'Press Start 2P', monospace",
              fontSize: '8px',
              cursor: loading ? 'not-allowed' : 'pointer',
              width: '100%',
            }}
          >
            {loading ? 'LOADING...' : 'LOGIN'}
          </button>
        </form>
      </div>
    </div>
  );
}
