import type { SubsystemHealth } from '../../types/agent';

export default function AgentCards({ agents }: { agents: SubsystemHealth[] }) {
  if (!agents || agents.length === 0) return null;
  
  return (
    <div style={{ position: 'absolute', right: '8px', top: '50%', transform: 'translateY(-50%)', maxHeight: '300px', overflowY: 'auto', display: 'flex', flexDirection: 'column', gap: '8px', width: '200px' }}>
      {agents.map((agent, i) => (
        <div key={i} className="rpg-panel">
          <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center', marginBottom: '4px' }}>
            <span style={{ fontWeight: 'bold' }}>{agent.name}</span>
            <span className={`badge badge-${agent.status.toLowerCase() === 'ok' ? 'success' : agent.status.toLowerCase() === 'error' ? 'error' : 'idle'}`} style={{ fontSize: '6px', padding: '2px 4px' }}>
              {agent.status}
            </span>
          </div>
          {agent.message && (
            <div style={{ fontSize: '6px', color: '#8b949e', whiteSpace: 'nowrap', overflow: 'hidden', textOverflow: 'ellipsis' }}>
              {agent.message}
            </div>
          )}
        </div>
      ))}
    </div>
  );
}
