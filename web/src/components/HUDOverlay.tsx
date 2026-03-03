import type { AgentState, SystemStatus, SubsystemHealth } from '../types/agent';
import StatusBadge from './hud/StatusBadge';
import SystemInfo from './hud/SystemInfo';
import AgentCards from './hud/AgentCards';
import LogBubble from './hud/LogBubble';

interface HUDOverlayProps {
  catState: AgentState;
  status: SystemStatus;
  agents: SubsystemHealth[];
  messages: string[];
}

export default function HUDOverlay({ catState, status, agents, messages }: HUDOverlayProps) {
  return (
    <div className="hud-overlay">
      <StatusBadge state={catState} />
      <SystemInfo status={status} />
      <AgentCards agents={agents} />
      <LogBubble messages={messages} />
    </div>
  );
}
