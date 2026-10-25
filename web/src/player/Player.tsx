import { ProjectorScreenIcon } from '@phosphor-icons/react'
import { useCallback, useEffect, useMemo, useRef, useState } from 'react'
import { deskState, findSource, indexSources, numberSources, runTotals } from '../replay/derive'
import type { Run } from '../replay/types'
import { Ctx, usePlayer, type PlayerCtx, type UserGate, type View } from './context'
import type { ColourBy } from './weather'
import { Desk } from './Desk'
import { Drawer } from './Drawer'
import { Inside } from './Inside'
import { Mobile } from './Mobile'
import { Presenter } from './Presenter'
import { Controls, NowBar } from './Timeline'
import { fleetSteps } from './VoyageParts'
import './player.css'

const VIEWS: View[] = ['Desk', 'Inside', 'Split']
const STEP_MS = 1600
/** Time per weather step when a weather result plays through its day. */
const WX_STEP_MS = 520

/** What the page says about a scene above its player. */
export interface SceneMeta {
  number: number
  title: string
  /** Scenes whose weather is real ECMWF data get a tag saying so. */
  realWeather?: boolean
}

// With several players on the page, the keyboard drives the one used last
// (the first, until the reader touches another).
let activePlayer: symbol | null = null

function useReducedMotion() {
  const q = '(prefers-reduced-motion: reduce)'
  const [on, setOn] = useState(() => window.matchMedia?.(q).matches ?? false)
  useEffect(() => {
    const m = window.matchMedia?.(q)
    if (!m) return
    const f = () => setOn(m.matches)
    m.addEventListener('change', f)
    return () => m.removeEventListener('change', f)
  }, [])
  return on
}

/** Reads ?scene=2&run=2&event=25&view=Inside so a step can be linked to (and captured for video). */
function fromUrl(runs: Run[], scene: number) {
  const q = new URLSearchParams(window.location.search)
  if (Number(q.get('scene') ?? 1) !== scene) return { run: 0, seq: 1, view: 'Split' as View, presenter: false, linked: false }
  const run = Math.max(0, runs.findIndex((r) => r.header.attempt === Number(q.get('run'))))
  const view = VIEWS.find((v) => v.toLowerCase() === q.get('view')?.toLowerCase()) ?? 'Split'
  return { run, seq: Number(q.get('event')) || 1, view, presenter: q.has('presenter'), linked: q.has('scene') }
}

export function Player({ runs, meta }: { runs: Run[]; meta: SceneMeta }) {
  const [me] = useState(() => Symbol('player'))
  const init = useMemo(() => fromUrl(runs, meta.number), [runs, meta.number])
  if (init.linked) activePlayer ??= me
  useEffect(() => { activePlayer ??= me }, [me])
  const [runIndex, setRunIndexRaw] = useState(init.run)
  const run = runs[runIndex]
  const { events } = run
  const total = events.length
  const [seqRaw, setSeq] = useState(init.seq)
  const seq = Math.min(Math.max(1, seqRaw), total)
  const [view, setView] = useState<View>(init.view)
  const [playing, setPlaying] = useState(false)
  const [speed, setSpeed] = useState(1)
  const [presenter, setPresenter] = useState(init.presenter)
  const [src, setSrc] = useState<string | null>(null)
  const [userGate, setUserGate] = useState<UserGate>(null)
  const [editText, setEditText] = useState('')

  const sources = useMemo(() => indexSources(events), [events])
  const final = useMemo(() => deskState(events, total), [events, total])
  const desk = useMemo(() => deskState(events, seq), [events, seq])
  const nums = useMemo(() => numberSources(desk), [desk])

  // Weather: on arriving at a weather result, the chart steps through the
  // day's samples once. Reduced motion shows the report time at once.
  const reduced = useReducedMotion()
  const [colourBy, setColourBy] = useState<ColourBy>('sea')
  const [wxStepRaw, setWxStepRaw] = useState<number | null>(null)
  const [wxAnimating, setWxAnimating] = useState(false)
  // The fleet scene steps through the forecast's 3-hourly times instead (Scene 4).
  const fleet = useMemo(() => fleetSteps(desk.ahead), [desk.ahead])
  const nSamples = desk.weather?.samples.length ?? fleet.length
  const wxStep = Math.max(0, Math.min(wxStepRaw ?? (desk.weather ? nSamples - 1 : 0), nSamples - 1))
  const ev = events[seq - 1]
  const atWeather = (ev.type === 'tool_result' && ev.detail.tool === 'weather_on_track') || (ev.type === 'check' && !!ev.source_refs[0]?.startsWith('tool:weather_ahead'))
  useEffect(() => {
    if (atWeather && !reduced) {
      setWxStepRaw(0)
      setWxAnimating(true)
    } else {
      setWxStepRaw(null)
      setWxAnimating(false)
    }
  }, [seq, atWeather, reduced])
  useEffect(() => {
    if (!wxAnimating) return
    const t = setTimeout(() => {
      if (wxStep >= nSamples - 1) setWxAnimating(false)
      else setWxStepRaw(wxStep + 1)
    }, WX_STEP_MS / speed)
    return () => clearTimeout(t)
  }, [wxAnimating, wxStep, nSamples, speed])
  const setWxStep = useCallback((i: number | null) => {
    setWxAnimating(false)
    setWxStepRaw(i)
  }, [])
  const playWeather = useCallback(() => {
    if (wxAnimating) { setWxAnimating(false); return }
    setWxStepRaw(0)
    setWxAnimating(true)
  }, [wxAnimating])

  const go = useCallback((n: number) => {
    const next = Math.min(Math.max(1, n), total)
    setSeq(next)
    setUserGate(null)
  }, [total])

  const setRunIndex = useCallback((i: number) => {
    setRunIndexRaw(i)
    setPlaying(false)
    setUserGate(null)
  }, [])

  const togglePlay = useCallback(() => {
    setPlaying((p) => {
      if (!p && seq >= total) setSeq(1)
      return !p
    })
  }, [seq, total])

  useEffect(() => {
    // Playback holds while a weather result plays through its day.
    if (!playing || wxAnimating) return
    const t = setTimeout(() => (seq >= total ? setPlaying(false) : go(seq + 1)), STEP_MS / speed)
    return () => clearTimeout(t)
  }, [playing, seq, total, speed, go, wxAnimating])

  useEffect(() => {
    if (activePlayer !== me) return
    const q = new URLSearchParams(window.location.search)
    q.set('scene', String(meta.number))
    q.set('run', String(run.header.attempt))
    q.set('event', String(seq))
    q.set('view', view.toLowerCase())
    window.history.replaceState(null, '', `${window.location.pathname}?${q}${window.location.hash}`)
  }, [run, seq, view, me, meta.number])

  useEffect(() => {
    const onKey = (e: KeyboardEvent) => {
      if (activePlayer !== me) return
      const tag = (e.target as HTMLElement).tagName?.toLowerCase()
      if (tag === 'textarea' || (tag === 'input' && e.key !== 'Escape')) return
      if (e.key === 'Escape') {
        if (src) setSrc(null)
        else if (presenter) setPresenter(false)
        return
      }
      if (src || e.metaKey || e.ctrlKey || e.altKey) return
      if (e.key === 'ArrowRight') { e.preventDefault(); go(seq + 1) }
      else if (e.key === 'ArrowLeft') { e.preventDefault(); go(seq - 1) }
      else if (e.key === 'Home') { e.preventDefault(); go(1) }
      else if (e.key === 'End') { e.preventDefault(); go(total) }
      else if (e.key === ' ' && tag !== 'button') { e.preventDefault(); togglePlay() }
      else if (e.key === 'p' || e.key === 'P') setPresenter(!presenter)
      else if (e.key === '1' || e.key === '2' || e.key === '3') setView(VIEWS[Number(e.key) - 1])
    }
    window.addEventListener('keydown', onKey)
    return () => window.removeEventListener('keydown', onKey)
  }, [go, seq, total, togglePlay, src, presenter, me])

  const root = useRef<HTMLDivElement>(null)
  const [width, setWidth] = useState(1400)
  useEffect(() => {
    const el = root.current
    if (!el) return
    const ro = new ResizeObserver(([e]) => setWidth(e.contentRect.width))
    ro.observe(el)
    return () => ro.disconnect()
  }, [])

  const ctx: PlayerCtx = {
    runs, runIndex, setRunIndex, events, seq, total, ev, go, desk, final, nums, sources,
    openSource: setSrc, view, setView, playing, togglePlay, speed, setSpeed, presenter, setPresenter,
    userGate, setUserGate, editText, setEditText,
    meta, reduced, wxStep, setWxStep, wxAnimating, playWeather, colourBy, setColourBy,
  }

  const drawer = src ? (nums[src] ? { num: nums[src], source: findSource(sources, nums[src].ref) } : { num: undefined, source: findSource(sources, src) }) : null

  return (
    <Ctx.Provider value={ctx}>
      <div ref={root} className="player-root" onPointerDownCapture={() => { activePlayer = me }} onFocusCapture={() => { activePlayer = me }}>
        {width < 820 ? (
          <Mobile />
        ) : (
          <section className={`player ${width < 1180 ? 'is-narrow' : ''}`} aria-label={`Scene ${meta.number} replay player`}>
            <Header />
            {view !== 'Inside' && <Desk />}
            {view !== 'Desk' && <Inside />}
            <div className="bottom">
              <NowBar />
              <Controls />
            </div>
          </section>
        )}
        {presenter && <Presenter />}
        {drawer?.source && (
          <Drawer
            num={drawer.num}
            source={drawer.source}
            events={events}
            onClose={() => setSrc(null)}
            onJump={(n) => { setSrc(null); setPresenter(false); go(n) }}
          />
        )}
      </div>
    </Ctx.Provider>
  )
}

function Header() {
  const { view, setView, setPresenter, meta } = usePlayer()
  return (
    <header className="p-head">
      <div className="p-title">
        <div className="title-row">
          <span className="scene-no mono">Scene {meta.number}</span>
          <h2>{meta.title}</h2>
        </div>
        <div className="tags">
          <span className="tag">Synthetic data</span>
          <RunPicker />
          <span className="tag">Operator actions simulated</span>
          <span className="tag">Nothing is sent</span>
          {meta.realWeather && <span className="tag tag-real">Weather: real ECMWF data</span>}
        </div>
      </div>
      <div className="p-tools">
        <div className="seg" role="radiogroup" aria-label="View">
          {VIEWS.map((v, i) => (
            <button key={v} type="button" role="radio" aria-checked={view === v} className={view === v ? 'is-on' : ''} onClick={() => setView(v)} title={`${v} view (${i + 1})`}>{v}</button>
          ))}
        </div>
        <button type="button" className="btn" onClick={() => setPresenter(true)} title="Presenter mode (P)">
          <ProjectorScreenIcon size={15} aria-hidden="true" />Presenter <kbd>P</kbd>
        </button>
      </div>
    </header>
  )
}

/** Picks which of the recorded runs to replay. All runs are published (principle 4). */
export function RunPicker() {
  const { runs, runIndex, setRunIndex, events } = usePlayer()
  const h = runs[runIndex].header
  const t = runTotals(events)
  const date = new Date(h.recorded_at).toLocaleDateString('en-GB', { day: 'numeric', month: 'short', year: 'numeric', timeZone: 'UTC' })
  return (
    <div className="run-picker">
      <span className="tag tag-run" title={`${h.model}, recorded ${date}. ${h.notes} ${t.inputTokens.toLocaleString('en-GB')} input and ${t.outputTokens.toLocaleString('en-GB')} output tokens.`}>
        Recorded run {h.attempt} of {h.total_attempts}, {h.model}
      </span>
      <div className="seg seg-sm" role="radiogroup" aria-label="Recorded run">
        {runs.map((r, i) => (
          <button key={r.header.run_id} type="button" role="radio" aria-checked={i === runIndex} aria-label={`Run ${r.header.attempt}`} className={i === runIndex ? 'is-on' : ''} onClick={() => setRunIndex(i)}>{r.header.attempt}</button>
        ))}
      </div>
    </div>
  )
}
