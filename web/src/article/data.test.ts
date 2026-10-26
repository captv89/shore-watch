import { describe, expect, it } from 'vitest'
import { bookRows, fmtWeek, missRows, weekRows, type FleetWeek, type WeekTotals } from './data'

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

describe('bookRows', () => {
  const fw: FleetWeek = {
    items: 374,
    books: [10, 30],
    desks: [
      { name: 'rules-only', estimate: false, week: week(), by_book: { '10': week({ items: 139, reached: 63, to_write: 62 }), '30': week() } },
      { name: 'nvidia/nemotron-3.5-super-vl-120b-a12b', estimate: true, week: week(), by_book: { '10': week({ prepared: 61.7, to_write: 0 }), '30': week({ prepared: 155.2, to_write: 0 }) } },
    ],
  }
  it('gives a day of the week for each book, rules against the agent desk', () => {
    expect(bookRows(fw)).toEqual([
      { ships: 10, items: 20, reached: 9, rulesToWrite: 9, agentPrepared: 9, agentToWrite: 0 },
      { ships: 30, items: 53, reached: 22, rulesToWrite: 22, agentPrepared: 22, agentToWrite: 0 },
    ])
  })
  it('renders nothing for a file without books', () => {
    expect(bookRows({ items: 0, desks: [] })).toEqual([])
  })
})

describe('missRows', () => {
  const task = (id: string, grade: { pass: boolean; missed?: string[]; false_alarms?: string[] }) => ({ id, kind: 'forecast', planted: 'storm ahead; the Master has it in hand', key: { topics: [], block: false }, grade })
  const results = [
    { model: 'gemma-4-31b-it', set: 'heldout', recorded_at: '', passed: 0, total: 2, tasks: [task('a', { pass: false, false_alarms: ['heavy_weather'] }), task('b', { pass: true })] },
    { model: 'nvidia/nemotron-3.5-super-vl-120b-a12b', set: 'heldout', recorded_at: '', passed: 0, total: 2, tasks: [task('a', { pass: false, false_alarms: ['heavy_weather'] }), task('b', { pass: false, missed: ['rob_vlsfo', 'eta_notice'] })] },
  ]
  it('groups by case, models with the same outcome together, in plain words', () => {
    expect(missRows(results)).toEqual([
      { id: 'a', kind: 'A new forecast', planted: 'Storm ahead; the Master has it in hand', outcomes: [{ models: ['Gemma 4 31B', 'Nemotron 3.5 Super'], text: 'raised a heavy-weather note to the Master when it was not needed' }] },
      { id: 'b', kind: 'A new forecast', planted: 'Storm ahead; the Master has it in hand', outcomes: [{ models: ['Nemotron 3.5 Super'], text: 'missed the query about VLSFO remaining on board and the ETA notice' }] },
    ])
  })
})
