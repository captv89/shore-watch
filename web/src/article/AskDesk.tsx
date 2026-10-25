import type { ReactNode } from 'react'
import { ASK_TIME, CALL, DIGEST, FERN, type Source } from './askData'
import { REPO } from './data'

function Sources({ list }: { list: Source[] }) {
  return (
    <ul className="ask-src" aria-label="Where the figures come from">
      {list.map((s) => (
        <li key={s.label + s.path}>
          {REPO ? <a className="a-chip" href={`${REPO}/blob/main/${s.path}`}>{s.label}</a> : <span className="a-chip" title={s.path}>{s.label}</span>}
        </li>
      ))}
    </ul>
  )
}

function Turn({ who, time, children }: { who: 'operator' | 'agent'; time?: string; children: ReactNode }) {
  return (
    <li className="ask-turn" data-who={who}>
      <div className="ask-who a-mono">{who === 'operator' ? 'Operator' : 'Ops Agent'}{time && <span className="a-muted"> · {time}</span>}</div>
      <div className="ask-body">{children}</div>
    </li>
  )
}

/** "Ask the desk": an illustrated exchange, not a recording. */
export function AskDesk() {
  return (
    <figure className="figure ask">
      <div className="ask-head">
        <span className="ask-badge a-mono">Illustration</span>
        <span className="a-small a-muted">The questions and answers are written for this page. Every figure in them comes from the recorded fleet-week; each chip names the tool call or file it came from.</span>
      </div>
      <ol className="ask-turns">
        <Turn who="operator" time={ASK_TIME}><p className="ask-q">What needs me today?</p></Turn>
        <Turn who="agent">
          <p>Twenty-seven things, in four groups. Most are drafted and only need your approval.</p>
          <ul className="ask-groups">
            {DIGEST.groups.map((g) => (
              <li key={g.title}>
                <span className="ask-n a-mono" aria-hidden>{g.ships.length}</span>
                <div>
                  <strong>{g.title}</strong>
                  <span className="ask-state">{g.state}</span>
                  <span className="ask-ships a-small a-muted">{g.ships.join(' · ')}</span>
                </div>
              </li>
            ))}
          </ul>
          <p className="a-small a-muted">{DIGEST.quiet}</p>
          <Sources list={[DIGEST.source]} />
        </Turn>

        <Turn who="operator"><p className="ask-q">FERN ESTUARY: what happened?</p></Turn>
        <Turn who="agent">
          <p>{FERN.story}</p>
          <ul className="ask-facts">
            {FERN.facts.map(([k, v]) => <li key={k}><span className="a-muted">{k}</span><strong>{v}</strong></li>)}
          </ul>
          <p>{FERN.next}</p>
          <Sources list={FERN.sources} />
        </Turn>

        <Turn who="operator"><p className="ask-q">I'll ring the owner first. What do I need?</p></Turn>
        <Turn who="agent">
          <dl className="ask-sheet">
            {CALL.rows.map(([k, v]) => <div key={k}><dt>{k}</dt><dd>{v}</dd></div>)}
          </dl>
          <Sources list={CALL.sources} />
        </Turn>
      </ol>
      <figcaption>Not a chatbot: the page makes no live model calls. On the desk, the same answers would come from the Ops Agent reading the tools it already uses.</figcaption>
    </figure>
  )
}
