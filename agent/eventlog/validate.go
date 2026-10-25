package eventlog

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"slices"
	"strings"
)

var (
	actors = []string{ActorAgent, ActorSubagent, ActorTool, ActorChecker, ActorGuardrail, ActorHumanSim, ActorExternalSim, ActorSim}
	types  = []string{TypeMessageReceived, TypeExtracted, TypePlan, TypeToolCall, TypeToolResult, TypeCheck, TypeRationale, TypeDraft,
		TypeApprovalRequested, TypeApproved, TypeEdited, TypeRejected, TypeBlocked, TypeAlert, TypeError, TypeDegraded, TypeRecovered, TypeQuestion, TypeAnswer, TypeEntry, TypeForecastAvailable}
)

// Read parses a JSON Lines event log.
func Read(r io.Reader) ([]Event, error) {
	var events []Event
	sc := bufio.NewScanner(r)
	sc.Buffer(make([]byte, 0, 1<<20), 16<<20)
	for line := 1; sc.Scan(); line++ {
		if strings.TrimSpace(sc.Text()) == "" {
			continue
		}
		var e Event
		dec := json.NewDecoder(strings.NewReader(sc.Text()))
		dec.DisallowUnknownFields()
		if err := dec.Decode(&e); err != nil {
			return nil, fmt.Errorf("line %d: %w", line, err)
		}
		events = append(events, e)
	}
	return events, sc.Err()
}

// Validate checks a run's events against the §6 rules and returns every problem found.
func Validate(events []Event) []error {
	var errs []error
	fail := func(e Event, format string, args ...any) {
		errs = append(errs, fmt.Errorf("seq %d: "+format, append([]any{e.Seq}, args...)...))
	}
	// Tool refs seen so far. A result must point at a call logged earlier,
	// whether that call was a tool_call, a draft or a delegation (plan).
	calls := map[string]bool{}
	for i, e := range events {
		if !slices.Contains(Readable, e.SchemaVersion) {
			fail(e, "schema_version %q, want one of %v", e.SchemaVersion, Readable)
		}
		if e.SchemaVersion != events[0].SchemaVersion {
			fail(e, "schema_version %q differs from first event", e.SchemaVersion)
		}
		if e.Seq != i+1 {
			fail(e, "seq out of order at line %d", i+1)
		}
		if e.RunID == "" || e.Scene == "" || e.Ship == "" || e.SimTime.IsZero() {
			fail(e, "missing run_id, scene, ship or sim_time")
		}
		if e.RunID != events[0].RunID {
			fail(e, "run_id %q differs from first event", e.RunID)
		}
		if !slices.Contains(actors, e.Actor) {
			fail(e, "unknown actor %q", e.Actor)
		}
		if !slices.Contains(types, e.Type) {
			fail(e, "unknown type %q", e.Type)
		}
		if strings.TrimSpace(e.Summary) == "" {
			fail(e, "empty summary")
		}
		switch e.Type {
		case TypeToolCall, TypeDraft, TypeEntry, TypeQuestion, TypeAnswer, TypeForecastAvailable:
			if len(e.SourceRefs) == 0 {
				fail(e, "%s without source_refs", e.Type)
			}
		case TypeToolResult:
			if len(e.SourceRefs) == 0 {
				fail(e, "tool_result without source_refs")
			}
			for _, r := range e.SourceRefs {
				if strings.HasPrefix(r, "tool:") && !calls[r] {
					fail(e, "tool_result refers to %s, which has no earlier tool_call", r)
				}
			}
		case TypeRationale:
			if s, _ := e.Detail["reason"].(string); strings.TrimSpace(s) == "" {
				fail(e, "rationale without detail.reason")
			}
		}
		if e.Type != TypeToolResult {
			for _, r := range e.SourceRefs {
				calls[r] = true
			}
		}
	}
	return errs
}
