// Everything the player shows is derived here from the event log, up to the
// current step. Nothing is typed in by hand: if it is not in the log, the
// page does not show it (principle 1).

import type { Actor, RunEvent } from './types'

type Obj = Record<string, unknown>
const obj = (v: unknown): Obj => (v && typeof v === 'object' && !Array.isArray(v) ? (v as Obj) : {})
const num = (v: unknown): number | undefined => (typeof v === 'number' ? v : undefined)
const str = (v: unknown): string | undefined => (typeof v === 'string' ? v : undefined)

export type LonLat = [number, number]

/** A tool call, or another source a number can come from (the e-mail). */
export interface Source {
  ref: string
  tool: string
  /** Event that asked for it, if the call was logged separately. */
  callSeq?: number
  /** Event that holds the result. */
  seq: number
  args?: unknown
  result: unknown
}

/** Tool name and call ID from a ref like "tool:distance_to_go#call_812149". */
export function parseRef(ref: string): { kind: string; name: string; id?: string } {
  const [kind, rest = ''] = ref.split(/:(.*)/s)
  const [name, id] = rest.split('#')
  return { kind, name, id }
}

/** Every source in a run, keyed by ref. */
export function indexSources(events: RunEvent[]): Map<string, Source> {
  const out = new Map<string, Source>()
  const put = (ref: string | undefined, s: Omit<Source, 'ref'>) => {
    if (!ref) return
    const prev = out.get(ref)
    out.set(ref, { ...prev, ...s, ref })
  }
  for (const e of events) {
    const d = e.detail
    const first = e.source_refs[0]
    switch (e.type) {
      case 'message_received': {
        const email = obj(d.email)
        // Noon reports have nr- IDs; anything else is an e-mail from outside.
        put(`email:${str(email.id)}`, { tool: str(email.id)?.startsWith('nr-') ? 'inbox' : 'inbox_mail', seq: e.seq, result: str(email.body) ?? '' })
        break
      }
      case 'forecast_available':
        // A forecast run, not an e-mail, started the work.
        put(first, { tool: 'forecast', seq: e.seq, result: d.forecast })
        break
      case 'tool_call':
        if (str(d.tool) && first) put(first, { tool: str(d.tool)!, callSeq: e.seq, seq: e.seq, args: d.args ?? { email_id: d.email_id }, result: undefined })
        break
      case 'tool_result':
        if (str(d.tool) && first) {
          const r = obj(d.result)
          put(first, { tool: str(d.tool)!, seq: e.seq, result: 'output' in r ? r.output : d.result, callSeq: out.get(first)?.callSeq, args: out.get(first)?.args })
        }
        break
      case 'extracted':
        if (first) put(first, { tool: 'submit_extraction', seq: e.seq, result: d.report })
        break
      case 'check':
        if (e.actor === 'checker' && /^tool:(check_|classify_|weather_ahead#)/.test(first ?? '')) put(first, { tool: parseRef(first!).name, seq: e.seq, result: d.result })
        else if (d.tool === 'report_checks' && first) put(first, { tool: 'report_checks', seq: e.seq, result: d.flagged })
        else if (first?.startsWith('tool:report_findings') && !d.problems) put(first, { tool: 'report_findings', seq: e.seq, result: d.findings })
        // A text the number guard refused never reaches its tool: the check holds it.
        else if ('unsupported' in d && d.passed === false && first && !out.has(first)) put(first, { tool: parseRef(first).name, seq: e.seq, result: d.text ?? e.summary })
        break
      case 'draft':
        if (first) put(first, { tool: 'draft_email', seq: e.seq, result: d.draft })
        break
      case 'entry':
        if (first) put(first, { tool: 'propose_entry', seq: e.seq, result: obj(d.draft).entry })
        break
      case 'plan':
        // A hand-off to another agent; its reply arrives as a tool_result on the same ref.
        if (str(d.delegate_to) && first) put(first, { tool: str(d.delegate_to)!, callSeq: e.seq, seq: e.seq, args: d.args, result: undefined })
        break
      case 'alert':
        if (first?.startsWith('tool:alert_operator')) put(first, { tool: 'alert_operator', seq: e.seq, result: d.text })
        break
      case 'approval_requested':
        if (first?.startsWith('approval:')) put(first, { tool: 'approval_gate', seq: e.seq, result: undefined })
        break
      case 'question':
        // The operator's answer cites the question; the question cites its call.
        for (const ref of e.source_refs.slice(0, 2)) if (/^(question:|tool:ask_operator#)/.test(ref)) put(ref, { tool: 'ask_operator', seq: e.seq, result: d.question })
        break
    }
    if (e.type === 'tool_result' && e.actor === 'subagent' && first) {
      put(first, { tool: e.agent ?? parseRef(first).name, seq: e.seq, result: d.result, callSeq: out.get(first)?.callSeq, args: out.get(first)?.args })
    }
  }
  return out
}

/** Finds a source; "contacts:SIM-0001" resolves to the contacts tool call. */
export function findSource(sources: Map<string, Source>, ref: string): Source | undefined {
  if (sources.has(ref)) return sources.get(ref)
  const { kind } = parseRef(ref)
  if (kind === 'contacts') return [...sources.values()].find((s) => s.tool === 'contacts')
  return undefined
}

export interface Email {
  id: string
  from: string
  time: string
  subject: string
  body: string
  ref: string
}

export interface Report {
  ref: string
  shipName: string
  position: string
  lat: number
  lon: number
  time: string
  distanceRunNm?: number
  speedKn?: number
  robVlsfoT?: number
  robMgoT?: number
  nextPort: string
  masterEta: string
  consVlsfoT?: number
  etaBasisKn?: number
  remarks?: string
}

export interface TrackPoint {
  pos: LonLat
  time: string
  remarks?: string
}

export interface Draft {
  ref: string
  seq: number
  to: string[]
  subject: string
  body: string
}

export type GateStatus = 'none' | 'waiting' | 'approved' | 'edited' | 'rejected'

/** Weather where the ship was at one time (weather_on_track, schema 1.1). */
export interface WxSample {
  time: string
  pos: LonLat
  headingDeg: number
  windFromDeg: number
  windKn: number
  beaufort: number
  windRelDeg: number
  windSector: string
  swhM?: number
  douglas?: number
  wavesRelDeg?: number
  seaSector?: string
}

/** The forecast where the ship is expected, one an hour (check_forecast_eta). */
export interface FcSample {
  time: string
  pos: LonLat
  beaufort: number
  windRelDeg: number
  sector: string
  lossPct: number
  speedKn: number
  forecast: boolean
}

export interface GridRef {
  file: string
  sha256: string
}

/** One earlier report in the same run, kept for the faded track and the comparison. */
export interface PastDay {
  time: string
  weather?: DeskState['weather']
  consumption?: DeskState['consumption']
  weatherDay?: DeskState['weatherDay']
}

/** A draft anywhere in the run, with what happened to it at the gate. */
export interface QueueItem {
  ref: string
  approvalRef?: string
  seq: number
  day: number
  /** An e-mail draft, or an entry for the voyage management system. */
  kind: 'email' | 'entry'
  to: string[]
  subject: string
  status: GateStatus
  /** Set when the approval gate has filed it. */
  filed?: string
  draft?: Draft
  entry?: Entry
}

/** An entry an agent proposed for the voyage management system (Round 2). It waits at the gate and is never posted. */
export interface Entry {
  ref: string
  seq: number
  approvalRef: string
  kind: string
  port?: string
  eta?: string
  from?: string
  to?: string
  reason: string
  agent?: string
}

/** One stretch of heavy weather on a ship's remaining route (weather_ahead). */
export interface Stretch {
  from: string
  to: string
  maxBf: number
  maxSwhM: number
  maxWindKn: number
  windSector: string
  seaSector: string
  startsInH: number
}

/** The forecast along one ship's remaining route (Scene 4). */
export interface AheadShip {
  ref: string
  seq: number
  shipId: string
  ship: string
  to: string
  speedKn: number
  distanceNm: number
  path: LonLat[]
  samples: WxSample[]
  stretches: Stretch[]
  maxBf: number
  maxSwhM: number
  until: string
  arrivesWithin: boolean
  grid?: GridRef
}

export interface Finding { kind: string; quote: string; why: string }
export interface Reason { by: string; kind: string; text: string; quote?: string }

const reasons = (v: unknown): Reason[] => ((v as unknown[] | undefined) ?? []).map((x) => {
  const m = obj(x)
  return { by: str(m.by) ?? '', kind: str(m.kind) ?? '', text: str(m.text) ?? '', quote: str(m.quote) }
})

export interface DeskState {
  email?: Email
  /** A new forecast run, not an e-mail, started the work (Scene 4). */
  trigger?: { seq: number; ref: string; model: string; run: string; available: string; ships: string[]; notScanned: string[]; grid?: GridRef }
  /** Weather ahead, one per ship scanned. */
  ahead: AheadShip[]
  /** Voyage-file notes filed so far in this episode, oldest first. */
  notes: { ref: string; seq: number; text: string; agent?: string; shipId?: string }[]
  /** The ship's AIS position, when no noon report gives one (Scene 5). */
  ais?: { ref: string; lat: number; lon: number; time: string; position: string }
  /** The voyage as the voyage management system (stand-in) holds it. */
  vms?: { ref: string; ship: string; portCalls: { port: string; purpose: string; eta: string; berth?: string }[]; owner: string; rateUsdPerDay: number; offHire: { from: string; to: string; reason: string }[]; bunkerPrices: Record<string, number> }
  /** Speed advice from the voyage optimisation stand-in: figures made up for this simulation. */
  advice?: { ref: string; seq: number; fromTime: string; speedKn: number; baseKn: number; currentKn: number; arrival: string; berth: string; currentEta: string; pilotH: number; anchorH: number; vlsfoSavedT: number; distanceNm: number; note: string; service: string }
  /** The Checker's check of the speed advice against seaRoute's distance. */
  speedPlan?: { ref: string; seq: number; consistent: boolean; distanceNm: number; adviceDistanceNm: number; hoursToGo: number; arrival: string; adviceArrival: string; diffH: number; beforeBerth: boolean; berth: string }
  /** The report checks run at extraction (speed against distance run and others). */
  reportChecks?: { ref: string; seq: number; passed: boolean; flags: { id: string; finding: string }[] }
  /** The Checker's check of a stop the Master reported in the remarks (Scene 6). */
  stoppage?: { ref: string; seq: number; consistent: boolean; from: string; to: string; stopH: number; steamingH: number; speedKn: number; expectedNm: number; runNm: number; diffPct: number; withinDay: boolean; fuel: Record<string, number> }
  /** The hire statement preview from the voyage management system (stand-in): figures made up for this simulation. */
  offHire?: { ref: string; seq: number; from: string; to: string; hours: number; rateUsdPerDay: number; hireUsd: number; bunkers: { grade: string; tonnes: number; usdPerT: number; amountUsd: number }[]; bunkersUsd: number; totalUsd: number; owner: string; note: string }
  /** The latest entry an agent proposed, and which kind of item came last. */
  entry?: Entry
  latest?: 'email' | 'entry'
  /** The queue item the latest gate event was about: the one the desk shows in full. */
  focus?: string
  senderKnown?: boolean
  report?: Report
  voyage?: { ref: string; ship: string; from: string; to: string; cargo: string; track: TrackPoint[] }
  route?: { ref: string; distanceNm: number; passages: string[]; path: LonLat[] }
  cp?: { ref: string; changeHours: number; noticeDays: number[] }
  contacts?: { ref: string; count: number }
  eta?: { ref: string; consistent: boolean; computedEta: string; differenceH: number; toleranceH: number; hoursToGo: number }
  notices?: { ref: string; due: boolean; daysToEta: number; thresholdDays?: number; movedH: number; limitH: number; lastNoticeEta?: string }
  recipientsOk?: boolean
  unsupported?: string[]
  draft?: Draft
  gate: GateStatus
  /** Simulated time of the operator's recorded decision. */
  decidedAt?: string
  /** Set when the draft is filed after a decision. */
  filed?: string
  /** Index of the current incoming e-mail in the run (0 for the first). */
  day: number
  /** Event at which each e-mail so far arrived. */
  dayStarts: number[]
  consumption?: { ref: string; reportedT: number; warrantedT: number; excessPct: number; tolerancePct: number; flagged: boolean }
  weather?: {
    ref: string
    seq: number
    samples: WxSample[]
    grid?: GridRef
    minBf: number
    maxBf: number
    maxWindKn: number
    minDouglas: number
    maxDouglas: number
    minSwhM: number
    maxSwhM: number
    hoursBf7Ahead: number
    hoursSea5Ahead: number
  }
  weatherDay?: { ref: string; classified: boolean; good: boolean; maxBf: number; maxDouglas: number; limitBf: number; limitDouglas: number }
  forecast?: {
    ref: string
    seq: number
    run: string
    masterEta: string
    forecastEta: string
    differenceH: number
    limitH: number
    atRisk: boolean
    meanSpeedKn: number
    plannedKn: number
    distanceNm: number
    hoursOver10: number
    maxBf: number
    hoursBeyondModel: number
    hoursWithoutForecast: number
    model: string
    samples: FcSample[]
    grid?: GridRef
  }
  note?: { ref: string; seq: number; text: string; agent?: string }
  alert?: { ref: string; seq: number; text: string; agent?: string }
  /** A sender the guardrail did not know, and the contact it imitates, if any. */
  screen?: { seq: number; ref: string; from: string; domain: string; diffAt?: number; lookalike?: { name: string; domain: string; phone?: string } }
  /** What the Security Agent reported, each quote checked against the e-mail. */
  findings?: { seq: number; ref: string; items: Finding[]; refused: number }
  blocked?: { seq: number; reasons: Reason[] }
  /** The guardrail's alert on a held message: reasons and what to do. */
  fraud?: { seq: number; action: string; reasons: Reason[] }
  /** A decision the Ops Agent put to the operator, and the answer once given. */
  question?: { ref: string; seq: number; text: string; proposal: string; answer?: { seq: number; yes: boolean; text: string } }
  /** The simulated operator's edit of the current draft. */
  edit?: { seq: number; before: string; after: string; note: string }
  past: PastDay[]
  queue: QueueItem[]
  /** Every e-mail so far, newest last. */
  inbox: Email[]
}

const gridRef = (v: unknown): GridRef | undefined => {
  const g = obj(v)
  return str(g.file) ? { file: str(g.file)!, sha256: str(g.sha256) ?? '' } : undefined
}

function wxSample(v: unknown): WxSample {
  const m = obj(v)
  return {
    time: str(m.time) ?? '', pos: [num(m.lon) ?? 0, num(m.lat) ?? 0], headingDeg: num(m.heading_deg) ?? 0,
    windFromDeg: num(m.wind_from_deg) ?? 0, windKn: num(m.wind_kn) ?? 0, beaufort: num(m.beaufort) ?? 0,
    windRelDeg: num(m.wind_relative_deg) ?? 0, windSector: str(m.wind_sector) ?? '',
    swhM: num(m.swh_m), douglas: num(m.douglas), wavesRelDeg: num(m.waves_relative_deg), seaSector: str(m.sea_sector),
  }
}

function fcSample(v: unknown): FcSample {
  const m = obj(v)
  return {
    time: str(m.time) ?? '', pos: [num(m.lon) ?? 0, num(m.lat) ?? 0], beaufort: num(m.beaufort) ?? 0, windRelDeg: num(m.wind_relative_deg) ?? 0,
    sector: str(m.kwon_sector) ?? '', lossPct: num(m.speed_loss_pct) ?? 0, speedKn: num(m.speed_kn) ?? 0, forecast: m.forecast === true,
  }
}

export function deskState(events: RunEvent[], seq: number): DeskState {
  const s: DeskState = { gate: 'none', day: -1, dayStarts: [], past: [], queue: [], inbox: [], ahead: [], notes: [] }
  // The ship each add_voyage_note call was for: its result does not say.
  const noteShip = new Map<string, { shipId?: string; agent?: string }>()
  const queued = (approvalRef: string | undefined) => s.queue.find((q) => q.approvalRef === approvalRef)
  for (const e of events) {
    if (e.seq > seq) break
    const d = e.detail
    const first = e.source_refs[0] ?? ''
    const tool = str(d.tool)
    switch (e.type) {
      case 'message_received': {
        // A new e-mail starts a new episode: what the desk worked out for the
        // previous one is kept only as a past day.
        if (s.report) s.past.push({ time: s.report.time, weather: s.weather, consumption: s.consumption, weatherDay: s.weatherDay })
        for (const k of ['report', 'route', 'eta', 'notices', 'recipientsOk', 'unsupported', 'draft', 'decidedAt', 'filed', 'consumption', 'weather', 'weatherDay', 'forecast', 'note', 'alert', 'question', 'screen', 'findings', 'blocked', 'fraud', 'edit', 'senderKnown', 'voyage', 'cp', 'contacts', 'trigger', 'ais', 'vms', 'advice', 'speedPlan', 'reportChecks', 'stoppage', 'offHire', 'entry', 'latest', 'focus'] as const) delete s[k]
        s.ahead = []
        s.notes = []
        s.gate = 'none'
        s.day++
        s.dayStarts.push(e.seq)
        const m = obj(d.email)
        s.email = { id: str(m.id) ?? '', from: str(m.from) ?? '', time: str(m.time) ?? '', subject: str(m.subject) ?? '', body: str(m.body) ?? '', ref: `email:${str(m.id)}` }
        s.inbox.push(s.email)
        break
      }
      case 'forecast_available': {
        const f = obj(d.forecast)
        s.day++
        s.dayStarts.push(e.seq)
        s.trigger = { seq: e.seq, ref: first, model: str(f.model) ?? '', run: str(f.run) ?? '', available: str(f.available) ?? e.sim_time, ships: (d.ships as string[] | undefined) ?? [], notScanned: (f.not_scanned as string[] | undefined) ?? [], grid: gridRef(d.grid_ref) }
        break
      }
      case 'check':
        if (e.actor === 'guardrail' && 'sender' in d) {
          s.senderKnown = d.passed === true
          const m = obj(d.sender)
          const l = obj(m.lookalike_of)
          if (!s.senderKnown) s.screen = { seq: e.seq, ref: first, from: str(m.from) ?? '', domain: str(m.domain) ?? '', diffAt: num(m.diff_at), lookalike: str(l.domain) ? { name: str(l.name) ?? '', domain: str(l.domain)!, phone: str(l.phone) } : undefined }
        } else if (first.startsWith('tool:report_findings')) {
          const items = ((d.findings as unknown[] | undefined) ?? []).map((x) => { const m = obj(x); return { kind: str(m.kind) ?? '', quote: str(m.quote) ?? '', why: str(m.why) ?? '' } })
          const refused = (s.findings?.refused ?? 0) + (d.problems ? 1 : 0)
          s.findings = d.problems ? { seq: e.seq, ref: first, items: s.findings?.items ?? [], refused } : { seq: e.seq, ref: first, items, refused }
        }
        else if (e.actor === 'guardrail' && 'recipients' in d) s.recipientsOk = obj(d.recipients).ok === true
        else if (e.actor === 'checker' && 'unsupported' in d) s.unsupported = (d.unsupported as string[] | null) ?? []
        else if (first.startsWith('tool:check_eta')) {
          const r = obj(d.result)
          s.eta = { ref: first, consistent: r.consistent === true, computedEta: str(r.computed_eta) ?? '', differenceH: num(r.difference_hours) ?? 0, toleranceH: num(r.tolerance_hours) ?? 0, hoursToGo: num(r.hours_to_go) ?? 0 }
        } else if (first.startsWith('tool:check_notices')) {
          const r = obj(d.result)
          s.notices = { ref: first, due: r.due === true, daysToEta: num(r.days_to_eta) ?? 0, thresholdDays: num(r.threshold_due_days), movedH: num(r.eta_moved_hours) ?? 0, limitH: num(r.change_limit_hours) ?? 0, lastNoticeEta: str(r.last_notice_eta) }
        } else if (first.startsWith('tool:check_consumption')) {
          const r = obj(d.result)
          s.consumption = { ref: first, reportedT: num(r.reported_vlsfo_t) ?? 0, warrantedT: num(r.warranted_vlsfo_t) ?? 0, excessPct: num(r.excess_pct) ?? 0, tolerancePct: num(r.tolerance_pct) ?? 0, flagged: r.flagged === true }
        } else if (first.startsWith('tool:classify_weather_day')) {
          const r = obj(d.result)
          s.weatherDay = { ref: first, classified: r.classified === true, good: r.good_weather_day === true, maxBf: num(r.max_beaufort) ?? 0, maxDouglas: num(r.max_douglas) ?? 0, limitBf: num(r.limit_beaufort) ?? 0, limitDouglas: num(r.limit_douglas) ?? 0 }
        } else if (tool === 'report_checks') {
          const flags = ((d.flags as unknown[] | undefined) ?? []).map((x) => { const m = obj(x); return { id: str(m.id) ?? '', finding: str(m.finding) ?? '' } })
          s.reportChecks = { ref: first, seq: e.seq, passed: d.passed === true, flags }
        } else if (first.startsWith('tool:weather_ahead')) {
          const r = obj(d.result)
          const a: AheadShip = {
            ref: first, seq: e.seq, shipId: str(r.ship_id) ?? '', ship: str(r.ship) ?? '', to: str(r.to) ?? '', speedKn: num(r.speed_kn) ?? 0, distanceNm: num(r.distance_to_go_nm) ?? 0,
            path: ((d.path as LonLat[] | undefined) ?? []).map((p) => [p[0], p[1]] as LonLat), samples: ((d.samples as unknown[] | undefined) ?? []).map(wxSample),
            stretches: ((r.heavy_weather as unknown[] | undefined) ?? []).map((x) => {
              const m = obj(x)
              return { from: str(m.from) ?? '', to: str(m.to) ?? '', maxBf: num(m.max_beaufort) ?? 0, maxSwhM: num(m.max_swh_m) ?? 0, maxWindKn: num(m.max_wind_kn) ?? 0, windSector: str(m.wind_sector) ?? '', seaSector: str(m.sea_sector) ?? '', startsInH: num(m.starts_in_hours) ?? 0 }
            }),
            maxBf: num(r.max_beaufort) ?? 0, maxSwhM: num(r.max_swh_m) ?? 0, until: str(r.until) ?? '', arrivesWithin: r.arrives_within_forecast === true, grid: gridRef(d.grid_ref),
          }
          s.ahead = [...s.ahead.filter((x) => x.shipId !== a.shipId), a]
        } else if (first.startsWith('tool:check_speed_plan')) {
          const r = obj(d.result)
          s.speedPlan = {
            ref: first, seq: e.seq, consistent: r.consistent === true, distanceNm: num(r.distance_nm) ?? 0, adviceDistanceNm: num(r.advice_distance_nm) ?? 0, hoursToGo: num(r.hours_to_go) ?? 0,
            arrival: str(r.computed_arrival_utc) ?? '', adviceArrival: str(r.advice_arrival_utc) ?? '', diffH: num(r.arrival_diff_hours) ?? 0, beforeBerth: r.arrives_before_berth === true, berth: str(r.berth_utc) ?? '',
          }
        } else if (first.startsWith('tool:check_stoppage')) {
          const r = obj(d.result)
          s.stoppage = {
            ref: first, seq: e.seq, consistent: r.consistent === true, from: str(r.from_utc) ?? '', to: str(r.to_utc) ?? '', stopH: num(r.stop_hours) ?? 0, steamingH: num(r.steaming_hours) ?? 0,
            speedKn: num(r.avg_speed_kn) ?? 0, expectedNm: num(r.expected_distance_nm) ?? 0, runNm: num(r.distance_run_nm) ?? 0, diffPct: num(r.distance_diff_pct) ?? 0, withinDay: r.within_report_day === true,
            fuel: obj(d.fuel_during_stop) as Record<string, number>,
          }
        } else if (first.startsWith('tool:check_forecast_eta')) {
          const r = obj(d.result)
          s.forecast = {
            ref: first, seq: e.seq, run: str(r.forecast_run) ?? '', masterEta: str(r.master_eta) ?? '', forecastEta: str(r.forecast_eta) ?? '',
            differenceH: num(r.difference_hours) ?? 0, limitH: num(r.limit_hours) ?? 0, atRisk: r.at_risk === true, meanSpeedKn: num(r.mean_speed_kn) ?? 0,
            plannedKn: num(r.planned_speed_kn) ?? 0, distanceNm: num(r.distance_to_go_nm) ?? 0, hoursOver10: num(r.hours_speed_loss_over_10pct) ?? 0,
            maxBf: num(r.max_beaufort) ?? 0, hoursBeyondModel: num(r.hours_wind_above_model_range) ?? 0, hoursWithoutForecast: num(r.hours_without_forecast) ?? 0,
            model: str(r.model) ?? '', samples: ((d.samples as unknown[] | undefined) ?? []).map(fcSample), grid: gridRef(d.grid_ref),
          }
        }
        break
      case 'extracted': {
        const r = obj(d.report)
        s.report = {
          ref: first, shipName: str(r.ship_name) ?? '', position: str(r.position) ?? '', lat: num(r.lat) ?? 0, lon: num(r.lon) ?? 0, time: str(r.report_time_utc) ?? '',
          distanceRunNm: num(r.distance_run_nm), speedKn: num(r.avg_speed_kn), robVlsfoT: num(r.rob_vlsfo_t), robMgoT: num(r.rob_mgo_t),
          nextPort: str(r.next_port) ?? '', masterEta: str(r.master_eta_utc) ?? '',
          consVlsfoT: num(r.cons_vlsfo_t), etaBasisKn: num(r.eta_basis_kn), remarks: str(r.remarks),
        }
        break
      }
      case 'tool_result': {
        const out = obj(obj(d.result).output)
        if (tool === 'voyage_history') {
          const track = ((out.previous_noon_reports as Obj[] | undefined) ?? []).map((p) => ({ pos: [num(p.lon) ?? 0, num(p.lat) ?? 0] as LonLat, time: str(p.time) ?? '', remarks: str(p.remarks) }))
          s.voyage = { ref: first, ship: str(out.ship) ?? '', from: str(out.from) ?? '', to: str(out.to) ?? '', cargo: str(out.cargo) ?? '', track }
        } else if (tool === 'distance_to_go') {
          const path = ((d.path as LonLat[] | undefined) ?? []).map((p) => [p[0], p[1]] as LonLat)
          s.route = { ref: first, distanceNm: num(out.distance_nm) ?? 0, passages: (out.passages as string[] | undefined) ?? [], path }
        } else if (tool === 'cp_terms') {
          s.cp = { ref: first, changeHours: num(out.eta_change_hours) ?? 0, noticeDays: (out.eta_notice_days as number[] | undefined) ?? [] }
        } else if (tool === 'contacts') {
          s.contacts = { ref: first, count: ((out.contacts as unknown[] | undefined) ?? []).length }
        } else if (tool === 'weather_on_track') {
          const m = obj(out.summary)
          s.weather = {
            ref: first, seq: e.seq, samples: ((d.samples as unknown[] | undefined) ?? []).map(wxSample), grid: gridRef(d.grid_ref),
            minBf: num(m.min_beaufort) ?? 0, maxBf: num(m.max_beaufort) ?? 0, maxWindKn: num(m.max_wind_kn) ?? 0,
            minDouglas: num(m.min_douglas) ?? 0, maxDouglas: num(m.max_douglas) ?? 0, minSwhM: num(m.min_swh_m) ?? 0, maxSwhM: num(m.max_swh_m) ?? 0,
            hoursBf7Ahead: num(m.hours_wind_bf7_or_more_ahead) ?? 0, hoursSea5Ahead: num(m.hours_sea_douglas5_or_more_ahead) ?? 0,
          }
        } else if (tool === 'add_voyage_note') {
          const by = noteShip.get(first)
          s.note = { ref: first, seq: e.seq, text: str(d.note) ?? '', agent: by?.agent }
          s.notes.push({ ref: first, seq: e.seq, text: str(d.note) ?? '', agent: by?.agent, shipId: str(d.ship_id) ?? by?.shipId })
        } else if (tool === 'ais_position') {
          s.ais = { ref: first, lat: num(out.lat) ?? 0, lon: num(out.lon) ?? 0, time: str(out.time) ?? '', position: `${fmtLat(num(out.lat) ?? 0)} ${fmtLon(num(out.lon) ?? 0)}` }
        } else if (tool === 'voyage_system') {
          const h = obj(out.hire)
          s.vms = {
            ref: first, ship: str(out.ship) ?? '',
            portCalls: ((out.port_calls as unknown[] | undefined) ?? []).map((x) => { const m = obj(x); return { port: str(m.port) ?? '', purpose: str(m.purpose) ?? '', eta: str(m.eta) ?? '', berth: str(m.berth) } }),
            owner: str(h.owner) ?? '', rateUsdPerDay: num(h.rate_usd_per_day) ?? 0,
            offHire: ((h.off_hire as unknown[] | undefined) ?? []).map((x) => { const m = obj(x); return { from: str(m.from) ?? '', to: str(m.to) ?? '', reason: str(m.reason) ?? '' } }),
            bunkerPrices: obj(out.bunker_prices_usd_per_t) as Record<string, number>,
          }
        } else if (tool === 'speed_advice') {
          s.advice = {
            ref: first, seq: e.seq, fromTime: str(out.from_time) ?? '', speedKn: num(out.advised_speed_kn) ?? 0, baseKn: num(out.base_speed_kn) ?? 0, currentKn: num(out.current_speed_kn) ?? 0,
            arrival: str(out.advised_arrival_utc) ?? '', berth: str(out.berth_utc) ?? '', currentEta: str(out.current_eta_utc) ?? '', pilotH: num(out.pilot_allowance_hours) ?? 0,
            anchorH: num(out.anchor_hours_saved) ?? 0, vlsfoSavedT: num(out.vlsfo_saved_t) ?? 0, distanceNm: num(out.distance_nm) ?? 0, note: str(out.note) ?? '', service: str(out.service) ?? '',
          }
        } else if (tool === 'off_hire_statement') {
          s.offHire = {
            ref: first, seq: e.seq, from: str(out.from_utc) ?? '', to: str(out.to_utc) ?? '', hours: num(out.hours) ?? 0, rateUsdPerDay: num(out.hire_rate_usd_per_day) ?? 0, hireUsd: num(out.hire_deducted_usd) ?? 0,
            bunkers: ((out.bunkers_owners_account as unknown[] | undefined) ?? []).map((x) => { const m = obj(x); return { grade: str(m.grade) ?? '', tonnes: num(m.tonnes) ?? 0, usdPerT: num(m.usd_per_t) ?? 0, amountUsd: num(m.amount_usd) ?? 0 } }),
            bunkersUsd: num(out.bunkers_usd) ?? 0, totalUsd: num(out.total_owners_account_usd) ?? 0, owner: str(out.owner) ?? '', note: str(out.note) ?? '',
          }
        } else if (first.startsWith('tool:draft_email') || tool === 'approval_gate') {
          s.filed = str(d.status)
          const q = queued(first)
          if (q) { q.filed = str(d.status); s.focus = q.ref }
          if (q && s.gate !== 'rejected') q.status = s.gate
        }
        break
      }
      case 'tool_call':
        if (tool === 'add_voyage_note') noteShip.set(first, { shipId: str(d.ship_id), agent: e.agent })
        break
      case 'blocked':
        s.blocked = { seq: e.seq, reasons: reasons(d.reasons) }
        break
      case 'alert':
        if (e.actor === 'guardrail') s.fraud = { seq: e.seq, action: str(d.action) ?? '', reasons: reasons(d.reasons) }
        else if (first.startsWith('tool:alert_operator')) s.alert = { ref: first, seq: e.seq, text: str(d.text) ?? '', agent: e.agent }
        break
      case 'question': {
        const q = obj(d.question)
        s.question = { ref: first, seq: e.seq, text: str(q.question) ?? '', proposal: str(q.proposal) ?? '' }
        break
      }
      case 'answer':
        if (s.question && s.question.ref === first) s.question.answer = { seq: e.seq, yes: d.yes === true, text: str(d.text) ?? '' }
        break
      case 'draft': {
        const m = obj(d.draft)
        s.draft = { ref: first, seq: e.seq, to: (m.to as string[] | undefined) ?? [], subject: str(m.subject) ?? '', body: str(m.body) ?? '' }
        s.queue.push({ ref: first, approvalRef: `approval:${str(m.call_id)}`, seq: e.seq, day: s.day, kind: 'email', to: s.draft.to, subject: s.draft.subject, status: 'none', draft: s.draft })
        s.focus = first
        s.latest = 'email'
        break
      }
      case 'entry': {
        const m = obj(d.draft)
        const en = obj(m.entry)
        const approvalRef = `approval:${str(m.call_id)}`
        s.entry = { ref: first, seq: e.seq, approvalRef, kind: str(en.kind) ?? '', port: str(en.port), eta: str(en.eta), from: str(en.from), to: str(en.to), reason: str(en.reason) ?? str(m.purpose) ?? '', agent: e.agent }
        s.queue.push({ ref: first, approvalRef, seq: e.seq, day: s.day, kind: 'entry', to: [], subject: entryTitle(s.entry), status: 'none', entry: s.entry })
        s.focus = first
        s.latest = 'entry'
        break
      }
      case 'approval_requested':
        s.gate = 'waiting'
        if (queued(first)) { queued(first)!.status = 'waiting'; s.focus = queued(first)!.ref }
        break
      case 'approved':
      case 'edited':
      case 'rejected':
        // An edit is followed by an approval; the edit is what the page shows.
        if (!(e.type === 'approved' && s.gate === 'edited')) s.gate = e.type
        s.decidedAt = e.sim_time
        if (e.type === 'edited') s.edit = { seq: e.seq, before: str(d.before) ?? '', after: str(d.after) ?? '', note: e.summary.replace(/^.*?:\s*/, '') }
        if (queued(first)) { queued(first)!.status = s.gate; s.focus = queued(first)!.ref }
        break
    }
  }
  return s
}

// ---- Labels -------------------------------------------------------------

const ACTOR: Record<Actor, string> = {
  sim: 'Inbox',
  guardrail: 'Guardrail',
  agent: 'Agent (AI)',
  subagent: 'Sub-agent (AI)',
  tool: 'Tool',
  checker: 'Checker (code)',
  human_sim: 'Operator (simulated)',
  external_sim: 'Outside party (simulated)',
}

/** Display names of the agents, keyed by the log's `agent` field. */
export const AGENT_TITLE: Record<string, string> = {
  ops_agent: 'Ops Agent',
  reports_agent: 'Reports Agent',
  voyage_agent: 'Voyage Agent',
  weather_agent: 'Weather Agent',
  performance_agent: 'Performance Agent',
  correspondence_agent: 'Correspondence Agent',
  security_agent: 'Security Agent',
}

export function actorLabel(e: RunEvent): string {
  const title = e.agent ? AGENT_TITLE[e.agent] : undefined
  if (e.type === 'rationale') return title ? `${title}'s stated reason` : "Agent's stated reason"
  if (title) return `${title} (AI)`
  if (e.actor === 'checker' && ('unsupported' in e.detail)) return 'Number guard (code)'
  return ACTOR[e.actor] ?? e.actor
}

/** The agent's stated reason, without the log's prefix. */
export function reasonText(e: RunEvent): string {
  return str(e.detail.reason) ?? e.summary.replace(/^[\w ]+'s stated reason:\s*/, '')
}

/** What the timeline shows for an event: the reason for rationale events, the summary otherwise. */
export function eventText(e: RunEvent): string {
  return e.type === 'rationale' ? reasonText(e) : e.summary
}

// ---- Formatting ---------------------------------------------------------

const MONTHS = ['Jan', 'Feb', 'Mar', 'Apr', 'May', 'Jun', 'Jul', 'Aug', 'Sep', 'Oct', 'Nov', 'Dec']
const pad = (n: number) => String(n).padStart(2, '0')

/** "25 Jan 2100 UTC" */
export function fmtEta(iso: string): string {
  const t = new Date(iso)
  if (Number.isNaN(t.getTime())) return iso
  return `${t.getUTCDate()} ${MONTHS[t.getUTCMonth()]} ${pad(t.getUTCHours())}${pad(t.getUTCMinutes())} UTC`
}

/** "22 Jan 2025 12:05 UTC" */
export function fmtSimTime(iso: string): string {
  const t = new Date(iso)
  if (Number.isNaN(t.getTime())) return iso
  return `${t.getUTCDate()} ${MONTHS[t.getUTCMonth()]} ${t.getUTCFullYear()} ${pad(t.getUTCHours())}:${pad(t.getUTCMinutes())} UTC`
}

/** "22 Jan" */
export function fmtDay(iso: string): string {
  const t = new Date(iso)
  return Number.isNaN(t.getTime()) ? iso : `${t.getUTCDate()} ${MONTHS[t.getUTCMonth()]}`
}

/** "1,009.1" with one decimal. */
export function fmt1(n: number): string {
  return n.toLocaleString('en-GB', { minimumFractionDigits: 1, maximumFractionDigits: 1 })
}

/** Signed hours with a true minus sign: "−0.4 h". */
export function fmtSignedH(n: number): string {
  return `${n < 0 ? '−' : n > 0 ? '+' : ''}${fmt1(Math.abs(n))} h`
}

/** A position the way the Master writes it: "44-14.1N", "009-25.5W" (sim.FormatLat/FormatLon). */
function fmtDM(v: number, width: number, pos: string, neg: string): string {
  const tenths = Math.round(Math.abs(v) * 600)
  return `${String(Math.floor(tenths / 600)).padStart(width, '0')}-${((tenths % 600) / 10).toFixed(1).padStart(4, '0')}${v < 0 ? neg : pos}`
}
export const fmtLat = (lat: number) => fmtDM(lat, 2, 'N', 'S')
export const fmtLon = (lon: number) => fmtDM(lon, 3, 'E', 'W')

/** "USD 2,177.08" */
export function fmtUsd(n: number): string {
  return `USD ${n.toLocaleString('en-GB', { minimumFractionDigits: 2, maximumFractionDigits: 2 })}`
}

/** "26 Jan 0610" */
export function fmtDayTime(iso: string): string {
  return fmtEta(iso).replace(' UTC', '')
}

/** What a proposed entry changes, in one line. */
export function entryTitle(e: Pick<Entry, 'kind' | 'port' | 'eta' | 'from' | 'to'>): string {
  if (e.kind === 'port_call_eta') return `Port call ETA ${e.port ?? ''}: ${e.eta ? fmtEta(e.eta) : ''}`
  if (e.kind === 'off_hire') return `Off hire ${e.from ? fmtDayTime(e.from) : ''} to ${e.to ? fmtDayTime(e.to) : ''} UTC`
  return 'Entry for the voyage management system'
}

/** "Dover", "Kiel Canal" style passage names. */
export function passageName(tag: string): string {
  return tag.replace(/_/g, ' ').replace(/\b\w/g, (c) => c.toUpperCase())
}

export function nodeLabelForTool(tool: string): string {
  return TOOL_INFO[tool]?.name ?? tool
}

// ---- Page copy per source ----------------------------------------------
// Plain words for managers: who or what produced a source. Written for the
// page, keyed by tool; every value shown beside it comes from the log.

export const TOOL_INFO: Record<string, { name: string; by: string }> = {
  inbox: { name: 'Noon report e-mail', by: 'The ship. Written and sent by the Master.' },
  inbox_mail: { name: 'Incoming e-mail', by: 'Whoever sent it. Untrusted until the guardrail has checked the sender.' },
  read_email: { name: 'Read e-mail', by: 'Reports Agent (AI) opening the e-mail through the mail connector (a stand-in).' },
  submit_extraction: { name: 'Report extraction', by: "Reports Agent (AI) copying the Master's e-mail into fields. Code checked them against a schema." },
  check_consumption: { name: 'Consumption check', by: 'Checker: fixed rules in code, not the AI.' },
  classify_weather_day: { name: 'Good weather check', by: 'Checker: applies the CP good weather definition in code, not the AI.' },
  check_forecast_eta: { name: 'Forecast ETA check', by: 'Checker: sails the remaining route through the forecast in code, with a simplified speed-loss model (Kwon 2008). Not the AI.' },
  weather_on_track: { name: 'Weather on track', by: 'Weather connector: samples the ECMWF forecast file issued at the time where the ship was, in code.' },
  add_voyage_note: { name: 'Voyage-file note', by: 'An agent wrote it; the number guard checked it. Not an e-mail.' },
  alert_operator: { name: 'Operator alert', by: 'An agent wrote it; the number guard checked it. Stays on the desk.' },
  report_findings: { name: "Security Agent's findings", by: 'Security Agent (AI), reading the held e-mail alone. Code checked every quote against the e-mail.' },
  security_agent: { name: 'Security Agent', by: 'Security Agent (AI). It cannot act: no tool that sends, pays or changes anything.' },
  ask_operator: { name: 'Question to the operator', by: 'Ops Agent (AI) asking the operator to decide; the number guard checked it.' },
  approval_gate: { name: 'Approval gate', by: 'Plain code: drafts wait here for the operator.' },
  ops_agent: { name: 'Ops Agent', by: 'Ops Agent (AI), handing work to a colleague.' },
  reports_agent: { name: 'Reports Agent', by: 'Reports Agent (AI).' },
  voyage_agent: { name: 'Voyage Agent', by: 'Voyage Agent (AI).' },
  performance_agent: { name: 'Performance Agent', by: 'Performance Agent (AI).' },
  correspondence_agent: { name: 'Correspondence Agent', by: 'Correspondence Agent (AI).' },
  voyage_history: { name: 'Voyage history', by: 'Voyage file (a stand-in for the company system).' },
  cp_terms: { name: 'CP terms', by: 'Charter party summary. The terms are fictional.' },
  contacts: { name: 'Contacts', by: 'Verified contact list (a stand-in for the address book).' },
  distance_to_go: { name: 'Distance to go', by: 'seaRoute: shortest path over the Eurostat MARNET sea network, computed by code.' },
  check_eta: { name: 'ETA check', by: 'Checker: fixed rules in code, not the AI.' },
  check_notices: { name: 'Notice check', by: 'Checker: fixed rules in code, not the AI.' },
  draft_email: { name: 'Draft e-mail', by: 'The AI wrote the text. The guardrail and the number guard checked it.' },
  forecast: { name: 'New forecast run', by: 'ECMWF, as issued: the run the desk had when it came out. No e-mail started this.' },
  weather_ahead: { name: 'Weather ahead', by: "Weather connector: sails the ship's remaining route through the new ECMWF forecast, in code. Not the AI." },
  weather_agent: { name: 'Weather Agent', by: 'Weather Agent (AI).' },
  voyage_system: { name: 'Voyage management system', by: 'Voyage management system (a stand-in for the company system). Read only.' },
  propose_entry: { name: 'Proposed entry', by: 'An agent proposed it; the number guard checked it. It waits for the operator and is never posted.' },
  ais_position: { name: 'AIS position', by: 'AIS feed (a stand-in: a simulated track for this fictional ship).' },
  speed_advice: { name: 'Speed advice', by: "Voyage optimisation service, a stand-in. Its figures are made up for this simulation; in practice this advice comes from the ship's voyage optimisation and routing service." },
  check_speed_plan: { name: 'Speed plan check', by: "Checker: divides seaRoute's distance by the advised speed in code, and compares the arrival with the advice and the berth. Not the AI." },
  report_checks: { name: 'Report checks', by: 'Checker: fuel remaining, speed against distance run and position, in code, as the report is extracted.' },
  check_stoppage: { name: 'Stoppage check', by: "Checker: checks the Master's stop times, fuel and distance against each other in code. Not the AI." },
  off_hire_statement: { name: 'Hire statement preview', by: "Voyage management system's hire statement, a stand-in. Its figures are made up for this simulation; a real system would give its own." },
}

// ---- Clickable numbers --------------------------------------------------

export interface NumberSource {
  label: string
  value: string
  ref: string
  field: string
  plain: string
}

/** Numbers in a draft that can be linked to a source: [number key, value]. */
export function draftNumbers(s: DeskState): [string, number][] {
  const out: [string, number][] = []
  if (s.notices?.thresholdDays) out.push(['threshold', s.notices.thresholdDays])
  if (s.cp) out.push(['limit', s.cp.changeHours])
  if (s.notices) out.push(['moved', s.notices.movedH], ['days', Math.round(s.notices.daysToEta * 10) / 10])
  if (s.route) out.push(['dist', s.route.distanceNm])
  if (s.report?.speedKn !== undefined) out.push(['speed', s.report.speedKn])
  if (s.eta) out.push(['diff', Math.abs(s.eta.differenceH)])
  if (s.consumption) out.push(['fuel', s.consumption.reportedT], ['warranted', s.consumption.warrantedT], ['excess', Math.abs(s.consumption.excessPct)])
  if (s.forecast) out.push(['fcDiff', Math.abs(s.forecast.differenceH)])
  if (s.advice) out.push(['adviceKn', s.advice.speedKn], ['anchorH', s.advice.anchorH], ['fuelSaved', s.advice.vlsfoSavedT], ['baseKn', s.advice.baseKn], ['pilotH', s.advice.pilotH])
  if (s.stoppage) out.push(['stopH', s.stoppage.stopH], ['steamH', s.stoppage.steamingH])
  if (s.offHire) {
    out.push(['hireUsd', s.offHire.hireUsd], ['rate', s.offHire.rateUsdPerDay], ['totalUsd', s.offHire.totalUsd], ['bunkerUsd', s.offHire.bunkersUsd])
    s.offHire.bunkers.forEach((b, i) => out.push([`bunkerT${i}`, b.tonnes], [`bunkerP${i}`, b.usdPerT]))
  }
  s.ahead.forEach((a, i) => a.stretches.forEach((x, j) => out.push([`ah${i}s${j}bf`, x.maxBf], [`ah${i}s${j}m`, x.maxSwhM])))
  return out
}

/** Times in a draft that can be linked to a source: [number key, ISO time]. */
export function draftTimes(s: DeskState): [string, string][] {
  const out: [string, string][] = []
  if (s.advice) out.push(['adviceArr', s.advice.arrival], ['berth', s.advice.berth], ['vmsEtaOld', s.advice.currentEta])
  if (s.stoppage) out.push(['stopFrom', s.stoppage.from], ['stopTo', s.stoppage.to])
  s.ahead.forEach((a, i) => a.stretches.forEach((x, j) => out.push([`ah${i}s${j}from`, x.from], [`ah${i}s${j}to`, x.to])))
  return out
}

/** "2025-01-25 2100 UTC", the way a draft usually writes an ETA. */
export function isoToDraftTime(iso: string): string {
  const t = new Date(iso)
  if (Number.isNaN(t.getTime())) return iso
  return `${t.getUTCFullYear()}-${pad(t.getUTCMonth() + 1)}-${pad(t.getUTCDate())} ${pad(t.getUTCHours())}${pad(t.getUTCMinutes())} UTC`
}

/** Every number the Desk view can show, with the source behind it. */
export function numberSources(s: DeskState): Record<string, NumberSource> {
  const out: Record<string, NumberSource> = {}
  const r = s.report
  if (r) {
    const fromMaster = 'Reported by the Master in the noon report. The Reports Agent copied it, and code checked the format.'
    out.pos = { label: 'Reported position', value: r.position, ref: r.ref, field: 'position', plain: 'The Master reported this position. The Reports Agent copied it from the e-mail, and code checked the format. The AI did not work it out.' }
    if (r.speedKn !== undefined) out.speed = { label: 'Average speed, last 24 h', value: `${fmt1(r.speedKn)} kn`, ref: r.ref, field: `avg_speed_kn = ${r.speedKn}`, plain: fromMaster }
    if (r.distanceRunNm !== undefined) out.run = { label: 'Distance run, last 24 h', value: `${fmt1(r.distanceRunNm)} nm`, ref: r.ref, field: `distance_run_nm = ${r.distanceRunNm}`, plain: fromMaster }
    if (r.consVlsfoT !== undefined) out.fuelReported = { label: 'VLSFO used, last 24 h', value: `${fmt1(r.consVlsfoT)} t`, ref: r.ref, field: `cons_vlsfo_t = ${r.consVlsfoT}`, plain: fromMaster }
    if (r.robVlsfoT !== undefined) out.vlsfo = { label: 'Remaining on board, VLSFO', value: `${fmt1(r.robVlsfoT)} t`, ref: r.ref, field: `rob_vlsfo_t = ${r.robVlsfoT}`, plain: fromMaster }
    if (r.robMgoT !== undefined) out.mgo = { label: 'Remaining on board, MGO', value: `${fmt1(r.robMgoT)} t`, ref: r.ref, field: `rob_mgo_t = ${r.robMgoT}`, plain: fromMaster }
    out.eta = { label: `Master's ETA ${r.nextPort}`, value: fmtEta(r.masterEta), ref: r.ref, field: 'master_eta_utc', plain: "The Master set this ETA. The agent did not calculate it. Code checked that it fits the distance and speed." }
  }
  if (s.route) {
    const via = s.route.passages.length ? `, via ${s.route.passages.map(passageName).join(' and ')}` : ''
    out.dist = { label: 'Distance to go', value: `${fmt1(s.route.distanceNm)} nm`, ref: s.route.ref, field: `distance_nm = ${s.route.distanceNm}`, plain: `seaRoute computed this over a real sea-lane network${via}. The AI asked for it; it did not calculate it.` }
  }
  if (s.eta) {
    out.compEta = { label: 'ETA from distance and speed', value: fmtEta(s.eta.computedEta), ref: s.eta.ref, field: 'computed_eta', plain: `The Checker divided distance to go by reported speed in code (${fmt1(s.eta.hoursToGo)} h of steaming) and compared the result with the Master's ETA.` }
    out.diff = { label: "Difference from the Master's ETA", value: fmtSignedH(s.eta.differenceH), ref: s.eta.ref, field: `difference_hours = ${s.eta.differenceH}`, plain: `The tolerance is ±${s.eta.toleranceH} h. ${s.eta.consistent ? 'Within it, so the ETA counts as consistent.' : 'Outside it, so the ETA needs a question to the ship.'}` }
  }
  if (s.notices) {
    out.days = { label: 'Days to ETA', value: `${fmt1(s.notices.daysToEta)} days`, ref: s.notices.ref, field: `days_to_eta = ${s.notices.daysToEta}`, plain: s.notices.thresholdDays ? `The ${s.notices.thresholdDays}-day point passes before the next noon report, so the CP ${s.notices.thresholdDays}-day notice is due now.` : 'No notice point passes before the next noon report.' }
    out.moved = { label: 'ETA change since the last notice', value: `${s.notices.movedH} h`, ref: s.notices.ref, field: `eta_moved_hours = ${s.notices.movedH}`, plain: `The last notice said ${s.notices.lastNoticeEta ? fmtEta(s.notices.lastNoticeEta) : 'an earlier ETA'}. The CP asks for a new notice when the ETA moves more than ${s.notices.limitH} h.` }
  }
  if (s.notices?.thresholdDays) {
    const n = s.notices.thresholdDays
    out.threshold = { label: 'Notice point due', value: `${n}-day`, ref: s.notices.ref, field: `threshold_due_days = ${n}`, plain: `The CP asks for notices at set days before arrival. The ${n}-day point is the one that passes before the next noon report.` }
  }
  if (s.weather) {
    const w = s.weather
    out.wxHead = { label: 'Hours of high sea from ahead', value: `${w.hoursSea5Ahead} h`, ref: w.ref, field: `summary.hours_sea_douglas5_or_more_ahead = ${w.hoursSea5Ahead}`, plain: 'Counted in code from the 3-hourly samples: hours with sea state 5 or more coming from within 60° of the bow.' }
    out.wxWind = { label: 'Wind force over the day', value: w.minBf === w.maxBf ? `Beaufort ${w.maxBf}` : `Beaufort ${w.minBf}–${w.maxBf}`, ref: w.ref, field: `summary.min_beaufort, max_beaufort`, plain: `From ECMWF's 10 m wind where the ship was, every 3 h. Strongest: ${w.maxWindKn} kn.` }
    out.wxSea = { label: 'Wave height over the day', value: `${fmt1(w.minSwhM)}–${fmt1(w.maxSwhM)} m`, ref: w.ref, field: 'summary.min_swh_m, max_swh_m', plain: `Significant wave height from ECMWF's wave model, Douglas sea state ${w.minDouglas} to ${w.maxDouglas}.` }
    w.samples.forEach((x, i) => {
      out[`wx${i}`] = { label: `Weather at ${fmtEta(x.time)}`, value: `Bf ${x.beaufort}${x.swhM !== undefined ? ` · ${fmt1(x.swhM)} m` : ''}`, ref: w.ref, field: `samples[${i}]`, plain: `${x.windKn} kn wind ${x.windSector} (${x.windRelDeg}° off the bow)${x.swhM !== undefined ? `, ${fmt1(x.swhM)} m sea ${x.seaSector} (Douglas ${x.douglas})` : ''}. Sampled in code from the forecast file where the ship was at that time.` }
    })
  }
  if (s.consumption) {
    const c = s.consumption
    out.fuel = { label: 'VLSFO used, last 24 h', value: `${fmt1(c.reportedT)} t`, ref: s.report?.ref ?? c.ref, field: `cons_vlsfo_t = ${c.reportedT}`, plain: 'Reported by the Master in the noon report. The Reports Agent copied it, and code checked it against the e-mail.' }
    out.excess = { label: 'Over the CP warranty', value: `${c.excessPct > 0 ? '+' : ''}${fmt1(c.excessPct)}%`, ref: c.ref, field: `excess_pct = ${c.excessPct}`, plain: `Worked out in code: (${fmt1(c.reportedT)} − ${fmt1(c.warrantedT)}) ÷ ${fmt1(c.warrantedT)}. The desk looks into anything over +${c.tolerancePct}%.` }
    out.warranted = { label: 'CP warranted consumption', value: `${fmt1(c.warrantedT)} t a day`, ref: c.ref, field: `warranted_vlsfo_t = ${c.warrantedT}`, plain: 'From the fictional charter party terms: VLSFO a day in good weather, in ballast.' }
  }
  if (s.weatherDay) {
    const g = s.weatherDay
    out.gwd = { label: 'Good weather day under the CP', value: g.good ? 'Yes' : 'No: excluded', ref: g.ref, field: `good_weather_day = ${g.good}`, plain: `The Checker compared every sample with the CP limits in code: Beaufort ${g.limitBf} and sea state ${g.limitDouglas}. The day reached Beaufort ${g.maxBf} and sea state ${g.maxDouglas}.` }
  }
  if (s.forecast) {
    const f = s.forecast
    out.fcEta = { label: 'Forecast ETA', value: fmtEta(f.forecastEta), ref: f.ref, field: `forecast_eta = ${f.forecastEta}`, plain: `Code sailed the remaining ${fmt1(f.distanceNm)} nm hour by hour through ECMWF's forecast as issued, at ${fmt1(f.plannedKn)} kn less the speed lost in the forecast wind (Kwon 2008, simplified, for learning). Mean speed ${fmt1(f.meanSpeedKn)} kn.` }
    out.fcDiff = { label: "Forecast ETA against the Master's", value: fmtSignedH(f.differenceH), ref: f.ref, field: `difference_hours = ${f.differenceH}`, plain: `The CP asks for a new notice when the ETA moves more than ${f.limitH} h. The Master owns the ETA, so the operator decides whether to ask the Master to review it. The system sends no notice of its own.` }
  }
  if (s.ais) {
    out.aisPos = { label: 'AIS position', value: s.ais.position, ref: s.ais.ref, field: `lat ${s.ais.lat}, lon ${s.ais.lon}`, plain: `From the AIS feed at ${fmtEta(s.ais.time)}. A simulated track for this fictional ship; no noon report was due.` }
  }
  if (s.vms) {
    const pc = s.vms.portCalls[0]
    if (pc) out.vmsEta = { label: `ETA ${pc.port} in the voyage management system`, value: fmtEta(pc.eta), ref: s.vms.ref, field: `port_calls[0].eta = ${pc.eta}`, plain: "As entered in the voyage management system (a stand-in). It is the Master's last ETA." }
    out.vmsRate = { label: 'Hire rate', value: `${fmtUsd(s.vms.rateUsdPerDay).replace('.00', '')} a day`, ref: s.vms.ref, field: `hire.rate_usd_per_day = ${s.vms.rateUsdPerDay}`, plain: 'As entered in the voyage management system (a stand-in). The rate is fictional, near the market of late January 2025.' }
  }
  if (s.advice) {
    const a = s.advice
    const made = 'Made up for this simulation by a simple stand-in. In practice this figure comes from the voyage optimisation service.'
    out.adviceKn = { label: 'Advised speed', value: `${fmt1(a.speedKn)} kn`, ref: a.ref, field: `advised_speed_kn = ${a.speedKn}`, plain: `One steady speed to reach the pilot station ${a.pilotH} h before the berth. ${made}` }
    out.adviceArr = { label: 'Advised arrival', value: fmtEta(a.arrival), ref: a.ref, field: `advised_arrival_utc = ${a.arrival}`, plain: `The berthing time less ${a.pilotH} h for the pilot. ${made}` }
    out.anchorH = { label: 'Hours at anchor saved', value: `${fmt1(a.anchorH)} h`, ref: a.ref, field: `anchor_hours_saved = ${a.anchorH}`, plain: `At ${fmt1(a.currentKn)} kn the ship would arrive ${fmtEta(a.currentEta)} and wait. ${made}` }
    out.fuelSaved = { label: 'VLSFO saved', value: `${fmt1(a.vlsfoSavedT)} t`, ref: a.ref, field: `vlsfo_saved_t = ${a.vlsfoSavedT}`, plain: `Fuel by the cube law from the last noon reports, main engine only, against ${fmt1(a.baseKn)} kn. ${made}` }
    out.baseKn = { label: 'Base speed', value: `${fmt1(a.baseKn)} kn`, ref: a.ref, field: `base_speed_kn = ${a.baseKn}`, plain: `The speed the fuel saving is counted against. ${made}` }
    out.pilotH = { label: 'Pilot allowance', value: `${a.pilotH} h`, ref: a.ref, field: `pilot_allowance_hours = ${a.pilotH}`, plain: `Time allowed between the pilot station and the berth. ${made}` }
    out.vmsEtaOld = { label: 'ETA before the change', value: fmtEta(a.currentEta), ref: s.vms?.ref ?? a.ref, field: `current_eta_utc = ${a.currentEta}`, plain: 'The ETA in the voyage management system before the berth news.' }
    out.berth = { label: 'Berthing time', value: `about ${fmtEta(a.berth)}`, ref: s.email?.ref ?? a.ref, field: `berth_utc = ${a.berth}`, plain: 'From the Rotterdam agent\'s e-mail: "subject to the vessel ahead completing as planned". The agent copied it; it is an estimate, not a fixed time.' }
  }
  if (s.speedPlan) {
    const p = s.speedPlan
    out.planArr = { label: 'Arrival from distance and speed', value: fmtEta(p.arrival), ref: p.ref, field: `computed_arrival_utc = ${p.arrival}`, plain: `The Checker divided seaRoute's ${fmt1(p.distanceNm)} nm by the advised speed in code: ${fmt1(p.hoursToGo)} h. ${p.consistent ? 'It agrees with the advice within 1 h, before the berth.' : 'It does not agree with the advice.'}` }
    out.planDist = { label: 'Distance to go (seaRoute)', value: `${fmt1(p.distanceNm)} nm`, ref: s.route?.ref ?? p.ref, field: `distance_nm = ${p.distanceNm}`, plain: 'seaRoute computed this from the AIS position over a real sea-lane network. The advice used the same distance.' }
  }
  if (s.reportChecks) {
    s.reportChecks.flags.forEach((f, i) => {
      out[`rc${i}`] = { label: `Report check: ${f.id}`, value: 'flagged', ref: s.reportChecks!.ref, field: `flags[${i}]`, plain: `${f.finding} Checked in code as the report was extracted.` }
    })
  }
  if (s.stoppage) {
    const t = s.stoppage
    const fromRemark = 'From the Master\'s remarks. The Reports Agent copied them; the agent passed them to the Checker.'
    out.stopFrom = { label: 'Main engine stopped', value: fmtEta(t.from), ref: s.report?.ref ?? t.ref, field: `from_utc = ${t.from}`, plain: fromRemark }
    out.stopTo = { label: 'Main engine restarted', value: fmtEta(t.to), ref: s.report?.ref ?? t.ref, field: `to_utc = ${t.to}`, plain: fromRemark }
    out.stopH = { label: 'Length of the stop', value: `${fmt1(t.stopH)} h`, ref: t.ref, field: `stop_hours = ${t.stopH}`, plain: `Worked out in code from the Master's times, ${fmtDayTime(t.from)} to ${fmtDayTime(t.to)} UTC.` }
    out.steamH = { label: 'Hours steaming', value: `${fmt1(t.steamingH)} h`, ref: t.ref, field: `steaming_hours = ${t.steamingH}`, plain: 'The report day less the stop, in code.' }
    out.expNm = { label: 'Distance from speed and steaming time', value: `${fmt1(t.expectedNm)} nm`, ref: t.ref, field: `expected_distance_nm = ${t.expectedNm}`, plain: `${fmt1(t.speedKn)} kn × ${fmt1(t.steamingH)} h, in code. The report gives ${fmt1(t.runNm)} nm (${t.diffPct > 0 ? '+' : ''}${fmt1(t.diffPct)}%; limit ±2%).` }
    const v = t.fuel.VLSFO
    if (v !== undefined) out.stopFuel = { label: 'VLSFO used during the stop', value: `${fmt1(v)} t`, ref: s.report?.ref ?? t.ref, field: `fuel_during_stop.VLSFO = ${v}`, plain: `${fromRemark} Generators and boiler.` }
  }
  if (s.offHire) {
    const o = s.offHire
    const made = 'From the voyage management system\'s hire statement, a stand-in. Made up for this simulation; a real system would give its own figures, and nothing re-checks them.'
    out.hireUsd = { label: 'Hire deducted', value: fmtUsd(o.hireUsd), ref: o.ref, field: `hire_deducted_usd = ${o.hireUsd}`, plain: `${fmt1(o.hours)} h at ${fmtUsd(o.rateUsdPerDay).replace('.00', '')} a day. ${made}` }
    out.rate = { label: 'Hire rate', value: `${fmtUsd(o.rateUsdPerDay).replace('.00', '')} a day`, ref: o.ref, field: `hire_rate_usd_per_day = ${o.rateUsdPerDay}`, plain: made }
    out.bunkerUsd = { label: 'Bunkers for owner\'s account', value: fmtUsd(o.bunkersUsd), ref: o.ref, field: `bunkers_usd = ${o.bunkersUsd}`, plain: `Fuel used while off hire is for the owner's account under the clause. ${made}` }
    out.totalUsd = { label: 'Total for owner\'s account', value: fmtUsd(o.totalUsd), ref: o.ref, field: `total_owners_account_usd = ${o.totalUsd}`, plain: made }
    o.bunkers.forEach((b, i) => {
      out[`bunkerT${i}`] = { label: `${b.grade} used while off hire`, value: `${fmt1(b.tonnes)} t`, ref: s.report?.ref ?? o.ref, field: `bunkers_owners_account[${i}].tonnes = ${b.tonnes}`, plain: "The Master's figure from the remarks." }
      out[`bunkerP${i}`] = { label: `${b.grade} price`, value: `${fmtUsd(b.usdPerT).replace('.00', '')} a tonne`, ref: o.ref, field: `bunkers_owners_account[${i}].usd_per_t = ${b.usdPerT}`, plain: made }
    })
  }
  s.ahead.forEach((a, i) => {
    a.stretches.forEach((x, j) => {
      const plain = `Code sailed ${a.ship.replace(/^MV /, '')}'s remaining seaRoute route through ECMWF's forecast at ${fmt1(a.speedKn)} kn, the Master's ETA speed. Wind ${x.windSector}, sea ${x.seaSector}.`
      out[`ah${i}s${j}bf`] = { label: 'Strongest wind in the stretch', value: `Beaufort ${x.maxBf}`, ref: a.ref, field: `heavy_weather[${j}].max_beaufort = ${x.maxBf}`, plain: `${plain} Up to ${x.maxWindKn} kn.` }
      out[`ah${i}s${j}m`] = { label: 'Highest sea in the stretch', value: `${fmt1(x.maxSwhM)} m`, ref: a.ref, field: `heavy_weather[${j}].max_swh_m = ${x.maxSwhM}`, plain: `${plain} Significant wave height.` }
      out[`ah${i}s${j}from`] = { label: 'Heavy weather from', value: fmtEta(x.from), ref: a.ref, field: `heavy_weather[${j}].from = ${x.from}`, plain }
      out[`ah${i}s${j}to`] = { label: 'Heavy weather until', value: fmtEta(x.to), ref: a.ref, field: `heavy_weather[${j}].to = ${x.to}`, plain }
    })
  })
  if (s.cp) {
    out.limit = { label: 'CP change limit', value: `${s.cp.changeHours} h`, ref: s.cp.ref, field: `eta_change_hours = ${s.cp.changeHours}`, plain: 'From the fictional charter party terms used in this demo.' }
  }
  return out
}

// ---- Inside view --------------------------------------------------------
// The Inside view is a hub: the Ops Agent in the middle hands work to the
// other agents and is the only one that talks to the operator. Each event is
// placed on the part that acted, and on the link it travelled, if any.

export type Tone = 'ok' | 'warn' | 'wait' | 'fail'
export type HubId = 'security' | 'inbox' | 'operator' | 'gate' | 'ops' | 'reports' | 'voyage' | 'weather' | 'performance' | 'correspondence'
export type InstrumentId = 'guardrail' | 'checker' | 'numguard'
export type LinkKind = 'arrives' | 'holds' | 'cut' | 'alert' | 'asks' | 'reports' | 'summary' | 'question' | 'answer' | 'draft' | 'decides'

export interface HubStep {
  seq: number
  node?: HubId
  line: string
  tone?: Tone
  /** Tool the acting agent used, by name. */
  tool?: string
  instrument?: InstrumentId
  link?: { from: HubId; to: HubId; kind: LinkKind }
}

export const HUB_AGENTS: HubId[] = ['reports', 'voyage', 'weather', 'performance', 'correspondence']

const AGENT_NODE: Record<string, HubId> = { security_agent: 'security', ops_agent: 'ops', reports_agent: 'reports', voyage_agent: 'voyage', weather_agent: 'weather', performance_agent: 'performance', correspondence_agent: 'correspondence' }

/** Short line for a tool or check result, as the hub shows it. */
function resultLine(tool: string, d: Obj): { line: string; tone?: Tone } {
  const out = obj(obj(d.result).output)
  const r = obj(d.result)
  switch (tool) {
    case 'voyage_history': {
      const n = ((out.eta_notices_sent as Obj[] | undefined) ?? []).at(-1)
      return { line: n ? `Last notice: ${num(n.days_before_arrival)}-day` : 'No notices yet' }
    }
    case 'cp_terms': return { line: 'CP terms read' }
    case 'contacts': return { line: `${((out.contacts as unknown[]) ?? []).length} verified contacts` }
    case 'distance_to_go': return { line: `${fmt1(num(out.distance_nm) ?? 0)} nm to go` }
    case 'weather_on_track': {
      const m = obj(out.summary)
      return { line: `Weather: Bf ${num(m.min_beaufort)}–${num(m.max_beaufort)}` }
    }
    case 'add_voyage_note': return { line: 'Note filed in the voyage file' }
    case 'check_eta': return r.consistent ? { line: `ETA consistent (${fmtSignedH(num(r.difference_hours) ?? 0)})`, tone: 'ok' } : { line: `ETA off (${fmtSignedH(num(r.difference_hours) ?? 0)})`, tone: 'warn' }
    case 'check_notices': return r.due ? { line: `Notice due (${num(r.threshold_due_days) ?? ''}-day)`, tone: 'warn' } : { line: 'No notice due', tone: 'ok' }
    case 'check_consumption': return r.flagged ? { line: `Fuel ${fmt1(num(r.excess_pct) ?? 0)}% over warranty`, tone: 'warn' } : { line: 'Fuel within limits', tone: 'ok' }
    case 'classify_weather_day': return r.good_weather_day ? { line: 'Good weather day', tone: 'warn' } : { line: 'Not a good weather day', tone: 'ok' }
    case 'report_checks': {
      const f = (d.flagged as string[] | undefined) ?? []
      return f.length ? { line: `Report check flags ${f.join(', ')}`, tone: 'warn' } : { line: 'Report checks pass', tone: 'ok' }
    }
    case 'ais_position': return { line: 'AIS position read' }
    case 'voyage_system': return { line: 'Voyage system read' }
    case 'speed_advice': return { line: `Advice: ${fmt1(num(out.advised_speed_kn) ?? 0)} kn (stand-in)` }
    case 'off_hire_statement': return { line: `Hire statement: ${fmtUsd(num(out.total_owners_account_usd) ?? 0)} (stand-in)` }
    case 'check_speed_plan': return r.consistent ? { line: 'Speed advice adds up', tone: 'ok' } : { line: 'Speed advice does not add up', tone: 'warn' }
    case 'check_stoppage': return r.consistent ? { line: `Stop ${fmt1(num(r.stop_hours) ?? 0)} h agrees`, tone: 'ok' } : { line: 'Stop does not agree', tone: 'warn' }
    case 'weather_ahead': return (r.heavy_weather as unknown[] | undefined)?.length ? { line: `Heavy weather: up to Bf ${num(r.max_beaufort)}`, tone: 'warn' } : { line: 'No heavy weather ahead', tone: 'ok' }
    case 'check_forecast_eta': return r.at_risk ? { line: `Forecast ETA ${fmtSignedH(num(r.difference_hours) ?? 0)}: at risk`, tone: 'warn' } : { line: 'Forecast ETA holds', tone: 'ok' }
  }
  return { line: `${TOOL_INFO[tool]?.name ?? tool} returned` }
}

/** Where each event sits in the hub. Tool and check events carry no agent,
 *  so they are placed on the agent at work at the time. */
export function hubSteps(events: RunEvent[]): HubStep[] {
  let active: HubId = 'ops'
  let lastAgent: HubId = 'ops'
  return events.map((e): HubStep => {
    const d = e.detail
    const first = e.source_refs[0] ?? ''
    const tool = str(d.tool) ?? (first.startsWith('tool:') ? parseRef(first).name : '')
    const me = e.agent ? AGENT_NODE[e.agent] : undefined
    if (me) lastAgent = me
    const at = { seq: e.seq }
    switch (e.type) {
      case 'message_received':
        active = 'ops'
        return { ...at, node: 'inbox', line: 'E-mail arrives' }
      case 'forecast_available':
        active = 'ops'
        return { ...at, node: 'inbox', line: 'New forecast run is out' }
      case 'blocked':
        return { ...at, node: 'inbox', line: 'Blocked: no agent that can act sees it', tone: 'fail', instrument: 'guardrail', link: { from: 'inbox', to: 'ops', kind: 'cut' } }
      case 'question':
        return { ...at, node: 'ops', line: 'Asks you to decide', tone: 'wait', link: { from: 'ops', to: 'operator', kind: 'question' } }
      case 'answer':
        return { ...at, node: 'operator', line: d.yes ? 'Answers yes (simulated)' : 'Answers no (simulated)', tone: d.yes ? 'ok' : 'fail', link: { from: 'operator', to: 'ops', kind: 'answer' } }
      case 'approved':
      case 'edited':
      case 'rejected':
        return { ...at, node: 'operator', line: e.type === 'approved' ? (e.summary.includes('entry') ? 'Approves the entry (simulated)' : 'Approves the draft (simulated)') : e.type === 'edited' ? 'Edits the draft (simulated)' : 'Rejects the draft (simulated)', tone: e.type === 'rejected' ? 'fail' : 'ok', link: { from: 'operator', to: 'gate', kind: 'decides' } }
      case 'approval_requested':
        return { ...at, node: 'gate', line: first.startsWith('approval:') && e.source_refs[1]?.startsWith('tool:propose_entry') ? 'Entry waits for you' : 'Draft waits for you', tone: 'wait' }
      case 'entry':
        return { ...at, node: me ?? 'voyage', line: 'Entry proposed', tool: 'propose_entry', link: { from: me ?? 'voyage', to: 'gate', kind: 'draft' } }
      case 'draft':
        return { ...at, node: 'correspondence', line: 'Draft written', link: { from: 'correspondence', to: 'gate', kind: 'draft' } }
      case 'extracted':
        return { ...at, node: 'reports', line: d.passed ? 'Schema check passed' : 'Schema check failed', tone: d.passed ? 'ok' : 'fail', instrument: 'checker' }
      case 'rationale':
        return { ...at, node: me ?? active, line: 'States a reason' }
      case 'alert':
        if (e.actor === 'guardrail') return { ...at, node: 'operator', line: 'Alert: possible payment fraud', tone: 'fail', instrument: 'guardrail', link: { from: 'inbox', to: 'operator', kind: 'alert' } }
        if (e.actor === 'checker') return { ...at, node: lastAgent, line: 'A number no tool produced', tone: 'warn', instrument: 'numguard' }
        return { ...at, node: me ?? active, line: 'Alert raised for you', tone: 'warn', tool: 'alert_operator' }
      case 'plan':
        if ('delegate_to' in d) {
          const to = AGENT_NODE[str(d.delegate_to) ?? ''] ?? 'reports'
          active = to
          return { ...at, node: 'ops', line: `Asks the ${AGENT_TITLE[str(d.delegate_to) ?? ''] ?? 'agent'}`, link: { from: 'ops', to, kind: 'asks' } }
        }
        active = 'ops'
        return { ...at, node: 'ops', line: 'Sums up for you', link: { from: 'ops', to: 'operator', kind: 'summary' } }
      case 'tool_call':
        return { ...at, node: me ?? active, line: `Uses ${TOOL_INFO[tool]?.name ?? tool}`, tool }
      case 'tool_result':
        if (e.actor === 'subagent' && me) {
          active = 'ops'
          return { ...at, node: me, line: 'Reports to the Ops Agent', link: { from: me, to: 'ops', kind: 'reports' } }
        }
        if (tool === 'approval_gate' || first.startsWith('approval:')) return { ...at, node: 'gate', line: String(d.status ?? '').includes('posted') ? 'Filed, not posted' : 'Filed, not sent', tone: 'ok' }
        return { ...at, node: active, tool, ...resultLine(tool, d) }
      case 'check': {
        if (e.actor === 'guardrail' && 'sender' in d && d.passed) return { ...at, node: 'inbox', line: 'Sender known', tone: 'ok', instrument: 'guardrail', link: { from: 'inbox', to: 'ops', kind: 'arrives' } }
        if (e.actor === 'guardrail' && 'sender' in d) {
          active = 'security'
          return { ...at, node: 'inbox', line: obj(d.sender).lookalike_of ? 'Sender imitates a contact: held' : 'Sender not known: held', tone: 'fail', instrument: 'guardrail', link: { from: 'inbox', to: 'security', kind: 'holds' } }
        }
        if (tool === 'report_findings' || first.startsWith('tool:report_findings')) {
          if (d.problems) return { ...at, node: 'security', line: 'A quote not in the e-mail: refused', tone: 'fail', instrument: 'checker' }
          const n = ((d.findings as unknown[] | undefined) ?? []).length
          return { ...at, node: 'security', line: n ? `${n} finding${n === 1 ? '' : 's'} reported` : 'Nothing found', tone: n ? 'warn' : 'ok', tool: 'report_findings' }
        }
        if (e.actor === 'guardrail' && 'pending_decision' in d) return { ...at, node: active, line: 'Stopped: you decide first', tone: 'fail', instrument: 'guardrail' }
        if (e.actor === 'guardrail') return { ...at, node: active, line: d.passed ? 'Recipients verified' : 'Recipients stopped', tone: d.passed ? 'ok' : 'fail', instrument: 'guardrail' }
        if ('unsupported' in d || e.summary.startsWith('Number guard')) {
          const n = ((d.unsupported as unknown[] | null) ?? []).length
          return { ...at, node: active, line: n ? 'Number guard refused it' : 'Every number traced', tone: n ? 'fail' : 'ok', instrument: 'numguard' }
        }
        if (e.actor === 'subagent') return { ...at, node: me ?? active, line: 'Schema check failed', tone: 'fail', instrument: 'checker' }
        return { ...at, node: active, instrument: 'checker', tool, ...resultLine(tool, d) }
      }
      case 'error':
        return { ...at, node: me ?? active, line: 'Error', tone: 'fail' }
      default:
        return { ...at, node: me ?? active, line: e.summary }
    }
  })
}

/** Timeline marker shape for an event. */
export function markerKind(e: RunEvent): 'reason' | 'tool' | 'check' | 'operator' | 'ai' | 'inbox' {
  if (e.type === 'rationale') return 'reason'
  if (e.actor === 'human_sim') return 'operator'
  if (e.actor === 'checker' || e.actor === 'guardrail') return 'check'
  if (e.actor === 'tool') return 'tool'
  if (e.actor === 'sim') return 'inbox'
  return 'ai'
}

/** Model calls, tokens and wall time for a run: the page's cost line. */
export function runTotals(events: RunEvent[]) {
  let input = 0
  let output = 0
  for (const e of events) {
    input += e.cost?.input_tokens ?? 0
    output += e.cost?.output_tokens ?? 0
  }
  return { inputTokens: input, outputTokens: output, wallMs: events.at(-1)?.wall_ms ?? 0 }
}
