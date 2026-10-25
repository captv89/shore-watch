package desk

import (
	"context"
	"os"
	"slices"
	"strings"
	"testing"

	"github.com/captv89/shorewatch/agent/eventlog"
	"github.com/captv89/shorewatch/sim"
)

func TestScene3FraudBlocked(t *testing.T) {
	scene1(t) // loads the graph
	m := &staff{}
	res, err := Record(context.Background(), Config{
		Episodes: []sim.Scene{sim.Scene3()}, Graph: graph, Model: m, ModelID: "scripted", Attempt: 1, TotalAttempts: 1, OutDir: fixtureDir(t), MaxModelCalls: 10,
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
		t.Logf("%2d %-10s %-14s %-12s %s", e.Seq, e.Actor, e.Agent, e.Type, e.Summary)
	}
	// Only the Security Agent works on the message; nothing is drafted.
	for _, e := range events {
		if e.Agent != "" && e.Agent != securityName {
			t.Errorf("event %d: %s worked on a held message", e.Seq, e.Agent)
		}
	}
	if len(res.Drafts) != 0 {
		t.Errorf("drafts: %+v", res.Drafts)
	}
	i := slices.IndexFunc(events, func(e eventlog.Event) bool { return e.Type == eventlog.TypeAlert && e.Actor == eventlog.ActorGuardrail })
	if i < 0 {
		t.Fatal("no alert to the operator")
	}
	a := events[i]
	if rs, _ := a.Detail["reasons"].([]any); len(rs) != 3 {
		t.Errorf("alert has %d reasons, want 3: %v", len(rs), a.Detail["reasons"])
	}
	for _, want := range []string{"+44 20 7946 0518", "not the number in the e-mail (+44 20 7946 0884)", "so1entbunker.example"} {
		if !strings.Contains(a.Summary+events[1].Summary+events[2].Summary, want) {
			t.Errorf("no %q in the guardrail's events", want)
		}
	}
	if !slices.ContainsFunc(events, func(e eventlog.Event) bool { return e.Type == eventlog.TypeBlocked }) {
		t.Error("message not blocked")
	}
}

// A quote the e-mail does not contain is refused; the message is blocked anyway.
func TestScene3FindingQuoteChecked(t *testing.T) {
	scene1(t)
	res, err := Record(context.Background(), Config{
		Episodes: []sim.Scene{sim.Scene3()}, Graph: graph, Model: &staff{inventNumber: true}, ModelID: "scripted", Attempt: 1, TotalAttempts: 1, OutDir: t.TempDir(), MaxModelCalls: 10,
	})
	if err != nil {
		t.Fatal(err)
	}
	b, err := os.ReadFile(res.LogPath)
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"Security Agent's findings refused", `"type":"blocked"`} {
		if !strings.Contains(string(b), want) {
			t.Errorf("log has no %q", want)
		}
	}
}

// fixtureDir is where a scripted log is written: a temporary directory, or
// SHOREWATCH_FIXTURES to refresh the page's test fixture.
func fixtureDir(t *testing.T) string {
	if d := os.Getenv("SHOREWATCH_FIXTURES"); d != "" {
		return d
	}
	return t.TempDir()
}

// A call whose arguments the tool's schema rejects never reaches the tool,
// so the tool cannot log it; the log must still show it (principle 1).
func TestScene3RejectedArgsLogged(t *testing.T) {
	scene1(t)
	res, err := Record(context.Background(), Config{
		Episodes: []sim.Scene{sim.Scene3()}, Graph: graph, Model: &staff{stringFindings: true}, ModelID: "scripted", Attempt: 1, TotalAttempts: 1, OutDir: t.TempDir(), MaxModelCalls: 10,
	})
	if err != nil {
		t.Fatal(err)
	}
	b, err := os.ReadFile(res.LogPath)
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{`"type":"error"`, "report_findings"} {
		if !strings.Contains(string(b), want) {
			t.Errorf("log has no %q", want)
		}
	}
}
