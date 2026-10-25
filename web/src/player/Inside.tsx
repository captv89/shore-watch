import { SealCheckIcon, TrayIcon, UserIcon } from '@phosphor-icons/react'
import { actorLabel, AGENT_TITLE, eventText, reasonText, type HubId, type LinkKind } from '../replay/derive'
import type { RunEvent } from '../replay/types'
import { usePlayer } from './context'
import { INSTRUMENTS, useHub, type HubNodeView } from './hub'
import { ToneIcon } from './ui'

// The hub on a 12-column grid: four parts on the top row (the way in and
// the way out), three on the rows below, and the instruments last. Links run
// between cell centres, in a 100 × 100 box stretched over the grid.
const CELL: Record<HubId, { col: number; span: number; row: number }> = {
  security: { col: 0, span: 3, row: 0 }, inbox: { col: 3, span: 3, row: 0 }, operator: { col: 6, span: 3, row: 0 }, gate: { col: 9, span: 3, row: 0 },
  reports: { col: 0, span: 4, row: 1 }, ops: { col: 4, span: 4, row: 1 }, correspondence: { col: 8, span: 4, row: 1 },
  voyage: { col: 0, span: 4, row: 2 }, weather: { col: 4, span: 4, row: 2 }, performance: { col: 8, span: 4, row: 2 },
}
const ROWS = 4
const OPTIONAL = new Set(['voyage>gate'])
const LINKS: [HubId, HubId][] = [
  ['security', 'inbox'], ['inbox', 'ops'], ['inbox', 'operator'], ['ops', 'operator'], ['correspondence', 'gate'], ['gate', 'operator'],
  ['ops', 'reports'], ['ops', 'voyage'], ['ops', 'weather'], ['ops', 'performance'], ['ops', 'correspondence'], ['voyage', 'gate'],
]
const LINK_TEXT: Record<LinkKind, string> = {
  arrives: 'passes the guardrail', holds: 'holds it for the Security Agent', cut: 'blocks it', alert: 'alerts you',
  asks: 'asks', reports: 'reports back', summary: 'sums up for you',
  question: 'asks you to decide', answer: 'answers', draft: 'files a draft', decides: 'decides',
}
const centre = (id: HubId): [number, number] => {
  const c = CELL[id]
  return [((c.col + c.span / 2) / 12) * 100, ((c.row + 0.5) / ROWS) * 100]
}

const KIND_TAG = { ai: 'AI', code: 'Code', human: 'Human', inbox: 'Code' } as const

export function HubNode({ n, compact }: { n: HubNodeView; compact?: boolean }) {
  const Icon = n.kind === 'human' ? UserIcon : n.id === 'inbox' ? TrayIcon : n.id === 'gate' ? SealCheckIcon : null
  return (
    <div className={`hub-node kind-${n.kind} ${n.id === 'ops' ? 'is-ops' : ''} ${n.current ? 'is-current' : n.done ? 'is-done' : ''}`} aria-current={n.current ? 'step' : undefined}>
      <div className="hub-node-head">
        {Icon ? <Icon size={13} weight={n.current ? 'fill' : 'regular'} aria-hidden="true" /> : <span className="hub-dot" aria-hidden="true" />}
        <span className="hub-name">{n.name}</span>
        {n.calls > 1 && <span className="hub-calls mono" title={`Asked ${n.calls} times so far`}>×{n.calls}</span>}
        <span className={`hub-kind kind-${n.kind}`}>{KIND_TAG[n.kind]}</span>
      </div>
      {!compact && <span className="hub-role">{n.role}</span>}
      {!compact && n.tools.length > 0 && (
        <ul className="hub-tools" aria-label="Tools used so far">
          {n.tools.map((t) => <li key={t.name} className={t.current ? 'is-current' : ''}>{t.label}</li>)}
        </ul>
      )}
      {n.line && (
        <span className={`hub-line ${n.line.tone ? `tone-${n.line.tone}` : ''}`}>
          {n.line.tone ? <ToneIcon tone={n.line.tone} size={10} /> : <span aria-hidden="true" className="hub-bullet" />}
          <span>{n.line.text}</span>
        </span>
      )}
    </div>
  )
}

/** The parts in reading order, for narrow screens where the hub's links can't be drawn. */
export const HUB_ORDER: HubId[] = ['inbox', 'security', 'ops', 'reports', 'voyage', 'weather', 'performance', 'correspondence', 'gate', 'operator']

export function Inside() {
  const { view, ev, events, seq, go } = usePlayer()
  const hub = useHub()
  const full = view === 'Inside'
  const link = hub.step?.link
  const lastReason = [...events].reverse().find((e) => e.seq <= seq && e.type === 'rationale')
  const handoff = [...events].reverse().find((e) => e.seq <= seq && isHandoff(e))
  const label = link && (() => {
    const [x1, y1] = centre(link.from)
    const [x2, y2] = centre(link.to)
    // Neighbours on one row sit close: put the label above them, not on them.
    return { left: `${(x1 + x2) / 2}%`, top: `${y1 === y2 ? Math.max(4, y1 - 50 / ROWS) : (y1 + y2) / 2}%` }
  })()

  return (
    <section className={`inside ${full ? 'is-full' : ''}`} aria-label="Inside: how the agents work together">
      <div className="inside-flow">
        <div className="inside-head">
          <h3>Inside: how the agents work together</h3>
          <p className="muted">The Ops Agent is the hub. Every request and every answer passes through it, and only it talks to you. The agents write; code checks and counts.</p>
        </div>
        <div className="hub" role="group" aria-label="Agents and the links between them">
          <svg className="hub-links" viewBox="0 0 100 100" preserveAspectRatio="none" aria-hidden="true">
            {LINKS.map(([a, b]) => {
              const on = link && ((link.from === a && link.to === b) || (link.from === b && link.to === a))
              const [from, to] = on && link.from === b ? [b, a] : [a, b]
              const [x1, y1] = centre(from)
              const [x2, y2] = centre(to)
              const used = hub.used.has(`${a}>${b}`) || hub.used.has(`${b}>${a}`)
              const cut = hub.cut.has(`${a}>${b}`) || hub.cut.has(`${b}>${a}`)
              // Only the Voyage Agent's entries take this link: drawn once used.
              if (OPTIONAL.has(`${a}>${b}`) && !used && !on) return null
              return <line key={`${a}-${b}`} x1={x1} y1={y1} x2={x2} y2={y2} className={`hub-link ${cut ? 'is-cut' : on ? 'is-current' : used ? 'is-used' : ''} ${on && cut ? 'is-current' : ''}`} vectorEffect="non-scaling-stroke" />
            })}
          </svg>
          {(Object.keys(CELL) as HubId[]).map((id) => (
            <div key={id} className="hub-cell" style={{ gridColumn: `${CELL[id].col + 1} / span ${CELL[id].span}`, gridRow: CELL[id].row + 1 }}>
              <HubNode n={hub.nodes[id]} compact={!full} />
            </div>
          ))}
          <div className="hub-cell hub-cell-instruments" style={{ gridColumn: '4 / span 6', gridRow: 4 }}>
            <div className="instruments" aria-label="Instruments: plain code the agents use">
              <span className="instruments-label">Instruments, plain code</span>
              <ul>
                {INSTRUMENTS.map((i) => {
                  const st = hub.instruments[i.id]
                  return (
                    <li key={i.id} className={st.current ? 'is-current' : st.used ? 'is-used' : ''} aria-current={st.current ? 'step' : undefined}>
                      <span className="instrument-name">{i.name}</span>
                      {full && <span className="instrument-role">{i.role}</span>}
                    </li>
                  )
                })}
              </ul>
            </div>
          </div>
          {link && label && (
            <span key={hub.step?.seq} className={`hub-link-label link-${link.kind}`} style={label}>
              {link.from === 'operator' ? 'You' : AGENT_TITLE[`${link.from}_agent`] ?? hubName(link.from)} {LINK_TEXT[link.kind]}
            </span>
          )}
        </div>
      </div>
      <div className="inside-side">
        {handoff && <Handoff e={handoff} current={ev.seq === handoff.seq} />}
        {lastReason && <Reason e={lastReason} current={ev.seq === lastReason.seq} />}
        {full && (
          <div className="log">
            <h4 className="pane-sub">Event log so far</h4>
            <ol>
              {events.filter((e) => e.seq <= seq).reverse().map((e) => (
                <li key={e.seq}>
                  <button type="button" className={e.seq === seq ? 'is-current' : ''} onClick={() => go(e.seq)}>
                    <span className="mono faint">{e.seq}</span>
                    <span className="log-actor">{e.type === 'rationale' ? 'Stated reason' : actorLabel(e)}</span>
                    <span className="log-text">{eventText(e)}</span>
                  </button>
                </li>
              ))}
            </ol>
          </div>
        )}
      </div>
    </section>
  )
}

const hubName = (id: HubId) => ({ inbox: 'The Guardrail', gate: 'The gate', operator: 'You' } as Record<string, string>)[id] ?? id

/** Messages between the Ops Agent and the others: requests, reports, questions and answers. */
function isHandoff(e: RunEvent): boolean {
  return (e.type === 'plan' && 'delegate_to' in e.detail) || (e.type === 'tool_result' && e.actor === 'subagent') || e.type === 'question' || e.type === 'answer'
}

function Handoff({ e, current }: { e: RunEvent; current: boolean }) {
  const d = e.detail
  const title = (a?: string) => (a ? AGENT_TITLE[a] ?? a : 'Agent')
  let from = title(e.agent)
  let to = 'Ops Agent'
  let text = ''
  if (e.type === 'plan') {
    to = title(String(d.delegate_to))
    text = String((d.args as Record<string, unknown> | undefined)?.request ?? e.summary)
  } else if (e.type === 'question') {
    to = 'You'
    text = String((d.question as Record<string, unknown> | undefined)?.question ?? e.summary)
  } else if (e.type === 'answer') {
    from = 'You (simulated)'
    text = String(d.text ?? e.summary)
  } else {
    text = String(d.text ?? '') || e.summary.replace(/^.*?reports:\s*|^.*?hands /, '')
  }
  return (
    <figure className={`handoff ${current ? 'is-current' : ''}`}>
      <figcaption>
        <span>{from} → {to}</span>
        <span className="muted">{current ? `Event ${e.seq}` : `Latest, event ${e.seq}`}</span>
      </figcaption>
      <blockquote>{text}</blockquote>
    </figure>
  )
}

export function Reason({ e, current }: { e: RunEvent; current: boolean }) {
  return (
    <figure className={`reason ${current ? 'is-current' : ''}`}>
      <figcaption>
        <span>{e.agent ? `${AGENT_TITLE[e.agent]}'s stated reason` : "Agent's stated reason"}</span>
        <span className="muted">{current ? `Event ${e.seq}` : `Latest, event ${e.seq}`}</span>
      </figcaption>
      <blockquote>“{reasonText(e)}”</blockquote>
      <p className="muted small">Written by the agent as it worked. It is what the agent said, not a reading of its mind.</p>
    </figure>
  )
}
