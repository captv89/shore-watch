// Command eval runs the eval's tasks through the desk and scores them by
// code. Each task's event log is kept, like every scene run
// (principle 4), and the results go to eval/results for the page.
//
//	go run ./cmd/eval -set tuning                        (Gemini; needs GOOGLE_API_KEY)
//	go run ./cmd/eval -set heldout -model nvidia/nemotron-3-super-120b-a12b -rpm 30
//	go run ./cmd/eval -set heldout -baselines            (no model calls)
//	go run ./cmd/eval -set tuning -only rob-mgo-up,remark-engine-stop
//	go run ./cmd/eval -set heldout -kinds forecast,berth,engine_stop -model gemma-4-31b-it -rpm 5
//	go run ./cmd/eval -set fleetweek -baselines           (every fleet-week item)
//	go run ./cmd/eval -set fleetweek -model nvidia/nemotron-3.5-super-vl-120b-a12b -rpm 10   (the sample)

// -only and -kinds run part of a set and merge it into the set's results
// file: the tasks run replace their earlier results, the rest are kept.
package main

import (
	"bufio"
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"time"

	"github.com/captv89/seaRoute/route"
	"github.com/captv89/shorewatch/agent/desk"
	"github.com/captv89/shorewatch/eval"
	"github.com/captv89/shorewatch/internal/llm"
	"github.com/captv89/shorewatch/sim"
	"github.com/captv89/shorewatch/tools/weather"
)

// TaskResult is one task's outcome, as the page shows it.
type TaskResult struct {
	eval.Task
	Decisions    eval.Decisions `json:"decisions"`
	Grade        eval.Grade     `json:"grade"`
	ModelCalls   int            `json:"model_calls"`
	InputTokens  int            `json:"input_tokens"`
	OutputTokens int            `json:"output_tokens"`
	WallMS       int64          `json:"wall_ms"`
	Log          string         `json:"log,omitempty"`
	// PromptVersion is the prompt the task ran on, when a results file
	// merges runs on different prompts.
	PromptVersion string `json:"prompt_version,omitempty"`
	Error         string `json:"error,omitempty"`
}

// Results is one model's or baseline's run over one task set.
type Results struct {
	Model         string       `json:"model"`
	Set           string       `json:"set"`
	PromptVersion string       `json:"prompt_version,omitempty"`
	RecordedAt    time.Time    `json:"recorded_at"`
	Passed        int          `json:"passed"`
	Total         int          `json:"total"`
	Tasks         []TaskResult `json:"tasks"`
	// order is each task's place in the set, so merged results keep it.
	order map[string]int
}

func main() {
	set := flag.String("set", eval.Tuning, "task set: tuning, heldout, fleetweek or fleetweek-tuning")
	modelID := flag.String("model", "gemini-3.1-flash-lite", "Gemini model ID, or vendor/model for NVIDIA NIM")
	baselines := flag.Bool("baselines", false, "score the do-nothing and rules-only baselines instead of a model")
	only := flag.String("only", "", "comma-separated task IDs to run (tuning only)")
	kinds := flag.String("kinds", "", "comma-separated task kinds to run")
	rpm := flag.Int("rpm", 12, "model requests per minute")
	maxCalls := flag.Int("max-calls", 100, "model call budget per task")
	resume := flag.Bool("resume", false, "score a task from its existing log instead of running it again")
	network := flag.String("network", "third_party/seaRoute/dataset/marnet_plus_5km.geojson", "MARNET GeoJSON")
	flag.Parse()
	if err := run(*set, *modelID, *baselines, *only, *kinds, *rpm, *maxCalls, *network, *resume); err != nil {
		log.Fatal(err)
	}
}

func run(set, modelID string, baselines bool, only, kinds string, rpm, maxCalls int, network string, resume bool) error {
	g, err := route.Load(network)
	if err != nil {
		return err
	}
	fleet := strings.HasPrefix(set, eval.FleetWeek)
	var all []eval.Task
	if fleet {
		all, err = eval.BuildFleetWeek(g, ".")
	} else {
		all, err = eval.Build(g, ".")
	}
	if err != nil {
		return err
	}
	var tasks []eval.Task
	for _, t := range all {
		// The agents run only the fleet-week's sample; the baselines run it all.
		if set == eval.FleetWeek && !baselines && !t.Sample {
			continue
		}
		if t.Set == set && (only == "" || slices.Contains(strings.Split(only, ","), t.ID)) && (kinds == "" || slices.Contains(strings.Split(kinds, ","), t.Kind)) {
			tasks = append(tasks, t)
		}
	}
	if only != "" && set == eval.HeldOut {
		// The held-out score is reported only for a full run of the set.
		return fmt.Errorf("-only is for the tuning set")
	}
	if len(tasks) == 0 {
		return fmt.Errorf("no tasks")
	}

	if baselines {
		for _, b := range []string{eval.DoNothing, eval.RulesOnly, eval.RulesCoded} {
			res := Results{Model: b, Set: set, RecordedAt: time.Now().UTC()}
			for _, t := range tasks {
				d := eval.Baseline(b, t)
				res.add(TaskResult{Task: t, Decisions: d, Grade: eval.Score(t, d)})
			}
			if err := res.write(); err != nil {
				return err
			}
		}
		return nil
	}

	ctx := context.Background()
	m, err := llm.New(ctx, modelID, rpm)
	if err != nil {
		return err
	}
	wx, err := weather.Load(eval.WeatherS2, eval.WeatherS2)
	if err != nil {
		return err
	}
	// Forecast runs are loaded once each, as the tasks name them.
	forecasts := map[string]*weather.Grid{}
	res := Results{Model: modelID, Set: set, PromptVersion: desk.PromptVersion, RecordedAt: time.Now().UTC(), order: map[string]int{}}
	for i, t := range all {
		res.order[t.ID] = i
	}
	if only != "" || kinds != "" {
		// A part run: keep the earlier results of the tasks not run now.
		if err := res.keep(tasks); err != nil {
			return err
		}
	}
	for _, t := range tasks {
		ep := t.Episode
		ep.ID = "eval-" + t.ID
		cfg := desk.Config{
			Episodes: []sim.Scene{ep}, Graph: g, Model: m, ModelID: modelID, Attempt: 1, TotalAttempts: 1,
			OutDir: filepath.Join("eval", "runs", set), MaxModelCalls: maxCalls,
			ToolVersions: map[string]string{"adk-go": "v2.6.0", "seaRoute": "shorewatch branch", "network": "Eurostat marnet_plus_5km"},
		}
		if t.Weather != "" {
			cfg.Weather = wx
		}
		if t.Forecast != "" {
			if forecasts[t.Forecast] == nil {
				if forecasts[t.Forecast], err = weather.Load(t.Forecast, t.Forecast); err != nil {
					return err
				}
			}
			cfg.Forecast = forecasts[t.Forecast]
		}
		var tr TaskResult
		if done, _ := filepath.Glob(filepath.Join(cfg.OutDir, ep.ID+"-*-"+slug(modelID)+"-r1.jsonl")); resume && len(done) == 1 {
			tr = TaskResult{Task: t, Log: done[0]}
			tr.ModelCalls, tr.WallMS = logStats(done[0])
		} else {
			start := time.Now()
			out, err := desk.Record(ctx, cfg)
			tr = TaskResult{Task: t, Log: out.LogPath, ModelCalls: out.ModelCalls, WallMS: time.Since(start).Milliseconds()}
			if err != nil {
				// A failed run is scored on what it did before failing.
				tr.Error = err.Error()
				tr.ModelCalls, _ = logStats(out.LogPath)
			}
		}
		if tr.Log != "" {
			d, err := eval.ReadDecisions(tr.Log)
			if err != nil {
				tr.Error = strings.TrimSpace(tr.Error + " " + err.Error())
			}
			tr.Decisions = d
			tr.InputTokens, tr.OutputTokens = tokens(tr.Log)
		}
		tr.Grade = eval.Score(t, tr.Decisions)
		tr.PromptVersion = desk.PromptVersion
		log.Printf("%-34s pass=%-5v missed=%v false=%v calls=%d %s", t.ID, tr.Grade.Pass, tr.Grade.Missed, tr.Grade.FalseAlarms, tr.ModelCalls, tr.Error)
		res.add(tr)
		// Write after every task, so a stopped run keeps what it did.
		if err := res.write(); err != nil {
			return err
		}
	}
	return nil
}

// keep loads the set's results file, if any, and keeps the results of the
// tasks not about to run, each with the prompt it ran on.
func (r *Results) keep(run []eval.Task) error {
	b, err := os.ReadFile(r.path())
	if os.IsNotExist(err) {
		return nil
	} else if err != nil {
		return err
	}
	var old Results
	if err := json.Unmarshal(b, &old); err != nil {
		return err
	}
	for _, t := range old.Tasks {
		if slices.ContainsFunc(run, func(n eval.Task) bool { return n.ID == t.ID }) {
			continue
		}
		if t.PromptVersion == "" {
			t.PromptVersion = old.PromptVersion
		}
		r.add(t)
	}
	return nil
}

func (r *Results) path() string {
	return filepath.Join("eval", "results", fmt.Sprintf("%s-%s.json", r.Set, slug(r.Model)))
}

func (r *Results) add(t TaskResult) {
	r.Tasks = append(r.Tasks, t)
	r.Total++
	if t.Grade.Pass {
		r.Passed++
	}
}

func (r *Results) write() error {
	path := r.path()
	if r.order != nil {
		slices.SortStableFunc(r.Tasks, func(a, b TaskResult) int { return r.order[a.ID] - r.order[b.ID] })
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	b, err := json.MarshalIndent(r, "", "  ")
	if err != nil {
		return err
	}
	log.Printf("%s: %d of %d pass", path, r.Passed, r.Total)
	return os.WriteFile(path, append(b, '\n'), 0o644)
}

// tokens adds up a log's token counts.
func tokens(path string) (in, out int) {
	f, err := os.Open(path)
	if err != nil {
		return 0, 0
	}
	defer f.Close()
	sc := bufio.NewScanner(f)
	sc.Buffer(make([]byte, 0, 1<<20), 16<<20)
	for sc.Scan() {
		var e struct {
			Cost *struct {
				In  int `json:"input_tokens"`
				Out int `json:"output_tokens"`
			} `json:"cost"`
		}
		if json.Unmarshal(sc.Bytes(), &e) == nil && e.Cost != nil {
			in += e.Cost.In
			out += e.Cost.Out
		}
	}
	return in, out
}

// slug is a model ID as it appears in file names (desk's run IDs).
func slug(id string) string { return strings.NewReplacer("/", "-", ".", "-", ":", "-").Replace(id) }

// logStats reads a kept log's model calls (events with a cost) and length.
func logStats(path string) (calls int, wallMS int64) {
	f, err := os.Open(path)
	if err != nil {
		return 0, 0
	}
	defer f.Close()
	sc := bufio.NewScanner(f)
	sc.Buffer(make([]byte, 0, 1<<20), 16<<20)
	for sc.Scan() {
		var e struct {
			WallMS int64 `json:"wall_ms"`
			Cost   *struct {
				In int `json:"input_tokens"`
			} `json:"cost"`
		}
		if json.Unmarshal(sc.Bytes(), &e) == nil {
			if e.Cost != nil && e.Cost.In > 0 {
				calls++
			}
			wallMS = max(wallMS, e.WallMS)
		}
	}
	return calls, wallMS
}
