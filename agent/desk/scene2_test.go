package desk

import (
	"context"
	"os"
	"slices"
	"strings"
	"testing"

	"github.com/captv89/shorewatch/agent/eventlog"
	"github.com/captv89/shorewatch/sim"
	"github.com/captv89/shorewatch/tools/weather"
)

func TestScene2Pipeline(t *testing.T) {
	events, res, m := recordScene2(t, &staff{})
	checkScene2(t, events, res, m)
}

// TestScene2OperatorDecidesFirst: an Ops Agent that skips the question is
// stopped by code, then asks the operator, and only then gets a draft.
func TestScene2OperatorDecidesFirst(t *testing.T) {
	events, res, _ := recordScene2(t, &staff{skipQuestion: true})
	stop := slices.IndexFunc(events, func(e eventlog.Event) bool {
		return strings.HasPrefix(e.Summary, "Guardrail: draft stopped, the operator has not yet decided")
	})
	q := slices.IndexFunc(events, func(e eventlog.Event) bool { return e.Type == eventlog.TypeQuestion })
	d := slices.IndexFunc(events, func(e eventlog.Event) bool { return e.Type == eventlog.TypeDraft && strings.Contains(e.Summary, "ETA") })
	if stop < 0 || q < stop || d < q {
		t.Errorf("stop at %d, question at %d, ETA draft at %d: want stop, then question, then draft", stop, q, d)
	}
	if len(res.Drafts) != 2 {
		t.Errorf("drafts: %+v", res.Drafts)
	}
}

func recordScene2(t *testing.T, m *staff) ([]eventlog.Event, Result, *staff) {
	t.Helper()
	scene1(t) // loads the graph
	eps, err := sim.Scene2(graph)
	if err != nil {
		t.Fatal(err)
	}
	wx, err := weather.Load("../../data/weather/s2-tormar-spring.json", "data/weather/s2-tormar-spring.json")
	if err != nil {
		t.Fatal(err)
	}
	fc, err := weather.Load("../../data/weather/s2-forecast-20250124T00.json", "data/weather/s2-forecast-20250124T00.json")
	if err != nil {
		t.Fatal(err)
	}
	res, err := Record(context.Background(), Config{
		Episodes: eps, Graph: graph, Weather: wx, Forecast: fc, Model: m, ModelID: "scripted", Attempt: 1, TotalAttempts: 1, OutDir: t.TempDir(), MaxModelCalls: 60,
		Operator: AddLine("master@tormarspring.sim-fleet.example", "Please reply before tomorrow's noon report.", "adds one line"),
	})
	if err != nil {
		if b, rerr := os.ReadFile(res.LogPath); rerr == nil {
			t.Log(string(b))
		}
		t.Fatal(err)
	}
	f, err := os.Open(res.LogPath)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	events, err := eventlog.Read(f)
	if err != nil {
		t.Fatal(err)
	}
	for _, e := range events {
		t.Logf("%2d %s %-10s %-18s %s", e.Seq, e.SimTime.Format("02 1504"), e.Actor, e.Type, e.Summary)
	}
	return events, res, m
}

func checkScene2(t *testing.T, events []eventlog.Event, res Result, m *staff) {
	t.Helper()
	has := func(prefix string) bool {
		return slices.ContainsFunc(events, func(e eventlog.Event) bool { return strings.HasPrefix(e.Summary, prefix) })
	}
	for _, want := range []string{
		"Consumption flagged: 27.6 t VLSFO is +15.0%", "Good weather day under the CP", "Alert to the operator:",
		"Operator edits the draft", "Consumption flagged: 28.3 t VLSFO is +17.9%", "Not a good weather day under the CP", "Note in MV TORMAR SPRING's voyage file:",
		"Forecast ETA at risk:", "Question to the operator: The forecast puts", "Operator answers",
	} {
		if !has(want) {
			t.Errorf("no event starting %q", want)
		}
	}
	if has("Number guard") && slices.ContainsFunc(events, func(e eventlog.Event) bool {
		return strings.Contains(e.Summary, "refused") || strings.Contains(e.Summary, "stopped")
	}) {
		t.Error("number guard refused something in a clean run")
	}
	if m.sawLogOnly {
		t.Error("the model was shown weather samples or the grid file")
	}
	i := slices.IndexFunc(events, func(e eventlog.Event) bool {
		return e.Type == eventlog.TypeToolResult && e.Detail["tool"] == "weather_on_track"
	})
	if i < 0 {
		t.Fatal("no weather_on_track result")
	}
	if smp, _ := events[i].Detail["samples"].([]any); len(smp) != 8 {
		t.Errorf("weather result logs %d samples, want 8", len(smp))
	}
	if ref, _ := events[i].Detail["grid_ref"].(map[string]any); ref["sha256"] == nil {
		t.Errorf("weather result has no grid_ref: %v", events[i].Detail["grid_ref"])
	}
	if len(res.Drafts) != 2 || !strings.Contains(res.Drafts[0].Body, "Please reply before tomorrow's noon report.") || !strings.Contains(res.Drafts[1].Subject, "ETA") {
		t.Errorf("drafts: %+v", res.Drafts)
	}
	j := slices.IndexFunc(events, func(e eventlog.Event) bool {
		return e.Type == eventlog.TypeCheck && e.Detail["tool"] == "check_forecast_eta"
	})
	if j < 0 {
		t.Fatal("no forecast ETA check")
	}
	if smp, _ := events[j].Detail["samples"].([]any); len(smp) < 100 {
		t.Errorf("forecast check logs %d samples, want one an hour", len(smp))
	}
	// The operator decides before any request to the Master is drafted.
	q := slices.IndexFunc(events, func(e eventlog.Event) bool { return e.Type == eventlog.TypeQuestion })
	a := slices.IndexFunc(events, func(e eventlog.Event) bool { return e.Type == eventlog.TypeAnswer })
	d := slices.IndexFunc(events, func(e eventlog.Event) bool { return e.Type == eventlog.TypeDraft && strings.Contains(e.Summary, "ETA") })
	if q < 0 || a < q || d < a {
		t.Errorf("question at %d, answer at %d, ETA draft at %d: want question, then answer, then draft", q, a, d)
	}
	if slices.ContainsFunc(events[:j], func(e eventlog.Event) bool { return strings.HasPrefix(e.Summary, "Not a good weather day") }) {
		t.Error("forecast ETA check should come in D3, after D1's events and before the Performance Agent")
	}
}
