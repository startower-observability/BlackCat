import type { SystemStatus } from '../../types/agent';

export default function SystemInfo({ status }: { status: SystemStatus }) {
  return (
    <div className="rpg-panel" style={{ position: 'absolute', top: 8, right: 8, width: '200px' }}>
      <h3 style={{ margin: '0 0 8px 0', fontSize: '8px', color: '#8b949e', borderBottom: '2px solid #30363d', paddingBottom: '4px' }}>SYSTEM STATUS</h3>
      <div style={{ display: 'flex', flexDirection: 'column', gap: '6px' }}>
        <div style={{ display: 'flex', justifyContent: 'space-between' }}>
          <span style={{ color: '#484f58' }}>VERSION</span>
          <span>{status.version || 'v1.0.0'}</span>
        </div>
        <div style={{ display: 'flex', justifyContent: 'space-between' }}>
          <span style={{ color: '#484f58' }}>UPTIME</span>
          <span>{status.uptime || '0h 0m'}</span>
        </div>
        <div style={{ display: 'flex', justifyContent: 'space-between' }}>
          <span style={{ color: '#484f58' }}>SUBSYSTEMS</span>
          <span>{status.subsystem_count !== undefined ? status.subsystem_count : 0}</span>
        </div>
        <div style={{ display: 'flex', justifyContent: 'space-between' }}>
          <span style={{ color: '#484f58' }}>CORE</span>
          <span style={{ color: status.healthy !== false ? '#238636' : '#da3633' }}>
            {status.healthy !== false ? 'ONLINE' : 'ERROR'}
          </span>
        </div>
      </div>
    </div>
  );
}
