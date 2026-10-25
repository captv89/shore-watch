import { geoGraticule, geoMercator, geoPath, type GeoPermissibleObjects } from 'd3-geo'
import { useEffect, useMemo, useRef, useState } from 'react'
import { feature } from 'topojson-client'
import type { Topology } from 'topojson-specification'
import { fmtDay, fmtEta, passageName, type DeskState, type LonLat } from '../replay/derive'
import { CaretLeftIcon, CaretRightIcon, PauseIcon, PlayIcon } from '@phosphor-icons/react'
import { usePlayer } from './context'
import { Num, Status } from './ui'
import { extent, useGrid, wxTime } from './weather'
import { WeatherTrack, WindField } from './WeatherLayer'
import { fleetSteps } from './VoyageParts'

let landCache: Promise<GeoPermissibleObjects> | null = null
/** Natural Earth 1:50m land (public domain), loaded once and bundled with the page. */
function loadLand() {
  landCache ??= import('world-atlas/land-50m.json').then((m) => {
    const topo = (m.default ?? m) as unknown as Topology
    return feature(topo, topo.objects.land) as GeoPermissibleObjects
  })
  return landCache
}

function useSize<T extends HTMLElement>() {
  const ref = useRef<T>(null)
  const [size, setSize] = useState({ w: 0, h: 0 })
  useEffect(() => {
    const el = ref.current
    if (!el) return
    const ro = new ResizeObserver(([e]) => setSize({ w: Math.round(e.contentRect.width), h: Math.round(e.contentRect.height) }))
    ro.observe(el)
    return () => ro.disconnect()
  }, [])
  return [ref, size] as const
}

function allPoints(s: DeskState): LonLat[] {
  const pts: LonLat[] = []
  s.voyage?.track.forEach((t) => pts.push(t.pos))
  if (s.report) pts.push([s.report.lon, s.report.lat])
  if (s.ais) pts.push([s.ais.lon, s.ais.lat])
  s.route?.path.forEach((p) => pts.push(p))
  return pts
}

/** Point a fraction of the way along a path, by vertex count. Good enough to place a label. */
function along(path: LonLat[], f: number): LonLat {
  return path[Math.min(path.length - 1, Math.max(0, Math.round((path.length - 1) * f)))]
}

export function Chart({ scale = 1, legend = true, arrowEvery = 4 }: { scale?: number; legend?: boolean; arrowEvery?: number }) {
  const { desk, final, seq, wxStep, wxAnimating, setWxStep, playWeather, reduced } = usePlayer()
  // A run with weather keeps one fixed frame, the weather file's extent, for
  // every day: a zoom would hide the jump between days and change the scale.
  const frameRef = final.past.find((p) => p.weather?.grid)?.weather?.grid ?? final.weather?.grid ?? final.trigger?.grid
  const frame = useGrid(frameRef)
  const wx = desk.weather
  const now = useGrid(wx?.grid ?? (desk.ahead.length ? desk.trigger?.grid : undefined))
  // Scene 4: every ship scanned, on one forecast, at one forecast time.
  const ahead = desk.ahead
  const steps = fleetSteps(ahead)
  const fleetTime = steps[wxStep]
  const [ref, { w, h }] = useSize<HTMLDivElement>()
  const [land, setLand] = useState<GeoPermissibleObjects | null>(null)
  useEffect(() => {
    let live = true
    loadLand().then((l) => live && setLand(l), () => {})
    return () => { live = false }
  }, [])

  const pad = 34 * scale
  const proj = useMemo(() => {
    const pts = frame ? extent(frame.grid) : frameRef ? [] : allPoints(final)
    if (!pts.length || w < 40 || h < 40) return null
    return geoMercator().fitExtent([[pad, pad + 14 * scale], [w - pad, h - pad]], { type: 'MultiPoint', coordinates: pts })
  }, [final, frame, frameRef, w, h, pad, scale])
  const wxMode = !!frameRef
  const path = useMemo(() => (proj ? geoPath(proj) : null), [proj])

  const k = scale
  const at = (p: LonLat) => proj!(p) ?? [0, 0]
  // The fleet scene reads several voyage files: their tracks are not one ship's.
  const track = final.trigger ? [] : desk.voyage?.track ?? []
  const cur = desk.report ? ([desk.report.lon, desk.report.lat] as LonLat) : desk.ais ? ([desk.ais.lon, desk.ais.lat] as LonLat) : null
  const shipName = (desk.report?.shipName ?? desk.vms?.ship ?? desk.voyage?.ship ?? '').replace(/^MV /, '')
  const nextPort = desk.report?.nextPort ?? desk.vms?.portCalls[0]?.port
  const trackLine: LonLat[] = [...track.map((t) => t.pos), ...(cur ? [cur] : [])]
  const route = desk.route
  const dest = route?.path.at(-1)
  // Day labels only where they have room: newest first, skipping any too close to one shown.
  const labelled = new Set<number>()
  if (proj) {
    const shown: [number, number][] = cur ? [proj(cur) ?? [0, 0]] : []
    for (let i = track.length - 1; i >= 0; i--) {
      const [x, y] = proj(track[i].pos) ?? [0, 0]
      if (shown.every(([a, b]) => Math.hypot(x - a, y - b) > 44 * k)) { labelled.add(i); shown.push([x, y]) }
    }
  }

  const label = [
    'Chart of the voyage',
    track.length ? `with the track from ${track.length} earlier noon reports` : '',
    desk.report ? `and the reported position ${desk.report.position}` : desk.ais ? `and the AIS position ${desk.ais.position}` : '',
    route ? `and the seaRoute route to ${nextPort ?? 'the next port'}` : '',
    ahead.length ? `with the forecast along the remaining route of ${ahead.map((a) => a.ship).join(' and ')}` : '',
  ].filter(Boolean).join(' ')

  return (
    <div ref={ref} className="chart" style={{ ['--k' as string]: k }}>
      {proj && path && (
        <svg width={w} height={h} role="img" aria-label={label}>
          <rect width={w} height={h} fill="var(--map)" />
          <path d={path(geoGraticule().step([5, 5])()) ?? ''} fill="none" stroke="#172429" strokeWidth={1} />
          {land && <path d={path(land) ?? ''} fill="#172327" stroke="#2e4148" strokeWidth={0.8} />}
          <Graticule proj={proj} w={w} h={h} k={k} />

          {route && route.path.length > 1 && (
            <g key={route.ref}>
              <mask id={`draw-${k}`}>
                <path className="route-draw" d={path({ type: 'LineString', coordinates: route.path }) ?? ''} pathLength={1} fill="none" stroke="#fff" strokeWidth={8 * k} />
              </mask>
              <path d={path({ type: 'LineString', coordinates: route.path }) ?? ''} mask={`url(#draw-${k})`} fill="none" stroke="var(--teal)" strokeWidth={1.6 * k} strokeDasharray={`${6 * k} ${5 * k}`} />
            </g>
          )}

          {wx && now && <WindField proj={proj} grid={now.grid} time={wx.samples[wxStep]?.time ?? ''} k={k} every={arrowEvery} w={w} h={h} />}
          {!wx && ahead.length > 0 && now && fleetTime && <WindField proj={proj} grid={now.grid} time={fleetTime} k={k} every={arrowEvery} w={w} h={h} />}
          {ahead.map((a) => (
            <g key={a.ref} className="fade-in">
              <path d={path({ type: 'LineString', coordinates: a.path }) ?? ''} fill="none" stroke="var(--teal)" strokeOpacity={0.5} strokeWidth={1.2 * k} strokeDasharray={`${5 * k} ${5 * k}`} />
              <WeatherTrack proj={proj} samples={a.samples} k={k * 0.8} label={false} />
            </g>
          ))}
          {ahead.map((a) => {
            const s = shipAt(a.samples, fleetTime)
            if (!s) return null
            const [x, y] = at(s.pos)
            return (
              <g key={`ship-${a.ref}`}>
                <circle cx={x} cy={y} r={11 * k} fill="none" stroke="var(--brass)" strokeOpacity={0.5} />
                <rect x={x - 4.5 * k} y={y - 4.5 * k} width={9 * k} height={9 * k} fill="var(--brass)" transform={`rotate(45 ${x} ${y})`} />
              </g>
            )
          })}
          {trackLine.length > 1 && <path d={path({ type: 'LineString', coordinates: trackLine }) ?? ''} fill="none" stroke="var(--brass)" strokeWidth={1.8 * k} opacity={wx ? 0.45 : 1} />}
          {desk.past.map((p) => p.weather && <WeatherTrack key={p.time} proj={proj} samples={p.weather.samples} k={k} faded />)}
          {wx && <WeatherTrack proj={proj} samples={wx.samples} end={cur ?? undefined} k={k} />}
          {wx?.samples[wxStep] && (() => {
            const [x, y] = at(wx.samples[wxStep].pos)
            return <circle cx={x} cy={y} r={6 * k} fill="none" stroke="var(--brass)" strokeWidth={1.6} className="wx-step-ring" />
          })()}
          {track.map((t, i) => {
            const [x, y] = at(t.pos)
            return (
              <g key={t.time} className="fade-in">
                <circle cx={x} cy={y} r={3 * k} fill="var(--map)" stroke="var(--brass)" strokeWidth={1.4} />
                {labelled.has(i) && <text x={x + 8 * k} y={y + 3.5 * k} className="chart-label">{fmtDay(t.time)}{t.remarks ? ', remark' : ''}</text>}
                {t.remarks && <title>{`${fmtDay(t.time)}: ${t.remarks}`}</title>}
              </g>
            )
          })}

          {dest && (() => {
            const [x, y] = at(dest)
            return <rect x={x - 4 * k} y={y - 4 * k} width={8 * k} height={8 * k} fill="var(--map)" stroke="var(--teal)" strokeWidth={1.5} />
          })()}

          {cur && (() => {
            const [x, y] = at(cur)
            return (
              <g key={`cur-${seq > 0}`} className="fade-in">
                <circle cx={x} cy={y} r={13 * k} fill="none" stroke="var(--brass)" strokeOpacity={0.45} className="pulse" />
                <rect x={x - 5 * k} y={y - 5 * k} width={10 * k} height={10 * k} fill="var(--brass)" transform={`rotate(45 ${x} ${y})`} />
              </g>
            )
          })()}
        </svg>
      )}

      {proj && (
        <div className="chart-overlay">
          {cur && desk.report && (
            <Chip at={at(cur)} side="left" k={k} h={h} under={!!wx || (!!dest && dest[0] < cur[0])}>
              <span className="chip-title">{desk.report.shipName.replace(/^MV /, '')}</span>
              <span className="chip-sub">{fmtEta(desk.report.time)}</span>
              <Num k="speed" />
            </Chip>
          )}
          {cur && !desk.report && desk.ais && (
            <Chip at={at(cur)} side="left" k={k} h={h}>
              <span className="chip-title">{shipName}</span>
              <span className="chip-sub">AIS, {fmtEta(desk.ais.time)}</span>
            </Chip>
          )}
          {ahead.map((a) => {
            const s = shipAt(a.samples, fleetTime)
            if (!s) return null
            return (
              <Chip key={a.ref} at={at(s.pos)} side="right" k={k} h={h}>
                <span className="chip-title">{a.ship.replace(/^MV /, '')}</span>
                <span className="chip-sub mono">{fleetTime ? fmtEta(fleetTime) : ''} · Bf {s.beaufort}{s.swhM !== undefined ? ` · ${s.swhM.toFixed(1)} m` : ''}</span>
              </Chip>
            )
          })}
          {legend && wx && wx.samples.length > 1 && (
            <Chip at={at(wx.samples[Math.floor(wx.samples.length / 2)].pos)} side="right" k={k} h={h} below>
              {wx.hoursSea5Ahead > 0
                ? <span className="chip-kicker tone-fail">Head seas · <Num k="wxHead" /></span>
                : <span className="chip-kicker tone-ok">No head seas · <Num k="wxHead" /></span>}
              <span className="chip-sub">sea state 5+ within 60° of the bow</span>
            </Chip>
          )}
          {!wxMode && route && route.path.length > 1 && (
            <Chip at={at(along(route.path, cur && dest && dest[0] < cur[0] ? 0.6 : 0.3))} side="right" k={k} h={h}>
              <span><Num k="dist" /> to go</span>
              <span className="chip-sub">seaRoute{route.passages.length ? `, via ${route.passages.map(passageName).join(', ')}` : ''}</span>
            </Chip>
          )}
          {!wxMode && dest && desk.report && (
            <Chip at={at(dest)} side="left" k={k} h={h} lift>
              <span className="chip-sub">{desk.report.nextPort}, Master's ETA</span>
              <Num k="eta" />
              {desk.eta && <Status tone={desk.eta.consistent ? 'ok' : 'warn'}>{desk.eta.consistent ? 'consistent' : 'check with ship'}</Status>}
            </Chip>
          )}
          {!wxMode && dest && !desk.report && desk.vms && (
            <Chip at={at(dest)} side="left" k={k} h={h} lift>
              <span className="chip-sub">{nextPort}, berth</span>
              {desk.advice ? <Num k="berth" /> : <Num k="vmsEta" />}
            </Chip>
          )}
          {!track.length && !cur && !ahead.length && (
            <p className="chart-empty">{final.trigger ? 'The chart fills in as the Weather Agent scans each ship.' : 'The chart fills in as the agent reads the report and calls its tools.'}</p>
          )}
        </div>
      )}

      <div className="chart-kicker" aria-hidden="true">Mercator{desk.report ? `, ${fmtDay(desk.report.time)} 2025` : desk.ais ? `, ${fmtDay(desk.ais.time)} 2025` : desk.trigger ? `, forecast of ${fmtDay(desk.trigger.run)} 12z` : ''}</div>
      {!wx && steps.length > 1 && (
        <div className="wx-controls" role="group" aria-label="Forecast time step">
          <button type="button" className="icon-btn" aria-label="Earlier forecast step" onClick={() => setWxStep(Math.max(0, wxStep - 1))}><CaretLeftIcon size={14} /></button>
          <span className="mono" aria-live="polite">{fmtEta(fleetTime ?? '')} · +{Math.round((new Date(fleetTime ?? 0).getTime() - new Date(desk.trigger?.run ?? 0).getTime()) / 3600000)} h</span>
          <button type="button" className="icon-btn" aria-label="Later forecast step" onClick={() => setWxStep(Math.min(steps.length - 1, wxStep + 1))}><CaretRightIcon size={14} /></button>
          {!reduced && (
            <button type="button" className="btn btn-sm" onClick={playWeather}>
              {wxAnimating ? <><PauseIcon size={12} weight="fill" aria-hidden="true" />Pause</> : <><PlayIcon size={12} weight="fill" aria-hidden="true" />Play the forecast</>}
            </button>
          )}
        </div>
      )}
      {wx && (
        <div className="wx-controls" role="group" aria-label="Weather time step">
          <button type="button" className="icon-btn" aria-label="Earlier weather step" onClick={() => setWxStep(Math.max(0, wxStep - 1))}><CaretLeftIcon size={14} /></button>
          <span className="mono" aria-live="polite">{wxTime(wx.samples[wxStep]?.time ?? '').day} {wxTime(wx.samples[wxStep]?.time ?? '').hhmm} UTC · {wxStep + 1}/{wx.samples.length}</span>
          <button type="button" className="icon-btn" aria-label="Later weather step" onClick={() => setWxStep(Math.min(wx.samples.length - 1, wxStep + 1))}><CaretRightIcon size={14} /></button>
          {!reduced && (
            <button type="button" className="btn btn-sm" onClick={playWeather}>
              {wxAnimating ? <><PauseIcon size={12} weight="fill" aria-hidden="true" />Pause</> : <><PlayIcon size={12} weight="fill" aria-hidden="true" />Play the day</>}
            </button>
          )}
        </div>
      )}
      {legend && (wx || (ahead.length > 0 && h > 380)) && <WeatherLegend compact={h < 520} forecast={!wx} />}
      {legend && !wx && !ahead.length && !final.trigger && (
        <div className="chart-legend">
          <span><i className="lg-track" />Track, from noon reports</span>
          <span><i className="lg-route" />Route to {nextPort ?? 'next port'} (seaRoute)</span>
          <span><i className="lg-pos" />{desk.report || !desk.ais ? 'Reported position' : 'AIS position'}</span>
          {wxMode && <span className="lg-note">Weather appears once an agent asks for it</span>}
        </div>
      )}
      <div className="chart-credit">
        {legend && h > 400 && (wx?.grid ?? (ahead.length ? desk.trigger?.grid : undefined)) && (() => {
          const g = (wx?.grid ?? desk.trigger?.grid)!
          return <span>Wind: {g.file} · sha256 {g.sha256.slice(0, 8)}…{now ? (now.hashOk ? ' · hash checked' : ' · hash does not match the log') : ''}</span>
        })()}
        {wxMode && <span>{legend && h > 400 ? 'Weather: adapted from ECMWF IFS open data (© 2025 ECMWF), CC BY 4.0.' : 'Weather: ECMWF, CC BY 4.0'}</span>}
        <span>Coastline: Natural Earth</span>
      </div>
    </div>
  )
}

function Graticule({ proj, w, h, k }: { proj: ReturnType<typeof geoMercator>; w: number; h: number; k: number }) {
  const inv = proj.invert
  if (!inv) return null
  const [lonA, latA] = inv([0, h]) ?? [0, 0]
  const [lonB, latB] = inv([w, 0]) ?? [0, 0]
  const lats: number[] = []
  const lons: number[] = []
  for (let v = Math.ceil(latA / 5) * 5; v <= latB; v += 5) lats.push(v)
  for (let v = Math.ceil(lonA / 5) * 5; v <= lonB; v += 5) lons.push(v)
  const mid = (lonA + lonB) / 2
  const midLat = (latA + latB) / 2
  return (
    <g className="chart-grid-label" aria-hidden="true">
      {lats.map((la) => {
        const [, y] = proj([mid, la]) ?? [0, 0]
        return y > 18 && y < h - 18 ? <text key={`la${la}`} x={w - 8} y={y - 3} textAnchor="end">{Math.abs(la)}°{la >= 0 ? 'N' : 'S'}</text> : null
      })}
      {lons.map((lo) => {
        const [x] = proj([lo, midLat]) ?? [0, 0]
        return x > 60 && x < w - 60 ? <text key={`lo${lo}`} x={x + 3} y={h - 8 * k - 30}>{lo === 0 ? '0°' : `${Math.abs(lo)}°${lo < 0 ? 'W' : 'E'}`}</text> : null
      })}
    </g>
  )
}

/** Where a ship is at a forecast time: the hourly sample at that time. */
function shipAt(samples: DeskState['ahead'][number]['samples'], time: string | undefined) {
  if (!samples.length) return undefined
  if (!time) return samples[0]
  return samples.find((s) => s.time === time) ?? (new Date(time) > new Date(samples.at(-1)!.time) ? samples.at(-1) : samples[0])
}

function WeatherLegend({ compact, forecast }: { compact?: boolean; forecast?: boolean }) {
  const { colourBy, setColourBy } = usePlayer()
  const rows: [string, number, 0 | 1 | 2][] = colourBy === 'wind'
    ? [['Bf 0–4 · within CP limit', 3, 0], ['Bf 5–7', 6, 1], ['Bf 8+', 9, 2]]
    : [['Douglas 0–3 · within CP limit', 3, 0], ['Douglas 4–6', 5, 1], ['Douglas 7+', 8, 2]]
  return (
    <div className={`chart-legend wx-legend ${compact ? 'is-compact' : ''}`}>
      <div className="wx-legend-head">
        <span className="mono">{forecast ? 'Route ahead · hourly' : 'Track · 3-h steps'}</span>
        <div className="seg seg-xs" role="radiogroup" aria-label="Colour the track by">
          {(['sea', 'wind'] as const).map((c) => (
            <button key={c} type="button" role="radio" aria-checked={colourBy === c} className={colourBy === c ? 'is-on' : ''} onClick={() => setColourBy(c)}>{c === 'sea' ? 'Sea' : 'Wind'}</button>
          ))}
        </div>
      </div>
      {rows.map(([label, v, c]) => (
        <span key={label}><i className="lg-swatch" style={{ height: Math.round(1.6 + v * 0.55), background: ['var(--ok)', 'var(--warn)', 'var(--fail)'][c] }} />{label}</span>
      ))}
      <span><i className="lg-head" />{compact ? 'High head seas' : 'High head seas: sea state 5+ within 60° of the bow'}</span>
      {!compact && !forecast && <span className="lg-note">Number on the track: {colourBy === 'wind' ? 'Beaufort force' : 'Douglas sea state'} for that 3 h</span>}
      {!compact && <span><i className="lg-arrow" aria-hidden="true" />Wind, pointing downwind. Longer, heavier = stronger. Hover for Beaufort</span>}
    </div>
  )
}

/** A label beside a point on the chart, kept inside the chart's top and bottom edges. */
function Chip({ at, side, k, h, lift, below, under, children }: { at: [number, number]; side: 'left' | 'right'; k: number; h: number; lift?: boolean; below?: boolean; under?: boolean; children: React.ReactNode }) {
  const [x, y] = at
  const edge = 44 * k
  // Near the left edge a left-hand chip would leave the chart: put it on the right.
  if (side === 'left' && !below && x < 170 * k) side = 'right'
  const style = under
    ? { left: Math.max(x, 70 * k), top: Math.min(y + 22 * k, h - edge), transform: 'translate(-50%, 0)' }
    : below
    ? { left: x, top: Math.max(y - 26 * k, edge), transform: 'translate(-40%, -100%)' }
    : {
        left: side === 'left' ? x - 20 * k : x + 14 * k,
        top: Math.min(Math.max(lift ? y - 30 * k : y, edge), h - edge),
        transform: `translate(${side === 'left' ? '-100%' : '0'}, -50%)`,
      }
  return <div className="chip" style={style}>{children}</div>
}
