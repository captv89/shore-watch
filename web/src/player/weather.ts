import { useEffect, useState } from 'react'
import type { GridRef, WxSample } from '../replay/derive'

// The prepared ECMWF grids (data/weather) are bundled as static files and
// fetched only when a run needs one. The chart draws the wind from the same
// file the agent's tool read; the log names it and gives its SHA-256, and
// the page checks the hash before drawing (principle 1).

export interface Grid {
  lat_north: number
  lon_west: number
  step_deg: number
  rows: number
  cols: number
  times: string[]
  u10: number[][][]
  v10: number[][][]
}

export interface LoadedGrid {
  grid: Grid
  /** True when the file's SHA-256 matches the one in the log. */
  hashOk: boolean
}

const files = import.meta.glob<string>('../../../data/weather/*.json', { query: '?url', import: 'default' })
const cache = new Map<string, Promise<LoadedGrid>>()

async function sha256(text: string): Promise<string> {
  const buf = await crypto.subtle.digest('SHA-256', new TextEncoder().encode(text))
  return [...new Uint8Array(buf)].map((b) => b.toString(16).padStart(2, '0')).join('')
}

function load(ref: GridRef): Promise<LoadedGrid> {
  let p = cache.get(ref.file)
  if (!p) {
    const url = files[`../../../${ref.file}`]
    if (!url) return Promise.reject(new Error(`weather file ${ref.file} is not bundled`))
    p = url()
      .then((u) => fetch(u))
      .then((r) => r.text())
      .then(async (text) => ({ grid: JSON.parse(text) as Grid, hashOk: (await sha256(text)) === ref.sha256 }))
    cache.set(ref.file, p)
  }
  return p
}

/** The grid a log entry names, once loaded; null until then or if it fails. */
export function useGrid(ref: GridRef | undefined): LoadedGrid | null {
  const [state, setState] = useState<{ file: string; g: LoadedGrid } | null>(null)
  useEffect(() => {
    if (!ref) return
    let live = true
    load(ref).then((g) => live && setState({ file: ref.file, g }), () => {})
    return () => { live = false }
  }, [ref])
  return ref && state?.file === ref.file ? state.g : null
}

export const KNOTS_PER_MS = 3600 / 1852
const BEAUFORT_FLOOR = [1, 4, 7, 11, 17, 22, 28, 34, 41, 48, 56, 64]

/** Beaufort force for whole knots, as the tool computes it. */
export function beaufort(kn: number): number {
  let f = 0
  BEAUFORT_FLOOR.forEach((lo, i) => { if (kn >= lo) f = i + 1 })
  return f
}

export interface Arrow {
  lon: number
  lat: number
  u: number
  v: number
  kn: number
  bf: number
  fromDeg: number
}

/** Wind at every n-th grid point at one of the grid's times. */
export function arrowsAt(g: Grid, time: string, every: number): Arrow[] {
  const k = g.times.indexOf(time)
  if (k < 0) return []
  const out: Arrow[] = []
  for (let i = 0; i < g.rows; i += every) {
    for (let j = 0; j < g.cols; j += every) {
      const u = g.u10[k][i][j]
      const v = g.v10[k][i][j]
      const kn = Math.round(Math.hypot(u, v) * KNOTS_PER_MS)
      out.push({ lon: g.lon_west + j * g.step_deg, lat: g.lat_north - i * g.step_deg, u, v, kn, bf: beaufort(kn), fromDeg: (Math.atan2(-u, -v) * 180 / Math.PI + 360) % 360 })
    }
  }
  return out
}

/** The grid's extent as [[west, south], [east, north]]. */
export function extent(g: Grid): [[number, number], [number, number]] {
  return [[g.lon_west, g.lat_north - (g.rows - 1) * g.step_deg], [g.lon_west + (g.cols - 1) * g.step_deg, g.lat_north]]
}

export type ColourBy = 'sea' | 'wind'

/** Class 0 (within the CP's good weather limits), 1 or 2, by sea state or wind force. */
export function weatherClass(s: WxSample, by: ColourBy): { value: number; cls: 0 | 1 | 2 } | null {
  if (by === 'wind') return { value: s.beaufort, cls: s.beaufort <= 4 ? 0 : s.beaufort <= 7 ? 1 : 2 }
  if (s.douglas === undefined) return null
  return { value: s.douglas, cls: s.douglas <= 3 ? 0 : s.douglas <= 6 ? 1 : 2 }
}

export const CLASS_COLOUR = ['var(--ok)', 'var(--warn)', 'var(--fail)'] as const

/** High head seas: sea state 5 or more within 60° of the bow, as the tool counts them. */
export const headSea = (s: WxSample) => s.wavesRelDeg !== undefined && s.wavesRelDeg <= 60 && (s.douglas ?? 0) >= 5

/** "21 Jan 1500" */
export function wxTime(iso: string): { day: string; hhmm: string } {
  const t = new Date(iso)
  const MONTHS = ['Jan', 'Feb', 'Mar', 'Apr', 'May', 'Jun', 'Jul', 'Aug', 'Sep', 'Oct', 'Nov', 'Dec']
  return { day: `${t.getUTCDate()} ${MONTHS[t.getUTCMonth()]}`, hhmm: `${String(t.getUTCHours()).padStart(2, '0')}00` }
}
