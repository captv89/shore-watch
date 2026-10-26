import { describe, expect, it } from 'vitest'
import { fmtWeek, weekRows, type FleetWeek, type WeekTotals } from './data'

const week = (over: Partial<WeekTotals> = {}): WeekTotals => ({ items: 374, reached: 151, needed: 141, noise: 10, missed: 1, wrong: 6, prepared: 0, to_write: 155, ...over })

describe('fmtWeek', () => {
  it('shows counts as they are and estimates as "about"', () => {
    expect(fmtWeek(151, false)).toBe('151')
    expect(fmtWeek(150.2, true)).toBe('about 150')
  })
  it('never shows an estimated zero as a week count', () => {
    expect(fmtWeek(0, true)).toBe('0 in the sample')
    expect(fmtWeek(0, false)).toBe('0')
  })
})

describe('weekRows', () => {
  const fw: FleetWeek = {
    items: 374,
    desks: [
      { name: 'rules-only', estimate: false, week: week() },
      { name: 'gemma-4-31b-it', estimate: true, week: week({ prepared: 154.2, to_write: 1 }) },
      { name: 'nvidia/nemotron-3.5-super-vl-120b-a12b', estimate: true, sample_items: 37, week: week({ reached: 150.2, prepared: 155.2, to_write: 0 }) },
    ],
  }
  it('keeps the rules desks, then Nemotron, then Gemma', () => {
    expect(weekRows(fw).map((r) => r.model)).toEqual(['rules-only', 'nvidia/nemotron-3.5-super-vl-120b-a12b', 'gemma-4-31b-it'])
  })
  it('formats the agent row as an estimate', () => {
    const n = weekRows(fw)[1]
    expect(n.reached).toBe('about 150')
    expect(n.toWrite).toBe('0 in the sample')
    expect(n.prepared).toBe('about 155')
  })
  it('renders nothing for an empty file', () => {
    expect(weekRows({ items: 0, desks: [] })).toEqual([])
  })
})
