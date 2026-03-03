import { useState, useEffect, useRef } from 'react';
import type { SystemStatus } from '../types/agent';

export function useSystemStatus() {
  const [status, setStatus] = useState<SystemStatus>({});
  const esRef = useRef<EventSource | null>(null);

  const fetchStatus = async () => {
    try {
      const res = await fetch('/dashboard/api/status');
      if (res.status === 401) { window.location.href = '/dashboard/login'; return; }
      if (!res.ok) return;
      const data = await res.json();
      // Map Go response fields to our type
      setStatus({
        version: data.version,
        uptime: data.uptime,
        healthy: data.healthy,
        subsystem_count: data.subsystem_count ?? data.subsystems?.length,
      });
    } catch { /* ignore */ }
  };

  useEffect(() => {
    fetchStatus();
    const es = new EventSource('/dashboard/events');
    esRef.current = es;
    es.addEventListener('heartbeat', () => fetchStatus());
    es.onmessage = () => fetchStatus();
    return () => { es.close(); };
  // eslint-disable-next-line react-hooks/exhaustive-deps
  }, []);

  return status;
}
