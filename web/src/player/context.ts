import { createContext, useContext } from 'react'
import type { DeskState, NumberSource, Source } from '../replay/derive'
import type { Run, RunEvent } from '../replay/types'
import type { SceneMeta } from './Player'
import type { ColourBy } from './weather'

export type View = 'Desk' | 'Inside' | 'Split'
/** The reader's own choice at the approval gate. It stays in this replay and is never recorded. */
export type UserGate = null | 'editing' | 'approved' | 'edited' | 'rejected'

export interface PlayerCtx {
  runs: Run[]
  runIndex: number
  setRunIndex: (i: number) => void
  events: RunEvent[]
  seq: number
  total: number
  ev: RunEvent
  go: (seq: number) => void
  desk: DeskState
  /** Desk state at the end of the run: fixes the chart extent so it never jumps. */
  final: DeskState
  nums: Record<string, NumberSource>
  sources: Map<string, Source>
  /** Opens the source drawer for a number key (e.g. "dist") or a ref ("tool:cp_terms#call_1"). */
  openSource: (key: string) => void
  view: View
  setView: (v: View) => void
  playing: boolean
  togglePlay: () => void
  speed: number
  setSpeed: (n: number) => void
  presenter: boolean
  setPresenter: (on: boolean) => void
  userGate: UserGate
  setUserGate: (g: UserGate) => void
  editText: string
  setEditText: (t: string) => void
  meta: SceneMeta
  reduced: boolean
  /** Which weather sample the chart and strip show: the last unless stepping or animating. */
  wxStep: number
  /** Pick a sample (stops the animation); null returns to the report time. */
  setWxStep: (i: number | null) => void
  wxAnimating: boolean
  /** Plays the day's weather from the start, or pauses it. */
  playWeather: () => void
  colourBy: ColourBy
  setColourBy: (c: ColourBy) => void
}

export const Ctx = createContext<PlayerCtx | null>(null)

export function usePlayer(): PlayerCtx {
  const c = useContext(Ctx)
  if (!c) throw new Error('usePlayer outside Player')
  return c
}
