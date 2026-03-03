import { useAgents } from '../hooks/useAgents';

export default function AgentsPage() {
  const agents = useAgents();
  return (
    <div style={{ padding: '16px', color: '#c9d1d9' }}>
      <h2 style={{ fontSize: '10px', marginBottom: '16px', borderBottom: '2px solid #30363d', paddingBottom: '8px' }}>AGENTS</h2>
      {agents.length === 0 ? (
        <p style={{ color: '#484f58' }}>LOADING...</p>
      ) : (
        <div style={{ display: 'grid', gridTemplateColumns: 'repeat(auto-fill, minmax(200px, 1fr))', gap: '8px' }}>
          {agents.map((agent, i) => (
            <div key={i} className="rpg-panel">
              <div style={{ fontWeight: 'bold', marginBottom: '4px' }}>{agent.name}</div>
              <span className={`badge badge-${agent.status.toLowerCase().includes('ok') || agent.status.toLowerCase().includes('healthy') ? 'success' : agent.status.toLowerCase().includes('error') || agent.status.toLowerCase().includes('fail') ? 'error' : 'idle'}`}>
                {agent.status}
              </span>
              {agent.message && <div style={{ marginTop: '4px', fontSize: '6px', color: '#8b949e' }}>{agent.message}</div>}
            </div>
          ))}
        </div>
      )}
    </div>
  );
}
