// "Ask the desk": an illustration, not a recorded run and not
// a chatbot. The questions and the answers' wording are written for the
// page. Every figure in them comes from the fleet-week: the digest from the
// week's answer keys (eval/results/fleetweek-rules-only.json, items of
// 22 Jan 2025 to 1600 UTC), FERN ESTUARY's figures from a recorded sample
// run, named in each Source.

export interface Source {
  /** What gave the figure: a tool call, or a results file. */
  label: string
  /** Where it is in the repository. */
  path: string
}

export interface DigestGroup {
  title: string
  state: string
  ships: string[]
}

export const ASK_TIME = '22 Jan 2025, 1600 UTC'

export const DIGEST: { groups: DigestGroup[]; quiet: string; source: Source } = {
  groups: [
    {
      title: 'ETA notices due',
      state: '15 drafts to the agents and receivers, ready to approve',
      ships: ['BRACKEN ISLE', 'QUILLAN ROCK', 'ASHBY FELL', 'INGLE NESS', 'ALDER CROSS', 'SKERRY DAWN', 'TEAL HOLLOW', 'DUNLIN GATE', 'EYOT SPRING', 'FERN ESTUARY', 'LARCH QUAY', 'MERLIN STRAND', 'KITTIWAKE SHOAL', 'SOLWAY FIRTH', 'PELL HAVEN'],
    },
    {
      title: 'Heavy weather ahead',
      state: '6 notes to the Masters, ready to approve',
      ships: ['MARRAM STAR', 'GANNET RIDGE', 'ORRIN WATER', 'PELL HAVEN', 'GORSE LANDING', 'HEATH WARREN'],
    },
    {
      title: 'Noon report figures that do not add up',
      state: "3 questions for you: ask the Master to check? JUNIPER TOR's ETA notice waits for the answer",
      ships: ['JUNIPER TOR (VLSFO ROB)', 'RUNNEL POINT (position, ETA)', 'HOLLIN REACH (speed)'],
    },
    {
      title: 'Red Sea passage declined by the owner',
      state: '3 decisions for you: confirm the Cape route?',
      ships: ['DUNLIN GATE', 'EYOT SPRING', 'FERN ESTUARY (fuel short)'],
    },
  ],
  quiet: '31 more read and filed with nothing to do: 10 noon reports, 21 forecast scans.',
  source: { label: "fleet-week answer keys, 22 Jan to 1600 UTC", path: 'eval/results/fleetweek-rules-only.json' },
}

/** The recorded sample run FERN ESTUARY's figures come from (Nemotron, staff-17). */
const FERN_RUN = 'eval/runs/fleetweek/eval-fw-0123-reroute-fern-estuary-nvidia-nemotron-3-5-super-vl-120b-a12b-r1.jsonl'

export const FERN: { story: string; facts: [string, string][]; next: string; sources: Source[] } = {
  story:
    "At 0700 UTC her owner wrote that, after the overnight security advisory for the southern Red Sea, the Master and owners judge the passage through Bab el-Mandeb exposes her to war risks, and she will not go that way. The charter party's war risks clause gives them that right; the extra time and fuel on the alternative are for our account. They propose the Cape of Good Hope.",
  facts: [
    ['Cape route to Antwerp', '10,343.7 nm'],
    ['Longer than planned by', '4,898.1 nm, 16.3 days'],
    ['Planned ETA Antwerp', '9 Feb 0800 UTC'],
    ['New ETA Antwerp', '25 Feb 1800 UTC'],
    ['VLSFO on board, about', '843.7 t'],
    ['Short of the 3-day reserve by', '25.8 t'],
  ],
  next: 'Waiting for you: confirm the route. If you do, the reply to the owner, the revised ETA notice to the Antwerp agent and terminal, and the new ETA in the voyage management system are drafted for your approval. Where she takes the extra fuel is still open.',
  sources: [
    { label: 'check_route_change', path: FERN_RUN },
    { label: 'check_notices', path: FERN_RUN },
    { label: "owner's e-mail", path: FERN_RUN },
  ],
}

export const CALL: { rows: [string, string][]; sources: Source[] } = {
  rows: [
    ['Who', "Fern Estuary Shipping, the owner's vessel technical management: technical@fernestuary-shipping.example"],
    ['Their right', "The war risks clause: she need not go where the Master and owners reasonably judge she may be exposed to war risks; we give other orders, and the extra time and bunkers are ours."],
    ['The figures', '4,898.1 nm and 16.3 days more via the Cape; ETA Antwerp 25 Feb 1800 UTC; VLSFO 25.8 t short of the 3-day reserve.'],
    ['Ready on your yes', 'The reply confirming the Cape route, the ETA notice to the agent and the terminal, and the ETA entry in the voyage management system, drafted for your approval. Nothing goes until you approve.'],
    ['To settle on the call', 'That the Master holds clear of the area until our orders arrive, and where she bunkers on the way.'],
  ],
  sources: [
    { label: 'contacts', path: FERN_RUN },
    { label: 'cp_terms: war risks clause', path: FERN_RUN },
    { label: 'check_route_change', path: FERN_RUN },
    { label: 'drafts in the run', path: FERN_RUN },
  ],
}
