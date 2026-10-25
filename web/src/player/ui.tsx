import { CheckIcon, HourglassMediumIcon, WarningIcon, XIcon } from '@phosphor-icons/react'
import type { ReactNode } from 'react'
import type { Tone } from '../replay/derive'
import { usePlayer } from './context'

/** A number on screen. Clicking it opens the tool call behind it (principle 2). */
export function Num({ k, className, children }: { k: string; className?: string; children?: ReactNode }) {
  const { nums, openSource } = usePlayer()
  const n = nums[k]
  if (!n) return <span className="num-pending">not yet known</span>
  return (
    <button type="button" className={`num ${className ?? ''}`} onClick={() => openSource(k)} aria-label={`${n.label}: ${n.value}. Show where it comes from`}>
      {children ?? n.value}
    </button>
  )
}

const TONE_ICON = { ok: CheckIcon, warn: WarningIcon, wait: HourglassMediumIcon, fail: XIcon }

export function ToneIcon({ tone, size = 12 }: { tone: Tone; size?: number }) {
  const I = TONE_ICON[tone]
  return <I size={size} weight="bold" aria-hidden="true" />
}

/** Small status chip: icon + text, never colour alone. */
export function Status({ tone, children }: { tone: Tone; children: ReactNode }) {
  return (
    <span className={`status tone-${tone}`}>
      <ToneIcon tone={tone} />
      {children}
    </span>
  )
}
