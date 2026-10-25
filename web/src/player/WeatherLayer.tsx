import type { geoMercator } from 'd3-geo'
import { useMemo, useState } from 'react'
import type { LonLat, WxSample } from '../replay/derive'
import { usePlayer } from './context'
import { arrowsAt, CLASS_COLOUR, extent, headSea, weatherClass, wxTime, type Grid } from './weather'

type Proj = ReturnType<typeof geoMercator>

/** The day's track, one segment per 3-hourly sample: colour and thickness by sea state or wind force. */
export function WeatherTrack({ proj, samples, end, k, faded, label = true }: { proj: Proj; samples: WxSample[]; end?: LonLat; k: number; faded?: boolean; label?: boolean }) {
  const { colourBy } = usePlayer()
  const at = (p: LonLat) => proj(p) ?? [0, 0]
  return (
    <g opacity={faded ? 0.42 : 1} aria-hidden="true">
      {samples.map((s, i) => {
        const b = i + 1 < samples.length ? samples[i + 1].pos : end ?? s.pos
        const [x1, y1] = at(s.pos)
        const [x2, y2] = at(b)
        const c = weatherClass(s, colourBy)
        const w = (1.6 + (c?.value ?? 0) * 0.55) * k
        const len = Math.hypot(x2 - x1, y2 - y1) || 1
        // Normal pointing to the south of a westbound track, for the label and the head-sea rail.
        const nx = (y2 - y1) / len
        const ny = -(x2 - x1) / len
        const d = `M${x1},${y1}L${x2},${y2}`
        const off = w / 2 + 8 * k
        return (
          <g key={s.time}>
            {headSea(s) && (
              <>
                <path d={d} stroke="var(--fail)" strokeOpacity={0.17} strokeWidth={w + 14 * k} strokeLinecap="butt" />
                <path d={`M${x1 + nx * off},${y1 + ny * off}L${x2 + nx * off},${y2 + ny * off}`} stroke="var(--fail)" strokeWidth={1} strokeDasharray={`${3 * k} ${3 * k}`} />
              </>
            )}
            <path d={d} stroke="var(--map)" strokeWidth={w + 4 * k} strokeLinecap="round" />
            <path d={d} stroke={c ? CLASS_COLOUR[c.cls] : 'var(--faint)'} strokeWidth={w} strokeLinecap="round" />
            {label && c && !faded && (
              <text x={x1 - nx * (w / 2 + 11 * k)} y={y1 - ny * (w / 2 + 11 * k) + 3.5 * k} textAnchor="middle" className="chart-num">{c.value}</text>
            )}
          </g>
        )
      })}
    </g>
  )
}

/** Wind arrows from the logged weather file, at the selected sample's time. Hover or focus shows the force. */
export function WindField({ proj, grid, time, k, every, w, h }: { proj: Proj; grid: Grid; time: string; k: number; every: number; w: number; h: number }) {
  const arrows = useMemo(() => arrowsAt(grid, time, every), [grid, time, every])
  const [tip, setTip] = useState<{ x: number; y: number; a: (typeof arrows)[number] } | null>(null)
  const cell = Math.abs((proj([grid.step_deg * every, 51]) ?? [0, 0])[0] - (proj([0, 51]) ?? [0, 0])[0])
  const pts = arrows.map((a) => ({ a, p: proj([a.lon, a.lat]) ?? [0, 0] }))
  const [[w0, s0], [e0, n0]] = extent(grid)
  const box = [[w0, s0], [e0, s0], [e0, n0], [w0, n0], [w0, s0]].map((p) => proj(p as LonLat) ?? [0, 0])
  const tr = proj([e0, n0]) ?? [0, 0]

  const near = (mx: number, my: number) => {
    let best: (typeof pts)[number] | null = null
    let bd = Infinity
    for (const q of pts) {
      const dd = (q.p[0] - mx) ** 2 + (q.p[1] - my) ** 2
      if (dd < bd) { bd = dd; best = q }
    }
    return best && bd < (cell * 0.8) ** 2 ? best : null
  }
  const { hhmm, day } = wxTime(time)

  return (
    <g>
      <path d={`M${box.map((p) => p.join(',')).join('L')}`} fill="rgba(127,176,171,.025)" stroke="#2b4a4a" strokeDasharray={`${4 * k} ${4 * k}`} />
      <text x={Math.min(tr[0], w - 40) - 5} y={tr[1] - 5} textAnchor="end" className="chart-file-label">Weather file · {grid.step_deg}° grid, every {grid.step_deg * every}° shown</text>
      <g strokeLinecap="round" fill="none" aria-hidden="true">
        {pts.map(({ a, p: [x, y] }) => {
          const spd = Math.min(a.kn, 60)
          const L = 4 * k + (spd / 60) * cell * 0.82
          const ux = a.u / (Math.hypot(a.u, a.v) || 1)
          const uy = -a.v / (Math.hypot(a.u, a.v) || 1)
          const x0 = x - (ux * L) / 2, y0 = y - (uy * L) / 2, x1 = x + (ux * L) / 2, y1 = y + (uy * L) / 2
          const hd = Math.min(5 * k, L * 0.4)
          const f = (n: number) => n.toFixed(1)
          const d = `M${f(x0)},${f(y0)}L${f(x1)},${f(y1)}M${f(x1 - ux * hd - uy * hd * 0.6)},${f(y1 - uy * hd + ux * hd * 0.6)}L${f(x1)},${f(y1)}L${f(x1 - ux * hd + uy * hd * 0.6)},${f(y1 - uy * hd - ux * hd * 0.6)}`
          return <path key={`${a.lon},${a.lat}`} className="wind-arrow" d={d} stroke={a.bf <= 4 ? '#5f8f8a' : a.bf <= 7 ? '#b9c6c8' : 'var(--fail)'} strokeWidth={(0.7 + (spd / 60) * 1.9) * k} opacity={a.bf <= 4 ? 0.75 : 0.9} />
        })}
      </g>
      <rect
        width={w} height={h} fill="transparent"
        onMouseMove={(e) => {
          const r = (e.currentTarget.ownerSVGElement ?? e.currentTarget).getBoundingClientRect()
          const q = near(e.clientX - r.left, e.clientY - r.top)
          setTip(q ? { x: q.p[0], y: q.p[1], a: q.a } : null)
        }}
        onMouseLeave={() => setTip(null)}
      />
      {tip && (
        <g transform={`translate(${tip.x + 12},${tip.y - 12})`} pointerEvents="none">
          <rect y={-30 * k} width={188 * k} height={34 * k} rx={3} fill="#111b1f" stroke="#3a4f58" />
          <text x={7 * k} y={-16 * k} className="chart-tip"><tspan fontWeight={500}>Beaufort {tip.a.bf}</tspan> · {tip.a.kn} kn from {Math.round(tip.a.fromDeg)}°</text>
          <text x={7 * k} y={-3 * k} className="chart-tip chart-tip-sub">{day} {hhmm} UTC · file grid point</text>
        </g>
      )}
    </g>
  )
}
