import { CaretLeftIcon, CaretRightIcon, CheckIcon, PauseIcon, PencilSimpleIcon, PlayIcon, XIcon } from '@phosphor-icons/react'
import { useState } from 'react'
import { actorLabel, eventText, fmtSimTime, reasonText } from '../replay/derive'
import { Chart } from './Chart'
import { usePlayer } from './context'
import { HUB_ORDER, HubNode } from './Inside'
import { useHub } from './hub'
import { RunPicker } from './Player'
import { Compare, ForecastCallout, FuelFlag, WeatherDayBadge, WeatherStrip } from './DeskParts'
import { FraudCard, MessageReview } from './SecurityParts'
import { Status } from './ui'
import { FleetStrip, OffHireCard, ReportDayStrip, SpeedPlanStrip } from './VoyageParts'

/** Stacked layout for phones: map, then the current event; Inside view as a tab. */
export function Mobile() {
  const { final, ev, total, seq, go, playing, togglePlay, desk, userGate, setUserGate, editText, setEditText, meta } = usePlayer()
  const [tab, setTab] = useState<'desk' | 'inside'>('desk')
  const hub = useHub()
  const item = desk.queue.find((q) => q.ref === desk.focus)
  const atGate = item?.status === 'waiting' && !userGate
  return (
    <section className="player mobile" aria-label={`Scene ${meta.number} replay player`}>
      <header className="m-head">
        <div className="title-row">
          <span className="scene-no mono">Scene {meta.number}</span>
          <h2>{meta.title}</h2>
        </div>
        <p className="mono muted small">Synthetic data. Operator actions simulated. Nothing is sent.{meta.realWeather && <span className="tag-real-text"> Weather: real ECMWF data.</span>}</p>
        <RunPicker />
      </header>
      <div role="tablist" aria-label="View" className="m-tabs">
        {(['desk', 'inside'] as const).map((t) => (
          <button key={t} type="button" role="tab" aria-selected={tab === t} className={tab === t ? 'is-on' : ''} onClick={() => setTab(t)}>{t === 'desk' ? 'Desk' : 'Inside'}</button>
        ))}
      </div>
      {tab === 'desk' && final.screen ? (
        <div className="m-held">
          <MessageReview />
          <FraudCard />
        </div>
      ) : tab === 'desk' ? (
        <>
          <div className="m-map"><Chart scale={0.9} legend={false} arrowEvery={8} /></div>
          <WeatherStrip compact />
          <FleetStrip compact />
          <SpeedPlanStrip compact />
          <ReportDayStrip compact />
          {desk.offHire && <div className="m-perf"><OffHireCard /></div>}
          {(desk.consumption || desk.forecast) && (
            <div className="m-perf">
              <FuelFlag />
              <WeatherDayBadge />
              <ForecastCallout />
              <Compare />
            </div>
          )}
        </>
      ) : (
        <ol className="m-nodes" aria-label="Agents and code parts, in the order work flows">
          {HUB_ORDER.map((id) => <li key={id}><HubNode n={hub.nodes[id]} /></li>)}
        </ol>
      )}
      <div className="m-now" aria-live="polite">
        <div className="m-now-head">
          <span className="now-seq mono">{ev.seq} / {total}</span>
          <span className="muted small">{actorLabel(ev)}</span>
          <span className="mono muted small m-time">{fmtSimTime(ev.sim_time)}</span>
        </div>
        <p className="m-text">{ev.type === 'rationale' ? `“${reasonText(ev)}”` : eventText(ev)}</p>
        {atGate && (
          <div className="m-gate">
            <Status tone="wait">{item?.kind === 'entry' ? 'Entry' : 'Draft'} at the approval gate</Status>
            <span className="m-subject">{item?.subject}</span>
            <div className="m-actions">
              <button type="button" className="btn btn-primary" onClick={() => setUserGate('approved')}><CheckIcon size={16} weight="bold" aria-hidden="true" />Approve</button>
              {item?.draft && <button type="button" className="btn" onClick={() => { setEditText(item.draft!.body.trim()); setUserGate('editing') }}><PencilSimpleIcon size={16} aria-hidden="true" />Edit</button>}
              <button type="button" className="btn" onClick={() => setUserGate('rejected')}><XIcon size={16} aria-hidden="true" />Reject</button>
            </div>
          </div>
        )}
        {userGate === 'editing' && (
          <div className="m-gate">
            <label className="edit">
              Edit draft (this replay only)
              <textarea value={editText} onChange={(e) => setEditText(e.target.value)} rows={8} autoFocus />
            </label>
            <div className="m-actions">
              <button type="button" className="btn btn-primary" onClick={() => setUserGate('edited')}><CheckIcon size={16} weight="bold" aria-hidden="true" />Approve edited</button>
              <button type="button" className="btn" onClick={() => setUserGate(null)}>Cancel</button>
            </div>
          </div>
        )}
        {userGate && userGate !== 'editing' && <Status tone={userGate === 'rejected' ? 'fail' : 'ok'}>{userGate === 'rejected' ? 'You rejected it. Nothing is sent.' : 'You approved it. Not sent (demo).'}</Status>}
      </div>
      <div className="m-controls">
        <input type="range" min={1} max={total} step={1} value={seq} onChange={(e) => go(Number(e.target.value))} aria-label="Scrub through events" aria-valuetext={`Event ${seq} of ${total}`} />
        <div className="m-buttons">
          <button type="button" className="btn" onClick={() => go(seq - 1)} disabled={seq <= 1} aria-label="Previous event"><CaretLeftIcon size={18} weight="bold" aria-hidden="true" /></button>
          <button type="button" className="btn btn-primary" onClick={togglePlay} aria-label={playing ? 'Pause' : 'Play'}>{playing ? <PauseIcon size={18} weight="fill" aria-hidden="true" /> : <PlayIcon size={18} weight="fill" aria-hidden="true" />}</button>
          <button type="button" className="btn" onClick={() => go(seq + 1)} disabled={seq >= total} aria-label="Next event"><CaretRightIcon size={18} weight="bold" aria-hidden="true" /></button>
        </div>
      </div>
    </section>
  )
}
