import { useState, useEffect, useRef } from 'react';
import type { SubsystemHealth } from '../types/agent';

export function useAgents() {
  const [agents, setAgents] = useState<SubsystemHealth[]>([]);
  const esRef = useRef<EventSource | null>(null);

  const fetchAgents = async () => {
    try {
      const res = await fetch('/dashboard/api/agents');
      if (res.status === 401) { window.location.href = '/dashboard/login'; return; }
      if (!res.ok) return;
      const data = await res.json();
      // Map subsystems array to SubsystemHealth
      const subsystems = data.subsystems || data || [];
      setAgents(subsystems.map((s: Record<string, string>) => ({
        name: s.name,
        status: s.status || s.state || 'unknown',
        message: s.message || s.description,
      })));
    } catch { /* ignore */ }
  };

  useEffect(() => {
    fetchAgents();
    const es = new EventSource('/dashboard/events');
    esRef.current = es;
    es.addEventListener('agent-update', () => fetchAgents());
    es.onmessage = () => fetchAgents();
    return () => { es.close(); };
  // eslint-disable-next-line react-hooks/exhaustive-deps
  }, []);

  return agents;
}
