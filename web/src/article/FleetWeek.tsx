import { fleetWeek, weekItems, weekRows, weekSample, type WeekRow } from './data'

// Fleet-week figure: what reached the operator in the week,
// and who wrote the follow-up. Fill, outline and hatch carry the meaning, as
// in the eval figure, so colour is never the only cue.

const rows = weekRows(fleetWeek)
const scale = Math.max(1, ...rows.flatMap((r) => [r.raw.reached, r.raw.prepared + r.raw.to_write]))
const pct = (v: number) => `${(v / scale) * 100}%`

function Bar({ parts, label }: { parts: { v: number; kind: string; text: string }[]; label: string }) {
  return (
    <div className="wk-bar" role="img" aria-label={label}>
      {parts.filter((p) => p.v > 0).map((p) => <span key={p.kind} className="wk-seg" data-kind={p.kind} style={{ width: pct(p.v) }} title={p.text} />)}
    </div>
  )
}

function Row({ r }: { r: WeekRow }) {
  const w = r.raw
  return (
    <div className="wk-row" data-agent={r.estimate}>
      <div className="wk-desk">
        <span className="wk-name">{r.name}</span>
        <span className="a-mono a-muted a-small">{r.note}</span>
      </div>
      <div className="wk-cell">
        <Bar label={`${r.name}: ${r.reached} items reached the operator, ${r.needed} needed, ${r.noise} not needed`}
          parts={[{ v: w.needed, kind: 'needed', text: `${r.needed} needed` }, { v: w.noise, kind: 'noise', text: `${r.noise} not needed` }]} />
        <span className="wk-fig a-mono"><strong>{r.reached}</strong> <span className="a-muted">· {r.noise} not needed</span></span>
      </div>
      <div className="wk-cell">
        <Bar label={`${r.name}: ${r.prepared} follow-ups prepared by the agents, ${r.toWrite} left for the operator to write`}
          parts={[{ v: w.prepared, kind: 'prepared', text: `${r.prepared} prepared` }, { v: w.to_write, kind: 'write', text: `${r.toWrite} to write` }]} />
        <span className="wk-fig a-mono">{w.prepared > 0 ? <><strong>{r.prepared}</strong> <span className="a-muted">prepared</span></> : <><strong>{r.toWrite}</strong> <span className="a-muted">to write</span></>}</span>
      </div>
      <div className="wk-miss a-mono a-small">
        <span>Missed {r.missed}</span>
        <span className="a-muted">Wrong {r.wrong}</span>
      </div>
    </div>
  )
}

export function FleetWeek() {
  if (!rows.length) return null
  return (
    <figure className="figure wk">
      <div className="wk-head a-mono">
        <span>Desk</span><span>Reached the operator</span><span>Follow-up work</span><span>Missed</span>
      </div>
      {rows.map((r) => <Row key={r.model} r={r} />)}
      <div className="wk-legend a-mono a-small" aria-hidden>
        <span><span className="wk-key" data-kind="needed" /> needed</span>
        <span><span className="wk-key" data-kind="noise" /> not needed</span>
        <span><span className="wk-key" data-kind="prepared" /> prepared, ready to approve</span>
        <span><span className="wk-key" data-kind="write" /> left for the operator to write</span>
      </div>
      <figcaption>
        One operator, 30 ships, 20–26 January 2025: {weekItems} items in the week (noon reports, forecasts, e-mails and port news). The rules desks are counted on every item. The agent figures are an estimate, scaled from {weekSample} items the agents ran, chosen across every kind of item; a zero means none in the sample. Gemma 4 is not shown: its week leaves out the three reroute items, whose rerun did not complete.
      </figcaption>
    </figure>
  )
}
