package desk

import (
	"bytes"
	"strings"
	"testing"
	"time"

	"google.golang.org/adk/v2/agent"

	"github.com/captv89/shorewatch/agent/checker"
	"github.com/captv89/shorewatch/agent/eventlog"
	"github.com/captv89/shorewatch/sim"
)

// toolCtx is the part of a tool's context the desk's tools read.
type toolCtx struct {
	agent.Context
	id, name string
}

func (c toolCtx) FunctionCallID() string { return c.id }
func (c toolCtx) AgentName() string      { return c.name }

// entryRun is a run with one episode whose voyage has a voyage management
// system record, and whose known numbers include its ETA.
func entryRun(t *testing.T, op func(Draft) Decision) (*run, *bytes.Buffer) {
	t.Helper()
	eta := time.Date(2025, 1, 25, 18, 0, 0, 0, time.UTC)
	ep := sim.Scene{ID: "test", SimTime: eta.Add(-48 * time.Hour), Voyage: sim.Voyage{
		Ship: sim.Ship{ID: "SIM-0001"},
		VMS:  &sim.VMS{PortCalls: []sim.PortCall{{Port: "Rotterdam", Purpose: "discharge", ETA: eta}}},
	}}
	var buf bytes.Buffer
	r := &run{cfg: Config{Operator: op}, log: eventlog.NewWriter(&buf, eventlog.Event{RunID: "t", Scene: "test", Ship: "SIM-0001"}, ep.SimTime)}
	r.start(ep)
	r.known = checker.NewKnown()
	r.known.AddValue(map[string]any{"eta": eta.Add(36 * time.Hour)})
	return r, &buf
}

func TestProposeEntryWaitsForOperator(t *testing.T) {
	for _, approve := range []bool{true, false} {
		r, buf := entryRun(t, func(Draft) Decision { return Decision{Approve: approve} })
		out, _ := r.proposeEntry(toolCtx{id: "c1", name: voyageName}, entryIn{Kind: "port_call_eta", Port: "Rotterdam", ETAUTC: "2025-01-27T06:00:00Z", Reason: "Berth not free until then.", About: []string{"eta"}})
		if !strings.HasPrefix(out["status"].(string), "in the approval queue") {
			t.Fatalf("status %v", out)
		}
		// Before the gate, the entry is queued, not posted.
		if got := r.drafts[0].Status; got != statusQueued {
			t.Fatalf("before the gate: status %q", got)
		}
		r.reviewQueue()
		want := "rejected by the operator"
		if approve {
			want = "approved, not posted (demo)"
		}
		if got := r.drafts[0].Status; got != want {
			t.Errorf("approve %v: status %q, want %q", approve, got, want)
		}
		if !strings.Contains(buf.String(), `"type":"entry"`) {
			t.Error("no entry event logged")
		}
	}
}

func TestProposeEntryRefusals(t *testing.T) {
	for name, in := range map[string]entryIn{
		"invented ETA": {Kind: "port_call_eta", Port: "Rotterdam", ETAUTC: "2025-01-28T09:00:00Z", Reason: "x", About: []string{"eta"}},
		"unknown port": {Kind: "port_call_eta", Port: "Hamburg", ETAUTC: "2025-01-27T06:00:00Z", Reason: "x", About: []string{"eta"}},
		"bad kind":     {Kind: "hire_rate", Reason: "x", About: []string{"other"}},
		"no topic":     {Kind: "port_call_eta", Port: "Rotterdam", ETAUTC: "2025-01-27T06:00:00Z", Reason: "x"},
		"no reason":    {Kind: "port_call_eta", Port: "Rotterdam", ETAUTC: "2025-01-27T06:00:00Z", About: []string{"eta"}},
		"backwards":    {Kind: "off_hire", FromUTC: "2025-01-27T06:00:00Z", ToUTC: "2025-01-27T05:00:00Z", Reason: "x", About: []string{"other"}},
		"not RFC 3339": {Kind: "off_hire", FromUTC: "27 Jan 0600", ToUTC: "2025-01-27T09:00:00Z", Reason: "x", About: []string{"other"}},
	} {
		r, _ := entryRun(t, nil)
		out, _ := r.proposeEntry(toolCtx{id: "c1", name: voyageName}, in)
		if s := out["status"].(string); s == "" || strings.HasPrefix(s, "in the approval queue") {
			t.Errorf("%s: accepted (%v)", name, out)
		}
		if len(r.drafts) != 0 {
			t.Errorf("%s: queued", name)
		}
	}
}

// A decision left to the operator stops entries as it stops drafts.
func TestProposeEntryStoppedBeforeOperatorDecides(t *testing.T) {
	r, _ := entryRun(t, nil)
	r.decide = "whether to change the ETA"
	out, _ := r.proposeEntry(toolCtx{id: "c1", name: voyageName}, entryIn{Kind: "port_call_eta", Port: "Rotterdam", ETAUTC: "2025-01-27T06:00:00Z", Reason: "x", About: []string{"eta"}})
	if out["status"] != "stopped by guardrail" || len(r.drafts) != 0 {
		t.Errorf("got %v, %d queued", out, len(r.drafts))
	}
}

// A refused entry is logged, so an agent that claims it proposed one is caught.
func TestProposeEntryRefusalLogged(t *testing.T) {
	r, buf := entryRun(t, nil)
	r.proposeEntry(toolCtx{id: "c1", name: voyageName}, entryIn{Kind: "off_hire", FromUTC: "2025-01-26T06:10–1140", ToUTC: "x", Reason: "x", About: []string{"off_hire"}})
	if !strings.Contains(buf.String(), "Entry refused") {
		t.Error("refusal not logged")
	}
}
