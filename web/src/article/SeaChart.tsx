import { geoGraticule, geoMercator, geoPath, type GeoPermissibleObjects } from 'd3-geo'
import { useEffect, useMemo, useRef, useState } from 'react'
import { feature } from 'topojson-client'
import type { Topology } from 'topojson-specification'
import type { LonLat } from '../replay/derive'
import { fixAt, SHIP_STYLE, type ShipTrack } from './data'

let landCache: Promise<GeoPermissibleObjects> | null = null
function loadLand() {
  landCache ??= import('world-atlas/land-50m.json').then((m) => {
    const topo = m.default as unknown as Topology
    return feature(topo, topo.objects.land) as GeoPermissibleObjects
  })
  return landCache
}

const PORTS: [string, number, number][] = [['Rotterdam', 4.44, 51.9], ['Bilbao', -3.05, 43.34]]

function useSize<T extends HTMLElement>() {
  const ref = useRef<T>(null)
  const [size, setSize] = useState({ w: 0, h: 0 })
  useEffect(() => {
    const el = ref.current
    if (!el) return
    const ro = new ResizeObserver(([e]) => setSize({ w: e.contentRect.width, h: e.contentRect.height }))
    ro.observe(el)
    return () => ro.disconnect()
  }, [])
  return [ref, size] as const
}

function Marker({ shape, col }: { shape: string; col: string }) {
  const p = { fill: col, stroke: '#0e1619', strokeWidth: 1.5 }
  if (shape === 'circle') return <circle r={5.5} {...p} />
  if (shape === 'square') return <rect x={-5} y={-5} width={10} height={10} {...p} />
  return <path d="M0,-6.5L6,4.5L-6,4.5Z" {...p} />
}

/**
 * A chart of the ships' logged positions at time `at`: each ship sits at
 * its last logged fix, with the fixes so far as its trail, and the route
 * ahead dashed once the report that asked for it is in. Nothing is drawn between fixes that the log does not hold.
 */
export function SeaChart({ ships, at, box, label, notes }: { ships: ShipTrack[]; at: number; box: [LonLat, LonLat]; label: string; notes?: Record<number, string> }) {
  const [ref, { w, h }] = useSize<HTMLDivElement>()
  const [land, setLand] = useState<GeoPermissibleObjects | null>(null)
  useEffect(() => { loadLand().then(setLand) }, [])
  const proj = useMemo(() => (w && h ? geoMercator().fitExtent([[12, 36], [w - 12, h - 14]], { type: 'MultiPoint', coordinates: box }) : null), [w, h, box])
  const path = useMemo(() => (proj ? geoPath(proj) : null), [proj])
  const line = (pts: LonLat[]) => (proj ? pts.map((p, i) => { const [x, y] = proj(p) ?? [0, 0]; return `${i ? 'L' : 'M'}${x.toFixed(1)},${y.toFixed(1)}` }).join('') : '')

  return (
    <div ref={ref} className="sea-chart">
      {proj && path && (
        <svg width={w} height={h} role="img" aria-label={label}>
          <path d={path(geoGraticule().step([10, 5])()) ?? ''} fill="none" stroke="#17252b" />
          {land && <path d={path(land) ?? ''} fill="#18252b" stroke="#2e434c" strokeWidth={0.7} />}
          {PORTS.map(([n, lon, lat]) => {
            const [x, y] = proj([lon, lat]) ?? [0, 0]
            return <g key={n}><circle cx={x} cy={y} r={2.5} fill="#93a5aa" /><text x={x + 5} y={y - 5} className="sea-chart-port">{n}</text></g>
          })}
          {ships.map((s) => {
            const st = SHIP_STYLE[s.scene]
            const now = fixAt(s, at)
            if (!now) return null
            const trail = s.fixes.filter((f) => f.time <= at).map((f) => f.pos)
            const [x, y] = proj(now.pos) ?? [0, 0]
            const left = x > w * 0.6
            return (
              <g key={s.name}>
                {s.route && at >= s.fixes[s.fixes.length - 1].time && <path d={line(s.route)} fill="none" stroke={st.col} strokeOpacity={0.35} strokeDasharray="3 4" />}
                {trail.length > 1 && <path d={line(trail)} fill="none" stroke={st.col} strokeWidth={2} />}
                <g transform={`translate(${x},${y})`}>
                  <Marker shape={st.shape} col={st.col} />
                  <text x={left ? -10 : 10} y={4} textAnchor={left ? 'end' : 'start'} className="sea-chart-ship">{s.name}</text>
                  {notes?.[s.scene] && <text x={left ? -10 : 10} y={19} textAnchor={left ? 'end' : 'start'} className="sea-chart-note">{notes[s.scene]}</text>}
                </g>
              </g>
            )
          })}
        </svg>
      )}
    </div>
  )
}
