# Shore Watch

An agentic desk for vessel operations, on fictional ships. The agents read noon reports and e-mails, call tools for distance, weather and charter party terms, and draft what an operator would send. Code computes every number; the model explains it; a person approves everything that goes out. Every step is written to an event log.

The article built from this code is at [huggingface.co/spaces/captv89/shore-watch](https://huggingface.co/spaces/captv89/shore-watch). Every scene on it is a replay of a recorded run in this repository.

Synthetic data. Not a product. Nothing is ever sent.

## Layout

| Folder | What lives there |
|---|---|
| `sim/` | Simulated world: ships, voyages, noon reports, counterparties, clock |
| `tools/` | Tools the agents call (weather on track, speed advice, off-hire, voyage data) |
| `agent/` | The desk and its agents, prompts, checker (rules and numbers), event log |
| `internal/llm/` | Model clients: Gemini, NVIDIA NIM and OpenRouter |
| `cmd/record` | Records a scene against a live model |
| `cmd/eval` | Runs and scores the evaluation tasks |
| `cmd/fleetweek` | Builds the 30-ship fleet-week and its summary |
| `runs/` | Recorded scene runs (event logs and run headers); the page reads these |
| `eval/` | Tasks, answer keys, scorer, baselines; `eval/runs` and `eval/rounds` hold every eval log, `eval/results` the scores |
| `data/` | Prepared ECMWF weather subsets, licence note, and the Python scripts that make them |
| `web/` | The article and replay player (TypeScript, React, Vite) |
| `third_party/seaRoute/` | Sea routing over MARNET ([captv89/seaRoute](https://github.com/captv89/seaRoute)), as a git submodule |

## Requirements

- Go 1.26.6 or later
- Node.js 24 or later, with npm
- Python 3 with `eccodes`, `numpy` and `requests`, only to fetch and prepare new weather data

## Build and test

```sh
git clone --recurse-submodules https://github.com/captv89/shore-watch.git
cd shore-watch
cd web && npm ci && cd ..
make build   # Go packages and the web page (web/dist)
make test    # Go tests and the page's replay tests
```

In an existing clone without the submodule, run `git submodule update --init` first.

## View the article locally

```sh
make web-dev   # http://localhost:5173
```

The page makes no live model calls. It reads the logs in `runs/` and the scores in `eval/results/` at build time. Everything it shows is derived from the events in `web/src/replay/derive.ts`.

Links can open an exact step: `?run=2&event=25&view=inside` (views: desk, inside, split). Keys: ← → step, Space play, 1 2 3 views, P presenter mode, Esc close.

To serve the built page, run `cd web && npm run preview`, or host `web/dist/` on any static web server.

## Record a scene yourself

You need a key for one model provider. Keep keys in your shell or a local `.env`; `.env` is gitignored.

| Model ID | Provider | Key |
|---|---|---|
| `gemini-3.1-flash-lite` (default) | Google AI Studio | `GOOGLE_API_KEY` |
| `vendor/model`, e.g. `nvidia/nemotron-3-super-120b-a12b` | NVIDIA NIM | `NVIDIA_API_KEY` |
| `openrouter/vendor/model`, e.g. `openrouter/nvidia/nemotron-3.5-super-vl:free` | OpenRouter | `OPEN_ROUTER_API_KEY` |

```sh
export GOOGLE_API_KEY=...
go run ./cmd/record -scene s1 -attempts 1 -out runs-local
```

Scenes are `s1` to `s6`. Useful flags:

- `-max-calls` caps model calls per run (Scene 1 takes about 13–18).
- `-rpm` keeps under the provider's rate limit.
- `-first 3 -attempts 1 -total 5` re-records attempt 3 of a set of 5.

Run `go run ./cmd/record -h` for the full list.

## Run the evaluation

```sh
go run ./cmd/eval -baselines                    # score the do-nothing and rules-only baselines
go run ./cmd/eval -set tuning -model gemini-3.1-flash-lite
go run ./cmd/eval -set heldout -model openrouter/nvidia/nemotron-3.5-super-vl:free -rpm 10
```

Sets: `tuning`, `heldout`, `fleetweek`, `fleetweek-tuning`. Grading is by code, against the answer keys in `eval/`; no model marks another. Each task's event log is kept under `eval/runs/`. `-resume` scores a task from its existing log instead of running it again.

## Fleet-week

```sh
go run ./cmd/fleetweek -summary   # writes eval/results/fleet-week.json
```

`-jobs` writes the forecast boxes that `data/ecmwf/prepare.py --batch` cuts from ECMWF open data. Those grids are rebuilt locally and not committed, except the ones the recorded sample runs used.

## Weather data

`data/weather/` holds the prepared subsets the scenes use. To make new ones, `data/ecmwf/fetch.py` downloads ECMWF IFS open data and `data/ecmwf/prepare.py` crops and resamples it. Raw downloads go to `data/ecmwf/raw/` and stay out of git. Weather data: © ECMWF, CC BY 4.0 (`data/weather/LICENCE-ECMWF.txt`).

## Taking it further: product or live use

Shore Watch is a demonstration, not a product. It runs on a fictional world, and nothing in it touches a real ship, mailbox or company system. To build on it, plan for the work below. The parts worth keeping are the overall pattern: code computes the numbers, the model explains them, every step is logged, and a person approves everything that goes out.

**What is simulated today, and where**

| Part | Where | In a live system |
|---|---|---|
| Ships, voyages, noon reports, history | `sim/` | Your voyage management system and the ship's actual reports |
| Mailbox | `read_email` tool in `agent/desk/tools.go` | A read-only connector to the operations mailbox |
| Voyage history, CP terms, contacts | `tools/data.go` (MCP stand-ins) | Your voyage database, CP library and address book, behind the same tool shapes |
| Distance to go | seaRoute (`third_party/seaRoute`) | Real. Check routes against your own passage planning and restrictions |
| Operator decisions | `Decision` in `agent/desk/desk.go` (approves by default) | A real approval screen used by a named operator |
| Charter party rules and tolerances | `agent/checker` (fictional) | Your company's rules and each fixture's actual CP clauses |

**Steps before live use**

1. **Replace the connectors, not the agent.** Rewrite the tools in `tools/` and the mail reader against your own systems. Keep the same inputs and outputs, so prompts and checks keep working. Start every connector read-only.
2. **Put your own rules into the checker.** ETA tolerances, notice periods and change thresholds go in `agent/checker` as plain code with tests. Real CPs differ by fixture. Someone has to read each fixture and confirm the terms before the agent uses them.
3. **Keep the number guard.** Every number in a reason or a draft must come from the e-mail, a tool or a check (`agent/checker/numbers.go`). Don't relax this to make drafts read more smoothly.
4. **Build a real approval gate.** Replace the simulated operator with a screen where a named person approves, edits or rejects each draft. Record who decided and when. Keep sending as a separate step outside the agent. The agent should never hold send rights.
5. **Harden the guardrail.** Shore Watch's guardrail is built for the demonstration. Any system that reads outside e-mail needs full checks (lookalike domains, bank-detail changes, injected instructions in e-mails, recipient checks) and a security review before go-live.
6. **Decide where the data goes.** Noon reports and e-mails pass through the model provider. Get a data-processing agreement in place, or run an open-weights model on your own infrastructure. Treat crew names and e-mail addresses as personal data (GDPR).
7. **Measure before trusting it.** Build a test set from your own past reports with known answers, and score the agent against it, as `eval/` does here. Track misses and false alarms per model. Run the test set again after every prompt or model change.
8. **Use the event log as the audit trail.** Each run writes every step to a log (`agent/eventlog`). Store these logs, set a retention period, and make them searchable for incident reviews.
9. **Run it like a service.** Add budgets per run (`-max-calls`), retries with limits, alerts when a run fails or stalls, and a fallback: when the agent can't finish, the report goes to the operator untouched.

This code is offered "as is", without warranty. It has not been reviewed for production, security or regulatory use.

## Licence

Code: Apache-2.0, see [LICENSE](LICENSE). Weather data: CC BY 4.0, see `data/weather/LICENCE-ECMWF.txt`. seaRoute: MIT; its MARNET network data carries its own licence, see the submodule.
