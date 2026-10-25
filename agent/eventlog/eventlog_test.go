package eventlog

import (
	"bytes"
	"strings"
	"testing"
	"time"
)

func writeEvents(t *testing.T, events ...Event) []Event {
	t.Helper()
	var buf bytes.Buffer
	w := NewWriter(&buf, Event{RunID: "r1", Attempt: 1, Scene: "s1-routine", Ship: "SIM-0001"}, time.Date(2025, 1, 22, 12, 5, 0, 0, time.UTC))
	for _, e := range events {
		if _, err := w.Write(e); err != nil {
			t.Fatal(err)
		}
	}
	got, err := Read(&buf)
	if err != nil {
		t.Fatal(err)
	}
	return got
}

func TestValidRunPasses(t *testing.T) {
	ref := ToolRef("distance_to_go", "c1")
	events := writeEvents(t,
		Event{Actor: ActorSim, Type: TypeMessageReceived, Summary: "Noon report arrives", SourceRefs: []string{"email:nr-0001"}},
		Event{Actor: ActorAgent, Type: TypeRationale, Summary: "Agent's stated reason: check ETA", Detail: map[string]any{"reason": "check ETA"}},
		Event{Actor: ActorSubagent, Type: TypeToolCall, Summary: "Calls distance_to_go", SourceRefs: []string{ref}},
		Event{Actor: ActorTool, Type: TypeToolResult, Summary: "distance_to_go returns", SourceRefs: []string{ref}},
	)
	if errs := Validate(events); len(errs) > 0 {
		t.Fatalf("valid run failed: %v", errs)
	}
	if events[3].Seq != 4 || events[0].SchemaVersion != SchemaVersion {
		t.Errorf("writer did not fill seq or schema_version: %+v", events[3])
	}
}

func TestValidateCatchesContractBreaks(t *testing.T) {
	cases := []struct {
		name  string
		event Event
		want  string
	}{
		{"orphan tool result", Event{Actor: ActorTool, Type: TypeToolResult, Summary: "x", SourceRefs: []string{ToolRef("ais_position", "c9")}}, "no earlier tool_call"},
		{"rationale without reason", Event{Actor: ActorAgent, Type: TypeRationale, Summary: "x"}, "detail.reason"},
		{"unknown actor", Event{Actor: "robot", Type: TypePlan, Summary: "x"}, "unknown actor"},
		{"empty summary", Event{Actor: ActorAgent, Type: TypePlan}, "empty summary"},
	}
	for _, c := range cases {
		errs := Validate(writeEvents(t, c.event))
		if len(errs) == 0 || !strings.Contains(errs[0].Error(), c.want) {
			t.Errorf("%s: got %v, want error containing %q", c.name, errs, c.want)
		}
	}
}

func TestReadRejectsUnknownFields(t *testing.T) {
	if _, err := Read(strings.NewReader(`{"schema_version":"1.0","surprise":1}`)); err == nil {
		t.Error("want error for a field not in §6")
	}
}
