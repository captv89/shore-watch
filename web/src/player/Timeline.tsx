import { CaretLeftIcon, CaretRightIcon, PauseIcon, PlayIcon } from '@phosphor-icons/react'
import { actorLabel, eventText, findSource, fmtDay, fmtSimTime, markerKind, nodeLabelForTool } from '../replay/derive'
import type { RunEvent } from '../replay/types'
import { usePlayer } from './context'

const DAY_MS = 86400000

function Dot({ e }: { e: RunEvent }) {
  const { seq, go } = usePlayer()
  const kind = markerKind(e)
  const state = e.seq < seq ? 'past' : e.seq === seq ? 'current' : 'future'
  return (
    <button type="button" className={`dot ${state}`} onClick={() => go(e.seq)} aria-label={`Event ${e.seq}: ${eventText(e)}`} title={`${e.seq}. ${eventText(e)}`} aria-current={e.seq === seq ? 'step' : undefined}>
      <span className={`mark mark-${kind}`} />
    </button>
  )
}

/** One run of dots per incoming e-mail, with a visible break for the days between them. */
function DayDots() {
  const { events, final, seq } = usePlayer()
  const starts = final.dayStarts
  const first = new Date(events[starts[0] - 1].sim_time).getTime()
  const days = starts.map((st, i) => ({ events: events.slice(st - 1, (starts[i + 1] ?? events.length + 1) - 1), t: new Date(events[st - 1].sim_time).getTime() }))
  return (
    <div className="dots has-days">
      <div className="dots-days">
        {days.map((d, i) => {
          const on = seq >= d.events[0].seq && seq <= d.events.at(-1)!.seq
          const gap = i > 0 ? Math.round((d.t - days[i - 1].t) / DAY_MS) : 0
          const skipped = Array.from({ length: Math.max(0, gap - 1) }, (_, k) => fmtDay(new Date(days[i - 1].t + (k + 1) * DAY_MS).toISOString()))
          return [
            i > 0 && (
              <div key={`b${i}`} className="day-break" role="separator" aria-label={`Day break: ${gap} days pass.${skipped.length ? ` ${skipped.join(', ')} not in this run.` : ''}`}>
                <span className="day-break-label">+{gap} {gap === 1 ? 'DAY' : 'DAYS'}</span>
                <div className="day-break-band"><span>{skipped.length ? `${skipped.join(', ')} · not in run` : 'next day'}</span></div>
              </div>
            ),
            <div key={`d${i}`} className="dots-day" style={{ flex: `${d.events.length} 1 0` }}>
              <span className={`day-head ${on ? 'is-on' : ''}`}>{fmtDay(d.events[0].sim_time)} · day {1 + Math.round((d.t - first) / DAY_MS)}</span>
              <div className="dots-rail" />
              <div className="dots-row">{d.events.map((e) => <Dot key={e.seq} e={e} />)}</div>
            </div>,
          ]
        })}
      </div>
    </div>
  )
}

/** The current event: what happened, who did it, and its sources. */
export function NowBar() {
  const { ev, total, sources, openSource } = usePlayer()
  const refs = [...new Set(ev.source_refs)]
    .map((r) => ({ r, s: findSource(sources, r) }))
    .filter((x) => x.s)
    .filter((x, i, a) => a.findIndex((y) => y.s!.ref === x.s!.ref) === i)
  return (
    <div className="now" aria-live="polite">
      <span className="now-seq mono">{ev.seq} / {total}</span>
      <span className="now-actor">{actorLabel(ev)}</span>
      <div className="now-main">
        <span className="now-text">{eventText(ev)}</span>
        {refs.length > 0 && (
          <div className="now-refs">
            <span className="faint small">Sources</span>
            {refs.map(({ r, s }) => (
              <button key={r} type="button" className="ref" onClick={() => openSource(s!.ref)}>{nodeLabelForTool(s!.tool)}</button>
            ))}
          </div>
        )}
      </div>
      <div className="now-time">
        <span className="mono">{fmtSimTime(ev.sim_time)}</span>
        <span className="mono faint small">run clock +{(ev.wall_ms / 1000).toFixed(1)} s</span>
      </div>
    </div>
  )
}

export function Controls() {
  const { seq, total, go, playing, togglePlay, speed, setSpeed, events, ev, final } = usePlayer()
  const pct = total > 1 ? (seq - 1) / (total - 1) : 0
  return (
    <div className="controls">
      <div className="transport">
        <button type="button" className="btn btn-icon" onClick={() => go(seq - 1)} disabled={seq <= 1} aria-label="Previous event" title="Previous event (←)"><CaretLeftIcon size={15} weight="bold" aria-hidden="true" /></button>
        <button type="button" className="btn btn-primary btn-play" onClick={togglePlay} aria-label={playing ? 'Pause' : 'Play'} title="Play or pause (Space)">
          {playing ? <PauseIcon size={15} weight="fill" aria-hidden="true" /> : <PlayIcon size={15} weight="fill" aria-hidden="true" />}
        </button>
        <button type="button" className="btn btn-icon" onClick={() => go(seq + 1)} disabled={seq >= total} aria-label="Next event" title="Next event (→)"><CaretRightIcon size={15} weight="bold" aria-hidden="true" /></button>
        <div className="seg" role="radiogroup" aria-label="Playback speed">
          {[0.5, 1, 2, 4].map((x) => (
            <button key={x} type="button" role="radio" aria-checked={speed === x} className={speed === x ? 'is-on' : ''} onClick={() => setSpeed(x)}>{x}×</button>
          ))}
        </div>
      </div>

      <div className="track">
        {final.dayStarts.length > 1 ? <DayDots /> : (
          <div className="dots">
            <div className="dots-rail" />
            <div className="dots-progress" style={{ transform: `scaleX(${pct})` }} />
            <div className="dots-row">{events.map((e) => <Dot key={e.seq} e={e} />)}</div>
          </div>
        )}
        <input
          type="range"
          min={1}
          max={total}
          step={1}
          value={seq}
          onChange={(e) => go(Number(e.target.value))}
          aria-label="Scrub through events"
          aria-valuetext={`Event ${seq} of ${total}: ${eventText(ev)}`}
        />
        <div className="legend">
          <span><i className="mark mark-ai" />AI step</span>
          <span><i className="mark mark-reason" />Stated reason</span>
          <span><i className="mark mark-tool" />Tool</span>
          <span><i className="mark mark-check" />Check (code)</span>
          <span><i className="mark mark-operator" />Operator (simulated)</span>
          <span className="keys">← → step, Space play, 1 2 3 views, P presenter</span>
        </div>
      </div>
    </div>
  )
}
