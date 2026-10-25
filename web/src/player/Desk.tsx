import { ArrowRightIcon, CheckIcon, DatabaseIcon, EnvelopeSimpleIcon, PencilSimpleIcon, XIcon } from '@phosphor-icons/react'
import { useEffect, useRef, type ReactNode } from 'react'
import { draftNumbers, draftTimes, fmtDayTime, fmtEta, fmtSimTime, isoToDraftTime, passageName, type QueueItem } from '../replay/derive'
import { Chart } from './Chart'
import { usePlayer } from './context'
import { AlertCard, Compare, QuestionCard, dayLabel, EditDiff, ForecastCallout, FuelFlag, NoteCard, QueueHistory, WeatherDayBadge, WeatherStrip } from './DeskParts'
import { FraudCard, MessageReview } from './SecurityParts'
import { Num, Status, ToneIcon } from './ui'
import { FleetCard, FleetStrip, OffHireCard, QueueStatus, Remark, ReportDayStrip, SpeedPlanStrip, TriggerItem, VmsCard } from './VoyageParts'

export function Desk() {
  const { view, final } = usePlayer()
  // A message the guardrail holds never reaches a voyage: the review panel
  // takes the chart's place (Scene 3).
  const held = !!final.screen
  // A forecast run, not an e-mail, starts the fleet scene (Scene 4).
  const fleet = !!final.trigger
  return (
    <div className={`desk ${view === 'Desk' ? 'desk-tall' : ''}`}>
      <Inbox />
      <div className="desk-centre">
        {held ? <MessageReview /> : (
          <>
            <div className="desk-map"><Chart /></div>
            <WeatherStrip />
            <FleetStrip />
            <SpeedPlanStrip />
            <ReportDayStrip />
          </>
        )}
      </div>
      <aside className="desk-side" aria-label={held ? 'Alerts and approvals' : 'Ship and approvals'}>
        {held ? <FraudCard /> : fleet ? <FleetCard /> : <ShipCard />}
        <OffHireCard />
        <QuestionCard />
        <AlertCard />
        {!fleet && <NoteCard />}
        <VmsCard />
        <ApprovalQueue />
      </aside>
    </div>
  )
}

function Inbox() {
  const { desk } = usePlayer()
  const e = desk.email
  const earlier = desk.inbox.slice(0, -1).reverse()
  const drafts = desk.queue.filter((q) => q.kind === 'email' && q.day === desk.day)
  return (
    <aside className="inbox" aria-label="Operations inbox">
      <div className="pane-head">
        <h3>Operations inbox</h3>
        <span className="mono faint">ops@shorewatch-demo.example</span>
      </div>
      {e && (
        <div className="mail is-open" key={e.id}>
          <div className="mail-row">
            <span className="mail-from">{e.from}</span>
            <span className="mono muted">{fmtEta(e.time).split(' ')[2]}</span>
          </div>
          <div className="mail-subject">{e.subject}</div>
          <div className="chips-row">
            {earlier.length > 0 && desk.senderKnown === undefined && <span className="chip-new">New · {daysBetween(earlier[0].time, e.time)} later</span>}
            {desk.senderKnown !== undefined && <Status tone={desk.senderKnown ? 'ok' : desk.screen?.lookalike ? 'fail' : 'warn'}>{desk.senderKnown ? 'Known sender' : desk.screen?.lookalike ? 'Lookalike sender' : 'Unknown sender'}</Status>}
            {desk.blocked && <Status tone="fail">Blocked</Status>}
            {desk.report && <Status tone="ok">Read by agent</Status>}
          </div>
        </div>
      )}
      {!e && <TriggerItem />}
      {earlier.map((m) => (
        <div className="mail is-earlier" key={m.id}>
          <div className="mail-row">
            <span className="mail-from">{m.from}</span>
            <span className="mono muted">{fmtEta(m.time).replace(' UTC', '')}</span>
          </div>
          <div className="mail-subject">{m.subject}</div>
        </div>
      ))}
      {drafts.length > 0 && (
        <>
          <h4 className="pane-sub">Drafts</h4>
          {drafts.map((q) => (
            <div className="mail" key={q.ref}>
              <div className="mail-from">To {q.to.length} verified contact{q.to.length === 1 ? '' : 's'}</div>
              <div className="mail-subject">{q.subject}</div>
              <DraftStatusLine q={q} />
            </div>
          ))}
        </>
      )}
      <div className="mail-body">
        {e && desk.screen ? (
          <p className="empty">Held by the guardrail. The e-mail is shown in the review panel, as the Security Agent read it.</p>
        ) : e ? (
          <>
            <h4 className="pane-sub">Open: {e.id}</h4>
            <pre>{e.body.trim()}</pre>
          </>
        ) : desk.trigger ? null : (
          <p className="empty">Waiting for the next report.</p>
        )}
      </div>
    </aside>
  )
}

function daysBetween(a: string, b: string) {
  const d = Math.round((new Date(b).getTime() - new Date(a).getTime()) / 86400000)
  return d === 1 ? '1 day' : `${d} days`
}

function DraftStatusLine({ q }: { q: QueueItem }) {
  const { desk, userGate } = usePlayer()
  if (q.status === 'approved' || q.status === 'edited') return <Status tone="ok">Approved, not sent (demo)</Status>
  if (q.status === 'rejected') return <Status tone="fail">Rejected, nothing sent</Status>
  if (q.ref === desk.focus && (userGate === 'approved' || userGate === 'edited')) return <Status tone="ok">You approved it (not recorded)</Status>
  if (q.ref === desk.focus && userGate === 'rejected') return <Status tone="fail">You rejected it (not recorded)</Status>
  return q.status === 'waiting' ? <Status tone="wait">Awaiting approval</Status> : <Status tone="warn">Draft written</Status>
}

function Row({ label, children }: { label: string; children: ReactNode }) {
  return (
    <>
      <dt>{label}</dt>
      <dd>{children}</dd>
    </>
  )
}

function ShipCard() {
  const { desk, ev, final } = usePlayer()
  const r = desk.report
  const v = desk.voyage
  const multiDay = final.dayStarts.length > 1
  // An e-mail from ashore (Scene 5): no noon report, so the position is the
  // AIS feed's and the ETA the voyage management system's.
  const noReport = !final.report
  const pc = desk.vms?.portCalls[0]
  return (
    <section className="ship" aria-label="Ship card">
      <div className="ship-head">
        <h3>{r?.shipName || v?.ship || final.report?.shipName || 'Ship'}</h3>
        <span className="mono muted">{ev.ship}</span>
      </div>
      <p className="ship-voyage">{v ? `${v.cargo} · ${v.from} to ${v.to}` : 'Voyage details come from the voyage file.'}</p>
      {multiDay && desk.email && <span className="ship-day mono">{dayLabel(desk.report?.time ?? desk.email.time)}</span>}
      <dl className="facts">
        {noReport ? (
          <Row label="Position, AIS">{desk.ais ? <><Num k="aisPos" /> <span className="muted">{fmtDayTime(desk.ais.time).split(' ').slice(0, 2).join(' ')}</span></> : <Pending text="not yet asked" />}</Row>
        ) : (
          <>
            <Row label="Position">{r ? <Num k="pos" /> : <Pending text="waiting for the Reports Agent" />}</Row>
            <Row label="Speed, 24 h avg">{r ? <Num k="speed" /> : <Pending />}</Row>
            <Row label="Distance run, 24 h">{r ? <Num k="run" /> : <Pending />}</Row>
          </>
        )}
        {final.report?.consVlsfoT !== undefined && <Row label="VLSFO used, 24 h">{r?.consVlsfoT !== undefined ? <Num k="fuelReported" /> : <Pending />}</Row>}
        <Row label="Distance to go">
          {desk.route ? (
            <>
              <Num k="dist" />
              {desk.route.passages.length > 0 && <span className="muted"> via {desk.route.passages.map(passageName).join(', ')}</span>}
            </>
          ) : (
            <Pending text="not yet asked" />
          )}
        </Row>
        {!noReport && <Row label="ROB">
          {r ? (
            <>
              <Num k="vlsfo" /> <span className="muted">VLSFO,</span> <Num k="mgo" /> <span className="muted">MGO</span>
            </>
          ) : (
            <Pending />
          )}
        </Row>}
      </dl>
      <Remark />
      <div className="eta-block">
        {noReport ? (
          <div className="eta-row">
            <span className="muted">Voyage system ETA</span>
            {pc ? <Num k="vmsEta" className="num-lg" /> : <Pending />}
          </div>
        ) : (
          <div className="eta-row">
            <span className="muted">Master's ETA {r?.nextPort ?? ''}</span>
            {r ? <Num k="eta" className="num-lg" /> : <Pending />}
          </div>
        )}
        {desk.eta && (
          <Callout tone={desk.eta.consistent ? 'ok' : 'warn'} title={desk.eta.consistent ? 'Consistent.' : 'Does not fit.'}>
            Distance ÷ speed gives <Num k="compEta" />, <Num k="diff" /> from the Master (limit ±{desk.eta.toleranceH} h).
          </Callout>
        )}
        {desk.notices && (
          desk.notices.due ? (
            <Callout tone="warn" title="Notice due.">
              {desk.notices.thresholdDays ? <>{desk.notices.thresholdDays}-day ETA notice (<Num k="days" /> to go)</> : 'ETA notice'}
              {desk.notices.movedH > desk.notices.limitH && <>, and the ETA moved <Num k="moved" /> since the last notice (CP limit <Num k="limit" />)</>}.
            </Callout>
          ) : (
            <Callout tone="ok" title="No notice due.">Next notice point is after the next noon report.</Callout>
          )
        )}
        <ForecastCallout />
      </div>
      {(desk.consumption || desk.weatherDay) && (
        <div className="perf-block">
          <FuelFlag />
          <WeatherDayBadge />
          <Compare />
        </div>
      )}
    </section>
  )
}

function Pending({ text = 'not yet known' }: { text?: string }) {
  return <span className="num-pending">{text}</span>
}

function Callout({ tone, title, children }: { tone: 'ok' | 'warn'; title: string; children: ReactNode }) {
  return (
    <div className={`callout tone-${tone}`}>
      <span className="callout-icon" aria-hidden="true"><ToneIcon tone={tone} size={11} /></span>
      <span><strong>{title}</strong> {children}</span>
    </div>
  )
}

function ApprovalQueue() {
  const { desk, go } = usePlayer()
  const mine = desk.queue.filter((q) => q.day === desk.day)
  const focus = mine.find((q) => q.ref === desk.focus) ?? mine.at(-1)
  const waiting = mine.filter((q) => q.status === 'waiting').length
  return (
    <section className="queue" aria-label="Approval queue">
      <div className="queue-head">
        <h3>Approval queue</h3>
        <span className="mono muted">{waiting} waiting</span>
      </div>
      <QueueHistory />
      {mine.length > 1 && (
        <ol className="queue-list" aria-label="Items in the queue for this message">
          {mine.map((q) => (
            <li key={q.ref}>
              <button type="button" className={q.ref === focus?.ref ? 'is-on' : ''} onClick={() => go(q.seq)} aria-current={q.ref === focus?.ref ? 'true' : undefined}>
                {q.kind === 'entry' ? <DatabaseIcon size={12} aria-hidden="true" /> : <EnvelopeSimpleIcon size={12} aria-hidden="true" />}
                <span className="queue-subject">{q.subject}</span>
                <QueueStatus q={q} />
              </button>
            </li>
          ))}
        </ol>
      )}
      {focus?.draft ? <DraftCard item={focus} /> : focus?.entry ? <EntryCard item={focus} /> : <p className="empty boxed">{desk.screen ? 'Nothing drafted. A held message never reaches the agents that write.' : 'Nothing waiting. Drafts land here before anything can leave the desk.'}</p>}
    </section>
  )
}

/** An entry for the voyage management system, waiting at the gate like a draft. It is never posted. */
function EntryCard({ item }: { item: QueueItem }) {
  const { desk, userGate, setUserGate } = usePlayer()
  const e = item.entry!
  const recorded = item.status === 'approved' || item.status === 'edited' || item.status === 'rejected'
  const atGate = item.status === 'waiting'
  const mine = item.ref === desk.focus
  const pc = desk.vms?.portCalls.find((p) => p.port === e.port)
  let badge: { tone: 'ok' | 'warn' | 'wait' | 'fail'; text: string }
  if (recorded) badge = item.status === 'rejected' ? { tone: 'fail', text: 'Rejected by the simulated operator' } : { tone: 'ok', text: 'Approved, not posted (demo). Simulated operator.' }
  else if (mine && userGate === 'approved') badge = { tone: 'ok', text: 'You approved it. Not posted (demo).' }
  else if (mine && userGate === 'rejected') badge = { tone: 'fail', text: 'You rejected it. Nothing is posted.' }
  else badge = atGate ? { tone: 'wait', text: 'Waiting at the approval gate' } : { tone: 'warn', text: 'Entry proposed, checks done' }
  return (
    <article className={`draft entry ${atGate && !userGate ? 'is-waiting' : ''}`} aria-label={`Entry: ${item.subject}`}>
      <div className="draft-badge"><Status tone={badge.tone}>{badge.text}</Status></div>
      <div className="draft-main">
        <div className="draft-meta">
          <span className="faint">Into</span>
          <span className="small">Voyage management system <span className="muted">(stand-in)</span></span>
          <span className="faint">Entry</span>
          <span className="draft-subject">{e.kind === 'port_call_eta' ? `Port call ETA, ${e.port}` : e.kind === 'off_hire' ? 'Off-hire period' : e.kind}</span>
        </div>
        <div className="entry-change">
          {e.kind === 'port_call_eta' && e.eta && (
            <>
              <span className="muted">ETA</span>
              <span className="mono entry-eta">{pc && <><s className="muted">{fmtDayTime(pc.eta)}</s><ArrowRightIcon size={10} aria-label="becomes" /></>}{fmtDayTime(e.eta)} UTC</span>
            </>
          )}
          {e.kind === 'off_hire' && e.from && e.to && (
            <>
              <span className="muted">From</span><span className="mono">{fmtEta(e.from)}</span>
              <span className="muted">To</span><span className="mono">{fmtEta(e.to)}</span>
            </>
          )}
        </div>
        <p className="entry-reason">{e.reason}</p>
        <div className="draft-checks">
          {desk.unsupported && <Status tone={desk.unsupported.length ? 'fail' : 'ok'}>{desk.unsupported.length ? `Numbers with no source: ${desk.unsupported.join(', ')}` : 'Every number and time traces to the e-mail or a tool'}</Status>}
        </div>
      </div>
      {atGate && mine && !userGate && (
        <div className="draft-actions">
          <button type="button" className="btn btn-primary" onClick={() => setUserGate('approved')}><CheckIcon size={14} weight="bold" aria-hidden="true" />Approve</button>
          <button type="button" className="btn" onClick={() => setUserGate('rejected')}><XIcon size={14} aria-hidden="true" />Reject</button>
        </div>
      )}
      <p className="draft-note">{recorded ? `Simulated operator action at ${desk.decidedAt ? fmtSimTime(desk.decidedAt) : 'the end of the run'}. Filed; nothing is posted to any system.` : atGate ? 'Try it: your choice stays in this replay and is not recorded.' : 'Checks passed. Next: the approval gate.'}</p>
    </article>
  )
}

/** Text of the draft, with each number linked to the source that produced it. */
function LinkedBody({ body }: { body: string }) {
  const { desk } = usePlayer()
  const byValue = draftNumbers(desk)
  const byTime = draftTimes(desk)
  const eta = desk.report ? isoToDraftTime(desk.report.masterEta) : null
  const re = /\d{4}-\d{2}-\d{2} \d{4} UTC|\d{1,2} Jan(?: 2025)? \d{2}:?\d{2}(?: UTC)?|(?:US)?\$[\d,]+(?:\.\d+)?|\d+(?:\.\d+)?(?=-day|-hour| ?h\b| hours?| days?| nm| kn| knots| ?m?t\b| tonnes| m\b| ?%)|(?<=Beaufort )\d+/g
  const timeKey = (tok: string) => {
    const m = /^(\d{1,2}) Jan(?: 2025)? (\d{2}):?(\d{2})/.exec(tok)
    if (!m) return undefined
    const iso = new Date(Date.UTC(2025, 0, Number(m[1]), Number(m[2]), Number(m[3]))).getTime()
    return byTime.find(([, t]) => new Date(t).getTime() === iso)?.[0]
  }
  const linked = (text: string): ReactNode[] => {
    const out: ReactNode[] = []
    let last = 0
    for (const m of text.matchAll(re)) {
      const tok = m[0]
      const k = tok === eta ? 'eta' : /Jan/.test(tok) ? timeKey(tok) : byValue.find(([, v]) => v === Number(tok.replace(/^(?:US)?\$/, '').replace(/,/g, '')))?.[0]
      if (!k) continue
      out.push(text.slice(last, m.index), <Num key={m.index} k={k}>{tok}</Num>)
      last = m.index + tok.length
    }
    out.push(text.slice(last))
    return out
  }
  return (
    <div className="draft-body">
      {body.trim().split(/\n\s*\n/).map((p, i) => <p key={i}>{linked(p)}</p>)}
    </div>
  )
}

function DraftCard({ item }: { item: QueueItem }) {
  const { desk, userGate: anyGate, setUserGate, editText, setEditText } = usePlayer()
  const d = item.draft!
  // The reader's own choice applies to the item at the gate in focus only.
  const userGate = item.ref === desk.focus ? anyGate : null
  const status = item.status
  const recorded = status === 'approved' || status === 'edited' || status === 'rejected'
  const atGate = status === 'waiting'
  const area = useRef<HTMLTextAreaElement>(null)
  useEffect(() => {
    if (userGate === 'editing') area.current?.focus()
  }, [userGate])

  let badge: { tone: 'ok' | 'warn' | 'wait' | 'fail'; text: string }
  if (recorded) badge = status === 'rejected' ? { tone: 'fail', text: 'Rejected by the simulated operator' } : { tone: 'ok', text: item.filed ? `${status === 'edited' ? 'Edited and approved' : 'Approved'}, not sent (demo). Simulated operator.` : status === 'edited' ? 'Edited by the simulated operator' : 'Approved by the simulated operator' }
  else if (userGate === 'approved' || userGate === 'edited') badge = { tone: 'ok', text: userGate === 'edited' ? 'You approved an edited copy. Not sent (demo).' : 'You approved it. Not sent (demo).' }
  else if (userGate === 'rejected') badge = { tone: 'fail', text: 'You rejected it. Nothing is sent.' }
  else badge = atGate ? { tone: 'wait', text: 'Waiting at the approval gate' } : { tone: 'warn', text: 'Draft written, checks done' }

  const note = recorded
    ? `Simulated operator action at ${desk.decidedAt ? fmtSimTime(desk.decidedAt) : 'the end of the run'}. The draft is filed; nothing leaves the demo.`
    : atGate
      ? 'Try it: your choice stays in this replay and is not recorded. In the run, a simulated operator decides at the next step.'
      : 'Checks passed. Next: the approval gate.'

  return (
    <article className={`draft ${atGate && !userGate ? 'is-waiting' : ''}`} aria-label={`Draft: ${d.subject}`}>
      <div className="draft-badge">
        <Status tone={badge.tone}>{badge.text}</Status>
      </div>
      <div className="draft-main">
        <div className="draft-meta">
          <span className="faint">To</span>
          <span className="mono small">{d.to.map((t) => <span key={t}>{t}<br /></span>)}</span>
          <span className="faint">Subject</span>
          <span className="draft-subject">{d.subject}</span>
        </div>
        {userGate === 'editing' ? (
          <label className="edit">
            Edit draft (this replay only)
            <textarea ref={area} value={editText} onChange={(e) => setEditText(e.target.value)} rows={9} />
          </label>
        ) : (
          <>
            <LinkedBody body={userGate === 'edited' ? editText : d.body} />
            <EditDiff />
          </>
        )}
        <div className="draft-checks">
          {desk.recipientsOk !== undefined && <Status tone={desk.recipientsOk ? 'ok' : 'fail'}>{desk.recipientsOk ? 'Recipients are verified contacts' : 'Recipients not in the verified contacts'}</Status>}
          {desk.unsupported && <Status tone={desk.unsupported.length ? 'fail' : 'ok'}>{desk.unsupported.length ? `Numbers with no source: ${desk.unsupported.join(', ')}` : 'Every number traces to the e-mail or a tool'}</Status>}
        </div>
      </div>
      {atGate && !userGate && (
        <div className="draft-actions">
          <button type="button" className="btn btn-primary" onClick={() => setUserGate('approved')}><CheckIcon size={14} weight="bold" aria-hidden="true" />Approve</button>
          <button type="button" className="btn" onClick={() => { setEditText(d.body.trim()); setUserGate('editing') }}><PencilSimpleIcon size={14} aria-hidden="true" />Edit</button>
          <button type="button" className="btn" onClick={() => setUserGate('rejected')}><XIcon size={14} aria-hidden="true" />Reject</button>
        </div>
      )}
      {userGate === 'editing' && (
        <div className="draft-actions">
          <button type="button" className="btn btn-primary" onClick={() => setUserGate('edited')}><CheckIcon size={14} weight="bold" aria-hidden="true" />Approve edited draft</button>
          <button type="button" className="btn" onClick={() => setUserGate(null)}>Cancel</button>
        </div>
      )}
      <p className="draft-note">{note}</p>
    </article>
  )
}

