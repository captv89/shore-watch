import { describe, expect, it } from 'vitest'
import { deskState, findSource, fmtEta, fmtSignedH, hubSteps, indexSources, markerKind, numberSources } from './derive'
import { parseLog, sceneRuns } from './runs'
import type { RunEvent } from './types'
import { fleetSteps } from '../player/VoyageParts'

const runs = sceneRuns('s1-routine')

describe('recorded Scene 1 runs', () => {
  it('has all five attempts', () => {
    expect(runs.map((r) => r.header.attempt)).toEqual([1, 2, 3, 4, 5])
    for (const r of runs) expect(r.header.total_attempts).toBe(5)
  })

  for (const run of runs) {
    describe(run.header.run_id, () => {
      const { events } = run
      const last = events.at(-1)!.seq
      const end = deskState(events, last)
      const sources = indexSources(events)

      it('has steps in order', () => {
        events.forEach((e, i) => expect(e.seq).toBe(i + 1))
      })

      it('reaches the Scene 1 decisions', () => {
        expect(end.senderKnown).toBe(true)
        expect(end.report?.position).toBe('40-30.0N 011-05.3W')
        expect(end.eta?.consistent).toBe(true)
        expect(end.notices?.due).toBe(true)
        expect(end.recipientsOk).toBe(true)
        expect(end.unsupported).toEqual([])
        // The 3-day ETA notice, and the Master asked to resend
        // the report with the fuel figures it leaves out.
        expect(end.queue.map((q) => q.subject)).toEqual(expect.arrayContaining([expect.stringMatching(/ETA/), expect.stringMatching(/missing/i)]))
        expect(end.queue).toHaveLength(2)
        expect(end.gate).toBe('approved')
        expect(end.filed).toBe('approved, not sent (demo)')
      })

      it('logs the route geometry seaRoute returned', () => {
        const path = end.route!.path
        expect(path.length).toBeGreaterThan(10)
        const [lon, lat] = path.at(-1)!
        expect(lon).toBeCloseTo(4.4, 0)
        expect(lat).toBeCloseTo(51.9, 0)
      })

      it('links every number on screen to a logged source', () => {
        const nums = numberSources(end)
        expect(Object.keys(nums).length).toBeGreaterThanOrEqual(12)
        for (const [key, n] of Object.entries(nums)) {
          expect(findSource(sources, n.ref), `${key} -> ${n.ref}`).toBeDefined()
        }
      })

      it('resolves every tool and e-mail ref in event sources', () => {
        const skip = /^tool:(state_reason|checked_report)#/
        for (const e of events) {
          for (const ref of e.source_refs) {
            if (skip.test(ref)) continue
            expect(findSource(sources, ref), `event ${e.seq}: ${ref}`).toBeDefined()
          }
        }
      })

      it('shows nothing before it happens', () => {
        const start = deskState(events, 1)
        expect(start.email).toBeDefined()
        expect(start.report).toBeUndefined()
        expect(start.route).toBeUndefined()
        expect(start.draft).toBeUndefined()
        const extracted = events.find((e) => e.type === 'extracted')!.seq
        expect(deskState(events, extracted - 1).report).toBeUndefined()
        expect(deskState(events, extracted).report).toBeDefined()
      })

      it('maps every event to a part of the hub and a marker', () => {
        const steps = hubSteps(events)
        events.forEach((e, i) => {
          expect(steps[i].line, `event ${e.seq}`).not.toBe('')
          expect(steps[i].node, `event ${e.seq} ${e.actor}/${e.type}`).toBeDefined()
          expect(markerKind(e)).toBeTruthy()
        })
      })

      it('returns every hand-off to the Ops Agent', () => {
        const links = hubSteps(events).flatMap((s) => (s.link ? [s.link] : []))
        for (const a of links.filter((l) => l.kind === 'asks')) {
          expect(links.some((l) => l.kind === 'reports' && l.from === a.to), `${a.to} reports back`).toBe(true)
        }
      })
    })
  }
})

describe('formatting', () => {
  it('formats times and signed hours', () => {
    expect(fmtEta('2025-01-25T21:00:00Z')).toBe('25 Jan 2100 UTC')
    expect(fmtSignedH(-0.4)).toBe('−0.4 h')
    expect(fmtSignedH(1.25)).toBe('+1.3 h')
  })
})

const runs2 = sceneRuns('s2-consumption')

describe('recorded Scene 2 runs', () => {
  it('has recorded runs from the named agents', () => {
    expect(runs2.length).toBeGreaterThanOrEqual(4)
    for (const r of runs2) expect(r.header.prompt_version).toBe('staff-10')
  })

  for (const run of runs2) {
    describe(run.header.run_id, () => {
      const { events } = run
      const sources = indexSources(events)
      const end = deskState(events, events.length)
      const d3 = end.dayStarts[1]
      const d1 = deskState(events, d3 - 1)

      it('holds two noon reports, two days apart', () => {
        expect(end.dayStarts).toHaveLength(2)
        expect(end.inbox).toHaveLength(2)
        expect(end.past).toHaveLength(1)
      })

      it('D1: flagged, a good weather day, a query and an alert', () => {
        expect(d1.consumption?.flagged).toBe(true)
        expect(d1.consumption?.excessPct).toBe(15)
        expect(d1.weatherDay?.good).toBe(true)
        expect(d1.weather?.samples).toHaveLength(8)
        expect(d1.weather?.grid?.sha256).toMatch(/^[0-9a-f]{64}$/)
        expect(d1.alert).toBeDefined()
        expect(d1.edit?.after).toContain("Please reply before tomorrow's noon report.")
        expect(d1.gate).toBe('edited')
        expect(d1.forecast).toBeUndefined()
      })

      it('D3: excluded, a note, and the ETA at risk', () => {
        expect(end.consumption?.excessPct).toBe(17.9)
        expect(end.weatherDay?.good).toBe(false)
        expect(end.weatherDay?.maxBf).toBe(10)
        expect(end.weather?.hoursSea5Ahead).toBe(24)
        expect(end.note).toBeDefined()
        expect(end.forecast?.atRisk).toBe(true)
        expect(end.forecast?.differenceH).toBe(17.8)
        expect(end.forecast?.samples.length).toBeGreaterThan(100)
        // Run 4's Correspondence Agent, asked for an alert, also wrote a
        // second query to the Master; the page says so.
        expect(end.queue).toHaveLength(run.header.attempt === 4 ? 3 : 2)
      })

      it('starts D3 fresh: nothing from D1 shows as current', () => {
        const start = deskState(events, d3)
        expect(start.report).toBeUndefined()
        expect(start.weather).toBeUndefined()
        expect(start.draft).toBeUndefined()
        expect(start.past[0].weatherDay?.good).toBe(true)
      })

      it('links every number on screen to a logged source', () => {
        for (const s of [d1, end]) {
          for (const [key, n] of Object.entries(numberSources(s))) {
            expect(findSource(sources, n.ref), `${key} -> ${n.ref}`).toBeDefined()
          }
        }
      })

      it('maps every event to a part of the hub and a marker', () => {
        const steps = hubSteps(events)
        events.forEach((e, i) => {
          expect(steps[i].line, `event ${e.seq}`).not.toBe('')
          expect(steps[i].node, `event ${e.seq} ${e.actor}/${e.type}`).toBeDefined()
          expect(markerKind(e)).toBeTruthy()
        })
      })
    })
  }
})

// Scene 3 has no published runs until it is recorded; a scripted run from
// the Go test (agent/desk/scene3_test.go) stands in for the page's logic.
const fixture = Object.values(import.meta.glob<string>('./fixtures/*.jsonl', { query: '?raw', import: 'default', eager: true }))

describe('Scene 3, scripted fixture', () => {
  const events = parseLog(fixture[0] ?? '')
  const end = deskState(events, events.length)

  it('holds the e-mail, blocks it and alerts with three reasons', () => {
    expect(end.screen?.lookalike?.domain).toBe('solentbunker.example')
    expect(end.screen?.domain[end.screen.diffAt!]).toBe('1')
    expect(end.findings?.items.length).toBe(2)
    expect(end.blocked).toBeDefined()
    expect(end.fraud?.reasons.length).toBe(3)
    expect(end.draft).toBeUndefined()
  })

  it('never lets the e-mail reach the Ops Agent', () => {
    const steps = hubSteps(events)
    expect(steps.some((s) => s.node === 'ops')).toBe(false)
    expect(steps.some((s) => s.link?.kind === 'cut' && s.link.to === 'ops')).toBe(true)
    events.forEach((e, i) => expect(steps[i].node, `event ${e.seq}`).toBeDefined())
  })

  it('links every source the events name', () => {
    const sources = indexSources(events)
    expect(findSource(sources, end.findings!.ref)?.tool).toBe('report_findings')
  })
})

// Round 2. Every recorded run, checked for what the page shows of it.
function commonChecks(events: RunEvent[]) {
  const sources = indexSources(events)
  const end = deskState(events, events.length)

  it('links every number on screen to a logged source', () => {
    for (const [key, n] of Object.entries(numberSources(end))) {
      expect(findSource(sources, n.ref), `${key} -> ${n.ref}`).toBeDefined()
    }
  })

  it('resolves every tool and e-mail ref in event sources', () => {
    const skip = /^tool:(state_reason|checked_report)#/
    for (const e of events) for (const ref of e.source_refs) {
      if (!skip.test(ref)) expect(findSource(sources, ref), `event ${e.seq}: ${ref}`).toBeDefined()
    }
  })

  it('maps every event to a part of the hub and a marker', () => {
    const steps = hubSteps(events)
    events.forEach((e, i) => {
      expect(steps[i].line, `event ${e.seq}`).not.toBe('')
      expect(steps[i].node, `event ${e.seq} ${e.actor}/${e.type}`).toBeDefined()
      expect(markerKind(e)).toBeTruthy()
    })
  })

  it('files every queue item, none posted or sent', () => {
    expect(end.queue.length).toBeGreaterThan(0)
    for (const q of end.queue) {
      expect(q.status, q.subject).toBe('approved')
      expect(q.filed, q.subject).toBe(q.kind === 'entry' ? 'approved, not posted (demo)' : 'approved, not sent (demo)')
    }
  })
  return end
}

describe('recorded Scene 4 runs', () => {
  for (const run of sceneRuns('s4-weather')) {
    describe(run.header.run_id, () => {
      const end = commonChecks(run.events)
      it('starts from the forecast, not an e-mail, and scans two ships', () => {
        expect(end.email).toBeUndefined()
        expect(end.trigger?.run).toBe('2025-01-22T12:00:00Z')
        expect(end.trigger?.notScanned[0]).toMatch(/IVELLE STAR/)
        expect(end.ahead.map((a) => a.ship).sort()).toEqual(['MV TORMAR SPRING', 'MV VALDA CREST'])
        const tormar = end.ahead.find((a) => a.shipId === 'SIM-0002')!
        expect(tormar.maxBf).toBe(12)
        expect(tormar.maxSwhM).toBe(9.9)
        expect(fleetSteps(end.ahead).length).toBeGreaterThan(20)
      })
      it('names the ship each event is about (schema 1.6)', () => {
        const ev = run.events
        expect(run.header.schema_version).toBe('1.6')
        expect(ev[0].ship).toBe('fleet')
        for (const e of ev) {
          expect(['fleet', 'SIM-0001', 'SIM-0002']).toContain(e.ship)
          const r = e.detail.result as Record<string, unknown> | undefined
          if (e.type === 'check' && e.source_refs[0]?.startsWith('tool:weather_ahead')) expect(e.ship, `event ${e.seq}`).toBe(r?.ship_id)
          if (e.type === 'draft') {
            const to = (e.detail.draft as { to: string[] }).to[0]
            expect(e.ship, `event ${e.seq}`).toBe(to.includes('valdacrest') ? 'SIM-0001' : 'SIM-0002')
          }
        }
      })
      it('notes bad weather days for each ship, and writes to each Master only', () => {
        expect(new Set(end.notes.map((n) => n.shipId))).toEqual(new Set(['SIM-0001', 'SIM-0002']))
        for (const n of end.notes) expect(n.agent).toBe('weather_agent')
        expect(end.queue.every((q) => q.kind === 'email' && q.to.length === 1 && q.to[0].startsWith('master@'))).toBe(true)
      })
    })
  }
})

describe('recorded Scene 5 runs', () => {
  for (const run of sceneRuns('s5-berth')) {
    describe(run.header.run_id, () => {
      const end = commonChecks(run.events)
      it('has no noon report: the AIS position and the voyage system ETA instead', () => {
        expect(end.report).toBeUndefined()
        expect(end.ais?.position).toBe('44-14.1N 009-25.5W')
        expect(end.vms?.portCalls[0].eta).toBe('2025-01-25T21:00:00Z')
      })
      it('shows the stand-in advice, labelled, and the Checker agreeing', () => {
        expect(end.advice?.speedKn).toBe(8.3)
        expect(end.advice?.note).toMatch(/made up for this simulation/)
        expect(numberSources(end).fuelSaved.plain).toMatch(/Made up for this simulation/)
        expect(end.speedPlan?.consistent).toBe(true)
        expect(end.speedPlan?.beforeBerth).toBe(true)
      })
      it('queues the speed instruction, the notice and the port call entry', () => {
        expect(end.queue.filter((q) => q.kind === 'email')).toHaveLength(2)
        const entry = end.queue.find((q) => q.kind === 'entry')!
        expect(entry.entry?.kind).toBe('port_call_eta')
        expect(entry.entry?.eta).toBe('2025-01-27T04:00:00Z')
        expect(end.question?.answer?.yes).toBe(true)
      })
    })
  }
})

describe('recorded Scene 6 runs', () => {
  for (const run of sceneRuns('s6-offhire')) {
    describe(run.header.run_id, () => {
      const end = commonChecks(run.events)
      it('flags the speed, and the remark explains it', () => {
        expect(end.reportChecks?.flags.map((f) => f.id)).toEqual(['speed'])
        expect(end.report?.remarks).toMatch(/Main engine stopped 0610-1140 UTC/)
        expect(end.stoppage?.consistent).toBe(true)
        expect(end.stoppage?.stopH).toBe(5.5)
        expect(end.stoppage?.expectedNm).toBe(231.3)
      })
      it('shows the hire statement as given, labelled', () => {
        expect(end.offHire?.hireUsd).toBe(2177.08)
        expect(end.offHire?.bunkersUsd).toBe(660)
        expect(end.offHire?.totalUsd).toBe(2837.08)
        expect(numberSources(end).totalUsd.plain).toMatch(/Made up for this simulation/)
      })
      it('queues the off-hire entry and the e-mail to the owner', () => {
        const entry = end.queue.find((q) => q.kind === 'entry')!
        expect(entry.entry?.kind).toBe('off_hire')
        expect(entry.entry?.from).toBe('2025-01-26T06:10:00Z')
        expect(entry.entry?.to).toBe('2025-01-26T11:40:00Z')
        expect(end.queue.find((q) => q.kind === 'email')?.to).toEqual(['technical@tormar-bulk.example'])
      })
    })
  }
})
