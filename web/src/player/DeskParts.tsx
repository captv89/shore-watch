import { CheckIcon, NotePencilIcon, PlusIcon, QuestionIcon, WarningIcon } from '@phosphor-icons/react'
import { AGENT_TITLE, fmt1, fmtDay, fmtEta, type WxSample } from '../replay/derive'
import { usePlayer } from './context'
import { Num, Status } from './ui'
import { CLASS_COLOUR, headSea, weatherClass, wxTime } from './weather'

// Desk pieces for scenes with weather and fuel (Scene 2). Each shows only
// what the log holds at the current step.

/** Weather on the track, one cell per 3-hourly sample. Each cell opens its source. */
export function WeatherStrip({ compact }: { compact?: boolean }) {
  const { desk, wxStep, setWxStep, wxAnimating, openSource, colourBy } = usePlayer()
  const w = desk.weather
  if (!w) {
    if (desk.consumption?.flagged) return <div className="wx-strip-empty">Weather on the track: not asked for yet on {desk.report ? fmtDay(desk.report.time) : 'this day'}.</div>
    return null
  }
  return (
    <div className="wx-strip" aria-label="Weather on the track, every 3 hours">
      {!compact && (
        <div className="wx-strip-head">
          <span><span className="wx-strip-title">Weather on the track</span> · every 3 h · ECMWF IFS forecasts as issued</span>
          <span className="wx-strip-key">Ring = the ship, bow up <i className="key-wind" />wind <i className="key-sea" />sea</span>
        </div>
      )}
      <div className="wx-cells">
        {w.samples.map((s, i) => {
          const t = wxTime(s.time)
          const prev = i > 0 ? wxTime(w.samples[i - 1].time).day : ''
          const on = !wxAnimating || i <= wxStep
          return (
            <button
              key={s.time} type="button"
              className={`wx-cell ${i === wxStep ? 'is-on' : ''} ${on ? '' : 'is-ahead'}`}
              onClick={() => openSource(`wx${i}`)}
              onMouseEnter={() => !wxAnimating && setWxStep(i)}
              onFocus={() => !wxAnimating && setWxStep(i)}
              onMouseLeave={() => !wxAnimating && setWxStep(null)}
              aria-label={cellLabel(s)}
            >
              <span className="wx-cell-time mono"><span>{t.hhmm}</span>{t.day !== prev && <span className="brass">{t.day}</span>}</span>
              <span className="wx-cell-body">
                <Rose s={s} colour={weatherClass(s, colourBy)?.cls ?? 0} />
                <span className="wx-cell-nums mono">
                  <span className="wx-bf">Bf {s.beaufort}</span>
                  <span>{s.swhM !== undefined ? `${fmt1(s.swhM)} m` : 'no sea'}</span>
                </span>
              </span>
              <span className={`wx-rel ${headSea(s) ? 'tone-fail' : ''}`}>
                {headSea(s) ? <WarningIcon size={10} weight="fill" aria-hidden="true" /> : null}
                {s.seaSector ?? s.windSector}
              </span>
            </button>
          )
        })}
      </div>
    </div>
  )
}

function cellLabel(s: WxSample) {
  const t = wxTime(s.time)
  return `${t.day} ${t.hhmm} UTC: Beaufort ${s.beaufort}, ${s.windKn} knots ${s.windSector}${s.swhM !== undefined ? `; sea ${fmt1(s.swhM)} metres, sea state ${s.douglas}, ${s.seaSector}` : ''}. Show source`
}

/** Ship in a ring, bow up: a thin line for the wind and a coloured arrow for the sea, where each comes from. */
function Rose({ s, colour }: { s: WxSample; colour: 0 | 1 | 2 }) {
  // Relative angles are unsigned (0 ahead, 180 astern); the tool does not log the side, so both draw to starboard.
  return (
    <svg className="wx-rose" width={30} height={30} viewBox="-15 -15 30 30" aria-hidden="true">
      <circle r={14} fill="none" stroke="var(--line-2)" />
      <path d="M0,-5 L3,5 L-3,5 Z" fill="#5f7479" />
      <g transform={`rotate(${s.windRelDeg})`}><line y1={-13.5} y2={-6} stroke="var(--text-2)" strokeWidth={1.5} /></g>
      {s.wavesRelDeg !== undefined && (
        <g transform={`rotate(${s.wavesRelDeg})`}>
          <path d="M-1.5,-14 L1.5,-14 L1.5,-10 L4,-10 L0,-6 L-4,-10 L-1.5,-10 Z" fill={CLASS_COLOUR[colour]} />
        </g>
      )}
    </svg>
  )
}

/** Fuel against the CP warranty, with where the figure sits against the "about" margin (older logs: the desk's tolerance). */
export function FuelFlag() {
  const { desk } = usePlayer()
  const c = desk.consumption
  if (!c) return null
  const about = c.allowedT !== undefined
  const limit = c.allowedT ?? c.warrantedT * (1 + c.tolerancePct / 100)
  const fuelOver = c.fuelFlagged ?? c.flagged
  // Scale from 8% under the warranty to 25% over it.
  const lo = c.warrantedT * 0.92
  const hi = c.warrantedT * 1.25
  const at = (v: number) => `${Math.max(0, Math.min(100, ((v - lo) / (hi - lo)) * 100))}%`
  return (
    <div className={`fuel-flag ${c.flagged ? 'tone-warn' : 'tone-ok'}`}>
      <div className="fuel-line">
        <span className="callout-icon" aria-hidden="true">{c.flagged ? <WarningIcon size={11} weight="bold" /> : <CheckIcon size={11} weight="bold" />}</span>
        {about ? (
          <span>
            <strong>{fuelOver ? 'Over the warranty.' : 'Within the warranty.'}</strong> VLSFO <Num k="fuel" />
            {fuelOver ? <>, <Num k="overAllowed" /> over the <Num k="allowed" /> allowed</> : <>, within the <Num k="allowed" /> allowed</>}
            {' '}(<Num k="excess" /> on the CP warranted <Num k="warranted" />).
            {c.speedFlagged && <> Speed <Num k="speed" />, below the <Num k="minSpeed" /> allowed.</>}
          </span>
        ) : (
          <span>
            <strong>{c.flagged ? 'Over the warranty.' : 'Within the warranty.'}</strong> VLSFO <Num k="fuel" />, <Num k="excess" /> against the CP warranted <Num k="warranted" />.
          </span>
        )}
      </div>
      <div className="fuel-scale" role="img" aria-label={`Warranted ${fmt1(c.warrantedT)} t, ${about ? 'allowed' : 'desk tolerance'} ${fmt1(limit)} t, reported ${fmt1(c.reportedT)} t: ${fuelOver ? 'above' : 'within'} the ${about ? 'allowed figure' : 'tolerance'}.`}>
        <div className="fuel-rail" />
        <div className="fuel-band" style={{ left: at(c.warrantedT), width: `calc(${at(limit)} - ${at(c.warrantedT)})` }} />
        <div className="fuel-over" style={{ left: at(limit) }} />
        <div className="fuel-tick" style={{ left: at(c.warrantedT) }} />
        <div className="fuel-tick is-warn" style={{ left: at(limit) }} />
        <span className="fuel-tick-label" style={{ left: at(c.warrantedT) }}>{fmt1(c.warrantedT)} CP</span>
        <span className="fuel-tick-label is-warn" style={{ left: at(limit) }}>{about ? `${fmt1(limit)} allowed` : `+${c.tolerancePct}% desk`}</span>
        <span className="fuel-marker" style={{ left: at(c.reportedT) }} />
        <span className="fuel-marker-label" style={{ left: at(c.reportedT) }}>{fmt1(c.reportedT)} t</span>
      </div>
    </div>
  )
}

/** The Checker's verdict on the day's weather against the CP. */
export function WeatherDayBadge() {
  const { desk, openSource } = usePlayer()
  const g = desk.weatherDay
  if (!g?.classified) return null
  if (g.good) {
    return (
      <button type="button" className="badge-btn tone-warn" onClick={() => openSource('gwd')}>
        <span className="badge-ring" aria-hidden="true" />
        <span><strong>Good weather day:</strong> weather does not explain the extra fuel. <span className="muted">Bf ≤ {g.limitBf} and Douglas ≤ {g.limitDouglas} all day (CP limits).</span></span>
      </button>
    )
  }
  return (
    <button type="button" className="badge-btn tone-ok is-filled" onClick={() => openSource('gwd')}>
      <span className="badge-dot" aria-hidden="true"><CheckIcon size={10} weight="bold" /></span>
      <span><strong>Excluded: bad weather (Bf {g.maxBf}).</strong> <span className="muted">Sea state {g.maxDouglas}. Not counted against the warranty.</span></span>
    </button>
  )
}

/** The Voyage Agent's forecast check: is the Master's ETA at risk? Amber at most: the Master owns the ETA. */
export function ForecastCallout() {
  const { desk } = usePlayer()
  const f = desk.forecast
  if (!f) return null
  return (
    <div className={`callout tone-${f.atRisk ? 'warn' : 'ok'}`}>
      <span className="callout-icon" aria-hidden="true">{f.atRisk ? <WarningIcon size={11} weight="bold" /> : <CheckIcon size={11} weight="bold" />}</span>
      <span>
        <strong>{f.atRisk ? 'ETA at risk.' : 'Forecast agrees.'}</strong> The forecast of {fmtDay(f.run)} gives <Num k="fcEta" />, <Num k="fcDiff" /> on the Master's (CP limit {f.limitH} h).
        {f.atRisk && ' The Master owns the ETA; the operator decides whether to ask for a review.'}
        <span className="model-label">Simplified speed-loss model (Kwon 2008), for learning</span>
      </span>
    </div>
  )
}

/** Day by day, the same symptom and the answer each day got. */
export function Compare() {
  const { desk } = usePlayer()
  const days = [...desk.past, { time: desk.report?.time ?? '', consumption: desk.consumption, weatherDay: desk.weatherDay }]
    .filter((d) => d.consumption?.flagged && d.weatherDay?.classified)
  if (days.length < 2 || !desk.weatherDay) return null
  return (
    <div className="compare">
      <span className="compare-head">Two reports, same symptom</span>
      <div className="compare-grid">
        {days.map((d) => (
          <div key={d.time} className="compare-row">
            <span className="mono muted">{fmtDay(d.time)}</span>
            <span className="mono">+{fmt1(d.consumption!.excessPct)}%</span>
            {d.weatherDay!.good
              ? <span className="tone-warn"><span className="badge-ring sm" aria-hidden="true" /> <span className="text-2">Calm, so ask the Master and alert the operator</span></span>
              : <span className="tone-ok"><CheckIcon size={11} weight="bold" aria-hidden="true" /> <span className="text-2">Storm, so exclude the day and file a note</span></span>}
          </div>
        ))}
      </div>
      <p className="compare-line">Same symptom, two different answers, because the evidence changed.</p>
    </div>
  )
}

export function AlertCard() {
  const { desk } = usePlayer()
  const a = desk.alert
  if (!a) return null
  return (
    <article className="desk-card alert-card" aria-label="Operator alert written by an agent">
      <div className="desk-card-head tone-warn"><WarningIcon size={12} weight="fill" aria-hidden="true" /><span>Operator alert · {a.agent ? AGENT_TITLE[a.agent] : 'agent'}</span></div>
      <p>“{a.text}”</p>
      <span className="desk-card-foot">Stays on the desk. Not an e-mail; nothing is sent.</span>
    </article>
  )
}

/** A decision the Ops Agent put to the operator, and the simulated operator's answer. */
export function QuestionCard() {
  const { desk } = usePlayer()
  const q = desk.question
  if (!q) return null
  return (
    <article className={`desk-card question-card ${q.answer ? 'is-answered' : ''}`} aria-label="Question to the operator from the Ops Agent">
      <div className="desk-card-head tone-wait"><QuestionIcon size={12} weight="fill" aria-hidden="true" /><span>For you to decide · Ops Agent</span></div>
      <p>{q.text}</p>
      {q.proposal && <p className="question-proposal"><span>On yes</span>{q.proposal}</p>}
      {q.answer
        ? <Status tone={q.answer.yes ? 'ok' : 'fail'}>Simulated operator: “{q.answer.text}”</Status>
        : <Status tone="wait">Waiting for the operator. Nothing is drafted until then.</Status>}
    </article>
  )
}

export function NoteCard() {
  const { desk } = usePlayer()
  const n = desk.note
  if (!n) return null
  return (
    <article className="desk-card note-card" aria-label="Voyage-file note written by an agent">
      <div className="desk-card-head"><NotePencilIcon size={12} aria-hidden="true" /><span>Voyage-file note · Performance Agent</span><span className="tag-quiet">Not an e-mail · nothing sent</span></div>
      <p>{n.text}</p>
    </article>
  )
}

/** Lines the simulated operator added to the draft, shown as a calm diff. */
export function EditDiff() {
  const { desk } = usePlayer()
  const e = desk.edit
  if (!e) return null
  const before = new Set(e.before.split('\n').map((l) => l.trim()))
  const added = e.after.split('\n').map((l) => l.trim()).filter((l) => l && !before.has(l))
  if (!added.length) return null
  return (
    <>
      <div className="diff" role="group" aria-label="Lines added by the simulated operator">
        {added.map((l) => (
          <div key={l} className="diff-line"><PlusIcon size={11} weight="bold" aria-hidden="true" /><span>{l}</span></div>
        ))}
        <span className="diff-meta">Added · simulated operator action · event {e.seq}</span>
      </div>
      <span className="faint small">{added.length} line{added.length === 1 ? '' : 's'} added. Nothing else changed.</span>
    </>
  )
}

/** Drafts from earlier days in this run, with what happened to them. */
export function QueueHistory() {
  const { desk } = usePlayer()
  const earlier = desk.queue.filter((q) => q.day < desk.day)
  if (!earlier.length) return null
  return (
    <div className="queue-history">
      {earlier.map((q) => (
        <span key={q.ref} className="small">
          <span className="muted">{q.subject}</span> · {q.status === 'rejected' ? <Status tone="fail">rejected</Status> : <Status tone="ok">approved, not sent (demo)</Status>}
        </span>
      ))}
    </div>
  )
}

export function dayLabel(iso: string) {
  return `Noon report · ${fmtEta(iso)}`
}
