import { useState, useEffect, useRef } from 'react';
import RoomScene from '../components/RoomScene';
import HUDOverlay from '../components/HUDOverlay';
import type { AgentState } from '../types/agent';
import { useAgentStatus } from '../hooks/useAgentStatus';
import { useSystemStatus } from '../hooks/useSystemStatus';
import { useAgents } from '../hooks/useAgents';

export default function HomePage() {
  const { catState: serverCatState } = useAgentStatus();
  const status = useSystemStatus();
  const agents = useAgents();

  // Manage displayed catState with client-side overrides
  const [catState, setCatState] = useState<AgentState>('idle');
  const successTimerRef = useRef<ReturnType<typeof setTimeout> | null>(null);
  const [messages, setMessages] = useState<string[]>(['Pixel Cat Dashboard online', 'All systems nominal']);

  useEffect(() => {
    // success is temporary (5s), then revert to server state
    if (serverCatState === 'success') {
      setCatState('success');
      if (successTimerRef.current) clearTimeout(successTimerRef.current);
      successTimerRef.current = setTimeout(() => {
        setCatState(serverCatState);
      }, 5000);
    } else {
      setCatState(serverCatState);
    }
  }, [serverCatState]);

  // Add state changes to log
  useEffect(() => {
    setMessages(prev => [...prev.slice(-9), `Cat state: ${catState}`]);
  }, [catState]);

  return (
    <div className="room-wrapper">
      <RoomScene catState={catState} />
      <HUDOverlay catState={catState} status={status} agents={agents} messages={messages} />
    </div>
  );
}
