// Package eventlog writes the run event log: the contract between the agent
// and the web page. One JSON object per line.
package eventlog

import (
	"encoding/json"
	"fmt"
	"io"
	"sync"
	"time"
)

// SchemaVersion is the version of the event format in this package.
// 1.1 adds detail.samples and detail.grid_ref on weather_on_track results.
// 1.2 adds agent: which agent an agent or subagent event belongs to.
// 1.3 adds the question and answer types: the operator decides.
// 1.4 adds the entry type: a proposed entry in the voyage management system.
// 1.5 adds the forecast_available type: a forecast run, not an e-mail, starts the work.
// 1.6: in a run over several ships, each event names the ship it is about, or "fleet".
const SchemaVersion = "1.6"

// Readable are the versions this code still reads. Each change so far has
// only added fields, so older logs stay valid.
var Readable = []string{"1.0", "1.1", "1.2", "1.3", "1.4", "1.5", "1.6"}

// Actors.
const (
	ActorAgent       = "agent"
	ActorSubagent    = "subagent"
	ActorTool        = "tool"
	ActorChecker     = "checker"
	ActorGuardrail   = "guardrail"
	ActorHumanSim    = "human_sim"
	ActorExternalSim = "external_sim"
	ActorSim         = "sim"
)

// Event types.
const (
	TypeMessageReceived   = "message_received"
	TypeExtracted         = "extracted"
	TypePlan              = "plan"
	TypeToolCall          = "tool_call"
	TypeToolResult        = "tool_result"
	TypeCheck             = "check"
	TypeRationale         = "rationale"
	TypeDraft             = "draft"
	TypeApprovalRequested = "approval_requested"
	TypeApproved          = "approved"
	TypeEdited            = "edited"
	TypeRejected          = "rejected"
	TypeBlocked           = "blocked"
	TypeAlert             = "alert"
	TypeError             = "error"
	TypeDegraded          = "degraded"
	TypeRecovered         = "recovered"
	TypeQuestion          = "question"
	TypeAnswer            = "answer"
	TypeEntry             = "entry"
	TypeForecastAvailable = "forecast_available"
)

// Cost is the model usage for the step that produced an event.
type Cost struct {
	InputTokens  int32   `json:"input_tokens"`
	OutputTokens int32   `json:"output_tokens"`
	USD          float64 `json:"usd"`
}

// Event is one line of the event log.
type Event struct {
	SchemaVersion string         `json:"schema_version"`
	RunID         string         `json:"run_id"`
	Attempt       int            `json:"attempt"`
	Seq           int            `json:"seq"`
	SimTime       time.Time      `json:"sim_time"`
	WallMS        int64          `json:"wall_ms"`
	Scene         string         `json:"scene"`
	Ship          string         `json:"ship"`
	Actor         string         `json:"actor"`
	Agent         string         `json:"agent,omitempty"` // e.g. "voyage_agent", for actor agent or subagent
	Type          string         `json:"type"`
	Summary       string         `json:"summary"`
	Detail        map[string]any `json:"detail"`
	SourceRefs    []string       `json:"source_refs"`
	Cost          Cost           `json:"cost"`
	Model         *string        `json:"model"`
}

// Header describes one recorded run. It is stored next to the log.
type Header struct {
	SchemaVersion string            `json:"schema_version"`
	RunID         string            `json:"run_id"`
	Scene         string            `json:"scene"`
	Model         string            `json:"model"`
	RecordedAt    time.Time         `json:"recorded_at"`
	PromptVersion string            `json:"prompt_version"`
	ToolVersions  map[string]string `json:"tool_versions"`
	Seed          int64             `json:"seed"`
	Attempt       int               `json:"attempt"`
	TotalAttempts int               `json:"total_attempts"`
	Notes         string            `json:"notes,omitempty"`
}

// Writer appends events to a log, filling in run-wide fields, seq and wall_ms.
type Writer struct {
	mu      sync.Mutex
	enc     *json.Encoder
	start   time.Time
	seq     int
	base    Event
	SimTime time.Time
}

// NewWriter returns a Writer for one run. base supplies run_id, attempt,
// scene and ship; simTime is the starting simulated time. An event that
// names its own ship keeps it (a fleet run, since 1.6).

// Fleet is the ship field of an event about no single ship in a run that
// covers several (since 1.6).
const Fleet = "fleet"

func NewWriter(w io.Writer, base Event, simTime time.Time) *Writer {
	base.SchemaVersion = SchemaVersion
	return &Writer{enc: json.NewEncoder(w), start: time.Now(), base: base, SimTime: simTime}
}

// Write fills in the run-wide fields and appends e. It returns the event's seq.
func (w *Writer) Write(e Event) (int, error) {
	w.mu.Lock()
	defer w.mu.Unlock()
	w.seq++
	e.SchemaVersion = w.base.SchemaVersion
	e.RunID = w.base.RunID
	e.Attempt = w.base.Attempt
	e.Scene = w.base.Scene
	if e.Ship == "" {
		e.Ship = w.base.Ship
	}
	e.Seq = w.seq
	e.SimTime = w.SimTime
	e.WallMS = time.Since(w.start).Milliseconds()
	if e.Detail == nil {
		e.Detail = map[string]any{}
	}
	if e.SourceRefs == nil {
		e.SourceRefs = []string{}
	}
	if err := w.enc.Encode(e); err != nil {
		return 0, fmt.Errorf("write event %d: %w", e.Seq, err)
	}
	return e.Seq, nil
}

// ToolRef is the source_refs entry for a tool call.
func ToolRef(tool, callID string) string { return "tool:" + tool + "#" + callID }
