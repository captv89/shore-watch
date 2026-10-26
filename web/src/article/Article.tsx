import { useEffect, useMemo, useRef, useState, type ReactNode } from 'react'
import { CaretDownIcon, CaretUpIcon, CheckIcon, CodeIcon, MoonIcon, PauseIcon, PlayIcon, SunIcon, XIcon } from '@phosphor-icons/react'
import { Player, type SceneMeta } from '../player/Player'
import { sceneRuns } from '../replay/runs'
import type { Run } from '../replay/types'
import { deskState, fmt1, fmtUsd, type LonLat } from '../replay/derive'
import { ADDS, AGENT_EXTRA, DAY, FAQ, INDUSTRY, LINE, PARTS, REAL, RULES, SOURCES, TECH, TOC, WORK, type GroupId } from './content'
import { BASELINES, NEW_TAB, REPO, SHIP_STYLE, TUNING_LOG, falseAlarms, fleetWeek, fromRun, mergeShips, heldOut, KIND_LABEL, label, models, planted, runCost, weekItems, weekRows, weekUse, type Results } from './data'
import { BookTable, FleetWeek } from './FleetWeek'
import { AskDesk } from './AskDesk'
import { SeaChart } from './SeaChart'
import { useTheme } from './theme'
import './article.css'

const scene1 = sceneRuns('s1-routine')
const scene2 = sceneRuns('s2-consumption')
const scene3 = sceneRuns('s3-fraud')
const scene4 = sceneRuns('s4-weather')
const scene5 = sceneRuns('s5-berth')
const scene6 = sceneRuns('s6-offhire')

// Round 2 figures on the page come from the first run of each scene.
const end = (runs: Run[]) => (runs[0] ? deskState(runs[0].events, runs[0].events.length) : undefined)
const s4 = end(scene4)
const s5 = end(scene5)
const s6 = end(scene6)
const tormar4 = s4?.ahead.find((a) => a.shipId === 'SIM-0002')
const valda4 = s4?.ahead.find((a) => a.shipId === 'SIM-0001')

const valda = fromRun(scene1[0], 'VALDA CREST', 1)
// Scene 2 opens on run 2: run 1 missed its first report (decision 111).
const scene2Shown = scene2.find((r) => r.header.attempt === 2) ?? scene2[0]
const tormar = fromRun(scene2Shown, 'TORMAR SPRING', 2)
const ivelle = fromRun(scene3[0], 'IVELLE STAR', 3)
const SHIPS = [valda.ship, tormar.ship].filter((s) => s !== undefined)
// The hero follows the week through all six scenes: each ship on every
// position any scene logged for her (her ship style stays her first scene's).
const fleet4 = { valda: fromRun(scene4[0], 'VALDA CREST', 1, 'SIM-0001'), tormar: fromRun(scene4[0], 'TORMAR SPRING', 2, 'SIM-0002') }
const valda5 = fromRun(scene5[0], 'VALDA CREST', 1)
const tormar6 = fromRun(scene6[0], 'TORMAR SPRING', 2)
const HERO_SHIPS = [mergeShips([valda.ship, fleet4.valda.ship, valda5.ship]), mergeShips([tormar.ship, fleet4.tormar.ship, tormar6.ship])].filter((s) => s !== undefined)
const tag = <T extends { scene: number }>(m: T[], scene: number) => m.map((x) => ({ ...x, scene }))
const MAIL = [...valda.mail, ...tormar.mail, ...ivelle.mail, ...tag(fleet4.valda.mail, 4), ...tag(valda5.mail, 5), ...tag(tormar6.mail, 6)].sort((a, b) => a.time - b.time)

const HERO_BOX: [LonLat, LonLat] = [[-36, 40], [10, 57]]
const LOOP_MS = 48000

function useMedia(q: string) {
  const [on, setOn] = useState(() => window.matchMedia?.(q).matches ?? false)
  useEffect(() => {
    const m = window.matchMedia?.(q)
    if (!m) return
    const f = () => setOn(m.matches)
    m.addEventListener('change', f)
    return () => m.removeEventListener('change', f)
  }, [q])
  return on
}

const fmtClock = (ms: number) => {
  const d = new Date(ms)
  return `${d.getUTCDate()} Jan ${d.getUTCFullYear()} ${String(d.getUTCHours()).padStart(2, '0')}:${String(d.getUTCMinutes()).padStart(2, '0')} UTC`
}
const fmtDate = (iso: string) => new Date(iso).toLocaleDateString('en-GB', { day: 'numeric', month: 'short', year: 'numeric', timeZone: 'UTC' })
const n1 = (v: number) => v.toLocaleString('en-GB', { maximumFractionDigits: 1 })

/* ---------- Navigation ---------- */

function ThemeToggle() {
  const [theme, toggle] = useTheme()
  const dark = theme === 'dark'
  return (
    <button type="button" className="theme-toggle" onClick={toggle} aria-label={dark ? 'Switch to light theme' : 'Switch to dark theme'}>
      {dark ? <SunIcon size={16} aria-hidden /> : <MoonIcon size={16} aria-hidden />}
      <span>{dark ? 'Light' : 'Dark'}</span>
    </button>
  )
}

function useActiveSection() {
  const [active, setActive] = useState('top')
  useEffect(() => {
    const io = new IntersectionObserver((en) => en.forEach((e) => e.isIntersecting && setActive(e.target.id)), { rootMargin: '-25% 0px -65% 0px' })
    document.querySelectorAll('[data-toc]').forEach((n) => io.observe(n))
    return () => io.disconnect()
  }, [])
  return active
}

function Nav({ wide }: { wide: boolean }) {
  const active = useActiveSection()
  const [open, setOpen] = useState(false)
  const cur = TOC.find((t) => t[0] === active)
  const items = (onPick?: () => void) => (
    <ol className="toc-list">
      {TOC.map(([id, n, text]) => (
        <li key={id}>
          <a href={`#${id}`} aria-current={id === active ? 'location' : undefined} onClick={onPick}>
            <span className="toc-n">{n}</span><span>{text}</span>
            {TECH[id] && <CodeIcon size={12} className="toc-tech" aria-label="Technical section" />}
          </a>
        </li>
      ))}
    </ol>
  )
  if (wide) {
    return (
      <nav className="toc" aria-label="Contents">
        <a href="#top" className="toc-title"><span>Shore Watch</span><span className="a-mono">Research note · Oct 2026</span></a>
        {items()}
        <div className="toc-foot">
          <ThemeToggle />
          <div className="a-mono"><span>Synthetic data</span><span>Recorded runs</span><span>Operator approves every draft</span></div>
        </div>
      </nav>
    )
  }
  return (
    <div className="toc-bar">
      <div className="toc-bar-row">
        <button type="button" className="toc-bar-btn" aria-expanded={open} aria-controls="mtoc" onClick={() => setOpen((o) => !o)}>
          <span className="toc-bar-name">Shore Watch</span>
          {cur && <span className="toc-n">§ {cur[1]}</span>}
          <span className="toc-bar-cur">{cur ? cur[2] : 'Contents'}</span>
          {open ? <CaretUpIcon size={14} aria-hidden /> : <CaretDownIcon size={14} aria-hidden />}
        </button>
        <ThemeToggle />
      </div>
      {open && <div id="mtoc" className="toc-bar-list">{items(() => setOpen(false))}</div>}
    </div>
  )
}

/* ---------- Sections ---------- */

function Section({ id, n, title, children, wide }: { id: string; n: string; title: string; children: ReactNode; wide?: boolean }) {
  const tech = TECH[id]
  return (
    <section id={id} data-toc className="section">
      <div className={wide ? 'measure-wide' : 'measure'}>
        <div className="sec-n">§ {n}</div>
        <h2>{title}</h2>
        {tech && (
          <p className="tech-note">
            <span className="tech-tag a-mono"><CodeIcon size={12} aria-hidden /> Technical</span>
            <span>{tech.text}{tech.skip && <> <a href="#real">Skip to Section 10</a>.</>}</span>
          </p>
        )}
      </div>
      {children}
    </section>
  )
}

function Hero() {
  const reduced = useMedia('(prefers-reduced-motion: reduce)')
  const [paused, setPaused] = useState(false)
  const [start, end] = useMemo(() => {
    const times = [...HERO_SHIPS.flatMap((s) => s.fixes.map((f) => f.time)), ...MAIL.map((m) => m.time)]
    return [Math.min(...times), Math.max(...times) + 3 * 3600e3]
  }, [])
  const [at, setAt] = useState(() => (reduced ? end : start))
  const box = useRef<HTMLDivElement>(null)
  useEffect(() => {
    if (reduced || paused || !Number.isFinite(start)) return
    let visible = true
    const io = new IntersectionObserver(([e]) => { visible = e.isIntersecting })
    if (box.current) io.observe(box.current)
    let last = performance.now(), raf = 0
    const tick = (now: number) => {
      const dt = now - last
      last = now
      if (visible && !document.hidden) setAt((a) => { const n = a + (dt / LOOP_MS) * (end - start); return n > end ? start : n })
      raf = requestAnimationFrame(tick)
    }
    raf = requestAnimationFrame(tick)
    return () => { cancelAnimationFrame(raf); io.disconnect() }
  }, [reduced, paused, start, end])
  const shown = MAIL.filter((m) => m.time <= at).length

  return (
    <header id="top" data-toc className="hero">
      <div className="hero-strip a-mono"><span>Independent research note</span><span>October 2026</span><span>Not a product</span></div>
      <div className="hero-head">
        <h1>Shore Watch</h1>
        <p className="hero-lede">A fleet grows from 10 ships to 30. Can the operator's desk keep up? I built Shore Watch, a desk of AI agents, to find out: what they do, where they stop, and every action on the record.</p>
        <div className="a-chips"><span className="a-chip">Synthetic data</span><span className="a-chip">Recorded runs</span><span className="a-chip">Operator approves every draft</span></div>
        <p className="byline">By Vishnu Ravendranathan, independent researcher. Co-developed with Claude Code.</p>
      </div>
      <figure className="figure">
        <div className="screen hero-screen" ref={box}>
          <div className="hero-chart">
            <SeaChart ships={HERO_SHIPS} at={at} box={HERO_BOX} label="Chart of the North Atlantic: VALDA CREST sailing north past Portugal towards Rotterdam, and TORMAR SPRING sailing west from the Channel into the Atlantic, at their logged positions" />
            <div className="hero-clock"><span className="screen-label">Chart · logged positions</span><span className="a-mono">{Number.isFinite(at) ? fmtClock(at) : ''}</span></div>
            <button type="button" className="screen-btn hero-pause" aria-pressed={paused || reduced} disabled={reduced} onClick={() => setPaused((p) => !p)}>
              {reduced ? 'Still frame' : paused ? <><PlayIcon size={12} weight="fill" aria-hidden /> Play</> : <><PauseIcon size={12} weight="fill" aria-hidden /> Pause</>}
            </button>
          </div>
          <div className="a-inbox" aria-label="Operations inbox">
            <div className="a-inbox-head"><span>Operations inbox</span><span className="a-mono">{shown} of {MAIL.length}</span></div>
            {MAIL.map((m, i) => (
              <a key={i} href={`#scene-${m.scene}`} className="a-inbox-item" data-shown={i < shown} data-latest={i === shown - 1 && !reduced}>
                <span className="a-inbox-meta"><span className="a-inbox-from">{m.from}</span><span className="a-mono">{fmtClock(m.time).replace(/ 20\d\d/, '').replace(' UTC', '')}</span></span>
                <span className="a-inbox-subject">{m.subject}</span>
                <span className="a-inbox-scene a-mono">Scene {m.scene}</span>
              </a>
            ))}
          </div>
        </div>
        <figcaption>
          Figure 1. Three fictional ships in the inbox, two of them on the chart, 21–26 January 2025, drawn from the recorded runs of all six scenes: each ship sits at her last logged position, and nothing moves between positions the log does not hold. Scene 4's forecast arrives as an inbox item, though it is not an e-mail. IVELLE STAR is not on the chart: her scene logs no position, because the e-mail never reaches the agents that work on the voyage.{' '}
          {reduced ? 'Shown as a still frame because your system asks for reduced motion.' : `One loop is the ${Math.round((end - start) / 864e5)} days in ${LOOP_MS / 1000} seconds.`}
        </figcaption>
      </figure>
    </header>
  )
}

function Tldr() {
  return (
    <section className="section tldr" aria-labelledby="tldr-h">
      <div className="measure">
        <div className="sec-n">In short</div>
        <h2 id="tldr-h">What this project set out to do</h2>
      </div>
      <ul className="limits measure">
        <li><strong>The aim.</strong> To find out where AI agents help a vessel operator, where they should stop, and how every step they take can stay on the record. To find out, I built a working system of agents for ship operations and ran it.</li>
        <li><strong>What was built.</strong> A shore <em>desk</em>. In this article the desk is everything between the operator's inbox and the operator: it takes in each report, forecast and e-mail, checks it, and brings the operator what needs a decision. This desk is built from AI agents, plain-code guards and stand-ins for the company's systems. It works through six scenes from an operator's week, each recorded several times with every run published. It is then tested on cases it never saw while it was tuned, and run on a fleet of 30 ships through one week of real January 2025 weather.</li>
        <li><strong>What it showed.</strong> A rules-based system (the traditional kind) catches most wrong figures as well as agents do, and the agents use the same rules. The agents earn their place in the work that follows: they read the remark or the e-mail, weigh it, and bring the operator a reply ready to approve, with every number taken from a tool.</li>
        <li><strong>What you take away.</strong> Which parts of the job suit an agent and which should stay with rules and the systems a company already trusts. The guardrails an agent needs before it goes near real ships. And what it costs, where it failed, and what on this page is simulated.</li>
      </ul>
    </section>
  )
}

function Job() {
  return (
    <Section id="job" n="1" title="The job">
      <div className="measure prose">
        <p>Take a vessel operator today, without any AI. Working on the charterer's side, the operator looks after a fleet of two or three dozen ships. In this project each operator has 30. Every day at noon each ship sends a <em>noon report</em>: position, speed, fuel used, fuel remaining on board (ROB) and the Master's estimated time of arrival (ETA). The operator checks that the figures hold together and passes the ETA on when the charter party (CP) says a notice is due. The CP is the contract with the shipowner, or with the cargo interests.</p>
        <p>Speed and fuel are watched against the <em>warranty</em>, the speed and consumption the owner promised in good weather. The charter says "about", which by usual English practice allows 0.5 kn less speed and 5% more fuel. Forecasts arrive twice a day. Owners, port agents, cargo interests and suppliers write all day, and now and then someone who only pretends to be one of them.</p>
        <p>Most of that day is reading, checking and moving figures from one system to another. The decisions are the operator's, and so is the phone call when something goes wrong.</p>
        <p>This piece covers part of that job. A berth delay and an off-hire are shown once each. Laytime and demurrage, bunker planning, emissions reporting and the shore side of an emergency are left out. Read the scenes as a sample of the work, not a map of all of it.</p>
      </div>
      <figure className="figure">
        <div className="fig-label a-mono">An operator's day, and which scene covers it</div>
        <ol className="a-day">
          {DAY.map((d) => (
            <li key={d.title}>
              <span className="a-mono a-muted">{d.time}</span>
              <span className="a-day-title">{d.title}</span>
              <span className="a-muted">{d.text}</span>
              <a href={d.href} className="a-tag a-mono">{d.scene}</a>
            </li>
          ))}
        </ol>
      </figure>
    </Section>
  )
}

function Fit() {
  return (
    <Section id="fit" n="2" title="Where an agent fits">
      <div className="measure prose">
        <p className="lede">Noon report forms and voyage reporting systems ask for set fields, and for good reason. A common format means every ship reports the same way, and nobody has to guess what the Master meant. Some of those fields, though, are there only so a rule can trigger.</p>
        <p>An agent reads the free text as well and joins the dots, so each of those fields can be looked at again. Where the agent can carry the load without losing that clarity, the crew types less. Where it cannot, the field stays. The aim is an easier day for the people who enter the data and for the people who act on it.</p>
        <p>That needs a clear line between what calculates, what judges and what must never happen. I built a working set of agents for ship operations to find where that line falls. Here is where it fell.</p>
      </div>
      <ol className="fit-line measure-wide">
        {LINE.map(([who, does, kind]) => (
          <li key={kind} data-kind={kind}><span className="fit-who">{who}</span><span className="fit-does">{does}</span></li>
        ))}
      </ol>
      <div className="measure prose">
        <p>The systems a company already trusts keep their jobs. The voyage management system (VMS) handles the commercial side: hire, port costs and the voyage accounts. The voyage optimisation service, weather routing included, advises the speed and the route. The accounts system holds each supplier's bank details. The agents call these systems as tools and take their answers as given. This project does not rebuild them. Where a scene needs one, a simple stand-in plays the part, and every figure it gives says it was made up for this simulation.</p>
        <h3 className="h-sub">What an agent adds</h3>
      </div>
      <ol className="adds measure-wide">
        {ADDS.map(([a, b], i) => <li key={a}><span className="a-mono adds-n">{String(i + 1).padStart(2, '0')}</span><strong>{a}</strong><span>{b}</span></li>)}
      </ol>
      <div className="measure">
        <p className="pull">Rules, expert systems and the company's own systems are the foundation. The agent is the assistant that works with them.</p>
        <div className="prose"><p>It is not flawless. Runs vary, and a model can ignore an instruction. So the steps that must never be skipped are enforced in code, and every number comes from a tool, never from the model. Section 10 sets out the limits.</p></div>
      </div>
    </Section>
  )
}

function Fleet() {
  const at = Math.max(...SHIPS.map((s) => s.fixes[s.fixes.length - 1].time))
  return (
    <Section id="fleet" n="3" title="Meet the fleet">
      <div className="measure prose"><p>The scenes follow three of the operator's ships through one week in January 2025, when Storm Éowyn crossed the North Atlantic. The ships are invented. The weather is real. Section 5 then widens the view to the whole fleet of 30 ships over the same week.</p></div>
      <figure className="figure screen a-desk">
        <div className="a-desk-chart">
          <SeaChart ships={SHIPS} at={at} box={[[-24, 40.5], [12, 57]]} label="Chart showing VALDA CREST off Portugal and TORMAR SPRING west of Ireland at their last logged positions" notes={{ 1: '22 Jan · off Portugal', 2: '24 Jan · Storm Éowyn' }} />
          <span className="screen-label a-desk-label">Each ship at her last logged position</span>
        </div>
        <ul className="a-ships">
          {[
            [1, 'VALDA CREST', 'SIM-0001', 'Supramax, laden with coal, Richards Bay to Rotterdam. Off Portugal.'],
            [2, 'TORMAR SPRING', 'SIM-0002', 'Panamax in ballast, Rotterdam to Sept-Îles, sailing west into Storm Éowyn (real weather).'],
            [3, 'IVELLE STAR', 'SIM-0003', 'Handysize, laden with steel products, Bilbao to Gdynia via Skagen. Not on the chart: Scene 3 needs no position.'],
          ].map(([sc, name, id, text]) => (
            <li key={id as string}>
              <span className="a-ship-head">
                <svg width="12" height="12" viewBox="-7 -7 14 14" aria-hidden><Swatch scene={sc as number} /></svg>
                <span className="a-ship-name">{name}</span><span className="a-mono screen-muted">{id} · Scene {sc}</span>
              </span>
              <span className="screen-text">{text}</span>
            </li>
          ))}
        </ul>
      </figure>
      <p className="a-note measure-wide">Names and <span className="a-mono">SIM-</span> numbers are invented and were checked against public ship lists. The charter party terms are simplified and fictional. Coastline: Natural Earth.</p>
    </Section>
  )
}

function Swatch({ scene }: { scene: number }) {
  const s = SHIP_STYLE[scene]
  if (s.shape === 'circle') return <circle r={5} fill={s.col} />
  if (s.shape === 'square') return <rect x={-5} y={-5} width={10} height={10} fill={s.col} />
  return <path d="M0,-6L5.5,4.5L-5.5,4.5Z" fill={s.col} />
}

/** Mounts the player when it comes near the viewport; a poster frame before. */
function LazyPlayer({ runs, meta, poster }: { runs: Run[]; meta: SceneMeta; poster: string }) {
  const ref = useRef<HTMLDivElement>(null)
  const [on, setOn] = useState(false)
  useEffect(() => {
    if (on || !ref.current) return
    const io = new IntersectionObserver(([e]) => { if (e.isIntersecting) { setOn(true); io.disconnect() } }, { rootMargin: '600px 0px' })
    io.observe(ref.current)
    return () => io.disconnect()
  }, [on])
  if (!runs.length) return <div className="screen poster"><span className="screen-label">Scene {meta.number}</span><span className="poster-line">Not recorded yet.</span></div>
  return (
    <div ref={ref} className="player-slot">
      {on ? <Player runs={runs} meta={meta} /> : (
        <div className="screen poster">
          <div className="a-chips a-chips-dark"><span className="a-chip">Synthetic data</span><span className="a-chip">Operator actions simulated</span><span className="a-chip">Nothing is sent</span></div>
          <span className="poster-line">{poster}</span>
          <button type="button" className="screen-btn" onClick={() => setOn(true)}><PlayIcon size={12} weight="fill" aria-hidden /> Load Scene {meta.number}</button>
        </div>
      )}
    </div>
  )
}

function RunLine({ runs, scene, openRun }: { runs: Run[]; scene: string; openRun?: number }) {
  if (!runs.length) return null
  const h = (runs.find((r) => r.header.attempt === openRun) ?? runs[0]).header
  return (
    <p className="run-line a-mono">
      Showing recorded run {h.attempt} of {h.total_attempts} with {h.model} on {fmtDate(h.recorded_at)}, prompt {h.prompt_version}. The player opens on {h.attempt === 1 ? 'the first run' : `run ${h.attempt}`}; switch runs in its menu. Every run: <code>runs/{scene}/</code>
    </p>
  )
}

function Scene({ id, n, kicker, title, children, runs, meta, poster, folder, callout, shows, notes }: {
  id: string; n: string; kicker: string; title: string; children: ReactNode; runs: Run[]; meta: SceneMeta; poster: string; folder: string; callout: string; shows: string; notes?: ReactNode
}) {
  const total = runs[0]?.header.total_attempts
  return (
    <article id={id} data-toc className="a-scene">
      <div className="measure">
        <div className="a-scene-head"><span className="sec-n">§ {n} · Scene {meta.number}</span><span className="a-muted">{kicker}</span></div>
        <h3>{title}</h3>
        <div className="prose">{children}</div>
      </div>
      <LazyPlayer runs={runs} meta={meta} poster={poster} />
      <div className="measure">
        <RunLine runs={runs} scene={folder} openRun={meta.openRun} />
        <blockquote>{callout}</blockquote>
        <div className="shows"><span className="a-mono label">What this shows</span><span>{shows}</span></div>
        {notes && total && (
          <details className="runs-note">
            <summary><span>How the {total} runs went</span><CaretDownIcon size={14} aria-hidden className="faq-caret" /></summary>
            <div className="prose">{notes}</div>
          </details>
        )}
      </div>
    </article>
  )
}

function Scenes() {
  return (
    <section id="scenes" className="section">
      <div className="measure">
        <div className="sec-n">§ 4</div>
        <h2>Six scenes</h2>
        <div className="prose"><p>Each scene replays a recorded run, step by step. Scenes 1 to 3 cover the basics: a noon report, extra fuel and a fraud e-mail. Scenes 4 to 6 go further: a new forecast that starts the work, a berth delay that raises a speed decision, and an engine stop that takes a ship off-hire. Two labels recur, and they mean exactly this:</p></div>
      </div>
      <dl className="labels measure-wide">
        <div className="label-reason"><dt className="a-mono">Agent's stated reason</dt><dd>The agent's own words, written as it worked. Not a guess at what it was thinking.</dd></div>
        <div className="label-sim"><dt className="a-mono">Simulated operator action</dt><dd>A scripted stand-in for the operator approving, editing or answering. No real person acted.</dd></div>
      </dl>

      <Scene id="scene-1" n="4.1" kicker="What an agent is; the human stays in charge." title="A normal noon report" runs={scene1} folder="s1-routine"
        meta={{ number: 1, title: 'A normal noon report' }} poster="VALDA CREST's noon report, off Portugal: an ETA check, a 3-day notice, and a question about missing fuel figures."
        callout="The agent didn't calculate the ETA. The Master did. The agent checked it was believable and did the paperwork."
        shows="An agent reads, checks and drafts; every message waits for the operator.">
        <p>VALDA CREST's noon report arrives from off Portugal. Code first checks that it really comes from the ship. The agent copies the figures into the voyage file, asks the route tool how far is left to Rotterdam and checks the Master's ETA against it. The CP says a three-day ETA notice is due, so the agent drafts one, and the operator approves it.</p>
        <p>The report has no fuel figures. The agent does not guess them. It asks the operator whether the Master should be asked to send them again.</p>
      </Scene>

      <Scene id="scene-2" n="4.2" kicker="Agents look for evidence and escalate; they don't hide problems." title="Consumption is up: questioned, then explained" runs={scene2} folder="s2-consumption"
        meta={{ number: 2, title: 'Consumption is up', realWeather: true, openRun: 2 }} poster="TORMAR SPRING, 22 and 24 January: the same symptom on a calm day and in Storm Éowyn."
        callout="Same symptom, two different answers, because the evidence changed."
        shows="The agent asks what explains a number before it raises it, and raises it when nothing does."
        notes={<><p>Four of the five runs reach the same decisions. Run 1 does not. On the first report the Ops Agent wrote that its colleagues had done their checks, but it never asked them. Nothing was checked and the query about the fuel was missed. It handled the second report correctly. This is the failure an operator cannot see from the drafts, because nothing arrives. The log shows it: no check ran on that report. Every run gives material to improve, and this one led to a new check in code: the desk now confirms that each noon report really went through its checks, sends the Ops Agent back if not, and alerts the operator if it still has not. Three more runs of the same model were recorded with the check in place (<code>eval/runs/coverage-check/</code>): every check ran on all six reports, and the check had nothing to catch. In one of them the Performance Agent sent back empty replies on the first report, so the operator got the fuel alert but no draft query to the Master.</p><p>In run 2 the Performance Agent asked for the report before it was ready; the tool refused, and the agent went back a step. In run 3 the agent also raised an alert about the ETA, beside its question to the operator. Run 5 was stopped once by the model provider, which sent back an empty reply, and was run again.</p></>}>
        <p>On 22 January TORMAR SPRING reports 27.6 t of fuel. Her charter warrants about 24 t a day, and "about" allows up to 25.2 t, so she is 2.4 t over. The agent fetches the real weather along her track from the European Centre for Medium-Range Weather Forecasts (ECMWF): light winds and small seas. That is a good weather day, so the weather does not explain the fuel. The agent drafts a query to the Master and alerts the operator.</p>
        <p>Two days later she reports more fuel again, this time in Storm Éowyn, with head seas of several metres. Now the day is not a good weather day, so it is left out of the warranty check. A note goes on the voyage file and no query is sent. The forecast also puts the Master's ETA at risk, so the agent asks the operator whether the Master should review it. Every figure is in the player, linked to the tool that produced it.</p>
      </Scene>

      <Scene id="scene-3" n="4.3" kicker="E-mail is untrusted input; guardrails and approval protect the operator." title="A convincing fraud e-mail" runs={scene3} folder="s3-fraud"
        meta={{ number: 3, title: 'A convincing fraud e-mail' }} poster="Blocked: lookalike sender, a change of bank details, and a line addressed to an AI."
        callout="Payment fraud by e-mail is a real and costly problem in shipping. The desk treats every e-mail as information, never as orders."
        shows="The part that reads untrusted e-mail has no power to act on it."
        notes={<p>All five runs block the e-mail. Four also report the pressure to pay at once; run 1 does not. In run 5 the Security Agent's first two reports were in the wrong form and were refused. The third went through.</p>}>
        <p>An e-mail that looks as if it comes from the bunker supplier asks for IVELLE STAR's Bilbao invoice to be paid to a new bank account. The names are right, and so is the invoice number. The domain is <span className="a-mono">so<mark>1</mark>entbunker</span>, one character off.</p>
        <p>Code spots it before any agent sees the message. Only the Security Agent reads it, alone, with no tool that can act. It reports the bank change and a hidden line addressed to an AI. Code blocks the e-mail and alerts the operator: verify by telephone, on the number on file.</p>
        <p>In a company, the invoice would be paid through the accounts system, which checks the bank details against the supplier's record on file. That is the right place for the check, and it beats both rules and agents at it. This project has no such system, so the scene ends at the block and the alert.</p>
      </Scene>

      <Scene id="scene-4" n="4.4" kicker="An agent can start from something other than an e-mail." title="Heavy weather ahead" runs={scene4} folder="s4-weather"
        meta={{ number: 4, title: 'Heavy weather ahead', realWeather: true }} poster="ECMWF's 22 January 12z forecast is out: a following gale in Biscay for VALDA CREST, Storm Éowyn head on for TORMAR SPRING."
        callout="The agent read the forecast for every ship and wrote to each Master in the tone the weather called for. The Master decides how to sail."
        shows="A new forecast, not an e-mail, starts the work, and the agent judges what is worth saying."
        notes={<p>All five runs write both notes and both e-mails. The number guard refused a stated reason in run 1. In run 5 it alerted the operator when the Weather Agent's report held figures no tool gave. Each time, the work carried on with the figures from the tools.</p>}>
        <p>ECMWF's forecast run of 22 January 12z comes out at 2000 UTC. No e-mail arrives. The forecast itself wakes the Ops Agent, which hands it to the Weather Agent. For each ship at sea, code sails her remaining route through the forecast at the speed her Master's ETA assumes, and lists every stretch of Beaufort 7 or more, or seas of 4 m or more.{valda4 && tormar4 && <> VALDA CREST meets a following gale in Biscay, up to Beaufort {valda4.maxBf} with seas of {fmt1(valda4.maxSwhM)} m. TORMAR SPRING sails head on into Storm Éowyn, up to Beaufort {tormar4.maxBf} with seas of {fmt1(tormar4.maxSwhM)} m.</>} IVELLE STAR is not scanned, because her scene has no track.</p>
        <p>Precautions and a safer route are for the ship's weather routing service to advise and for the Master to decide. So the office says little. Each Master gets a short note: the office has seen the forecast, the routing advice and the Master's own judgement come first, and the office supports whatever is decided. Each voyage file gets a note that these are bad weather days, to be left out of any performance assessment. Nothing goes to cargo interests or port agents, because the Master owns the ETA.</p>
      </Scene>

      <Scene id="scene-5" n="4.5" kicker="Code does the arithmetic; the agent judges how firm the news is." title="Berth not ready: slow down?" runs={scene5} folder="s5-berth"
        meta={{ number: 5, title: 'Berth not ready: slow down?' }} poster="The Rotterdam port agent writes: the coal berth is taken; VALDA CREST berths about 27 January 0600, subject to the ship ahead."
        callout="The optimisation service could work out the speed. Only reading the port agent's e-mail showed how far to trust the berth time."
        shows="Code checks the speed plan; the agent sees that the berth time is only an estimate, and leaves the decision to the operator."
        notes={<p>All five runs ask the operator and queue the same three items. In run 3 the number guard twice alerted the operator to a figure no tool gave (29 hours).</p>}>
        <p>The Rotterdam port agent writes that the coal berth is occupied. VALDA CREST is next in line and should berth about 27 January 0600 UTC, “subject to the vessel ahead completing as planned”. There is no noon report this time, so the Voyage Agent takes the position from the automatic identification system (AIS), the ETA from the VMS and the distance from seaRoute.</p>
        <p>The speed is a job for the ship's voyage optimisation service, an expert system whose answer the agents take as given. Here a simple stand-in plays that part{s5?.advice && <>: {fmt1(s5.advice.speedKn)} kn to reach the pilot station {s5.advice.pilotH} hours before the berth, saving about {Math.round(s5.advice.anchorH)} hours at anchor and {Math.round(s5.advice.vlsfoSavedT)} t of fuel</>}. Code checks that the distance and the speed give that arrival.</p>
        <p>The agent's part is reading the e-mail. The berth time is an estimate that depends on another ship, and a gale lies ahead in Biscay (Scene 4). As time charterer the operator may order the speed, so the Ops Agent puts the decision to them. On yes, three items wait at the approval gate: a speed instruction that leaves safety judgement with the Master, an ETA notice to the port agent and the receiver, and the new ETA for the VMS.</p>
        <p className="a-note a-standin">Voyage optimisation is an expert system not built for this project. The speed and fuel figures here come from a simple stand-in and are made up for this simulation. A real service would give different figures, and those are the ones an operator uses.</p>
      </Scene>

      <Scene id="scene-6" n="4.6" kicker="The agent connects the Master's words to the contract." title="Main engine stopped: off-hire" runs={scene6} folder="s6-offhire"
        meta={{ number: 6, title: 'Main engine stopped: off-hire' }} poster="TORMAR SPRING's noon report of 26 January: main engine stopped 0610–1140 UTC, fuel filters choked."
        callout="The Master wrote one line in the remarks. The agent turned it into an off-hire claim: code checked the Master's figures, and the voyage management system priced it."
        shows="The agent reads a remark against the charter party and the hire statement; code checks only the Master's facts."
        notes={<><p>All five runs flag the stop, raise the point about the fuel and ask the operator. Three queue both items. In runs 3 and 4 the Voyage Agent, asked for the off-hire entry, said it would propose it and then reported it done, but never did; only the e-mail to the owner reached the queue. It is the same kind of miss as in Scene 2: a step reported done that never ran, visible only in the log. It led to a second check in code: after the operator says yes, the desk confirms that everything agreed reached the approval queue, sends the Ops Agent back if not, and alerts the operator if it still has not.</p><p>Three more runs of the same model were recorded with the check in place (<code>eval/runs/follow-through-check/</code>). In one of them the Voyage Agent made the same miss again. This time code caught it: the Ops Agent was sent back, the entry reached the queue, and the check closed. On the way back the Ops Agent also asked for the owner's e-mail again, so a second copy waited for the operator; code now asks only for what is missing.</p><p>In four runs the number guard refused a stated reason that turned the position into decimal degrees, figures no tool gave. Runs 2 and 3 were stopped by the model provider, which sent back empty replies, and were run again.</p></>}>
        <p>TORMAR SPRING's noon report of 26 January comes two days after Storm Éowyn. Code flags the speed: the distance run over 24 hours does not match the average speed reported. The Master's remark explains it. The main engine stopped from 0610 to 1140 UTC, its fuel filters choked with sludge stirred up in the heavy weather. So nobody queries the Master about the figures.</p>
        <p>A stop for a machinery breakdown is <em>off-hire</em> under the CP: the ship earns no hire for the time lost. The Voyage Agent reads the clause, modelled on clause 17 of the New York Produce Exchange form (NYPE 1993), and code checks the Master's facts{s6?.stoppage && <>: a {fmt1(s6.stoppage.stopH)}-hour stop, and {fmt1(s6.stoppage.speedKn)} kn over {fmt1(s6.stoppage.steamingH)} hours steaming gives the distance the Master reported</>}. The cost comes from the VMS hire statement{s6?.offHire && <>: {fmtUsd(s6.offHire.hireUsd)} of hire and {fmtUsd(s6.offHire.bunkersUsd)} for the fuel burnt while stopped, for the owner's account</>}.</p>
        <p>The agent also warns that the owner may blame the fuel, which the charterer supplied. How to answer that is for the operator to decide. On yes, the off-hire entry and an e-mail to the owner's technical managers asking for the engine log extract wait at the gate.</p>
        <p className="a-note a-standin">The VMS is a company system not built for this project. Its hire and bunker figures here come from a simple stand-in and are made up for this simulation. A real system gives its own figures, and those are the ones an operator trusts.</p>
      </Scene>
    </section>
  )
}

function Week() {
  const [rules, coded, agent] = weekRows(fleetWeek)
  return (
    <Section id="week" n="5" title="From 10 ships to 30, in steps">
      <div className="measure">
        {rules && agent && <p className="headline">Rules and agents bring the operator about the same number of items. The difference is who writes the follow-up.</p>}
        <div className="prose">
          <p>The scenes show one message at a time. An operator's week is hundreds of them. So I built a fleet of 30 dry bulk ships, Capesize to Handysize, on real routes around the world, and ran them through the same week of January 2025. Each ship keeps her own time, so noon reports arrive around the clock. Three desks work that week: two rules desks, plain code only, and the agent desk from the scenes.</p>
          <p>The week includes what happens at sea: wrong figures on about one report in ten, berth changes, an engine stop, a fraud e-mail, three ships rerouted round the Cape of Good Hope after a security advisory, and two cranes broken down in port. The heavy weather was not planted. It is wherever the real ECMWF forecasts put it on a ship's track.</p>
          {rules && agent && <p>The first rules desk and the agent desk send the operator a similar number of items: {rules.reached} from the rules, {agent.reached} from the agents. Most of them need the operator either way. The agents' unneeded items, {agent.noise}, are mostly weather notes to Masters who already had the forecast. What differs is what comes with them. Behind every rules alert the operator still has the work to do: read the report, work out the reply and write it. That is {rules.toWrite} follow-ups in the week. The agents bring {agent.prepared} of them ready to approve, {agent.raw.to_write === 0 ? 'and in the sample left none for the operator to write' : `and leave ${agent.toWrite} for the operator to write`}.</p>}
        </div>
      </div>
      <FleetWeek />
      <div className="measure">
        <p className="pull">Not fewer alerts. An operator who approves instead of writing, with the facts at hand when the phone rings.</p>
        <div className="prose">
          {coded && <p>The second rules desk is given the extra coded fields it would need, such as the reason for a stop and who ordered it. It sends less noise ({coded.noise} {coded.raw.noise === 1 ? 'item' : 'items'} not needed, against {rules?.noise}), but the crew fills in those fields, and the follow-up work stays with the operator.</p>}
          <p>Late and missing reports are left out of the count. A timer sends the reminder at either desk, so they show nothing about agents.</p>
        </div>
      </div>
      <div className="measure">
        <h3 className="h-sub">In steps, not all at once</h3>
        <div className="prose">
          <p>No company should hand one operator 30 ships on the first day. The safer path is in steps. Start the assistant on a book of 10 ships, then go to 20, then 30. Take each step only when the last one has proved itself: nothing missed, drafts approved as written, and a day the operator can manage.</p>
          <p>The recordings show why. In one of Scene 2's five runs the agent said a noon report had been checked when it had not, and the query about the fuel never reached the operator (<a href="#scene-2">Scene 2</a>). In two of Scene 6's runs it said an off-hire entry had been proposed when it had not (<a href="#scene-6">Scene 6</a>). Nothing on the desk showed them; only the log did. With 10 ships someone can read the logs of a week and count misses like these. With 30 ships nobody does it by hand. So each step should come only after the misses at the last step have been counted, explained and fixed in code where they can be. These have been. Code now confirms that every noon report went through its checks, and that what the operator agreed to reached the approval queue.</p>
        </div>
      </div>
      <BookTable />
      <div className="measure">
        <div className="prose">
          <p>With rules alone, the follow-up work grows with the book: the operator writes about as many replies a day as items reach the desk. With the agent desk the same items arrive, but drafted. What grows is the approving, not the writing.</p>
        </div>
        <p className="a-note">Every item, answer key and agent run: <code>eval/runs/fleetweek/</code> and <code>eval/results/fleet-week.json</code>.</p>
      </div>
    </Section>
  )
}

function Ask() {
  return (
    <Section id="ask" n="6" title="One point of access">
      <div className="measure prose">
        <p>An operator does not have to wait for an alert. One question to the Ops Agent reaches every system it is connected to. Most operators have already used a chat assistant. What is different here is what sits behind the chat: the company's own systems, and the same guardrails as in every scene above.</p>
        <p>Here it is at the end of the fleet-week's busiest day, when one owner has refused to send her ship through the Red Sea and the operator would rather pick up the phone than send the draft. The exchange is an illustration written for this page. Every figure in it comes from a recorded run or the week's answer keys, and each names its source.</p>
      </div>
      <AskDesk />
      <div className="measure prose">
        <p>The same desk could sit behind a chat window of its own, or be reached through the Model Context Protocol (MCP), an open standard that lets an assistant the company already uses call the desk's tools. Either way the rules hold: e-mail is information, not orders; every number comes from a tool; and nothing leaves without the operator's approval. Connecting another system means adding a connector, not rebuilding the desk.</p>
        <p className="a-note">Neither the chat nor an MCP server is built. This section shows how one point of access would look.</p>
      </div>
    </Section>
  )
}

function Built({ wide }: { wide: boolean }) {
  const [sel, setSel] = useState<Record<GroupId, string | null>>({ agents: 'ops', instruments: null, connectors: null })
  const flat = Object.fromEntries(Object.values(PARTS).flatMap((g) => g.items.map(([id, l, t]) => [id, { label: l, text: t, kind: g.kind }])))
  return (
    <Section id="built" n="7" title="How it's built">
      <div className="measure prose"><p>The desk has three kinds of part, matching the line drawn in Section 2. <strong>Agents</strong> are AI and make judgements. Each is named after a job a person could hold and gets only the tools that job needs. <strong>Instruments</strong> are plain code. They calculate and enforce, and never decide. <strong>Connectors</strong> stand in for the company's own systems and the expert services it uses; a real installation would connect the real ones. {wide ? 'Select any part to read what it does.' : 'Listed in the order they work on a message.'}</p></div>
      {wide ? (
        <figure className="parts">
          {(Object.entries(PARTS) as [GroupId, (typeof PARTS)[GroupId]][]).map(([gid, g]) => {
            const it = g.items.find((i) => i[0] === sel[gid])
            return (
              <div key={gid} className="parts-row">
                <div><span className="a-mono parts-title">{g.title}</span><span className="a-muted a-small">{g.sub}</span></div>
                <div className="parts-body">
                  <div className="parts-btns">
                    {g.items.map(([id, l]) => (
                      <button key={id} type="button" className={`part part-${gid}`} data-hub={id === 'ops'} aria-pressed={sel[gid] === id} aria-controls={`panel-${gid}`}
                        onClick={() => setSel((s) => ({ ...s, [gid]: s[gid] === id ? null : id }))}>
                        {l}{id === 'ops' && <span className="a-mono a-hub">Hub</span>}
                      </button>
                    ))}
                  </div>
                  {it && <div id={`panel-${gid}`} role="region" aria-label={it[1]} className="panel"><strong>{it[1]}</strong><span>{it[2]}</span></div>}
                </div>
              </div>
            )
          })}
        </figure>
      ) : (
        <ol className="work">
          {WORK.map((id, i) => (
            <li key={id}><span className="a-mono sec-n">{String(i + 1).padStart(2, '0')}</span><span><strong>{flat[id].label}</strong> <span className="a-mono a-muted a-small">{flat[id].kind}</span></span><span /><span>{flat[id].text}</span></li>
          ))}
        </ol>
      )}
      <div className="measure">
        <h3 className="h-sub">When a job gets its own agent</h3>
        <ol className="rules prose">{RULES.map(([a, b]) => <li key={a}><strong>{a}</strong> {b}</li>)}</ol>
        <p className="pull">If you remember one rule: whoever reads untrusted text cannot act.</p>
      </div>
    </Section>
  )
}

/* ---------- Eval ---------- */

function TaskCells({ r }: { r: Results }) {
  return (
    <div role="img" aria-label={`${label(r.model)[0]}: ${r.passed} of ${r.total} test cases right, ${falseAlarms(r)} with a false alarm`} className="cells">
      {r.tasks.map((t) => {
        const fa = (t.grade.false_alarms ?? []).length > 0
        return <span key={t.id} className="cell" data-state={t.grade.pass ? 'pass' : fa ? 'alarm' : 'miss'} title={`${t.id}: ${t.grade.pass ? 'right' : fa ? 'false alarm' : 'missed'}`} />
      })}
    </div>
  )
}

function Eval() {
  const rules = heldOut.find((r) => r.model === 'rules-only')
  const kinds = rules ? [...new Set(rules.tasks.map((t) => t.kind))] : []
  const byKind = (r: Results, k: string) => { const ts = r.tasks.filter((t) => t.kind === k); return `${ts.filter((t) => t.grade.pass).length}/${ts.length}` }
  const cols = [rules, ...models].filter((r): r is Results => !!r)
  return (
    <Section id="eval" n="8" title="How it was tested">
      <div className="measure">
        {models.length ? models.map((m) => (
          <p key={m.model} className="headline">{label(m.model)[0]} got {m.passed} of {m.total} test cases right, with {falseAlarms(m)} false {falseAlarms(m) === 1 ? 'alarm' : 'alarms'}.</p>
        )) : <p className="headline">Model results on the test cases are not in yet. The baselines below are.</p>}
        <div className="prose">
          <p>I wrote {heldOut[0]?.total ?? 29} test cases, like an exam. {heldOut[0] ? heldOut[0].tasks.filter(planted).length : '—'} have a problem hidden in them, such as fuel figures that do not add up or a fraud e-mail. The rest are clean, to see whether the desk raises an alarm when nothing is wrong.</p>
          <p>The agents never saw these cases while they were tuned. Their instructions were tuned on a separate set, so they could not learn the answers in advance. {heldOut[0]?.total ?? 29} cases give a rough picture, not a precise one: one case moves a score by about {Math.round(100 / (heldOut[0]?.total ?? 29))} points.</p>
        </div>
      </div>
      <figure className="figure eval">
        <div className="eval-head a-mono"><span>Test cases · one square per case</span>
          <span className="a-legend"><span><span className="cell" data-state="pass" aria-hidden /> right</span><span><span className="cell" data-state="miss" aria-hidden /> missed</span><span><span className="cell" data-state="alarm" aria-hidden /> false alarm</span></span></div>
        {heldOut.map((r) => (
          <div key={r.model} className="eval-row" data-model={!BASELINES.includes(r.model)}>
            <div><span className="eval-name">{label(r.model)[0]}</span><span className="a-mono a-muted a-small">{label(r.model)[1]}</span></div>
            <TaskCells r={r} />
            <div className="a-mono a-small"><span>{r.passed} / {r.total} right</span><span className="a-muted">{falseAlarms(r)} false {falseAlarms(r) === 1 ? 'alarm' : 'alarms'}</span></div>
          </div>
        ))}
        <figcaption>Two baselines to compare against. <strong>Do nothing</strong> never raises anything, so it gets every clean case right and every problem wrong. <strong>Rules only</strong> is the same checks written as ordinary code, with keyword rules for e-mail and no AI: a fair opponent, not a straw man. A case stopped by a model provider's error is run again until it finishes; only clean runs are scored.</figcaption>
      </figure>

      {cols.length > 0 && (
        <div className="measure-wide">
          <h3 className="h-sub">Where an agent beats a rule, and where it doesn't</h3>
          <p className="a-muted a-small">Cases right, by kind. Where rules score as well as the agents, plain code is enough for that job.</p>
          <div className="table-wrap">
            <table className="table">
              <thead><tr><th>Kind of task</th>{cols.map((c) => <th key={c.model}>{label(c.model)[0]}</th>)}</tr></thead>
              <tbody>{kinds.map((k) => <tr key={k}><td>{KIND_LABEL[k] ?? k}</td>{cols.map((c) => <td key={c.model} className="a-mono">{byKind(c, k)}</td>)}</tr>)}</tbody>
            </table>
          </div>
        </div>
      )}

      <div className="measure">
        <h3 className="h-sub">Beyond the score: what the agents add</h3>
        <div className="prose"><p>The scores say rules alone do well at checking figures. That is a fair result, and the agents use the same rules. Section 2 lists what an agent adds. Three points from the test itself sit behind the score:</p></div>
        <ol className="rules prose">{AGENT_EXTRA.map(([a, b]) => <li key={a}><strong>{a}</strong> {b}</li>)}</ol>
        <p className="pull">Rules catch the problem. Agents do the work that follows: read, weigh, write and ask.</p>
      </div>

      <div className="measure-wide">
        <h3 className="h-sub">What others have found</h3>
        <ul className="sources">{INDUSTRY.map(([src, href, text]) => <li key={src}><span><a href={href} {...NEW_TAB}>{src}</a><br /><span className="a-muted">{text}</span></span></li>)}</ul>
        <p className="a-note">Rules are not the opponent here. They are the floor the agents stand on, and the agents are the next step up: the same checks, plus the reading and writing an operator does around them.</p>
      </div>

      {models.length > 0 && (
        <div className="measure-wide">
          <h3 className="h-sub">What it got wrong</h3>
          <p className="a-muted a-small">Every miss and every false alarm in the test cases is listed.</p>
          <ol className="misses">
            {models.flatMap((m) => m.tasks.filter((t) => !t.grade.pass).map((t) => (
              <li key={m.model + t.id}>
                <span className="a-mono miss-id"><XIcon size={12} weight="bold" aria-hidden /> {t.id}</span>
                <span><strong>{label(m.model)[0]}:</strong> {t.planted}.</span>
                <span />
                <span className="a-muted">{(t.grade.missed ?? []).length > 0 && `Missed: ${t.grade.missed!.join(', ')}. `}{(t.grade.false_alarms ?? []).length > 0 && `Raised without cause: ${t.grade.false_alarms!.join(', ')}. `}{t.error && `The run stopped: ${t.error.slice(0, 120)}`}</span>
              </li>
            )))}
          </ol>
        </div>
      )}

      <div className="measure">
        <h3 className="h-sub">How it is marked</h3>
        <div className="prose"><p>A plain computer program marks the answers against an answer key. No AI marks the AI. Each question, draft and alert the agents produce is labelled with what it is about, and the program checks four things:</p></div>
        <ul className="checks prose">
          <li><CheckIcon size={14} aria-hidden /> Did it raise the issue planted, and only that?</li>
          <li><CheckIcon size={14} aria-hidden /> Did it stay quiet on a clean report, or where a remark explains the figure?</li>
          <li><CheckIcon size={14} aria-hidden /> Was an ETA notice drafted exactly when the charter party requires?</li>
          <li><CheckIcon size={14} aria-hidden /> Was a fraud e-mail blocked, with the right reasons?</li>
        </ul>
        <p className="a-note">How the prompts were tuned, and what I learned: <a href={TUNING_LOG} {...NEW_TAB}>the tuning log</a>.</p>
      </div>
    </Section>
  )
}

function Cost() {
  const rows: [string, string, Run | undefined][] = [
    ['Scene 1', 'one noon report', scene1[0]], ['Scene 2', 'two noon reports, per report', scene2Shown], ['Scene 3', 'one e-mail', scene3[0]],
    ['Scene 4', 'one forecast run, two ships', scene4[0]], ['Scene 5', 'one e-mail', scene5[0]], ['Scene 6', 'one noon report', scene6[0]],
  ]
  return (
    <Section id="cost" n="9" title="What it costs and how fast it is">
      <div className="measure prose"><p>These figures come from the event logs of the first run of each scene. A <em>model call</em> is one request to the AI model. <em>Tokens</em> are the pieces of text it reads and writes, and what providers charge for.</p></div>
      <div className="table-wrap measure-wide">
        <table className="table">
          <thead><tr><th>Per message</th><th>Model calls</th><th>Input tokens</th><th>Output tokens</th><th>Time</th></tr></thead>
          <tbody>
            {rows.map(([name, sub, run]) => {
              if (!run) return <tr key={name}><td><strong>{name}</strong><br /><span className="a-muted a-small">{sub}</span></td><td colSpan={4} className="a-muted">Not recorded yet.</td></tr>
              const c = runCost(run), k = c.reports
              return (
                <tr key={name}>
                  <td><strong>{name}</strong><br /><span className="a-muted a-small">{sub} · run {run.header.attempt}</span></td>
                  <td className="a-mono">{n1(c.calls / k)}</td><td className="a-mono">{Math.round(c.inputTokens / k).toLocaleString('en-GB')}</td>
                  <td className="a-mono">{Math.round(c.outputTokens / k).toLocaleString('en-GB')}</td><td className="a-mono">{n1(c.minutes / k)} min</td>
                </tr>
              )
            })}
          </tbody>
        </table>
      </div>
      <p className="a-note measure">These runs used a provider's free tier, which paces requests, so the time includes waiting. The logs record a cost of zero; list prices change, so the table gives tokens rather than a price.</p>
      {weekUse.length > 0 && (
        <div className="measure prose">
          <h3 className="h-sub">Over the fleet-week</h3>
          <p>Scaled to the fleet-week in Section 5 the same way as the alerts, kind by kind from the sample, the desk's model use over the {weekItems} items comes to about:</p>
          <ul>{weekUse.map((u) => (
            <li key={u.model}><strong>{label(u.model)[0]}</strong>: {(Math.round(u.calls / 10) * 10).toLocaleString('en-GB')} model calls, {n1(u.input / 1e6)} million tokens read and {n1(u.output / 1e6)} million written. That is about {Math.round(u.calls / 30 / 7)} calls and {Math.round((u.input + u.output) / 30 / 7 / 1000)} thousand tokens per ship per day.</li>
          ))}</ul>
          <p>These are estimates, like the alert figures. Most of it is reading: each call sends the agent its instructions and the case so far again.</p>
        </div>
      )}
    </Section>
  )
}

function Real() {
  const n = scene3[0]?.header.total_attempts ?? 5
  return (
    <Section id="real" n="10" title="Limits, and what is real" wide>
      <div className="measure prose">
        <p>A desk like this earns trust by being plain about what it cannot do. These are the limits I found, and what on this page is simulated.</p>
      </div>
      <ul className="limits measure">
        <li><strong>Runs vary.</strong> The same scene can go slightly differently each time, which is why every run is published. In an early recording one run skipped asking the operator, so that step is now enforced in code, not left to the instructions.</li>
        <li><strong>A silent miss is the hardest failure to see.</strong> In three of the ten runs of Scenes 2 and 6 an agent reported a step done that never ran: a report checked, an off-hire entry proposed. A wrong draft is caught at approval; a missing one is not. Only the log shows it, which is why the desk should take on more ships in steps, with the logs read at each step. Both misses led to checks in code: every noon report must go through its checks, and what the operator agrees to must reach the approval queue.</li>
        <li><strong>A model can fail on the mechanics.</strong> One large model sent its tool calls in the wrong format and repeated them until its budget ran out. Its results are in the repository and are not used on this page.</li>
        <li><strong>Numbers never come from the model.</strong> Every figure is checked against a tool, and the Number guard stops anything else.</li>
        <li><strong>Systems not built for this project</strong> (voyage optimisation with weather routing, the VMS, the accounts system) appear as simple stand-ins. Their figures say they are made up for this simulation.</li>
        <li><strong>The fleet-week agent figures are an estimate</strong> from a sample, not a count. The rules figures are counted on every item.</li>
        <li><strong>The page covers part of the job.</strong> Laytime and demurrage, bunker planning, emissions reporting and emergencies are not in it.</li>
      </ul>
      <div className="table-wrap measure-wide">
        <table className="table">
          <thead><tr><th>Layer</th><th>Real or not</th><th>Notes</th></tr></thead>
          <tbody>{REAL.map(([layer, kind, l, note]) => <tr key={layer}><td>{layer}</td><td><span className="realness" data-kind={kind}>{l}</span></td><td className="a-muted">{note}</td></tr>)}</tbody>
        </table>
      </div>
      <div className="measure">
        <h3 className="h-sub">Recording policy</h3>
        <div className="prose"><p>Each scene is recorded {n} times and every run is published. Each scene says which run it shows. Where runs disagree, the page says so. A run stopped by the model provider's outage, before the agents could answer, is run again and the page says so.</p><p>The early scenes were recorded on Google's Gemini 3.1 Flash-Lite. Its free tier allows 500 requests a day, so one set of recordings had to be spread over several days, and "high demand" errors kept stopping runs. The scenes on this page were recorded on NVIDIA's Nemotron 3.5 Super through NVIDIA's API instead. Google's Gemma 4 31B still runs in the evaluation, so the evaluation compares models from two providers.</p></div>
      </div>
    </Section>
  )
}

function Faq({ wide }: { wide: boolean }) {
  return (
    <Section id="faq" n="11" title="FAQ" wide>
      <div className="faq measure-wide">
        {FAQ.map(([q, a]) => (
          <details key={q} open={wide}>
            <summary>{q}<CaretDownIcon size={14} aria-hidden className="faq-caret" /></summary>
            <p>{a}</p>
          </details>
        ))}
      </div>
    </Section>
  )
}

function Sources() {
  return (
    <Section id="sources" n="12" title="Data sources and licences" wide>
      <ul className="sources measure-wide">{SOURCES.map(([l, href, lic]) => <li key={l}><a href={href} {...NEW_TAB}>{l}</a><span className="a-mono a-muted a-small">{lic}</span></li>)}</ul>
      <p className="a-note measure-wide">Weather: adapted from ECMWF IFS open data (© 2025 ECMWF), CC BY 4.0. Cropped to the North Atlantic and resampled along the track.</p>
    </Section>
  )
}

function Code() {
  const cards: [string, string, string][] = [['Code', 'Repository · Apache-2.0', ''], ['Recorded runs', 'Every event log, every attempt', '/tree/main/runs'], ['Evaluation', 'Tasks, answer keys, scorer', '/tree/main/eval']]
  return (
    <Section id="code" n="13" title="Run it yourself" wide>
      <div className="links measure-wide">
        {cards.map(([t, sub, path]) => REPO
          ? <a key={t} href={REPO + path} {...NEW_TAB}><strong>{t}</strong><span className="a-mono a-muted a-small">{sub}</span></a>
          : <div key={t}><strong>{t}</strong><span className="a-mono a-muted a-small">{sub}</span></div>)}
      </div>
      {!REPO && <p className="a-note measure-wide">The repository link is added when the code is published with this page.</p>}
      <div className="measure-wide prose"><p>Inspired by FineEnvs' “Simulation RL Environments” on Hugging Face (Apache-2.0): an article, recorded replays and an honest real-versus-simulated section.</p></div>
      <p className="colophon measure-wide"><span>By Vishnu Ravendranathan, independent researcher. Co-developed with Claude Code.</span><span className="a-mono a-small">Apache-2.0</span></p>
    </Section>
  )
}

export function Article() {
  const wide = useMedia('(min-width: 1180px)')
  const roomy = useMedia('(min-width: 760px)')
  return (
    <div className="article" data-wide={wide}>
      <Nav wide={wide} />
      <div className="article-col">
        <main className="article-main">
          <Hero />
          <Tldr />
          <Job />
          <Fit />
          <Fleet />
          <Scenes />
          <Week />
          <Ask />
          <Built wide={roomy} />
          <Eval />
          <Cost />
          <Real />
          <Faq wide={roomy} />
          <Sources />
          <Code />
        </main>
      </div>
    </div>
  )
}
