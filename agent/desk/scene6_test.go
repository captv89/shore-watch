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
	"github.com/captv89/shorewatch/tools/weather"
)

// TestScene6Pipeline: the Master's remark reports a main engine stop. Code
// checks the stop against the report; the voyage management system stand-in
// prices the off-hire; the operator decides; then the off-hire entry and the
// e-mail to the owner wait at the Approval gate. No query to the Master.
func TestScene6Pipeline(t *testing.T) {
	scene6Pipeline(t, &staff{})
}

// TestScene6SkippedEntry: a Voyage Agent that reports the off-hire entry
// proposed without proposing it is caught by the Follow-through check, and
// the entry then reaches the queue.
func TestScene6SkippedEntry(t *testing.T) {
	events := scene6Pipeline(t, &staff{skipEntry: true})
	var sentBack, closed bool
	for _, e := range events {
		sentBack = sentBack || strings.HasPrefix(e.Summary, "Follow-through check: the operator said yes, but not in the approval queue: the off-hire entry")
		closed = closed || e.Summary == "Follow-through check: everything the operator agreed to is now in the approval queue"
	}
	if !sentBack || !closed {
		t.Errorf("sent back %v, closed %v: want both", sentBack, closed)
	}
}

func scene6Pipeline(t *testing.T, m *staff) []eventlog.Event {
	t.Helper()
	scene1(t) // loads the graph
	sc, err := sim.Scene6(graph)
	if err != nil {
		t.Fatal(err)
	}
	wx, err := weather.Load("../../data/weather/s2-tormar-spring.json", "data/weather/s2-tormar-spring.json")
	if err != nil {
		t.Fatal(err)
	}
	res, err := Record(context.Background(), Config{
		Episodes: []sim.Scene{sc}, Graph: graph, Weather: wx, Model: m, ModelID: "scripted", Attempt: 1, TotalAttempts: 1, OutDir: t.TempDir(), MaxModelCalls: 60,
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

	var stoppage, statement, answered bool
	for _, e := range events {
		switch {
		case e.Type == eventlog.TypeError:
			t.Errorf("seq %d: unexpected error: %s", e.Seq, e.Summary)
		case e.Type == eventlog.TypeCheck && e.Detail["tool"] == "check_stoppage":
			stoppage = e.Detail["passed"] == true
		case e.Type == eventlog.TypeToolResult && strings.HasPrefix(e.Summary, "Hire statement preview (stand-in"):
			statement = strings.Contains(e.Summary, "made up for this simulation")
		case e.Type == eventlog.TypeAnswer:
			answered = true
		case e.Type == eventlog.TypeDraft || e.Type == eventlog.TypeEntry:
			if !answered {
				t.Errorf("seq %d: %s before the operator decided", e.Seq, e.Type)
			}
		}
	}
	if !stoppage || !statement || !answered {
		t.Errorf("stoppage check %v, labelled statement %v, operator answer %v: want all", stoppage, statement, answered)
	}
	if len(res.Drafts) != 2 {
		t.Fatalf("%d queue items, want the e-mail to the owner and the off-hire entry", len(res.Drafts))
	}
	mail, entry := res.Drafts[0], res.Drafts[1]
	if !slices.Equal(mail.To, []string{"technical@tormar-bulk.example"}) || !slices.Equal(mail.About, []string{"off_hire"}) {
		t.Errorf("owner e-mail to %v about %v", mail.To, mail.About)
	}
	from, to := time.Date(2025, 1, 26, 6, 10, 0, 0, time.UTC), time.Date(2025, 1, 26, 11, 40, 0, 0, time.UTC)
	if entry.Entry == nil || entry.Entry.Kind != "off_hire" || !entry.Entry.From.Equal(from) || !entry.Entry.To.Equal(to) {
		t.Errorf("entry %+v, want off hire %s to %s", entry.Entry, from, to)
	}
	for _, d := range res.Drafts {
		if strings.Contains(strings.Join(d.To, ","), "master@") {
			t.Errorf("a draft to the Master: the remark explains the figures")
		}
	}
	return events
}
