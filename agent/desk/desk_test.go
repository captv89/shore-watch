package desk

import (
	"context"
	"encoding/json"
	"os"
	"slices"
	"strings"
	"sync"
	"testing"

	"github.com/captv89/seaRoute/route"
	"github.com/captv89/shorewatch/agent/eventlog"
	"github.com/captv89/shorewatch/sim"
)

var (
	graphOnce sync.Once
	graph     *route.Graph
)

func scene1(t *testing.T) sim.Scene {
	t.Helper()
	graphOnce.Do(func() {
		var err error
		if graph, err = route.Load("../../third_party/seaRoute/dataset/marnet_plus_5km.geojson"); err != nil {
			panic(err)
		}
	})
	sc, err := sim.Scene1(graph)
	if err != nil {
		t.Fatal(err)
	}
	return sc
}

func record(t *testing.T, m *staff) (Result, []eventlog.Event) {
	t.Helper()
	dir := t.TempDir()
	res, err := Record(context.Background(), Config{
		Episodes: []sim.Scene{scene1(t)}, Graph: graph, Model: m, ModelID: "scripted", Attempt: 1, TotalAttempts: 1, OutDir: dir, MaxModelCalls: 60,
	})
	if err != nil {
		if b, rerr := os.ReadFile(res.LogPath); rerr == nil {
			for _, l := range strings.Split(string(b), "\n") {
				var e eventlog.Event
				if json.Unmarshal([]byte(l), &e) == nil {
					t.Logf("%2d %-22s %-10s %-18s %s", e.Seq, e.Agent, e.Actor, e.Type, e.Summary)
				}
			}
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
		t.Logf("%2d %-10s %-18s %s", e.Seq, e.Actor, e.Type, e.Summary)
	}
	return res, events
}

func TestScene1Pipeline(t *testing.T) {
	res, events := record(t, &staff{})
	want := []string{
		"sim/message_received", "guardrail/check", "agent/rationale", "agent/plan", "subagent/extracted", "subagent/tool_result",
		"tool/tool_result", "checker/check", "checker/check", "subagent/tool_result", "agent/plan", "guardrail/check",
		"subagent/draft", "subagent/approval_requested", "agent/question", "human_sim/approved", "tool/tool_result",
		"human_sim/answer", "agent/plan", "subagent/draft", "human_sim/approved",
	}
	var got []string
	for _, e := range events {
		got = append(got, e.Actor+"/"+e.Type)
	}
	i := 0
	for _, g := range got {
		if i < len(want) && g == want[i] {
			i++
		}
	}
	if i < len(want) {
		t.Errorf("event sequence missing %q (in order); got %v", want[i], got)
	}
	// The notice, then the request to resend the report with fuel figures.
	if len(res.Drafts) != 2 || res.Drafts[0].Status != "approved, not sent (demo)" || !strings.Contains(res.Drafts[1].Body, "resend") {
		t.Errorf("drafts: %+v", res.Drafts)
	}
	for _, e := range events {
		if e.Type == eventlog.TypeCheck && e.Actor == eventlog.ActorChecker && strings.HasPrefix(e.Summary, "ETA ") && !strings.HasPrefix(e.Summary, "ETA consistent") {
			t.Errorf("Scene 1 ETA check should pass: %s", e.Summary)
		}
	}
	if !slices.ContainsFunc(events, func(e eventlog.Event) bool { return strings.Contains(e.Summary, "3-day ETA notice due") }) {
		t.Error("no 3-day notice due")
	}
	var tokens int32
	for _, e := range events {
		tokens += e.Cost.InputTokens
	}
	if want := int32(100 * res.ModelCalls); tokens != want {
		t.Errorf("logged %d input tokens over %d model calls, want %d", tokens, res.ModelCalls, want)
	}
}

func TestRoutePathLoggedNotShownToModel(t *testing.T) {
	m := &staff{}
	_, events := record(t, m)
	if m.sawLogOnly {
		t.Error("the model was shown the route path")
	}
	i := slices.IndexFunc(events, func(e eventlog.Event) bool {
		return e.Type == eventlog.TypeToolResult && e.Detail["tool"] == "distance_to_go"
	})
	if i < 0 {
		t.Fatal("no distance_to_go result in the log")
	}
	path, ok := events[i].Detail["path"].([]any)
	if !ok || len(path) < 2 {
		t.Fatalf("distance_to_go result has no path: %v", events[i].Detail)
	}
	end, _ := path[len(path)-1].([]any)
	if len(end) != 2 || end[0].(float64) < 3 || end[0].(float64) > 5 || end[1].(float64) < 51 || end[1].(float64) > 53 {
		t.Errorf("path ends at %v; want Rotterdam", end)
	}
}

func TestNumberGuardStopsInventedNumber(t *testing.T) {
	res, events := record(t, &staff{inventNumber: true})
	if !slices.ContainsFunc(events, func(e eventlog.Event) bool {
		return strings.HasPrefix(e.Summary, "Number guard: draft stopped") && strings.Contains(e.Summary, "79.2")
	}) {
		t.Error("invented number not stopped")
	}
	if len(res.Drafts) != 2 || strings.Contains(res.Drafts[0].Body, "79.2") {
		t.Errorf("draft with invented number reached the queue: %+v", res.Drafts)
	}
}
