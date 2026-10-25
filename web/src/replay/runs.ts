import type { Run, RunEvent, RunHeader } from './types'

// Recorded runs are bundled from /runs at build time. The page makes no
// network calls for them, and never calls a model (principle 3).
const logs = import.meta.glob<string>('../../../runs/*/*.jsonl', { query: '?raw', import: 'default', eager: true })
const headers = import.meta.glob<RunHeader>('../../../runs/*/*.header.json', { import: 'default', eager: true })

export function parseLog(text: string): RunEvent[] {
  return text
    .split('\n')
    .filter((l) => l.trim() !== '')
    .map((l) => JSON.parse(l) as RunEvent)
    .sort((a, b) => a.seq - b.seq)
}

/** All recorded runs of a scene (the folder runs/<scene>), in attempt order. */
export function sceneRuns(scene: string): Run[] {
  const runs: Run[] = []
  for (const [path, header] of Object.entries(headers)) {
    if (!path.includes(`/runs/${scene}/`)) continue
    const text = logs[path.replace(/\.header\.json$/, '.jsonl')]
    if (text) runs.push({ header, events: parseLog(text) })
  }
  return runs.sort((a, b) => a.header.attempt - b.header.attempt)
}
