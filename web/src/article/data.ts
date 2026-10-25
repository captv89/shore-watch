import type { Run, RunEvent } from '../replay/types'
import type { LonLat } from '../replay/derive'
import fleetWeekJson from '../../../eval/results/fleet-week.json'

// Everything the article shows that is not prose comes from here, and all
// of it is read from the recorded logs or the eval results (principle 1).

type Obj = Record<string, unknown>
const obj = (v: unknown): Obj => (v && typeof v === 'object' ? (v as Obj) : {})
const num = (v: unknown) => (typeof v === 'number' ? v : undefined)
const str = (v: unknown) => (typeof v === 'string' ? v : undefined)

/** Marker colour and shape per scene, so colour is never the only cue. */
export const SHIP_STYLE: Record<number, { col: string; shape: 'circle' | 'square' | 'tri' }> = {
  1: { col: '#c9b68c', shape: 'circle' },
  2: { col: '#7fb0ab', shape: 'square' },
  3: { col: '#d8a39a', shape: 'tri' },
}

export interface Fix { time: number; pos: LonLat }
export interface Mail { time: number; from: string; subject: string; scene: number }
export interface ShipTrack {
  name: string
  scene: number
  /** Positions in the log, oldest first: weather samples and noon reports. */
  fixes: Fix[]
  /** Route ahead, as seaRoute returned it in the log. */
  route?: LonLat[]
}

const t = (iso?: string) => (iso ? Date.parse(iso) : NaN)

/** Positions, route and e-mails one run logged. In a run over several ships, only `shipId`'s positions. */
export function fromRun(run: Run | undefined, name: string, scene: number, shipId?: string): { ship?: ShipTrack; mail: Mail[] } {
  if (!run) return { mail: [] }
  const fixes: Fix[] = []
  const mail: Mail[] = []
  let route: LonLat[] | undefined
  const push = (lat: unknown, lon: unknown, time: unknown) => {
    const la = num(lat), lo = num(lon)
    if (la !== undefined && lo !== undefined) fixes.push({ time: t(str(time)), pos: [lo, la] })
  }
  for (const e of run.events as RunEvent[]) {
    const d = obj(e.detail)
    const r = obj(d.result)
    const out = obj(r.output)
    const mine = (id: unknown) => !shipId || id === undefined || id === shipId
    if (e.type === 'message_received') {
      const m = obj(d.email)
      mail.push({ time: t(str(m.time) ?? e.sim_time), from: str(m.from) ?? '', subject: str(m.subject) ?? '', scene })
    } else if (e.type === 'forecast_available') {
      // No e-mail: a new forecast run starts the work (Scene 4).
      const f = obj(d.forecast)
      mail.push({ time: t(str(f.available) ?? e.sim_time), from: `${str(f.model) ?? 'Forecast'} · no e-mail`, subject: `New forecast run, ${new Date(t(str(f.run))).toUTCString().slice(5, 11)} ${new Date(t(str(f.run))).getUTCHours()}z`, scene })
    } else if (e.type === 'check' && String(e.source_refs[0] ?? '').startsWith('tool:weather_ahead#')) {
      if (mine(r.ship_id)) push(r.from_lat, r.from_lon, r.from)
    } else if (e.type === 'tool_result' && d.tool === 'ais_position') {
      if (mine(out.ship_id)) push(out.lat, out.lon, out.time)
    } else if (e.type === 'extracted') {
      const rp = obj(d.report)
      push(rp.lat, rp.lon, rp.report_time_utc)
    } else if (e.type === 'tool_result' && Array.isArray(d.samples) && d.grid_ref) {
      for (const s of d.samples as Obj[]) {
        const lat = num(s.lat), lon = num(s.lon)
        if (lat !== undefined && lon !== undefined) fixes.push({ time: t(str(s.time)), pos: [lon, lat] })
      }
    } else if (e.type === 'tool_result' && Array.isArray(d.path)) {
      route = d.path as LonLat[]
    }
  }
  const seen = new Set<number>()
  const clean = fixes.filter((f) => !Number.isNaN(f.time) && !seen.has(f.time) && seen.add(f.time)).sort((a, b) => a.time - b.time)
  return { ship: clean.length ? { name, scene, fixes: clean, route } : undefined, mail }
}

/** One ship across several scenes' runs: every position any of them logged, and the latest route. */
export function mergeShips(parts: (ShipTrack | undefined)[]): ShipTrack | undefined {
  const have = parts.filter((p): p is ShipTrack => !!p)
  if (!have.length) return undefined
  const seen = new Set<number>()
  const fixes = have.flatMap((p) => p.fixes).sort((a, b) => a.time - b.time).filter((f) => !seen.has(f.time) && seen.add(f.time))
  const last = [...have].sort((a, b) => a.fixes[a.fixes.length - 1].time - b.fixes[b.fixes.length - 1].time).at(-1)!
  return { name: have[0].name, scene: have[0].scene, fixes, route: last.route ?? have.find((p) => p.route)?.route }
}

/** The last logged position at or before time, or undefined before the first. */
export function fixAt(s: ShipTrack, time: number): Fix | undefined {
  let at: Fix | undefined
  for (const f of s.fixes) if (f.time <= time) at = f
  return at
}

export interface RunCost { calls: number; inputTokens: number; outputTokens: number; minutes: number; reports: number }

/** Model calls (events that carry token counts), tokens and run time. */
export function runCost(run: Run): RunCost {
  let calls = 0, inputTokens = 0, outputTokens = 0, wall = 0, reports = 0
  for (const e of run.events as RunEvent[]) {
    const c = obj(e.cost)
    const i = num(c.input_tokens) ?? 0
    if (i > 0) calls++
    inputTokens += i
    outputTokens += num(c.output_tokens) ?? 0
    wall = Math.max(wall, num(e.wall_ms) ?? 0)
    if (e.type === 'message_received') reports++
  }
  return { calls, inputTokens, outputTokens, minutes: wall / 60000, reports: Math.max(1, reports) }
}

// Eval results (cmd/eval). The held-out set is the reported score; the
// tuning set is shown only in the tuning log.
export interface TaskResult {
  id: string
  kind: string
  planted: string
  key: { topics: string[] | null; block: boolean }
  grade: { pass: boolean; missed?: string[]; false_alarms?: string[] }
  log?: string
  error?: string
}
export interface Results { model: string; set: string; recorded_at: string; passed: number; total: number; tasks: TaskResult[] }

// OpenRouter stand-ins for Gemma stay in eval/results but off the page.
const resultFiles = import.meta.glob<Results>(['../../../eval/results/heldout-*.json', '!../../../eval/results/heldout-openrouter-*.json'], { import: 'default', eager: true })

export const BASELINES = ['do-nothing', 'rules-only', 'rules-coded']
const LABEL: Record<string, [string, string]> = {
  'do-nothing': ['Do nothing', 'Baseline'],
  'rules-only': ['Rules only', 'Baseline · the same checks as code, no AI'],
  'rules-coded': ['Rules, coded report', 'Baseline · rules with extra coded fields the crew fills in'],
  'nvidia/nemotron-3.5-super-vl-120b-a12b': ['Nemotron 3.5 Super', 'Model · open weights · GPU server · the scenes above'],
  'gemma-4-31b-it': ['Gemma 4 31B', 'Model · open weights · workstation GPU · Round 1 cases only'],
}
export const label = (model: string) => LABEL[model] ?? [model, 'Model']

/** Held-out results, baselines first. */
export const heldOut: Results[] = Object.values(resultFiles).sort((a, b) => {
  const ia = BASELINES.indexOf(a.model), ib = BASELINES.indexOf(b.model)
  return (ia < 0 ? 9 : ia) - (ib < 0 ? 9 : ib) || a.model.localeCompare(b.model)
})
export const models = heldOut.filter((r) => !BASELINES.includes(r.model))

/** A task plants an issue when its key asks for a topic or a block. */
export const planted = (task: TaskResult) => task.key.block || (task.key.topics ?? []).length > 0
export const falseAlarms = (r: Results) => r.tasks.filter((t) => (t.grade.false_alarms ?? []).length > 0).length

export const KIND_LABEL: Record<string, string> = {
  clean: 'Clean noon report',
  eta_inconsistent: "Master's ETA does not add up",
  notice_due: 'ETA notice due',
  rob: 'Fuel remaining does not add up',
  speed: 'Speed against distance run',
  position: 'Position against distance run',
  missing: 'Figures missing',
  remarks: 'A remark explains the figure',
  weather: 'Fuel and weather',
  fraud: 'Fraud and payment changes',
  clean_email: 'Ordinary e-mail',
  forecast: 'A new forecast',
  berth: 'Berth news from the port agent',
  engine_stop: 'Main engine stopped: off-hire?',
}

// Fleet-week (cmd/fleetweek): rules desks counted on every
// item, agent desks estimated from a sample.
export interface WeekTotals { items: number; reached: number; needed: number; noise: number; missed: number; wrong: number; prepared: number; to_write: number }
export interface WeekDesk { name: string; estimate: boolean; sample_items?: number; week: WeekTotals }
export interface FleetWeek { items: number; desks: WeekDesk[] }
export interface WeekRow {
  model: string; name: string; note: string; estimate: boolean
  reached: string; needed: string; noise: string; missed: string; wrong: string; prepared: string; toWrite: string
  raw: WeekTotals
}

/** A count as it is; an estimate rounded, and a zero said to be the sample's. */
export const fmtWeek = (v: number, estimate: boolean) => (!estimate ? String(Math.round(v)) : Math.round(v) === 0 ? '0 in the sample' : `about ${Math.round(v)}`)

// Gemma's week leaves out the three reroute items (tuning log, 2026-10-25),
// so it is not a row.
const WEEK_DESKS = ['rules-only', 'rules-coded', 'nvidia/nemotron-3.5-super-vl-120b-a12b']
const WEEK_NOTE: Record<string, string> = {
  'rules-only': 'Every item, counted',
  'rules-coded': 'Every item, counted · extra coded fields',
  'nvidia/nemotron-3.5-super-vl-120b-a12b': 'Estimate from a sample',
}

export function weekRows(fw: FleetWeek): WeekRow[] {
  return WEEK_DESKS.flatMap((m) => {
    const d = fw.desks.find((x) => x.name === m)
    if (!d) return []
    const w = d.week, e = d.estimate
    return [{
      model: m, name: label(m)[0], note: WEEK_NOTE[m], estimate: e,
      reached: fmtWeek(w.reached, e), needed: fmtWeek(w.needed, e), noise: fmtWeek(w.noise, e), missed: fmtWeek(w.missed, e),
      wrong: fmtWeek(w.wrong, e), prepared: fmtWeek(w.prepared, e), toWrite: fmtWeek(w.to_write, e), raw: w,
    }]
  })
}

export const fleetWeek = fleetWeekJson as unknown as FleetWeek
export const weekItems = fleetWeek.items
export const weekSample = fleetWeek.desks.find((d) => d.name === WEEK_DESKS[2])?.sample_items ?? 0

/**
 * Props for links that leave the page. On its Space the page runs in a frame,
 * and GitHub refuses to load in one, so these open in a new tab.
 */
export const NEW_TAB = { target: '_blank', rel: 'noopener noreferrer' } as const

/** The public repository. */
export const REPO = 'https://github.com/captv89/shore-watch'
/** The tuning log, kept with the page on its Hugging Face Space. */
export const TUNING_LOG = 'https://huggingface.co/spaces/captv89/shore-watch/blob/main/tuning-log.md'
