// Shared: Domain types for Downshift
// Mirrors the JSON from `downshift session --panel`

export interface TokenSaving {
  children: number;
  saved_children: number;
  work_tokens: number;
  work_saved: number;
  cache_tokens: number;
  cache_saved: number;
  work_pricier?: number;
  cache_pricier?: number;
  saved_inferred?: number;
  at_floor?: number;
  gateway_unpriced?: number;
}

export interface AgentType {
  agent_type: string;
  tokens: TokenSaving;
  line: string;
}

export type SpawnState = 'switched' | 'held' | 'paused' | 'control' | 'blocked';

export interface Spawn {
  tool_use: string;
  state: SpawnState;
  from_model: string;
  to_model: string;
  observed_model?: string;
  label: string;
  tokens: TokenSaving;
  tokens_label: string;
}

export interface Compactor {
  scope: string;
  commands: number;
  compacted: number;
  raw_bytes: number;
  saved_bytes: number;
  estimated_tokens_saved: number;
  token_estimate: string;
}

export interface CostSaved {
  actual_cost_usd: number;
  baseline_cost_usd: number;
  saved_usd: number;
  saved_percent: number;
  quality: 'exact' | 'mixed' | 'partial' | 'unpriced';
}

export interface PanelView {
  session: string;
  switched: number;
  model_switched: number;
  effort_changed: number;
  held: number;
  held_guardrail: number;
  upshift_not_applied: number;
  observed: number;
  tokens: TokenSaving;
  tokens_line: string;
  by_agent_type: AgentType[];
  spawns: Spawn[];
}

export interface PanelOut extends PanelView {
  compactor?: Compactor;
  tokens_saved_estimate: number;
  cost_saved?: CostSaved;
}

// Session listing types
export interface TokensDetail {
  routing: TokenSaving;
  compactor?: Compactor;
}

export interface SessionSummary {
  session_id: string;
  harness: string;
  first_seen: string;
  last_seen: string;
  spawn_count: number;
  switched: number;
  held: number;
  tokens_saved: number;
  tokens: TokensDetail;
}

export interface SessionsResponse {
  sessions: SessionSummary[];
  harnesses: string[];
}
