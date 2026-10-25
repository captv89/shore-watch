import { useMemo } from 'react'
import { hubSteps, TOOL_INFO, type HubId, type HubStep, type InstrumentId, type Tone } from '../replay/derive'
import { usePlayer } from './context'

export type HubKind = 'ai' | 'code' | 'human' | 'inbox'

export interface HubNodeView {
  id: HubId
  name: string
  role: string
  kind: HubKind
  current: boolean
  done: boolean
  /** How many times the Ops Agent has handed this agent work so far. */
  calls: number
  /** Tools this part has used so far in the run, the one in use now marked. */
  tools: { name: string; label: string; current: boolean }[]
  line?: { text: string; tone?: Tone; seq: number }
}

// Page copy: what each part is for.
export const HUB_META: Record<HubId, { name: string; role: string; kind: HubKind }> = {
  security: { name: 'Security Agent', role: 'Reads a held e-mail on its own. Cannot act: no tool that sends, pays or changes anything.', kind: 'ai' },
  inbox: { name: 'Inbox', role: 'E-mails arrive here. The guardrail checks the sender before any agent sees them.', kind: 'inbox' },
  operator: { name: 'You, the operator', role: 'Decides. Simulated in this replay.', kind: 'human' },
  gate: { name: 'Approval gate', role: 'Nothing leaves without the operator.', kind: 'code' },
  ops: { name: 'Ops Agent', role: 'The hub: hands each part of the work to a colleague, and is the only agent that talks to you.', kind: 'ai' },
  reports: { name: 'Reports Agent', role: 'Turns the noon report into checked fields.', kind: 'ai' },
  voyage: { name: 'Voyage Agent', role: 'Position, ETA, charter party notices, the port call and off-hire.', kind: 'ai' },
  weather: { name: 'Weather Agent', role: 'Reads each new forecast for every ship at sea. Woken by the forecast, not by an e-mail.', kind: 'ai' },
  performance: { name: 'Performance Agent', role: 'Speed and fuel against the warranty, with the weather as evidence.', kind: 'ai' },
  correspondence: { name: 'Correspondence Agent', role: 'Writes what others ask to send: drafts and alerts.', kind: 'ai' },
}

export const INSTRUMENTS: { id: InstrumentId; name: string; role: string }[] = [
  { id: 'guardrail', name: 'Guardrail', role: 'Senders, recipients, your decisions' },
  { id: 'checker', name: 'Checker', role: 'Rules in code' },
  { id: 'numguard', name: 'Number guard', role: 'Every number traced' },
]

export interface HubView {
  nodes: Record<HubId, HubNodeView>
  step?: HubStep
  /** Links travelled so far, as "from>to". */
  used: Set<string>
  /** Links the guardrail has cut so far. */
  cut: Set<string>
  instruments: Record<InstrumentId, { current: boolean; used: boolean }>
}

/** State of the hub at the current step. */
export function useHub(): HubView {
  const { events, seq } = usePlayer()
  const all = useMemo(() => hubSteps(events), [events])
  return useMemo(() => {
    const past = all.filter((s) => s.seq <= seq)
    const step = past.at(-1)
    const used = new Set<string>()
    const cut = new Set<string>()
    for (const s of past) if (s.link) (s.link.kind === 'cut' ? cut : used).add(`${s.link.from}>${s.link.to}`)
    const nodes = {} as Record<HubId, HubNodeView>
    for (const id of Object.keys(HUB_META) as HubId[]) {
      const mine = past.filter((s) => s.node === id)
      const last = mine.at(-1)
      const tools: HubNodeView['tools'] = []
      for (const s of mine) {
        if (s.tool && !tools.some((t) => t.name === s.tool)) tools.push({ name: s.tool, label: TOOL_INFO[s.tool]?.name ?? s.tool, current: false })
      }
      for (const t of tools) t.current = step?.node === id && step.tool === t.name
      nodes[id] = {
        id, ...HUB_META[id], current: step?.node === id, done: mine.length > 0, tools,
        calls: past.filter((s) => s.link?.kind === 'asks' && s.link.to === id).length,
        line: last && { text: last.line, tone: last.tone, seq: last.seq },
      }
    }
    const instruments = {} as HubView['instruments']
    for (const { id } of INSTRUMENTS) instruments[id] = { current: step?.instrument === id, used: past.some((s) => s.instrument === id) }
    return { nodes, step, used, cut, instruments }
  }, [all, seq])
}
