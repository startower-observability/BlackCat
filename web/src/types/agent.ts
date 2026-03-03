export type AgentState = 'working' | 'idle' | 'error' | 'thinking' | 'success';

export interface SystemStatus {
  version?: string;
  uptime?: string;
  healthy?: boolean;
  subsystem_count?: number;
}

export interface SubsystemHealth {
  name: string;
  status: string;
  message?: string;
}

export interface CatStateResponse {
  state: AgentState;
  description: string;
  since: string;
}
