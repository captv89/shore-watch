import { AnchorIcon, ArrowRightIcon, CheckIcon, CloudLightningIcon, DatabaseIcon, NotePencilIcon, WarningIcon } from '@phosphor-icons/react'
import { Fragment, type ReactNode } from 'react'
import { AGENT_TITLE, fmt1, fmtDay, fmtDayTime, fmtEta, fmtUsd, type AheadShip, type QueueItem } from '../replay/derive'
import { usePlayer } from './context'
import { Num, Status } from './ui'
import { weatherClass, wxTime } from './weather'

// Desk pieces for the Round 2 scenes: a forecast as the trigger (Scene 4),
// the speed advice for a later berth (Scene 5), an engine stop as off-hire
// (Scene 6), and the voyage management system the entries go to. Each shows
// only what the log holds at the current step. Figures from a stand-in for a
// service Shore Watch does not have say so, every time.

const HOUR = 3600_000
const ms = (iso: string) => new Date(iso).getTime()

/** Label on every figure from a stand-in. Never dropped. */
export function MadeUp({ service }: { service: string }) {
  return (
    <span className="made-up">
      <WarningIcon size={11} weight="bold" aria-hidden="true" />
      <span><strong>Made up for this simulation.</strong> {service}</span>
    </span>
  )
}

/** A time axis over [t0, t1] with a tick at each midnight UTC. Children place themselves with `at`. */
function Axis({ t0, t1 }: { t0: number; t1: number }) {
  const ticks: number[] = []
  for (let t = Math.ceil(t0 / (24 * HOUR)) * 24 * HOUR; t <= t1; t += 24 * HOUR) ticks.push(t)
  return (
    <div className="tl-axis" aria-hidden="true">
      {ticks.map((t) => (
        <span key={t} className="tl-tick" style={{ left: pct(t, t0, t1) }}><span>{fmtDay(new Date(t).toISOString())}</span></span>
      ))}
    </div>
  )
}

const pct = (t: number, t0: number, t1: number) => `${Math.max(0, Math.min(100, ((t - t0) / (t1 - t0)) * 100))}%`
const span = (a: number, b: number, t0: number, t1: number) => ({ left: pct(a, t0, t1), width: `calc(${pct(b, t0, t1)} - ${pct(a, t0, t1)})` })

/* ---------- Scene 4: the forecast and the fleet ---------- */

/** The inbox has no e-mail: a forecast run started the work. */
export function TriggerItem() {
  const { desk } = usePlayer()
  const t = desk.trigger
  if (!t) return null
  return (
    <div className="mail is-open trigger" aria-label="New forecast run">
      <div className="mail-row">
        <span className="mail-from"><CloudLightningIcon size={12} aria-hidden="true" /> {t.model} · no e-mail</span>
        <span className="mono muted">{fmtEta(t.available).split(' ')[2]}</span>
      </div>
      <div className="mail-subject">New forecast run: {fmtDay(t.run)} {wxTime(t.run).hhmm.slice(0, 2)}z</div>
      <p className="trigger-text">Out at {fmtEta(t.available)}. The desk scans every ship at sea along her remaining route.</p>
      <ul className="trigger-ships">
        {t.ships.map((s) => <li key={s}><CheckIcon size={11} weight="bold" aria-hidden="true" /> {s.replace(/^SIM-\d+ \((.*)\)$/, '$1')}</li>)}
        {t.notScanned.map((s) => <li key={s} className="muted">Not scanned: {s.replace(/^MV /, '')}</li>)}
      </ul>
    </div>
  )
}

/** One stretch of heavy weather, worded the way the forecast puts it. */
function StretchLine({ a, i, j }: { a: AheadShip; i: number; j: number }) {
  const x = a.stretches[j]
  return (
    <li className="stretch">
      <span className="stretch-when mono">{fmtDayTime(x.from)} – {fmtDayTime(x.to)}</span>
      <span><Num k={`ah${i}s${j}bf`} /> <span className="muted">wind {x.windSector}</span>, seas to <Num k={`ah${i}s${j}m`} /> <span className="muted">{x.seaSector}</span></span>
    </li>
  )
}

/** Each ship scanned: the heavy weather on her route, the note filed, and the note to the Master. */
export function FleetCard() {
  const { desk, final, ev } = usePlayer()
  if (!final.trigger) return null
  const ships = final.ahead
  return (
    <section className="ship fleet" aria-label="Ships scanned">
      <div className="ship-head">
        <h3>Ships at sea</h3>
        <span className="mono muted">{desk.ahead.length} of {ships.length} scanned</span>
      </div>
      <p className="ship-voyage">ECMWF forecast along each ship's remaining route, at her Master's ETA speed. Heavy: Beaufort 7 or more, or seas of 4 m or more.</p>
      {ships.map((f) => {
        const i = desk.ahead.findIndex((a) => a.shipId === f.shipId)
        const a = desk.ahead[i]
        const note = desk.notes.find((n) => n.shipId === f.shipId)
        const draft = desk.queue.find((q) => q.kind === 'email' && q.to.some((t) => t.includes(f.ship.replace(/^MV /, '').replace(/\s+/g, '').toLowerCase())))
        const current = ev.seq === a?.seq
        return (
          <article key={f.shipId} className={`fleet-ship ${current ? 'is-current' : ''}`}>
            <div className="fleet-ship-head">
              <span className="fleet-name">{f.ship.replace(/^MV /, '')}</span>
              <span className="mono muted">{f.shipId} · to {f.to}</span>
            </div>
            {a ? (
              <>
                {a.stretches.length ? (
                  <ul className="stretches">{a.stretches.map((_, j) => <StretchLine key={j} a={a} i={i} j={j} />)}</ul>
                ) : <Status tone="ok">No heavy weather on the route</Status>}
                <div className="fleet-steps">
                  {note ? <Status tone="ok">Bad weather days noted in the voyage file</Status> : <span className="num-pending">No note filed yet</span>}
                  {draft && <QueueStatus q={draft} what="Note to the Master" />}
                </div>
              </>
            ) : <span className="num-pending">Waiting for the Weather Agent</span>}
          </article>
        )
      })}
      <p className="fleet-foot">Nothing goes to charterers or agents: the Master owns the ETA.</p>
    </section>
  )
}

/** Per ship, the forecast wind hour by hour along the route, the heavy stretches outlined. Steps with the chart. */
export function FleetStrip({ compact }: { compact?: boolean }) {
  const { desk, final, wxStep, setWxStep, wxAnimating, openSource, colourBy } = usePlayer()
  const steps = fleetSteps(desk.ahead)
  if (!final.trigger || !desk.ahead.length) return null
  const all = final.ahead.flatMap((a) => a.samples.map((s) => ms(s.time)))
  const t0 = Math.min(...all)
  const t1 = Math.max(...all) + HOUR
  const now = steps[wxStep] ? ms(steps[wxStep]) : undefined
  return (
    <div className="tl-strip fleet-strip" aria-label="Forecast wind along each ship's route">
      {!compact && (
        <div className="wx-strip-head">
          <span><span className="wx-strip-title">Weather ahead</span> · hourly, along the remaining route · ECMWF IFS {fmtDay(final.trigger.run)} 12z, as issued</span>
          <span className="wx-strip-key">{(colourBy === 'wind' ? ['Bf 0–4, within CP limit', '5–7', '8+'] : ['Douglas 0–3, within CP limit', '4–6', '7+']).map((l, i) => <span key={l}><i className={`bf-key bf-${i}`} />{l}</span>)}<span className="muted">· boxed: heavy weather</span></span>
        </div>
      )}
      <div className="tl-grid">
        <span />
        <Axis t0={t0} t1={t1} />
        {final.ahead.map((f) => {
          const a = desk.ahead.find((x) => x.shipId === f.shipId)
          return (
            <Lane key={f.shipId} label={f.ship.replace(/^MV /, '')} sub={a ? `${fmt1(a.speedKn)} kn` : 'not scanned yet'}>
              {a && (
                <>
                  {a.samples.map((s) => (
                    <span key={s.time} className={`bf-cell bf-${weatherClass(s, colourBy)?.cls ?? 0}`} style={span(ms(s.time), ms(s.time) + HOUR, t0, t1)} title={`${fmtEta(s.time)}: Beaufort ${s.beaufort}, ${s.windSector}${s.swhM !== undefined ? `; sea ${fmt1(s.swhM)} m ${s.seaSector}` : ''}`} />
                  ))}
                  {a.stretches.map((x, j) => (
                    <button key={j} type="button" className="tl-stretch" style={span(ms(x.from), ms(x.to) + HOUR, t0, t1)} onClick={() => openSource(`ah${desk.ahead.indexOf(a)}s${j}bf`)} aria-label={`Heavy weather ${fmtDayTime(x.from)} to ${fmtDayTime(x.to)} UTC, up to Beaufort ${x.maxBf}, seas ${fmt1(x.maxSwhM)} m. Show source`}>
                      {ms(x.to) - ms(x.from) >= 10 * HOUR && <span className="tl-stretch-label mono">Bf {x.maxBf} · {fmt1(x.maxSwhM)} m</span>}
                    </button>
                  ))}
                  {!a.arrivesWithin && <span className="tl-more mono" style={{ left: pct(ms(a.until), t0, t1) }}>forecast ends</span>}
                </>
              )}
            </Lane>
          )
        })}
        {now !== undefined && <span className="tl-now" style={{ left: `calc(var(--tl-label) + (100% - var(--tl-label)) * ${(now - t0) / (t1 - t0)})` }} aria-hidden="true" />}
      </div>
      {steps.length > 1 && !compact && (
        <input className="tl-scrub" type="range" min={0} max={steps.length - 1} value={wxStep} onChange={(e) => setWxStep(Number(e.target.value))} disabled={wxAnimating}
          aria-label="Forecast time shown on the chart" aria-valuetext={steps[wxStep] ? fmtEta(steps[wxStep]) : ''} />
      )}
    </div>
  )
}

/** The forecast's 3-hourly times the ships' samples cover: the steps the chart's wind can show. */
export function fleetSteps(ahead: AheadShip[]): string[] {
  const times = new Set<string>()
  for (const a of ahead) for (const s of a.samples) if (new Date(s.time).getUTCHours() % 3 === 0) times.add(s.time)
  return [...times].sort()
}

/* ---------- Scene 5: speed for a later berth ---------- */

function Lane({ label, sub, children }: { label: string; sub?: ReactNode; children: ReactNode }) {
  return (
    <>
      <span className="tl-label"><span>{label}</span>{sub && <span className="muted mono">{sub}</span>}</span>
      <div className="tl-lane">{children}</div>
    </>
  )
}

/** Two ways to Rotterdam on one time line: on as now and wait at anchor, or slow down for the berth. */
export function SpeedPlanStrip({ compact }: { compact?: boolean }) {
  const { desk } = usePlayer()
  const a = desk.advice
  if (!a) return null
  const t0 = ms(a.fromTime)
  const berth = ms(a.berth)
  const t1 = berth + 8 * HOUR
  const eta = ms(a.currentEta)
  const arr = ms(a.arrival)
  const p = desk.speedPlan
  return (
    <div className="tl-strip plan-strip" aria-label="Speed advice on a time line">
      <div className="wx-strip-head">
        {!compact && <span><span className="wx-strip-title">Speed for the berth</span> · {desk.vms?.portCalls[0]?.port ?? 'next port'} · from the AIS position {fmtEta(a.fromTime)}</span>}
        <MadeUp service="Voyage optimisation stand-in." />
      </div>
      <div className="tl-grid">
        <span />
        <Axis t0={t0} t1={t1} />
        <Lane label="Carry on" sub={`${fmt1(a.currentKn)} kn`}>
          <span className="tl-bar is-steam" style={span(t0, eta, t0, t1)}><span className="tl-bar-label">arrives <Num k="vmsEtaOld">{fmtDayTime(a.currentEta)}</Num></span></span>
          <span className="tl-bar is-anchor" style={span(eta, arr, t0, t1)}><span className="tl-bar-label" title="At anchor, waiting for the berth"><AnchorIcon size={11} aria-label="At anchor" /><Num k="anchorH" /></span></span>
          <span className="tl-bar is-pilot" style={span(arr, berth, t0, t1)} title={`${a.pilotH} h for the pilot`} />
        </Lane>
        <Lane label="Slow down" sub={<Num k="adviceKn" />}>
          <span className="tl-bar is-steam is-slow" style={span(t0, arr, t0, t1)}><span className="tl-bar-label">arrives <Num k="adviceArr">{fmtDayTime(a.arrival)}</Num> · saves <Num k="fuelSaved" /> VLSFO</span></span>
          <span className="tl-bar is-pilot" style={span(arr, berth, t0, t1)} title={`${a.pilotH} h for the pilot`} />
        </Lane>
        <span className="tl-berth" style={{ left: `calc(var(--tl-label) + (100% - var(--tl-label)) * ${(berth - t0) / (t1 - t0)})` }}>
          <span className="tl-berth-label"><Num k="berth">Berth about {fmtDayTime(a.berth)}</Num><span className="muted">, subject to the ship ahead</span></span>
        </span>
      </div>
      <div className="tl-foot" />
      {p && (
        <div className={`plan-check tone-${p.consistent ? 'ok' : 'warn'}`}>
          {p.consistent ? <CheckIcon size={11} weight="bold" aria-hidden="true" /> : <WarningIcon size={11} weight="bold" aria-hidden="true" />}
          <span>Checker: <Num k="planDist" /> ÷ <Num k="adviceKn" /> gives <Num k="planArr" />{p.consistent ? ', as advised and before the berth.' : ': not what the advice says.'}</span>
        </div>
      )}
      <span className="tl-key"><i className="tl-key-anchor" />at anchor <i className="tl-key-pilot" /><Num k="pilotH" /> for the pilot</span>
    </div>
  )
}

/* ---------- Scene 6: the report day and the hire statement ---------- */

/** The 24 h before the noon report: steaming, and the stop the Master reported. */
export function ReportDayStrip({ compact }: { compact?: boolean }) {
  const { desk } = usePlayer()
  const r = desk.report
  const rc = desk.reportChecks
  if (!r || !rc || rc.passed) return null
  const t1 = ms(r.time)
  const t0 = t1 - 24 * HOUR
  const st = desk.stoppage
  const speed = rc.flags.findIndex((f) => f.id === 'speed')
  return (
    <div className="tl-strip day-strip" aria-label="The report day">
      {!compact && (
        <div className="wx-strip-head">
          <span><span className="wx-strip-title">The report day</span> · {fmtDayTime(new Date(t0).toISOString())} to {fmtDayTime(r.time)} UTC</span>
          <span className="muted">From the Master's figures and remarks, checked in code</span>
        </div>
      )}
      <div className="tl-grid">
        <span />
        <Axis t0={t0} t1={t1} />
        <Lane label="Main engine" sub={st ? <>stopped <Num k="stopH" /></> : 'as reported'}>
          {st ? (
            <>
              <span className="tl-bar is-steam" style={span(t0, ms(st.from), t0, t1)}><span className="tl-bar-label"><Num k="steamH" /> steaming in all</span></span>
              <span className="tl-bar is-stop" style={span(ms(st.from), ms(st.to), t0, t1)}><span className="tl-bar-label mono"><Num k="stopFrom">{fmtDayTime(st.from).split(' ')[2]}</Num>–<Num k="stopTo">{fmtDayTime(st.to).split(' ')[2]}</Num></span></span>
              <span className="tl-bar is-steam" style={span(ms(st.to), t1, t0, t1)} />
            </>
          ) : <span className="tl-bar is-unknown" style={span(t0, t1, t0, t1)}><span className="tl-bar-label">24 h as reported: <Num k="run" /> at <Num k="speed" /></span></span>}
        </Lane>
      </div>
      <div className="day-checks">
        {speed >= 0 && (
          <span className={`plan-check ${st?.consistent ? 'is-resolved' : 'tone-warn'}`}>
            <WarningIcon size={11} weight="bold" aria-hidden="true" />
            <span><Num k={`rc${speed}`}>Report check</Num>: {rc.flags[speed].finding}</span>
          </span>
        )}
        {st && (
          <span className={`plan-check tone-${st.consistent ? 'ok' : 'warn'}`}>
            {st.consistent ? <CheckIcon size={11} weight="bold" aria-hidden="true" /> : <WarningIcon size={11} weight="bold" aria-hidden="true" />}
            <span>Explained by the remark: <Num k="speed" /> over <Num k="steamH" /> steaming gives <Num k="expNm" />, the distance run. No question to the Master.</span>
          </span>
        )}
      </div>
    </div>
  )
}

/** The voyage management system's hire statement lines for the stop. Figures made up; trusted as given. */
export function OffHireCard() {
  const { desk } = usePlayer()
  const o = desk.offHire
  if (!o) return null
  return (
    <article className="desk-card ledger" aria-label="Hire statement preview from the voyage management system stand-in">
      <div className="desk-card-head"><DatabaseIcon size={12} aria-hidden="true" /><span>Hire statement preview · voyage system</span><span className="tag-quiet">Preview · nothing entered</span></div>
      <dl className="ledger-rows">
        <dt>Off hire</dt><dd className="mono">{fmtDayTime(o.from)} – {fmtDayTime(o.to).split(' ')[2]} UTC, {fmt1(o.hours)} h</dd>
        <dt>Hire <span className="muted">at <Num k="rate" /></span></dt><dd><Num k="hireUsd" /></dd>
        {o.bunkers.map((b, i) => (
          <Fragment key={b.grade}>
            <dt>{b.grade} <span className="muted"><Num k={`bunkerT${i}`} /> at <Num k={`bunkerP${i}`} /></span></dt><dd className="mono">{fmtUsd(b.amountUsd)}</dd>
          </Fragment>
        ))}
        <dt className="ledger-total">Owner's account</dt><dd className="ledger-total"><Num k="totalUsd" /></dd>
      </dl>
      <MadeUp service="Voyage management system stand-in: a real system gives its own figures, and those are the ones an operator trusts." />
    </article>
  )
}

/* ---------- The voyage management system ---------- */

/** The voyage as the voyage management system holds it, and the change an agent proposed. */
export function VmsCard() {
  const { desk } = usePlayer()
  const v = desk.vms
  if (!v) return null
  const pc = v.portCalls[0]
  const e = desk.entry
  const q = e && desk.queue.find((x) => x.ref === e.ref)
  return (
    <article className="desk-card vms-card" aria-label="Voyage management system">
      <div className="desk-card-head"><DatabaseIcon size={12} aria-hidden="true" /><span>Voyage management system</span><span className="tag-quiet">Stand-in · read only</span></div>
      <dl className="ledger-rows">
        {pc && (
          <>
            <dt>{pc.port} <span className="muted">{pc.purpose}{pc.berth ? `, ${pc.berth}` : ''}</span></dt>
            <dd>
              {e?.kind === 'port_call_eta' && e.eta ? (
                <span className="vms-change" title="Proposed, not posted"><s className="mono muted">{fmtDayTime(pc.eta)}</s><ArrowRightIcon size={10} aria-label="proposed" /><span className="mono">{fmtDayTime(e.eta)}</span></span>
              ) : <Num k="vmsEta">{fmtDayTime(pc.eta)}</Num>}
            </dd>
          </>
        )}
        <dt>Hire <span className="muted">{v.owner.replace(/ \(owner, fictional\)$/, '')}</span></dt><dd><Num k="vmsRate" /></dd>
        <dt>Off hire so far</dt>
        <dd>
          {e?.kind === 'off_hire' && e.from && e.to
            ? <span className="vms-change"><span className="mono">+ {fmtDayTime(e.from)} – {fmtDayTime(e.to).split(' ')[2]}</span></span>
            : <span className="mono">{v.offHire.length ? v.offHire.length : 'none'}</span>}
        </dd>
      </dl>
      {e && q && <QueueStatus q={q} what={`Change proposed by the ${AGENT_TITLE[e.agent ?? ''] ?? 'agent'}`} />}
    </article>
  )
}

/** Where a queue item stands, in words. */
export function QueueStatus({ q, what }: { q: QueueItem; what?: string }) {
  const done = q.kind === 'entry' ? 'approved, not posted (demo)' : 'approved, not sent (demo)'
  const label = what ? `${what}: ` : ''
  if (q.status === 'approved' || q.status === 'edited') return <Status tone="ok">{label}{done}</Status>
  if (q.status === 'rejected') return <Status tone="fail">{label}rejected</Status>
  if (q.status === 'waiting') return <Status tone="wait">{label}waits for the operator</Status>
  return <Status tone="warn">{label}checks done</Status>
}

/** The Master's remark, quoted, when the report has one. */
export function Remark() {
  const { desk } = usePlayer()
  const t = desk.report?.remarks
  if (!t) return null
  return (
    <figure className="remark">
      <figcaption><NotePencilIcon size={11} aria-hidden="true" /> Master's remarks</figcaption>
      <blockquote>{t}</blockquote>
    </figure>
  )
}
