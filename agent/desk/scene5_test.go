package desk

import (
	"context"
	"os"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/captv89/shorewatch/agent/eventlog"
	"github.com/captv89/shorewatch/sim"
)

// TestScene5Pipeline: the Rotterdam agent's e-mail says the berth is later.
// The Voyage Agent gets the stand-in speed advice, code checks it, the
// operator decides, and only then come the speed instruction, the ETA
// notice and the port call entry, all at the Approval gate.
func TestScene5Pipeline(t *testing.T) {
	scene1(t) // loads the graph
	sc, err := sim.Scene5(graph)
	if err != nil {
		t.Fatal(err)
	}
	m := &staff{}
	res, err := Record(context.Background(), Config{
		Episodes: []sim.Scene{sc}, Graph: graph, Model: m, ModelID: "scripted", Attempt: 1, TotalAttempts: 1, OutDir: t.TempDir(), MaxModelCalls: 40,
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
		t.Logf("%2d %-10s %-18s %s", e.Seq, e.Actor, e.Type, e.Summary)
	}

	var advice, plan, answered bool
	for _, e := range events {
		switch {
		case e.Type == eventlog.TypeError:
			t.Errorf("seq %d: unexpected error: %s", e.Seq, e.Summary)
		case e.Type == eventlog.TypeToolResult && strings.HasPrefix(e.Summary, "Speed advice (stand-in"):
			advice = true
			if !strings.Contains(e.Summary, "made up for this simulation") {
				t.Errorf("seq %d: speed advice not labelled as made up: %s", e.Seq, e.Summary)
			}
		case e.Type == eventlog.TypeCheck && e.Detail["tool"] == "check_speed_plan":
			plan = true
			if e.Detail["passed"] != true {
				t.Errorf("seq %d: speed plan check failed: %s", e.Seq, e.Summary)
			}
		case e.Type == eventlog.TypeAnswer:
			answered = true
		case e.Type == eventlog.TypeDraft || e.Type == eventlog.TypeEntry:
			if !answered {
				t.Errorf("seq %d: %s before the operator decided", e.Seq, e.Type)
			}
		}
	}
	if !advice || !plan || !answered {
		t.Errorf("speed advice %v, plan check %v, operator answer %v: want all", advice, plan, answered)
	}

	if len(res.Drafts) != 3 {
		t.Fatalf("%d queue items, want the speed instruction, the ETA notice and the entry", len(res.Drafts))
	}
	speed, notice, entry := res.Drafts[0], res.Drafts[1], res.Drafts[2]
	if !slices.Equal(speed.To, []string{"master@valdacrest.sim-fleet.example"}) || !slices.Equal(speed.About, []string{"berth_delay"}) {
		t.Errorf("speed instruction to %v about %v", speed.To, speed.About)
	}
	if !slices.Equal(notice.To, []string{"ops@maasport-agencies.example", "receiving@rijnmond-coal.example"}) || !slices.Equal(notice.About, []string{"eta_notice"}) {
		t.Errorf("ETA notice to %v about %v", notice.To, notice.About)
	}
	want := time.Date(2025, 1, 27, 4, 0, 0, 0, time.UTC)
	if entry.Entry == nil || entry.Entry.Port != "Rotterdam" || !entry.Entry.ETA.Equal(want) {
		t.Errorf("entry %+v, want Rotterdam ETA %s", entry.Entry, want)
	}
	for _, d := range res.Drafts {
		if !strings.HasPrefix(d.Status, "approved, not") {
			t.Errorf("queue item %s: status %q", d.CallID, d.Status)
		}
	}
}
