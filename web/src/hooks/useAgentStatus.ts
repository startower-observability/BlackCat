import { useState, useEffect, useRef } from 'react';
import type { AgentState, CatStateResponse } from '../types/agent';

export function useAgentStatus() {
  const [catState, setCatState] = useState<AgentState>('idle');
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);
  const esRef = useRef<EventSource | null>(null);
  const retryTimerRef = useRef<ReturnType<typeof setTimeout> | null>(null);
  const retryCountRef = useRef(0);

  const fetchCatState = async () => {
    try {
      const res = await fetch('/dashboard/api/cat-state');
      if (res.status === 401) {
        window.location.href = '/dashboard/login';
        return;
      }
      if (!res.ok) throw new Error(`HTTP ${res.status}`);
      const data: CatStateResponse = await res.json();
      setCatState(data.state);
      setError(null);
    } catch (e) {
      setError(e instanceof Error ? e.message : 'Unknown error');
    } finally {
      setLoading(false);
    }
  };

  const connectSSE = () => {
    if (esRef.current) esRef.current.close();
    const es = new EventSource('/dashboard/events');
    esRef.current = es;

    es.onmessage = () => { fetchCatState(); };
    es.addEventListener('heartbeat', () => { fetchCatState(); });
    es.addEventListener('agent-update', () => { fetchCatState(); });

    es.onerror = () => {
      es.close();
      esRef.current = null;
      // Exponential backoff: 1s, 2s, 4s, 8s, 16s, max 30s
      const delay = Math.min(1000 * Math.pow(2, retryCountRef.current), 30000);
      retryCountRef.current++;
      retryTimerRef.current = setTimeout(() => { connectSSE(); }, delay);
    };

    es.onopen = () => { retryCountRef.current = 0; };
  };

  useEffect(() => {
    fetchCatState();
    connectSSE();
    return () => {
      if (esRef.current) esRef.current.close();
      if (retryTimerRef.current) clearTimeout(retryTimerRef.current);
    };
  // eslint-disable-next-line react-hooks/exhaustive-deps
  }, []);

  return { catState, loading, error };
}
