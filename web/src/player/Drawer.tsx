import { XIcon } from '@phosphor-icons/react'
import { useEffect, useRef } from 'react'
import { fmtSimTime, TOOL_INFO, type NumberSource, type Source } from '../replay/derive'
import type { RunEvent } from '../replay/types'

const fmt = (v: unknown) => (v == null ? '' : typeof v === 'string' ? v : JSON.stringify(v, null, 2))

/** Where a number comes from: the tool call, its inputs and what it returned. */
export function Drawer({ num, source, events, onClose, onJump }: {
  num?: NumberSource
  source: Source
  events: RunEvent[]
  onClose: () => void
  onJump: (seq: number) => void
}) {
  const panel = useRef<HTMLElement>(null)
  const close = useRef<HTMLButtonElement>(null)
  useEffect(() => {
    const opener = document.activeElement as HTMLElement | null
    close.current?.focus()
    return () => opener?.focus?.()
  }, [])

  // Keep Tab inside the drawer while it is open.
  const onKeyDown = (e: React.KeyboardEvent) => {
    if (e.key !== 'Tab' || !panel.current) return
    const f = panel.current.querySelectorAll<HTMLElement>('button, [href], textarea, input, [tabindex]:not([tabindex="-1"])')
    const first = f[0]
    const last = f[f.length - 1]
    if (e.shiftKey && document.activeElement === first) { e.preventDefault(); last.focus() }
    else if (!e.shiftKey && document.activeElement === last) { e.preventDefault(); first.focus() }
  }

  const info = TOOL_INFO[source.tool] ?? { name: source.tool, by: '' }
  const ev = events[source.seq - 1]
  const callId = source.ref.split('#')[1] ?? source.ref

  return (
    <>
      <div className="scrim" onClick={onClose} />
      <aside ref={panel} className="drawer" role="dialog" aria-modal="true" aria-labelledby="drawer-title" onKeyDown={onKeyDown}>
        <div className="drawer-head">
          <h2 id="drawer-title" className="drawer-kicker">{num ? 'Where this number comes from' : 'Tool call'}</h2>
          <button ref={close} type="button" className="btn btn-icon" onClick={onClose} aria-label="Close (Esc)"><XIcon size={14} aria-hidden="true" /></button>
        </div>
        <div className="drawer-body">
          {num && (
            <div className="drawer-value">
              <span className="muted">{num.label}</span>
              <span className="mono">{num.value}</span>
            </div>
          )}
          <dl className="drawer-facts">
            <dt>Produced by</dt><dd>{info.by}</dd>
            <dt>Tool</dt><dd className="mono teal">{source.tool === 'inbox' ? 'inbox' : source.tool}</dd>
            <dt>Call</dt><dd className="mono">{callId}</dd>
            <dt>Event</dt><dd className="mono">{source.seq} of {events.length}, {ev ? fmtSimTime(ev.sim_time) : ''}</dd>
            {num && <><dt>Field</dt><dd><span className="field mono">{num.field}</span></dd></>}
          </dl>
          <p className="drawer-plain">{num ? num.plain : ev?.summary}</p>
          {source.args != null && (
            <div className="drawer-code">
              <h3>Asked with</h3>
              <pre>{fmt(source.args)}</pre>
            </div>
          )}
          <div className="drawer-code">
            <h3>Returned</h3>
            <pre>{fmt(source.result)}</pre>
          </div>
          <button type="button" className="btn btn-outline" onClick={() => onJump(source.seq)}>Jump to event {source.seq} on the timeline</button>
        </div>
      </aside>
    </>
  )
}

