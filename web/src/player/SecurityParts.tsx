import { PhoneIcon, ProhibitIcon, ShieldWarningIcon } from '@phosphor-icons/react'
import type { ReactNode } from 'react'
import type { DeskState, Reason } from '../replay/derive'
import { usePlayer } from './context'
import { Status } from './ui'

// Desk pieces for a message the guardrail holds (Scene 3). Each shows only
// what the log holds at the current step.

const KIND_LABEL: Record<string, string> = {
  lookalike_sender: 'Sender imitates a known contact',
  unknown_sender: 'Sender not in the verified contacts',
  bank_details_change: 'Asks to change bank details',
  instruction_to_ai: 'Instruction addressed to an AI',
  pressure: 'Pressure to skip checks',
  other: 'Other concern',
}

/** The sender's domain with the character that differs from the real one marked. */
function Domain({ domain, at }: { domain: string; at?: number }) {
  if (at === undefined) return <span className="mono">{domain}</span>
  return (
    <span className="mono">
      {domain.slice(0, at)}<mark className="diff-char">{domain[at]}</mark>{domain.slice(at + 1)}
    </span>
  )
}

/** Quotes the Security Agent reported, numbered as on the alert card. */
function quotes(desk: DeskState): { quote: string; n: number }[] {
  const out: { quote: string; n: number }[] = []
  let n = desk.screen?.lookalike ? 1 : 0
  for (const f of desk.findings?.items ?? []) {
    if (f.kind === 'lookalike_sender' && desk.screen?.lookalike) continue
    n++
    out.push({ quote: f.quote, n })
  }
  return out
}

/** Body text with each reported quote highlighted and numbered. */
function Marked({ text, marks }: { text: string; marks: { quote: string; n: number }[] }) {
  const parts: ReactNode[] = []
  let rest = text
  let key = 0
  for (;;) {
    let best: { i: number; m: { quote: string; n: number } } | null = null
    for (const m of marks) {
      const i = rest.toLowerCase().indexOf(m.quote.toLowerCase())
      if (i >= 0 && (!best || i < best.i)) best = { i, m }
    }
    if (!best) break
    parts.push(rest.slice(0, best.i))
    parts.push(<mark key={key++} className="quote-mark"><span className="quote-n" aria-label={`Reason ${best.m.n}`}>{best.m.n}</span>{rest.slice(best.i, best.i + best.m.quote.length)}</mark>)
    rest = rest.slice(best.i + best.m.quote.length)
  }
  parts.push(rest)
  return <>{parts}</>
}

/** The held e-mail, as the Security Agent read it, with what it found marked. */
export function MessageReview() {
  const { desk } = usePlayer()
  const e = desk.email
  const s = desk.screen
  if (!e) return null
  const marks = quotes(desk)
  return (
    <section className={`review ${desk.blocked ? 'is-blocked' : ''}`} aria-label="Held e-mail under review">
      <header className="review-head">
        <h3>{s ? 'Held by the guardrail' : 'Incoming e-mail'}</h3>
        {desk.blocked ? <Status tone="fail">Blocked. No agent that can act has seen it</Status>
          : s ? <Status tone="wait">Only the Security Agent may read it</Status>
            : <Status tone="wait">Arrived. Checking the sender</Status>}
      </header>
      <dl className="review-meta">
        <dt>From</dt>
        <dd>
          {e.from.replace(/<.*>/, '').trim()}{' '}
          {s ? <>&lt;{e.from.match(/<(.*)@/)?.[1] ?? ''}@<Domain domain={s.domain} at={s.diffAt} />&gt;</> : <span className="mono">{e.from.match(/<.*>/)?.[0]}</span>}
        </dd>
        {s?.lookalike && (
          <>
            <dt>On file</dt>
            <dd><span className="mono">{s.lookalike.domain}</span> <span className="muted">{s.lookalike.name}</span>{s.diffAt !== undefined && <span className="review-flag"><span className="quote-n">1</span>one character differs</span>}</dd>
          </>
        )}
        <dt>Subject</dt>
        <dd>{e.subject}</dd>
      </dl>
      <pre className="review-body"><Marked text={e.body.trim()} marks={marks} /></pre>
      {desk.findings && desk.findings.refused > 0 && (
        <p className="review-note"><Status tone="fail">{desk.findings.refused === 1 ? 'One report' : `${desk.findings.refused} reports`} refused: a quote was not in the e-mail</Status></p>
      )}
    </section>
  )
}

function Reasons({ list }: { list: Reason[] }) {
  return (
    <ol className="fraud-reasons">
      {list.map((r, i) => (
        <li key={i}>
          <span className="quote-n">{i + 1}</span>
          <span>
            <strong>{KIND_LABEL[r.kind] ?? r.kind}.</strong> {r.text}
            <span className="fraud-by">{r.by === 'guardrail' ? 'Guardrail, code' : 'Security Agent, AI; quote checked by code'}</span>
          </span>
        </li>
      ))}
    </ol>
  )
}

/** The alert the guardrail raises on a held message. */
export function FraudCard() {
  const { desk } = usePlayer()
  const f = desk.fraud
  if (!f) {
    if (!desk.screen) return null
    return (
      <article className="desk-card fraud-card is-pending" aria-label="Held message">
        <div className="desk-card-head"><ShieldWarningIcon size={12} aria-hidden="true" /><span>Held · Guardrail</span></div>
        <p>The sender is not a known contact. The message is kept from every agent that can act while the Security Agent reads it.</p>
      </article>
    )
  }
  return (
    <article className="desk-card fraud-card" aria-label="Alert: possible payment fraud">
      <div className="desk-card-head tone-fail"><ProhibitIcon size={12} weight="bold" aria-hidden="true" /><span>Alert · possible payment fraud</span></div>
      <Reasons list={f.reasons} />
      <div className="fraud-action">
        <PhoneIcon size={14} aria-hidden="true" />
        <span>{f.action}</span>
      </div>
      <span className="desk-card-foot">Raised by code, not by an agent. Nothing was paid, changed or sent.</span>
    </article>
  )
}
