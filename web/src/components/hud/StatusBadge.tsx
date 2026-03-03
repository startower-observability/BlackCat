import type { AgentState } from '../../types/agent';

export default function StatusBadge({ state }: { state: AgentState }) {
  return (
    <div style={{ position: 'absolute', top: 8, left: 8 }}>
      <span className={`badge badge-${state}`}>{state}</span>
    </div>
  );
}
