// Event log types: the contract with agent/eventlog. The page depends only on these.

export type Actor =
  | 'agent'
  | 'subagent'
  | 'tool'
  | 'checker'
  | 'guardrail'
  | 'human_sim'
  | 'external_sim'
  | 'sim'

export type EventType =
  | 'message_received'
  | 'extracted'
  | 'plan'
  | 'tool_call'
  | 'tool_result'
  | 'check'
  | 'rationale'
  | 'draft'
  | 'approval_requested'
  | 'approved'
  | 'edited'
  | 'rejected'
  | 'blocked'
  | 'alert'
  | 'error'
  | 'degraded'
  | 'recovered'
  | 'question'
  | 'answer'
  | 'entry'
  | 'forecast_available'

export interface RunEvent {
  schema_version: string
  run_id: string
  attempt: number
  seq: number
  sim_time: string
  wall_ms: number
  scene: string
  ship: string
  actor: Actor
  /** Since schema 1.2: which agent an agent or subagent event belongs to. */
  agent?: string
  type: EventType
  summary: string
  detail: Record<string, unknown>
  source_refs: string[]
  cost: { input_tokens: number; output_tokens: number; usd: number }
  model: string | null
}

export interface RunHeader {
  schema_version: string
  run_id: string
  scene: string
  model: string
  recorded_at: string
  prompt_version: string
  tool_versions: Record<string, string>
  seed: number
  attempt: number
  total_attempts: number
  notes: string
}

export interface Run {
  header: RunHeader
  events: RunEvent[]
}
