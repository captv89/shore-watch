// Article text from the Claude Design handoff (Shore Watch Article.dc.html).
// Figures are not written here: they are read from runs/ and eval/results.

export const TOC: [id: string, n: string, label: string][] = [
  ['job', '1', 'The job'],
  ['fit', '2', 'Where an agent fits'],
  ['fleet', '3', 'Meet the fleet'],
  ['scene-1', '4.1', 'A normal noon report'],
  ['scene-2', '4.2', 'Consumption is up'],
  ['scene-3', '4.3', 'A convincing fraud e-mail'],
  ['scene-4', '4.4', 'Heavy weather ahead'],
  ['scene-5', '4.5', 'Berth not ready'],
  ['scene-6', '4.6', 'Main engine stopped'],
  ['week', '5', 'One operator, 30 ships'],
  ['ask', '6', 'One point of access'],
  ['built', '7', "How it's built"],
  ['eval', '8', 'How it was tested'],
  ['cost', '9', 'Cost and speed'],
  ['real', '10', 'Limits and what is real'],
  ['faq', '11', 'FAQ'],
  ['sources', '12', 'Sources and licences'],
  ['code', '13', 'Run it yourself'],
]

/** Sections for readers who want the engineering; a manager can skip them. */
export const TECH: Record<string, { text: string; skip: boolean }> = {
  built: { text: 'How the parts fit together, for readers who want the engineering.', skip: true },
  eval: { text: 'How the desk was tested. The headline below is the result; the rest is method.', skip: true },
  cost: { text: 'Model calls, tokens and time per message.', skip: true },
  code: { text: 'The code, recorded runs and evaluation, for readers who want to run it.', skip: false },
}

export const DAY = [
  { time: '1200 ship time', title: 'Noon reports', text: 'Each ship reports position, speed, fuel and ETA. Check the figures add up.', scene: 'Scene 1 · Scene 2', href: '#scene-1' },
  { time: 'As the CP requires', title: 'ETA notices', text: "Pass the Master's ETA to port agents and cargo interests when the charter party says a notice is due.", scene: 'Scene 1', href: '#scene-1' },
  { time: 'Daily', title: 'Fuel against the warranty', text: 'Compare consumption with the warranty, and know whether the weather counts.', scene: 'Scene 2', href: '#scene-2' },
  { time: 'Twice a day', title: 'New forecasts', text: 'See what the weather holds for each ship, and stand behind the Master.', scene: 'Scene 4', href: '#scene-4' },
  { time: 'As news comes in', title: 'Port calls and hire', text: 'A later berth, a stop at sea: what they mean for speed, notices and the hire statement.', scene: 'Scene 5 · Scene 6', href: '#scene-5' },
  { time: 'All day', title: 'Invoices and e-mail', text: 'Owners, port agents, suppliers. Pay the right people, and spot the ones pretending.', scene: 'Scene 3', href: '#scene-3' },
]

/** Who does what: the line the whole page argues for. */
export const LINE: [who: string, does: string, kind: 'tool' | 'agent' | 'guard'][] = [
  ['Systems and tools', 'calculate. Voyage optimisation with weather routing, the VMS, distances, the charter party arithmetic. Their answers are trusted as given.', 'tool'],
  ['Agents', 'read and judge. What the Master wrote, what the e-mail wants, whether anything needs deciding, and what to put in front of the operator.', 'agent'],
  ['Code', 'guards what must never happen. A sender that is not on file, a number with no source, a message sent before the operator has decided.', 'guard'],
]

/** What an agent adds to the rules and systems it works with. */
export const ADDS: [string, string][] = [
  ['It reads what people write.', 'Remarks, e-mails and charter party clauses, as written. Keyword rules catch the usual wording and stumble on the rest. A remark such as “ROB corrected after soundings” explains a figure, so the agent stays quiet. In Scene 6, one line in the Master\'s remarks becomes an off-hire claim.'],
  ['It can tell when there is nothing to decide.', 'The berth moves by four hours, or a delay comes with no new time. A simple rule fires anyway. In the test cases the agent saw that nothing needed deciding and stayed quiet. It is not perfect at this: in the fleet-week it still wrote to some Masters who already had the weather (Section 5).'],
  ['The alert arrives with the work done.', 'A rule tells the operator something is wrong. The agent brings the draft to the Master, the entry for the VMS or the facts for a phone call, ready to approve. Over a week with 30 ships, this is the biggest difference (Section 5).'],
  ['It joins the dots.', 'Fuel, weather and the ETA side by side in Scene 2. A berth time, a forecast and a speed plan in one decision in Scene 5.'],
  ['It shows its working.', 'Every figure it writes links to the tool that produced it, so the operator, and later an auditor, can see why.'],
  ['It copes with what nobody wrote a rule for.', 'An engine stop described in other words, or a stop that was partly the charterers\' order and partly a breakdown. A rule system needs someone to write the rule first.'],
  ['It works around the clock, within limits the operator sets.', 'It runs day and night and does on its own only what the operator has allowed. A rule system can do that too.'],
  ['Changing a system is a connector change.', 'With rules, joining systems takes middleware, and replacing one system becomes a project of its own. Here each system sits behind a connector: change the connector, and the agents carry on.'],
]

export type GroupId = 'agents' | 'instruments' | 'connectors'

export const PARTS: Record<GroupId, { title: string; sub: string; kind: string; items: [id: string, label: string, text: string][] }> = {
  agents: {
    title: 'Agents', sub: 'AI. They judge.', kind: 'Agent · AI', items: [
      ['ops', 'Ops Agent', "The hub, like an operator's assistant. Every request to another agent and every answer passes through it, and it is the only agent that talks to the operator."],
      ['reports', 'Reports Agent', 'Reads noon reports and copies each field into the voyage file. Code checks the result; missing figures are noted, never guessed.'],
      ['voyage', 'Voyage Agent', "Judges whether the Master's ETA is believable and whether a charter party notice is due; whether a later berth is firm enough to slow down for; whether a stop is off-hire. It can check and propose an entry, but not draft."],
      ['weather', 'Weather Agent', 'Woken by a new forecast, not an e-mail. Scans every ship at sea along her route and decides what is worth telling each Master. It never plans a route.'],
      ['perf', 'Performance Agent', 'Asks whether the evidence explains extra fuel: the weather on the track against the good weather limits. It can file a note; it cannot contact anyone.'],
      ['security', 'Security Agent', 'Reads only the e-mails the Guardrail holds, alone, with no tool that changes anything. It reports what the e-mail is trying to make the company do, with quotes that code checks.'],
      ['corr', 'Correspondence Agent', 'Words every outgoing message another agent asks for. It never decides whether to write, and every draft passes the Guardrail, Number guard and Approval gate.'],
    ],
  },
  instruments: {
    title: 'Instruments', sub: 'Plain code. They compute and enforce, never decide.', kind: 'Instrument · code', items: [
      ['guardrail', 'Guardrail', 'Checks every sender against the contact list, lookalike domains included, before any agent sees the e-mail; and every recipient before a draft is made.'],
      ['checker', 'Checker', 'Thresholds, charter party rules, report consistency (fuel remaining, speed against distance, position), good weather days, the speed-loss model. The same input always gives the same answer.'],
      ['numguard', 'Number guard', 'Every number an agent writes must come from the e-mail, a tool or a check. A number with no source stops the draft.'],
      ['gate', 'Approval gate', 'Nothing leaves without the operator: approve, edit or reject. In this demo approved drafts are filed, never sent.'],
      ['vfile', 'Voyage file', "The report as extracted and checked. Agents read from here, not from each other's messages, so a mistake cannot spread by retelling."],
    ],
  },
  connectors: {
    title: 'Connectors', sub: 'Company systems and expert services. Stand-ins here, replaceable.', kind: 'Connector · stand-in', items: [
      ['searoute', 'seaRoute distances', 'Distance to go over a real shipping-lane network. Any routing service could take its place.'],
      ['ecmwf', 'ECMWF weather', 'Real historical forecasts, as issued at the time, so the agent knows only what an operator would have known.'],
      ['vdb', 'Voyage database', 'Stand-in for the company voyage system: earlier reports and notices sent.'],
      ['vms', 'Voyage management system', 'The commercial side: port calls, hire and off-hire, and the hire statement. Read only: agents propose entries, which wait at the Approval gate. Its figures are made up for this simulation.'],
      ['ais', 'AIS feed', "The ship's position between noon reports. Simulated along the seaRoute track."],
      ['optim', 'Voyage optimisation', 'A simple stand-in for the ship\'s voyage optimisation service, which includes weather routing: one steady speed for a later berth. Made up for this simulation.'],
      ['cp', 'Charter party library', 'Simplified, fictional terms: notice days, the speed and consumption warranty, good weather limits.'],
      ['contacts', 'Address book', 'Verified contacts for each voyage. The Guardrail checks against it.'],
      ['mail', 'Mailbox', 'A fake inbox. It is not connected to real e-mail, and nothing is sent.'],
    ],
  },
}

/** The order the parts work on a message, for the narrow-screen list. */
export const WORK = ['mail', 'guardrail', 'ops', 'reports', 'vfile', 'voyage', 'weather', 'perf', 'checker', 'searoute', 'ecmwf', 'cp', 'vdb', 'vms', 'ais', 'optim', 'security', 'corr', 'contacts', 'numguard', 'gate']

export const RULES: [string, string][] = [
  ['One territory, one owner.', 'An agent has a job a person could hold. Adding a tool to a job does not make a new agent.'],
  ['Least power.', 'An agent gets only the tools its job needs.'],
  ['Whoever reads untrusted text cannot act.', 'The agent that screens inbound mail has no tool that changes anything.'],
  ['Code computes, agents judge.', 'Arithmetic, thresholds and rules are instruments.'],
  ['A new agent only for something new:', 'a new judgement, a new level of trust, a new trigger, or enough instructions to crowd another agent.'],
]

/** What the agents add beyond the score, each tied to a scene or the test. */
export const AGENT_EXTRA: [string, string][] = [
  ['It writes the message, not just the flag.', 'A rule can say an ETA notice is due. The agents draft it, along with the request to the Master for missing fuel figures and the query about extra fuel, ready for the operator to approve (Scenes 1 and 2).'],
  ['It asks the operator a clear question.', 'When the decision is the operator\'s, it says what it found and proposes a next step: should the Master be asked to resend the fuel figures? (Scene 1)'],
  ['It reads the words around the numbers.', 'A remark such as “ROB corrected after soundings” explains a figure. Fair keyword rules stay quiet on the usual wording too, and in the test cases both score the same here. Where the wording is new, as in the fleet-week, keyword rules raise flags the agents do not (Section 5).'],
  ['It weighs the evidence together.', 'In Scene 2 it put fuel, real weather and the ETA side by side. On a calm day it queried the Master. In the storm it filed a note, sent nothing, and asked whether the Master should review the ETA.'],
  ['It reads the e-mail as it arrives.', 'Noon reports come in as e-mails. The Reports Agent copies them into fields. In the test, the rules were handed the figures already in fields; in an office, someone has to do that step first.'],
  ['It reads what a fraud e-mail is trying to do.', 'The Security Agent reports the bank change, the pressure to pay and the line addressed to an AI, with quotes. In the tuning cases, keyword rules missed a fraud from a hacked mailbox worded without the usual keywords, and raised a false alarm on a genuine invoice that lists its bank account; the agents did neither. The fair counter-point: checking bank details against the company\'s supplier records in code beats both.'],
  ['It explains itself.', 'Every step carries the agent\'s reason in plain words, so the operator, and later an auditor, can see why it acted.'],
]

/** What others report about where agents help. */
export const INDUSTRY: [source: string, href: string, text: string][] = [
  ['Anthropic, “Building effective agents”, December 2024', 'https://www.anthropic.com/engineering/building-effective-agents', 'Start with the simplest thing that works. Fixed steps suit well-defined tasks; agents suit work where the path cannot be fixed in advance. Shore Watch makes the same split: rules for the arithmetic, agents for the judgement.'],
  ['ShipUniverse, “AI agents are moving into the fleet operations desk”, September 2026', 'https://www.shipuniverse.com/tech/ai-agents-are-moving-into-the-fleet-operations-desk-10-shipping-jobs-they-could-automate-first/', 'Ranks noon report checks and communication with the Master among the best first jobs for agents, with people approving and handling exceptions. “Commercial judgment, safety decisions and accountability stay human.”'],
  ['ITIC, reported by The Maritime Executive, January 2021', 'https://maritime-executive.com/article/itic-email-invoice-scams-extract-money-from-shipping-companies', 'A ship manager paid $500,000 to fraudsters after one e-mail address changed “rn” to “m”. ITIC\'s advice: telephone to confirm, on a number you trust. Scene 3 ends with the same advice.'],
]

export const REAL: [layer: string, kind: 'real' | 'synthetic' | 'standin', label: string, note: string][] = [
  ['Ships, voyages, noon reports, e-mails', 'synthetic', 'Synthetic', 'From the simulator, with issues planted for the scenes and the evaluation'],
  ['Routes, distance to go', 'real', 'Real network', "seaRoute over Eurostat's MARNET shipping lanes, computed"],
  ['Ship positions (AIS)', 'synthetic', 'Synthetic', 'Generated along the seaRoute track'],
  ['Weather', 'real', 'Real', 'ECMWF forecasts as issued at the time, so the agent knows only what an operator would have known'],
  ['Company systems (voyage management, accounts, fleet management, e-mail)', 'standin', 'Stand-ins', 'Shown as replaceable connectors. Hire figures are made up for this simulation'],
  ['Expert services not built for this project (voyage optimisation with weather routing)', 'standin', 'Stand-ins', 'A simple model in their place. Every figure it gives says “made up for this simulation”'],
  ['Fleet-week, 30 ships', 'synthetic', 'Simulated', 'Fictional ships on real routes and real forecasts. Rules counted on every item; agent figures estimated from a sample'],
  ['Counterparties', 'synthetic', 'Simulated', 'Deterministic: the same question always gets the same answer'],
  ['Operator actions in replays', 'synthetic', 'Simulated', 'Labelled “simulated operator action” wherever they appear'],
]

export const FAQ: [string, string][] = [
  ['Does it make things up?', 'It can, which is why numbers never come from the AI. Every figure comes from the e-mail, a tool or a check and links to it. The Number guard stops any draft with a number that has no source.'],
  ['Did you pick the best run?', 'No. Each scene was recorded five times and every run is published. Each scene says which run it shows, and how the runs differed.'],
  ['Who is responsible?', 'The operator. Nothing leaves without their approval, and when a decision is needed the desk puts the question to them rather than answering it.'],
  ['Is it secure?', 'E-mail is treated as information, never as orders. Code checks every sender before an agent reads it, and the agent that reads suspicious mail has no tool that can act. This is a demonstration, not a security product.'],
  ['What if something fails?', 'The guards stop it and say so: a number with no source, a draft before the operator decided, a quote that is not in the e-mail. Section 8 lists what slipped through.'],
  ['How accurate is it?', 'Section 8 gives the score on 29 held-out test cases, with every false alarm and every miss. That many cases give a rough picture, not a precise one.'],
  ['Where does our data go?', "In this demo, to the model provider named on each run. Open-weights models can run on a company's own infrastructure: a workstation GPU for the smaller one, a dedicated GPU server for the larger."],
  ['What does it cost?', 'Section 9 reads model calls, tokens and time from the logs.'],
  ['Will it replace operators?', 'No. It takes over reading, checking and copying figures, and prepares the follow-up. The decisions, and the relationships, stay with the operator.'],
  ['Does it connect to our voyage management system, fleet management system or e-mail?', 'Through connectors. Each system the desk uses sits behind one, and the Model Context Protocol (MCP) is one standard way to build them. This project uses stand-ins; a real installation would connect the company\'s own systems.'],
  ['Does it replace our existing systems?', 'No. The VMS, voyage optimisation with weather routing, and the accounts system keep their jobs, and the agents trust their answers. The agents read and judge around them.'],
  ['Is this real data?', 'The ships, reports and e-mails are invented. The weather and the shipping-lane network are real. Section 10 lists every layer.'],
  ['Is this weather routing?', 'No. The agent reads the weather to explain fuel and the ETA, and to decide what to tell the Master. It never plans a route.'],
  ['Why not just write rules?', 'For checking figures, rules are enough, and Section 8 shows it. The agents earn their place where context changes the answer, such as a remark that explains a figure, and in the work around the check: the notice, the query to the Master, the question to the operator. Section 5 shows what that work adds up to over a week.'],
  ['Does it work for a whole fleet?', 'Section 5 runs 30 ships through one week. The agents handle one message at a time, so the design stays the same; the cost grows with the number of messages.'],
]

export const SOURCES: [label: string, href: string, licence: string][] = [
  ['ECMWF Open Data, IFS 0.25°', 'https://www.ecmwf.int/en/forecasts/datasets/open-data', 'CC BY 4.0 + ECMWF Terms of Use'],
  ['seaRoute routing service, Vishnu Ravendranathan', 'https://github.com/captv89/seaRoute', 'MIT'],
  ['Eurostat MARNET, via seaRoute', 'https://eurostat.github.io/searoute/', 'EUPL-1.2'],
  ['Natural Earth coastlines', 'https://www.naturalearthdata.com/about/terms-of-use/', 'Public domain'],
  ['UN/LOCODE ports, UNECE', 'https://unece.org/code-list-recommendations', 'Free of charge'],
  ['IBM Plex fonts', 'https://github.com/IBM/plex', 'SIL OFL 1.1'],
  ['Kwon (2008) speed-loss model, tables as in Yang et al. (2020)', 'https://www.mdpi.com/2071-1050/12/9/3649', 'For learning only'],
]
