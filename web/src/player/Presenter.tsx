import { CheckIcon, PencilSimpleIcon, XIcon } from '@phosphor-icons/react'
import { useEffect, useRef } from 'react'
import { actorLabel, eventText, fmtSimTime, reasonText } from '../replay/derive'
import { Chart } from './Chart'
import { usePlayer } from './context'
import { Compare, ForecastCallout, FuelFlag, WeatherDayBadge, WeatherStrip } from './DeskParts'
import { FraudCard } from './SecurityParts'
import { Num, Status } from './ui'
import { FleetCard, FleetStrip, OffHireCard, ReportDayStrip, SpeedPlanStrip } from './VoyageParts'

/** Large-type view for talks and video capture: arrow keys step, Esc exits. */
export function Presenter() {
  const { final, ev, total, events, seq, desk, userGate, setUserGate, setEditText, setPresenter, runs, runIndex, meta } = usePlayer()
  const perf = !!(desk.consumption || desk.weather || desk.forecast)
  // Round 2: the speed advice, the report day and hire statement, or the fleet.
  const voyage = !!(desk.advice || desk.reportChecks?.flags.length || desk.offHire || final.trigger)
  const root = useRef<HTMLDivElement>(null)
  useEffect(() => {
    const opener = document.activeElement as HTMLElement | null
    root.current?.focus()
    return () => opener?.focus?.()
  }, [])
  const item = desk.queue.find((q) => q.ref === desk.focus)
  const atGate = item?.status === 'waiting' && !userGate
  const decided = item?.status === 'approved' || item?.status === 'edited' || item?.status === 'rejected'
  const done = item?.kind === 'entry' ? 'Not posted (demo).' : 'Not sent (demo).'
  return (
    <div ref={root} className="presenter" role="dialog" aria-modal="true" aria-label="Presenter mode" tabIndex={-1}>
      <div className="presenter-map"><Chart scale={1.35} legend={false} /></div>
      <div className="presenter-side">
        <div className="presenter-kicker mono">Scene {meta.number}, event {ev.seq} of {total}, {fmtSimTime(ev.sim_time)}</div>
        <div className="presenter-now" aria-live="polite">
          <span className="presenter-actor">{actorLabel(ev)}</span>
          {ev.type === 'rationale' ? <blockquote>“{reasonText(ev)}”</blockquote> : <p>{eventText(ev)}</p>}
        </div>
        {perf && (
          <div className="presenter-perf">
            <WeatherStrip compact />
            <FuelFlag />
            <WeatherDayBadge />
            <ForecastCallout />
            <Compare />
          </div>
        )}
        {final.screen && <div className="presenter-perf"><FraudCard /></div>}
        {voyage && (
          <div className="presenter-perf presenter-voyage">
            {final.trigger ? <><FleetStrip compact /><FleetCard /></> : <><SpeedPlanStrip compact /><ReportDayStrip compact /><OffHireCard /></>}
          </div>
        )}
        {!perf && !voyage && !final.screen && <div className="presenter-stats">
          <div><span>Distance to go</span><Num k="dist" /></div>
          <div><span>Speed</span><Num k="speed" /></div>
          <div>
            <span>Master's ETA</span>
            <Num k="eta" />
            {desk.eta && <Status tone={desk.eta.consistent ? 'ok' : 'warn'}>{desk.eta.consistent ? 'Consistent' : 'Check with ship'}</Status>}
          </div>
        </div>}
        {atGate && (
          <div className="presenter-gate">
            <Status tone="wait">{item?.subject} waiting at the approval gate</Status>
            <div className="presenter-actions">
              <button type="button" className="btn btn-primary" onClick={() => setUserGate('approved')}><CheckIcon size={20} weight="bold" aria-hidden="true" />Approve</button>
              {item?.draft && <button type="button" className="btn" onClick={() => { setEditText(item.draft!.body.trim()); setUserGate('editing'); setPresenter(false) }} title="Edit in the Desk view"><PencilSimpleIcon size={20} aria-hidden="true" />Edit</button>}
              <button type="button" className="btn" onClick={() => setUserGate('rejected')}><XIcon size={20} aria-hidden="true" />Reject</button>
            </div>
          </div>
        )}
        {(userGate === 'approved' || userGate === 'rejected') && <Status tone={userGate === 'rejected' ? 'fail' : 'ok'}>{userGate === 'rejected' ? 'You rejected it. Nothing leaves the demo.' : `You approved it. ${done}`}</Status>}
        {decided && <Status tone={item?.status === 'rejected' ? 'fail' : 'ok'}>{item?.status === 'rejected' ? 'Rejected by the simulated operator' : `Approved by the simulated operator. ${done}`}</Status>}
        <div className="presenter-foot">
          <div className="presenter-bars" aria-hidden="true">
            {events.map((e) => <span key={e.seq} className={e.seq === seq ? 'is-current' : e.seq < seq ? 'is-past' : ''} />)}
          </div>
          <div className="presenter-meta">
            <span>Synthetic data.{meta.realWeather ? ' Weather: real ECMWF data.' : ''} Recorded run {runs[runIndex].header.attempt} of {runs[runIndex].header.total_attempts}. Nothing is sent.</span>
            <span>← → step, Esc exit</span>
          </div>
        </div>
      </div>
    </div>
  )
}
