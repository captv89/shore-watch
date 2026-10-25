package main

import (
	"encoding/json"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"time"

	"github.com/captv89/shorewatch/eval"
)

// result is the part of a cmd/eval task result the summary reads.
type result struct {
	ID        string         `json:"id"`
	Kind      string         `json:"kind"`
	At        time.Time      `json:"at"`
	Sample    bool           `json:"sample"`
	Key       eval.Key       `json:"key"`
	Decisions eval.Decisions `json:"decisions"`
	Grade     eval.Grade     `json:"grade"`
	Calls     int            `json:"model_calls"`
	In        int            `json:"input_tokens"`
	Out       int            `json:"output_tokens"`
	Error     string         `json:"error"`
}

type results struct {
	Model         string   `json:"model"`
	Set           string   `json:"set"`
	PromptVersion string   `json:"prompt_version"`
	Tasks         []result `json:"tasks"`
}

// Tally is what reached the operator, item by item: an item reaches the
// operator when the desk raises anything on it (a question, a draft, an
// entry, an alert) or holds an e-mail.
type Tally struct {
	Items int `json:"items"`
	// Reached: items that reached the operator; Needed: of those, ones the
	// key says need the operator; Noise: ones that did not.
	Reached float64 `json:"reached"`
	Needed  float64 `json:"needed"`
	Noise   float64 `json:"noise"`
	// Missed: items that needed the operator and did not reach them.
	Missed float64 `json:"missed"`
	// Wrong: needed items that reached the operator but with part of the
	// work wrong or left out (a topic missed, a wrong off-hire period).
	Wrong float64 `json:"wrong"`
	// Prepared: needed actions (key topics) that came to the operator
	// drafted, ready to approve. ToWrite: needed actions the operator must
	// work out and write. Rules desks draft nothing.
	Prepared float64 `json:"prepared"`
	ToWrite  float64 `json:"to_write"`
}

func (t *Tally) add(o Tally, w float64) {
	t.Items += o.Items
	t.Reached += o.Reached * w
	t.Needed += o.Needed * w
	t.Noise += o.Noise * w
	t.Missed += o.Missed * w
	t.Wrong += o.Wrong * w
	t.Prepared += o.Prepared * w
	t.ToWrite += o.ToWrite * w
}

func (t *Tally) round() {
	for _, v := range []*float64{&t.Reached, &t.Needed, &t.Noise, &t.Missed, &t.Wrong, &t.Prepared, &t.ToWrite} {
		*v = math.Round(*v*10) / 10
	}
}

// tallyOne scores one item for a desk; drafts says whether the desk drafts
// what it raises (the agents) or leaves it to the operator (rules).
func tallyOne(r result, drafts bool) Tally {
	t := Tally{Items: 1}
	needed := len(r.Key.Topics) > 0 || r.Key.Block
	reached := len(r.Decisions.Topics) > 0 || r.Decisions.Blocked
	actions := float64(len(r.Key.Topics))
	if r.Key.Block {
		actions = 1
	}
	switch {
	case reached && needed:
		t.Reached, t.Needed = 1, 1
		if !r.Grade.Pass {
			t.Wrong = 1
		}
		if drafts {
			done := 0.0
			for _, k := range r.Key.Topics {
				if slices.Contains(r.Decisions.Topics, k) {
					done++
				}
			}
			if r.Key.Block && r.Decisions.Blocked {
				done = 1
			}
			t.Prepared, t.ToWrite = done, actions-done
		} else {
			t.ToWrite = actions
		}
	case reached:
		t.Reached, t.Noise = 1, 1
	case needed:
		t.Missed = 1
	}
	return t
}

// Desk is one desk's week: counted on every item (rules), or estimated from
// the sample (agents), by item kind and by day.
type Desk struct {
	Name          string           `json:"name"`
	PromptVersion string           `json:"prompt_version,omitempty"`
	Estimate      bool             `json:"estimate"`
	SampleItems   int              `json:"sample_items,omitempty"`
	Week          Tally            `json:"week"`
	ByKind        map[string]Tally `json:"by_kind"`
	ByDay         map[string]Tally `json:"by_day"`
	// Sample is the agents' own record on the sample items, not scaled.
	Sample       *Tally `json:"sample,omitempty"`
	ModelCalls   int    `json:"sample_model_calls,omitempty"`
	InputTokens  int    `json:"sample_input_tokens,omitempty"`
	OutputTokens int    `json:"sample_output_tokens,omitempty"`
	Errors       int    `json:"sample_errors,omitempty"`
}

// Summary is eval/results/fleet-week.json, for the page.
type Summary struct {
	Note   string         `json:"note"`
	Items  int            `json:"items"`
	ByKind map[string]int `json:"items_by_kind"`
	ByDay  map[string]int `json:"items_by_day"`
	Desks  []Desk         `json:"desks"`
}

func load(path string) (results, error) {
	var r results
	b, err := os.ReadFile(path)
	if err != nil {
		return r, err
	}
	return r, json.Unmarshal(b, &r)
}

func dayOf(t time.Time) string { return t.UTC().Format("2006-01-02") }

// summarise counts the rules desks on every item, and scales each model's
// sample up to the week, kind by kind: an item of a kind is taken to go as
// the sampled items of that kind went on average.
func summarise() error {
	week, err := load(filepath.Join("eval", "results", "fleetweek-rules-only.json"))
	if err != nil {
		return fmt.Errorf("%w (run go run ./cmd/eval -set fleetweek -baselines first)", err)
	}
	s := Summary{
		Note:   "Fleet-week, 30 ships, 20–26 Jan 2025. Rules desks: counted on every item. Agents: estimated from the recorded sample, kind by kind; see sample for what they did on the items they ran.",
		ByKind: map[string]int{}, ByDay: map[string]int{},
	}
	for _, t := range week.Tasks {
		s.Items++
		s.ByKind[t.Kind]++
		s.ByDay[dayOf(t.At)]++
	}
	for _, b := range []string{eval.RulesOnly, eval.RulesCoded} {
		r, err := load(filepath.Join("eval", "results", "fleetweek-"+b+".json"))
		if err != nil {
			return err
		}
		d := Desk{Name: b, ByKind: map[string]Tally{}, ByDay: map[string]Tally{}}
		for _, t := range r.Tasks {
			o := tallyOne(t, false)
			d.Week.add(o, 1)
			k, dy := d.ByKind[t.Kind], d.ByDay[dayOf(t.At)]
			k.add(o, 1)
			dy.add(o, 1)
			d.ByKind[t.Kind], d.ByDay[dayOf(t.At)] = k, dy
		}
		s.Desks = append(s.Desks, d)
	}

	files, _ := filepath.Glob(filepath.Join("eval", "results", "fleetweek-*.json"))
	for _, f := range files {
		base := strings.TrimSuffix(filepath.Base(f), ".json")
		if strings.HasPrefix(base, "fleetweek-tuning") || slices.Contains([]string{"fleetweek-do-nothing", "fleetweek-rules-only", "fleetweek-rules-coded"}, base) {
			continue
		}
		r, err := load(f)
		if err != nil {
			return err
		}
		d := Desk{Name: r.Model, PromptVersion: r.PromptVersion, Estimate: true, ByKind: map[string]Tally{}, ByDay: map[string]Tally{}, Sample: &Tally{}}
		// The sample's average outcome for each kind.
		perKind := map[string]Tally{}
		n := map[string]int{}
		for _, t := range r.Tasks {
			o := tallyOne(t, true)
			d.Sample.add(o, 1)
			k := perKind[t.Kind]
			k.add(o, 1)
			perKind[t.Kind] = k
			n[t.Kind]++
			d.SampleItems++
			d.ModelCalls += t.Calls
			d.InputTokens += t.In
			d.OutputTokens += t.Out
			if t.Error != "" {
				d.Errors++
			}
		}
		for _, t := range week.Tasks {
			if n[t.Kind] == 0 {
				return fmt.Errorf("%s: no sampled item of kind %s", r.Model, t.Kind)
			}
			avg := perKind[t.Kind]
			o := Tally{Items: 1}
			o.add(avg, 1/float64(n[t.Kind]))
			o.Items = 1
			d.Week.add(o, 1)
			k, dy := d.ByKind[t.Kind], d.ByDay[dayOf(t.At)]
			k.add(o, 1)
			dy.add(o, 1)
			d.ByKind[t.Kind], d.ByDay[dayOf(t.At)] = k, dy
		}
		d.Sample.round()
		s.Desks = append(s.Desks, d)
	}
	for i := range s.Desks {
		d := &s.Desks[i]
		d.Week.round()
		for k, v := range d.ByKind {
			v.round()
			d.ByKind[k] = v
		}
		for k, v := range d.ByDay {
			v.round()
			d.ByDay[k] = v
		}
	}
	b, err := json.MarshalIndent(s, "", "  ")
	if err != nil {
		return err
	}
	out := filepath.Join("eval", "results", "fleet-week.json")
	if err := os.WriteFile(out, append(b, '\n'), 0o644); err != nil {
		return err
	}
	for _, d := range s.Desks {
		w := d.Week
		est := ""
		if d.Estimate {
			est = fmt.Sprintf(" (estimate from %d sampled items)", d.SampleItems)
		}
		fmt.Printf("%-40s reached %5.1f  needed %5.1f  noise %4.1f  missed %4.1f  wrong %4.1f  prepared %5.1f  to write %5.1f%s\n",
			d.Name, w.Reached, w.Needed, w.Noise, w.Missed, w.Wrong, w.Prepared, w.ToWrite, est)
	}
	fmt.Println(out)
	return nil
}
