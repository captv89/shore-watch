// Package desk runs the operator-desk agent on one scene and records every
// step as event-log events (agent/eventlog).
package desk

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"maps"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
	"sync"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"google.golang.org/genai"

	"google.golang.org/adk/v2/agent"
	"google.golang.org/adk/v2/agent/llmagent"
	"google.golang.org/adk/v2/model"
	"google.golang.org/adk/v2/plugin"
	"google.golang.org/adk/v2/runner"
	"google.golang.org/adk/v2/session"
	"google.golang.org/adk/v2/tool"
	"google.golang.org/adk/v2/tool/mcptoolset"

	"github.com/captv89/seaRoute/route"
	"github.com/captv89/shorewatch/agent/checker"
	"github.com/captv89/shorewatch/agent/eventlog"
	"github.com/captv89/shorewatch/sim"
	"github.com/captv89/shorewatch/tools"
	"github.com/captv89/shorewatch/tools/weather"
)

// Decision is the simulated operator's answer to a draft.
type Decision struct {
	Approve    bool
	EditedBody string // optional; replaces the draft body when approving
	Note       string // shown on the page, e.g. "edits one line"
}

// Question is something the Ops Agent asks the operator to decide.
type Question struct {
	CallID   string   `json:"call_id"`
	Question string   `json:"question"`
	Proposal string   `json:"proposal"`
	About    []string `json:"about,omitempty"`
}

// Answer is the simulated operator's reply to a question.
type Answer struct {
	Yes  bool
	Note string // the operator's words, shown on the page and passed to the Ops Agent
}

// Config describes one recorded run.
type Config struct {
	// Episodes are the scene's incoming e-mails, handled in order in one
	// run and one log. Most scenes have one.
	Episodes []sim.Scene
	Graph    *route.Graph
	// Weather serves weather_on_track; nil for scenes that don't need it.
	Weather *weather.Grid
	// Forecast is a forecast run for the Voyage Agent's check_forecast_eta,
	// offered only for reports made after the run; nil for scenes that
	// don't need it.
	Forecast      *weather.Grid
	Model         model.LLM
	ModelID       string
	Attempt       int
	TotalAttempts int
	OutDir        string
	MaxModelCalls int // per episode
	ToolVersions  map[string]string
	// Operator decides on each draft. Defaults to approving unchanged.
	Operator func(Draft) Decision
	// Answer is the operator's reply to each question. Defaults to yes.
	Answer func(Question) Answer
}

// Result summarises a finished run.
type Result struct {
	RunID      string
	LogPath    string
	Events     int
	ModelCalls int
	Drafts     []Draft
}

// Draft is an outgoing message waiting for, or past, the approval gate.
type Draft struct {
	CallID  string   `json:"call_id"`
	To      []string `json:"to"`
	Subject string   `json:"subject"`
	Body    string   `json:"body"`
	Purpose string   `json:"purpose"`
	About   []string `json:"about,omitempty"`
	Status  string   `json:"status"`
	// Entry is set when this is a proposed entry in the voyage management
	// system rather than an e-mail; To, Subject and Body are then empty.
	// It waits at the same Approval gate.
	Entry *Entry `json:"entry,omitempty"`
}

// Entry is a change to the voyage management system, proposed by an agent.
// Only the operator's approval posts it; in this demo it is filed, never
// written to a real system.
type Entry struct {
	Kind string    `json:"kind"` // "port_call_eta" or "off_hire"
	Port string    `json:"port,omitempty"`
	ETA  time.Time `json:"eta,omitzero"`
	From time.Time `json:"from,omitzero"`
	To   time.Time `json:"to,omitzero"`
	// CranesOut of Cranes were out of use: the hire stops pro rata (schema 1.7).
	CranesOut int    `json:"cranes_out,omitempty"`
	Cranes    int    `json:"cranes,omitempty"`
	Reason    string `json:"reason"`
}

// run holds one run's state: the log, the numbers tools have produced, and
// the latest result of each tool. Everything from ep down is reset for
// each episode.
type run struct {
	cfg Config
	log *eventlog.Writer

	ep       sim.Scene
	emailRef string
	known    *checker.Known

	mu        sync.Mutex
	report    *Report
	reportRef string
	results   map[string]toolResult
	drafts    []Draft
	questions []Question
	// decide is set when a check leaves a decision to the operator; until
	// the operator answers yes, no draft may be written.
	decide  string
	decided bool
	// findings are the Security Agent's, for a message the guardrail held.
	findings    []finding
	findingsRef string
	pendingCost *eventlog.Cost
	modelCalls  int
	epCalls     int
	callNames   map[string]string
	// callShip is the ship each call was about, in a fleet run, by call ID.
	callShip map[string]string
	// forLog holds what a tool returned for the page but not for the
	// model, by call ID: route geometry, weather samples and the grid file.
	forLog map[string]map[string]any
}

type toolResult struct {
	ref    string
	args   map[string]any
	output any
}

// emit writes an event, attaching any model usage not yet accounted for.
// In a fleet run it names the ship the event is about (schema 1.6).
func (r *run) emit(e eventlog.Event) {
	if e.Ship == "" && len(r.fleet()) > 1 {
		e.Ship = r.eventShip(e)
	}
	r.mu.Lock()
	if r.pendingCost != nil {
		e.Cost = *r.pendingCost
		id := r.cfg.ModelID
		e.Model = &id
		r.pendingCost = nil
	}
	r.mu.Unlock()
	if _, err := r.log.Write(e); err != nil {
		panic(err) // a broken log makes the run worthless; stop loudly
	}
}

func (r *run) latest(name string) (toolResult, bool) {
	r.mu.Lock()
	defer r.mu.Unlock()
	t, ok := r.results[name]
	return t, ok
}

// fleet is every ship in the current work: the scene's fleet, or its one voyage.
func (r *run) fleet() []sim.Voyage {
	if len(r.ep.Fleet) > 0 {
		return r.ep.Fleet
	}
	return []sim.Voyage{r.ep.Voyage}
}

// voyage finds a ship in the current work by ID.
func (r *run) voyage(id string) (sim.Voyage, bool) {
	i := slices.IndexFunc(r.fleet(), func(v sim.Voyage) bool { return strings.EqualFold(v.Ship.ID, strings.TrimSpace(id)) })
	if i < 0 {
		return sim.Voyage{}, false
	}
	return r.fleet()[i], true
}

// forecastRef is the source of the numbers in a forecast trigger.
func forecastRef(f *sim.ForecastIssued) string {
	return "forecast:" + strings.ToLower(strings.ReplaceAll(f.Model, " ", "-")) + "-" + f.Run.Format("20060102T15")
}

// Record runs the scene once and writes <run_id>.jsonl and <run_id>.header.json.
func Record(ctx context.Context, cfg Config) (Result, error) {
	if cfg.Operator == nil {
		cfg.Operator = func(Draft) Decision { return Decision{Approve: true} }
	}
	if cfg.Answer == nil {
		cfg.Answer = func(Question) Answer { return Answer{Yes: true, Note: "Yes, go ahead: draft it for my approval."} }
	}
	if cfg.MaxModelCalls == 0 {
		cfg.MaxModelCalls = 20
	}
	if len(cfg.Episodes) == 0 {
		return Result{}, errors.New("no episodes to run")
	}
	sc := cfg.Episodes[0]
	who := shortShip(sc.Voyage.Ship.Name)
	if len(sc.Fleet) > 1 {
		who = "fleet"
	}
	runID := fmt.Sprintf("%s-%s-%s-r%d", sc.ID, who, modelSlug(cfg.ModelID), cfg.Attempt)
	if err := os.MkdirAll(cfg.OutDir, 0o755); err != nil {
		return Result{}, err
	}
	logPath := filepath.Join(cfg.OutDir, runID+".jsonl")
	f, err := os.OpenFile(logPath, os.O_RDWR|os.O_CREATE|os.O_TRUNC, 0o644)
	if err != nil {
		return Result{}, err
	}
	defer f.Close()

	r := &run{
		cfg:       cfg,
		log:       eventlog.NewWriter(f, eventlog.Event{RunID: runID, Attempt: cfg.Attempt, Scene: sc.ID, Ship: baseShip(sc)}, sc.SimTime),
		callNames: map[string]string{},
		callShip:  map[string]string{},
		forLog:    map[string]map[string]any{},
	}
	for _, ep := range cfg.Episodes {
		r.start(ep)
		if err := r.execute(ctx); err != nil {
			r.emit(eventlog.Event{Actor: eventlog.ActorAgent, Type: eventlog.TypeError, Summary: "Run stopped: " + firstLine(err.Error()), Detail: map[string]any{"error": err.Error()}})
			return Result{RunID: runID, LogPath: logPath}, err
		}
	}

	header := eventlog.Header{
		SchemaVersion: eventlog.SchemaVersion, RunID: runID, Scene: sc.ID, Model: cfg.ModelID,
		RecordedAt: time.Now().UTC(), PromptVersion: PromptVersion, ToolVersions: cfg.ToolVersions,
		Attempt: cfg.Attempt, TotalAttempts: cfg.TotalAttempts,
		Notes: fmt.Sprintf("%d model calls. Synthetic data; operator actions are simulated.", r.modelCalls),
	}
	hb, _ := json.MarshalIndent(header, "", "  ")
	if err := os.WriteFile(filepath.Join(cfg.OutDir, runID+".header.json"), append(hb, '\n'), 0o644); err != nil {
		return Result{}, err
	}

	if _, err := f.Seek(0, 0); err != nil {
		return Result{}, err
	}
	events, err := eventlog.Read(f)
	if err != nil {
		return Result{}, err
	}
	if problems := eventlog.Validate(events); len(problems) > 0 {
		return Result{}, fmt.Errorf("event log fails §6 checks: %w", errors.Join(problems...))
	}
	return Result{RunID: runID, LogPath: logPath, Events: len(events), ModelCalls: r.modelCalls, Drafts: r.drafts}, nil
}

// start resets the per-episode state for the next incoming e-mail.
func (r *run) start(ep sim.Scene) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.ep = ep
	r.emailRef = "email:" + ep.Email.ID
	r.known = checker.NewKnown()
	r.known.AddText(ep.Email.Body)
	r.known.AddText(ep.Voyage.Ship.ID + " " + ep.Email.ID)
	if ep.Forecast != nil {
		r.emailRef = forecastRef(ep.Forecast)
		r.known.AddValue(ep.Forecast)
		for _, v := range ep.Fleet {
			r.known.AddText(v.Ship.ID)
		}
	}
	r.report, r.reportRef = nil, ""
	r.decide, r.decided = "", false
	r.findings, r.findingsRef = nil, ""
	r.results = map[string]toolResult{}
	r.epCalls = 0
	r.log.SimTime = ep.SimTime
}

// emailArrives logs an incoming e-mail and runs the inbound guardrail on
// its sender. It returns the prompt for the Ops Agent, or screened when the
// sender is unknown and the e-mail goes to the Security Agent alone.
func (r *run) emailArrives() (prompt string, screened bool) {
	sc := r.ep
	v := sc.Voyage
	arrives := fmt.Sprintf("Noon report from %s arrives in the operations inbox", v.Ship.Name)
	if sc.Report.ID == "" {
		arrives = fmt.Sprintf("E-mail about %s arrives in the operations inbox: %s", v.Ship.Name, sc.Email.Subject)
	}
	r.emit(eventlog.Event{
		Actor: eventlog.ActorSim, Type: eventlog.TypeMessageReceived,
		Summary:    arrives,
		Detail:     map[string]any{"email": sc.Email},
		SourceRefs: []string{r.emailRef},
	})

	// Inbound guardrail: plain code, before any agent sees the message.
	sender := checker.CheckSender(sc.Email.From, v.Contacts)
	if !sender.Known {
		return "", true
	}
	said := fmt.Sprintf("Sender is the ship's known address (%s)", sender.Contact.Role)
	if sender.Contact.Role != "ship" {
		said = fmt.Sprintf("Sender is a verified contact: %s (%s)", sender.Contact.Name, sender.Contact.Role)
	}
	r.emit(eventlog.Event{Actor: eventlog.ActorGuardrail, Type: eventlog.TypeCheck,
		Summary: said, Detail: map[string]any{"sender": sender, "passed": true}, SourceRefs: []string{r.emailRef, "contacts:" + v.Ship.ID}})
	return fmt.Sprintf("New e-mail in the operations inbox.\nShip ID: %s (sender verified by the guardrail)\nE-mail ID: %s\nFrom: %s\nSubject: %s\n\n%s",
		v.Ship.ID, sc.Email.ID, sc.Email.From, sc.Email.Subject, sc.Email.Body), false
}

// forecastArrives logs a new forecast run as the trigger. There is no
// sender to check: the forecast comes from the weather connector.
func (r *run) forecastArrives() string {
	fi := r.ep.Forecast
	ships := make([]string, 0, len(r.ep.Fleet))
	for _, v := range r.ep.Fleet {
		ships = append(ships, fmt.Sprintf("%s (%s)", v.Ship.ID, v.Ship.Name))
	}
	detail := map[string]any{"forecast": fi, "ships": ships}
	if r.cfg.Forecast != nil {
		detail["grid_ref"] = r.cfg.Forecast.Ref
	}
	r.emit(eventlog.Event{
		Actor: eventlog.ActorSim, Type: eventlog.TypeForecastAvailable,
		Summary:    fmt.Sprintf("New forecast: the %s run of %s UTC is out. No e-mail; the desk scans the ships at sea", fi.Model, fi.Run.Format("02 Jan 15z")),
		Detail:     detail,
		SourceRefs: []string{r.emailRef},
	})
	return fmt.Sprintf("A new forecast run is out: %s, run of %s, available %s UTC. No e-mail has arrived.\nShips at sea: %s\nNot scanned: %s",
		fi.Model, fi.Run.Format("02 Jan 15z"), fi.Available.Format("02 Jan 1504"), strings.Join(ships, ", "), strings.Join(fi.NotScanned, "; "))
}

func (r *run) execute(ctx context.Context) error {
	var prompt string
	if r.ep.Forecast != nil {
		prompt = r.forecastArrives()
	} else {
		var screened bool
		prompt, screened = r.emailArrives()
		if screened {
			return r.screen(ctx, checker.CheckSender(r.ep.Email.From, r.ep.Voyage.Contacts))
		}
	}

	root, err := r.agents(ctx)
	if err != nil {
		return err
	}
	rn, sess, err := r.runner(ctx, root)
	if err != nil {
		return err
	}
	msg := genai.NewContentFromText(prompt, genai.RoleUser)
	if err := r.consume(rn.Run(ctx, "operator", sess, msg, agent.RunConfig{})); err != nil {
		return err
	}
	r.reviewQueue()

	// Questions wait for the operator. On a yes, the Ops Agent picks the
	// work up again in the same conversation; on a no, nothing more is done.
	for _, q := range r.takeQuestions() {
		r.log.SimTime = r.log.SimTime.Add(10 * time.Minute)
		a := r.cfg.Answer(q)
		r.emit(eventlog.Event{Actor: eventlog.ActorHumanSim, Type: eventlog.TypeAnswer, Summary: "Operator answers (simulated operator action): " + a.Note,
			Detail: map[string]any{"question": q.Question, "yes": a.Yes, "text": a.Note}, SourceRefs: []string{"question:" + q.CallID}})
		if !a.Yes {
			continue
		}
		r.mu.Lock()
		r.decided = true
		r.mu.Unlock()
		r.known.AddText(a.Note)
		reply := genai.NewContentFromText(fmt.Sprintf("The operator answers your question (%s): %s", q.Question, a.Note), genai.RoleUser)
		if err := r.consume(rn.Run(ctx, "operator", sess, reply, agent.RunConfig{})); err != nil {
			return err
		}
		r.reviewQueue()
	}
	return nil
}

// runner starts a session for an agent tree, with the call budget and the
// tool hooks attached.
func (r *run) runner(ctx context.Context, root agent.Agent) (*runner.Runner, string, error) {
	budget, err := plugin.New(plugin.Config{
		Name:                "shorewatch",
		BeforeModelCallback: r.beforeModel,
		BeforeToolCallback:  r.beforeTool,
		AfterToolCallback:   r.afterTool,
	})
	if err != nil {
		return nil, "", err
	}
	sessions := session.InMemoryService()
	sess, err := sessions.Create(ctx, &session.CreateRequest{AppName: "shorewatch", UserID: "operator"})
	if err != nil {
		return nil, "", err
	}
	rn, err := runner.New(runner.Config{AppName: "shorewatch", Agent: root, SessionService: sessions, PluginConfig: runner.PluginConfig{Plugins: []*plugin.Plugin{budget}}})
	return rn, sess.Session.ID(), err
}

// screen handles a message from a sender the guardrail does not know. Only
// the Security Agent reads it, alone and unable to act.
// Code then blocks it and alerts the operator, whatever the agent
// found: the unknown sender is reason enough.
func (r *run) screen(ctx context.Context, sender checker.SenderResult) error {
	v := r.ep.Voyage
	contactsRef := "contacts:" + v.Ship.ID
	summary := "Sender is not a known contact. The message is held from the agents; only the Security Agent reads it"
	var reasons []map[string]any
	if c := sender.LookalikeOf; c != nil {
		summary = fmt.Sprintf("Sender's domain %s is one character away from %s's %s. The message is held from the agents; only the Security Agent reads it", sender.Domain, c.Name, c.Domain)
		reasons = append(reasons, map[string]any{"by": "guardrail", "kind": "lookalike_sender", "text": fmt.Sprintf("The sender's domain, %s, imitates %s's real domain, %s.", sender.Domain, c.Name, c.Domain)})
	} else {
		reasons = append(reasons, map[string]any{"by": "guardrail", "kind": "unknown_sender", "text": fmt.Sprintf("The sender, %s, is not in the verified contacts.", sender.Domain)})
	}
	r.emit(eventlog.Event{Actor: eventlog.ActorGuardrail, Type: eventlog.TypeCheck, Summary: summary,
		Detail: map[string]any{"sender": sender, "passed": false, "routed_to": securityName}, SourceRefs: []string{r.emailRef, contactsRef}})

	instruments, err := r.instruments()
	if err != nil {
		return err
	}
	clientT, serverT := mcp.NewInMemoryTransports()
	if _, err := tools.Server(r.cfg.Graph, []sim.Voyage{v}, nil).Connect(ctx, serverT, nil); err != nil {
		return err
	}
	ts, err := mcptoolset.New(mcptoolset.Config{Transport: clientT, ToolFilter: tool.StringPredicate([]string{"contacts"})})
	if err != nil {
		return err
	}
	order := fmt.Sprintf("\n\nCurrent work: ship ID %s (%s), e-mail ID %s.", v.Ship.ID, v.Ship.Name, r.ep.Email.ID)
	sec, err := llmagent.New(llmagent.Config{
		Name: securityName, Model: r.cfg.Model, Instruction: securityInstruction + order,
		Tools:    []tool.Tool{instruments["state_reason"], instruments["read_email"], instruments["report_findings"]},
		Toolsets: []tool.Toolset{ts},
	})
	if err != nil {
		return err
	}
	rn, sess, err := r.runner(ctx, sec)
	if err != nil {
		return err
	}
	msg := genai.NewContentFromText(fmt.Sprintf("The guardrail held e-mail %s for ship %s. Screen it.", r.ep.Email.ID, v.Ship.ID), genai.RoleUser)
	if err := r.consume(rn.Run(ctx, "operator", sess, msg, agent.RunConfig{})); err != nil {
		return err
	}

	r.mu.Lock()
	findings, findingsRef := r.findings, r.findingsRef
	r.mu.Unlock()
	refs := []string{r.emailRef, contactsRef}
	if findingsRef != "" {
		refs = append(refs, findingsRef)
	}
	for _, f := range findings {
		if f.Kind == "lookalike_sender" && sender.LookalikeOf != nil {
			continue // the guardrail has said so already
		}
		reasons = append(reasons, map[string]any{"by": securityName, "kind": f.Kind, "text": f.Why, "quote": f.Quote})
	}
	r.emit(eventlog.Event{Actor: eventlog.ActorGuardrail, Type: eventlog.TypeBlocked, Summary: "Blocked: the e-mail is not passed to any agent that can act, and nothing is drafted",
		Detail: map[string]any{"reasons": reasons}, SourceRefs: refs})

	action := "Do not pay or change any details on the strength of this e-mail."
	if c := sender.LookalikeOf; c != nil && c.Phone != "" {
		action += fmt.Sprintf(" Verify by telephone with %s on the number on file, %s", c.Name, c.Phone)
		if n := phoneRe.FindString(r.ep.Email.Body); n != "" && n != c.Phone {
			action += fmt.Sprintf(", not the number in the e-mail (%s)", n)
		}
		action += "."
	}
	r.emit(eventlog.Event{Actor: eventlog.ActorGuardrail, Type: eventlog.TypeAlert,
		Summary: fmt.Sprintf("Alert to the operator: possible payment fraud, %d reasons. %s", len(reasons), action),
		Detail:  map[string]any{"reasons": reasons, "action": action, "contact": sender.LookalikeOf}, SourceRefs: refs})
	return nil
}

var phoneRe = regexp.MustCompile(`\+\d[\d ]{8,}\d`)

// takeQuestions returns the questions asked so far and clears them.
func (r *run) takeQuestions() []Question {
	r.mu.Lock()
	defer r.mu.Unlock()
	q := r.questions
	r.questions = nil
	return q
}

// reviewQueue is the Approval gate: plain code, not an agent. When the
// agents have finished with an e-mail, the simulated operator approves,
// edits or rejects each draft waiting in the queue. Nothing is ever sent.
func (r *run) reviewQueue() {
	for i := range r.drafts {
		d := &r.drafts[i]
		if d.Status != statusQueued {
			continue
		}
		r.log.SimTime = r.log.SimTime.Add(10 * time.Minute)
		dec := r.cfg.Operator(*d)
		ref := "approval:" + d.CallID
		switch {
		case dec.Approve && dec.EditedBody != "":
			r.emit(eventlog.Event{Actor: eventlog.ActorHumanSim, Type: eventlog.TypeEdited, Summary: "Operator edits the draft (simulated operator action): " + dec.Note,
				Detail: map[string]any{"before": d.Body, "after": dec.EditedBody}, SourceRefs: []string{ref}})
			d.Body = dec.EditedBody
			fallthrough
		case dec.Approve:
			what, status, filed := "draft", "approved, not sent (demo)", "Approved, not sent (demo): the draft is filed for the operator to send"
			if d.Entry != nil {
				what, status, filed = "entry", "approved, not posted (demo)", "Approved, not posted (demo): the entry is filed for the voyage management system"
			}
			r.emit(eventlog.Event{Actor: eventlog.ActorHumanSim, Type: eventlog.TypeApproved, Summary: "Operator approves the " + what + " (simulated operator action)", SourceRefs: []string{ref}})
			d.Status = status
			r.emit(eventlog.Event{Actor: eventlog.ActorTool, Type: eventlog.TypeToolResult, Summary: filed,
				Detail: map[string]any{"tool": "approval_gate", "status": d.Status}, SourceRefs: []string{ref}})
		default:
			what := "draft"
			if d.Entry != nil {
				what = "entry"
			}
			r.emit(eventlog.Event{Actor: eventlog.ActorHumanSim, Type: eventlog.TypeRejected, Summary: "Operator rejects the " + what + " (simulated operator action)", SourceRefs: []string{ref}})
			d.Status = "rejected by the operator"
		}
	}
}

func (r *run) agents(ctx context.Context) (agent.Agent, error) {
	v := r.ep.Voyage
	// connectors gives one agent its own connection to the stand-in
	// systems, limited to the tools its job needs (least power).
	connectors := func(names ...string) (tool.Toolset, error) {
		clientT, serverT := mcp.NewInMemoryTransports()
		if _, err := tools.Server(r.cfg.Graph, r.fleet(), r.cfg.Weather).Connect(ctx, serverT, nil); err != nil {
			return nil, err
		}
		return mcptoolset.New(mcptoolset.Config{Transport: clientT, ToolFilter: tool.StringPredicate(names)})
	}
	type spec struct {
		name, desc, instruction string
		tools                   []string // instruments and actions, by name
		connectors              []string
	}
	weatherTools := []string{}
	if r.cfg.Weather != nil {
		weatherTools = []string{"weather_on_track"}
	}
	voyageTools, voyageSteps := []string{"state_reason", "checked_report", "check_eta", "check_notices"}, voyageInstruction
	// Only a forecast issued before the report may be used: no hindsight.
	if fc := r.cfg.Forecast; fc != nil && v.Loading != nil && !checker.ForecastRun(fc).IsZero() && !checker.ForecastRun(fc).After(r.ep.Report.Time) {
		voyageTools, voyageSteps = append(voyageTools, "check_forecast_eta"), voyageInstruction+voyageForecastStep
	}
	voyageConnectors := []string{"distance_to_go", "voyage_history", "cp_terms"}
	// With a voyage management system record (Round 2), the Voyage Agent
	// also handles an engine stop as a possible off-hire.
	if v.VMS != nil {
		voyageTools, voyageSteps = append(voyageTools, "check_stoppage", "propose_entry"), voyageVMSInstruction
		voyageConnectors = append(voyageConnectors, "voyage_system", "off_hire_statement")
	}
	correspondence := spec{correspondenceName, "Writes outgoing e-mails as drafts for approval, and raises alerts for the operator.", correspondenceInstruction, []string{"state_reason", "draft_email", "alert_operator"}, []string{"contacts"}}
	staff := []spec{
		{reportsName, "Extracts a noon report e-mail into checked fields.", reportsInstruction, []string{"read_email", "submit_extraction"}, nil},
		{voyageName, "Checks the Master's ETA and the charter party ETA notices for a noon report.", voyageSteps, voyageTools, voyageConnectors},
		{performanceName, "Judges speed and fuel consumption against the charter party warranty, with the weather as evidence.", performanceInstruction, []string{"state_reason", "checked_report", "check_consumption", "classify_weather_day", "add_voyage_note"}, append([]string{"cp_terms"}, weatherTools...)},
		correspondence,
	}
	opsSteps := opsInstruction
	if v.VMS != nil {
		opsSteps += opsStopStep
	}
	// The work order: who and what this e-mail is about, set by code so no
	// agent depends on another agent remembering to pass it on.
	order := fmt.Sprintf("\n\nCurrent work: ship ID %s (%s), e-mail ID %s.", v.Ship.ID, v.Ship.Name, r.ep.Email.ID)
	if fi := r.ep.Forecast; fi != nil {
		// A forecast, not an e-mail: only the colleagues that work on it.
		staff = []spec{
			{weatherName, "Reads a new forecast along each ship's remaining route and says what, if anything, the office should tell each Master.", weatherInstruction, []string{"state_reason", "weather_ahead", "add_voyage_note"}, []string{"voyage_history"}},
			correspondence,
		}
		opsSteps = opsForecastInstruction
		ids := make([]string, len(r.ep.Fleet))
		for i, fv := range r.ep.Fleet {
			ids[i] = fmt.Sprintf("%s (%s)", fv.Ship.ID, fv.Ship.Name)
		}
		order = fmt.Sprintf("\n\nCurrent work: the %s forecast run of %s, available %s UTC. Ships at sea: %s.", fi.Model, fi.Run.Format("02 Jan 15z"), fi.Available.Format("02 Jan 1504"), strings.Join(ids, ", "))
	} else if r.ep.Report.ID == "" {
		// An e-mail about the voyage from someone ashore, not a noon report
		// (Scene 5): the Voyage Agent owns the port call; no report to extract.
		port := spec{voyageName, "Works out what an e-mail about the voyage means for the port call, the speed and the ETA notices, and proposes entries in the voyage management system.", voyagePortInstruction,
			[]string{"state_reason", "read_email", "check_speed_plan", "check_notices", "propose_entry"},
			[]string{"voyage_system", "voyage_history", "cp_terms", "ais_position", "distance_to_go", "speed_advice"}}
		opsSteps = opsPortInstruction
		// A fleet-week voyage: a passage declined under the
		// war risks clause, or a crane breakdown under the crane rider.
		if v.CP.WarRisks != "" {
			port.instruction += voyagePortFleetStep
			port.tools = append(port.tools, "check_route_change")
			port.connectors = append(port.connectors, "off_hire_statement")
			opsSteps += opsPortFleetStep
		}
		staff = []spec{port, correspondence}
	}
	instruments, err := r.instruments()
	if err != nil {
		return nil, err
	}
	pick := func(names []string) []tool.Tool {
		var out []tool.Tool
		for _, n := range names {
			out = append(out, instruments[n])
		}
		return out
	}
	var subs []agent.Agent
	for _, sp := range staff {
		cfg := llmagent.Config{
			Name: sp.name, Model: r.cfg.Model, Mode: llmagent.ModeSingleTurn,
			DisallowTransferToParent: true, DisallowTransferToPeers: true,
			Description: sp.desc, Instruction: sp.instruction + order, Tools: pick(sp.tools),
		}
		if len(sp.connectors) > 0 {
			ts, err := connectors(sp.connectors...)
			if err != nil {
				return nil, err
			}
			cfg.Toolsets = []tool.Toolset{ts}
		}
		a, err := llmagent.New(cfg)
		if err != nil {
			return nil, err
		}
		subs = append(subs, a)
	}
	return llmagent.New(llmagent.Config{
		Name: opsName, Model: r.cfg.Model, Instruction: opsSteps + order,
		Tools: pick([]string{"state_reason", "ask_operator"}), SubAgents: subs,
	})
}

// beforeModel enforces the call budget.
func (r *run) beforeModel(agent.Context, *model.LLMRequest) (*model.LLMResponse, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.modelCalls++
	r.epCalls++
	if r.epCalls > r.cfg.MaxModelCalls {
		return nil, fmt.Errorf("model call budget of %d exceeded", r.cfg.MaxModelCalls)
	}
	return nil, nil
}

// beforeTool asks seaRoute for the route geometry, so the page can draw the
// route this call produced.
func (r *run) beforeTool(_ agent.Context, t tool.Tool, args map[string]any) (map[string]any, error) {
	if t.Name() == "distance_to_go" {
		args["include_path"] = true
	}
	return nil, nil
}

// logOnly are tool output fields that go into the log for the page but are
// kept from the model: route geometry, weather samples and the grid file.
// They are long, and their numbers are not ones the agent may quote.
var logOnly = []string{"path", "samples", "grid_ref"}

// afterTool records every connector result, so checks and the number guard
// can use it, and sets aside the fields in logOnly.
func (r *run) afterTool(ctx agent.Context, t tool.Tool, args, result map[string]any, err error) (map[string]any, error) {
	if err != nil || result == nil {
		return nil, nil
	}
	out, ok := result["output"]
	if !ok {
		return nil, nil
	}
	var replaced map[string]any
	if m, ok := out.(map[string]any); ok && slices.ContainsFunc(logOnly, func(k string) bool { return m[k] != nil }) {
		kept := map[string]any{}
		m = maps.Clone(m)
		for _, k := range logOnly {
			if m[k] != nil {
				kept[k] = m[k]
				delete(m, k)
			}
		}
		r.mu.Lock()
		r.forLog[ctx.FunctionCallID()] = kept
		r.mu.Unlock()
		out = m
		replaced = maps.Clone(result)
		replaced["output"] = m
	}
	r.mu.Lock()
	r.results[t.Name()] = toolResult{ref: eventlog.ToolRef(t.Name(), ctx.FunctionCallID()), args: args, output: out}
	// A crane breakdown's hire statement leaves the off-hire to the operator,
	// as check_stoppage does for an engine stop.
	if n, _ := args["cranes_out"].(float64); t.Name() == "off_hire_statement" && n > 0 && r.decide == "" {
		r.decide = "whether to put the ship off hire pro rata and notify the owner"
	}
	r.mu.Unlock()
	r.known.AddValue(out)
	return replaced, nil
}

// consume maps the runner's events onto the log.
func (r *run) consume(events func(func(*session.Event, error) bool)) error {
	for ev, err := range events {
		if err != nil {
			return err
		}
		r.mapEvent(ev)
	}
	return nil
}

func (r *run) mapEvent(ev *session.Event) {
	if ev.Content == nil || ev.Partial {
		return
	}
	if u := ev.UsageMetadata; u != nil {
		// Add up: several model calls can happen before the next logged event.
		r.mu.Lock()
		if r.pendingCost == nil {
			r.pendingCost = &eventlog.Cost{}
		}
		r.pendingCost.InputTokens += u.PromptTokenCount
		r.pendingCost.OutputTokens += u.CandidatesTokenCount
		r.mu.Unlock()
	}
	actor := eventlog.ActorAgent
	if ev.Author != opsName {
		actor = eventlog.ActorSubagent
	}

	// Stated reasons first: they explain the calls made alongside them.
	parts := slices.Clone(ev.Content.Parts)
	slices.SortStableFunc(parts, func(a, b *genai.Part) int {
		isReason := func(p *genai.Part) bool { return p.FunctionCall != nil && p.FunctionCall.Name == "state_reason" }
		switch {
		case isReason(a) && !isReason(b):
			return -1
		case isReason(b) && !isReason(a):
			return 1
		}
		return 0
	})
	for _, p := range parts {
		switch {
		case p.Thought:
			// Hidden reasoning is never logged; only stated reasons are (principle 8).
		case p.FunctionCall != nil:
			r.mapCall(actor, ev.Author, p.FunctionCall)
		case p.FunctionResponse != nil:
			r.mapResponse(p.FunctionResponse)
		case strings.TrimSpace(p.Text) != "" && ev.Author != "user":
			r.mapText(actor, ev.Author, p.Text)
		}
	}
}

func (r *run) mapCall(actor, author string, fc *genai.FunctionCall) {
	r.mu.Lock()
	r.callNames[fc.ID] = fc.Name
	r.mu.Unlock()
	ref := eventlog.ToolRef(fc.Name, fc.ID)
	switch fc.Name {
	case "state_reason":
		r.logReason(actor, author, fc)
	case "checked_report", "check_eta", "check_notices", "check_consumption", "classify_weather_day", "add_voyage_note", "alert_operator", "ask_operator", "report_findings", "draft_email", "submit_extraction", "read_email", "weather_ahead", "propose_entry", "check_speed_plan", "check_stoppage":
		// These tools log their own, richer events.
	case reportsName, voyageName, performanceName, correspondenceName, weatherName:
		req, _ := fc.Args["request"].(string)
		r.emit(eventlog.Event{Actor: actor, Agent: author, Type: eventlog.TypePlan, Summary: fmt.Sprintf("%s asks the %s: %s", agentTitle[author], agentTitle[fc.Name], firstLine(req)),
			Detail: map[string]any{"delegate_to": fc.Name, "args": fc.Args}, SourceRefs: []string{ref}})
	default:
		r.emit(eventlog.Event{Actor: actor, Agent: author, Type: eventlog.TypeToolCall, Summary: callSummary(fc.Name, fc.Args), Detail: map[string]any{"tool": fc.Name, "args": fc.Args}, SourceRefs: []string{ref}})
	}
}

func (r *run) mapResponse(fr *genai.FunctionResponse) {
	r.mu.Lock()
	name := r.callNames[fr.ID]
	r.mu.Unlock()
	if name == "" {
		name = fr.Name
	}
	ref := eventlog.ToolRef(name, fr.ID)
	switch name {
	case "state_reason", "checked_report", "check_eta", "check_notices", "check_consumption", "classify_weather_day", "add_voyage_note", "alert_operator", "ask_operator", "report_findings", "draft_email", "submit_extraction", "read_email", "weather_ahead", "propose_entry", "check_speed_plan", "check_stoppage":
		// These log their own events, except when their arguments fail the
		// schema: then the tool never runs, and only this sees the error.
		if e, failed := fr.Response["error"]; failed {
			r.emit(eventlog.Event{Actor: eventlog.ActorTool, Type: eventlog.TypeError, Summary: fmt.Sprintf("%s refused the call: %v", name, e),
				Detail: map[string]any{"tool": name, "result": fr.Response}, SourceRefs: []string{ref}})
		}
		return
	case reportsName:
		r.emit(eventlog.Event{Actor: eventlog.ActorSubagent, Agent: name, Type: eventlog.TypeToolResult, Summary: "Reports Agent hands the checked report to the Ops Agent", Detail: map[string]any{"result": fr.Response}, SourceRefs: []string{ref}})
	case voyageName, performanceName, correspondenceName, weatherName:
		reply, _ := fr.Response["result"].(string)
		r.emit(eventlog.Event{Actor: eventlog.ActorSubagent, Agent: name, Type: eventlog.TypeToolResult, Summary: agentTitle[name] + " reports: " + firstLine(reply),
			Detail: map[string]any{"result": fr.Response, "text": reply}, SourceRefs: append([]string{ref}, r.numberRefs()...)})
		if u := r.known.Unsupported(reply); len(u) > 0 {
			r.emit(eventlog.Event{Actor: eventlog.ActorChecker, Type: eventlog.TypeAlert, Summary: "Number guard: the " + agentTitle[name] + "'s report contains " + checker.Summary(u), Detail: map[string]any{"unsupported": u, "text": reply}})
		}
	default:
		actor, typ := eventlog.ActorTool, eventlog.TypeToolResult
		if _, failed := fr.Response["error"]; failed {
			typ = eventlog.TypeError
		}
		detail := map[string]any{"tool": name, "result": fr.Response}
		r.mu.Lock()
		maps.Copy(detail, r.forLog[fr.ID])
		r.mu.Unlock()
		r.emit(eventlog.Event{Actor: actor, Type: typ, Summary: resultSummary(name, fr.Response), Detail: detail, SourceRefs: []string{ref}})
	}
}

// mapText logs the agent's own words. Numbers in them must come from tools;
// any that don't are flagged by the number guard, not hidden.
func (r *run) mapText(actor, author, text string) {
	if actor == eventlog.ActorSubagent {
		return // an agent's reply is logged when it reaches the Ops Agent
	}
	r.emit(eventlog.Event{Actor: actor, Agent: author, Type: eventlog.TypePlan, Summary: firstLine(text), Detail: map[string]any{"text": text}, SourceRefs: r.numberRefs()})
	if u := r.known.Unsupported(text); len(u) > 0 {
		r.emit(eventlog.Event{Actor: eventlog.ActorChecker, Type: eventlog.TypeAlert, Summary: "Number guard: the agent's message contains " + checker.Summary(u), Detail: map[string]any{"unsupported": u, "text": text}})
	}
}

// numberRefs are the sources of every number known so far in this run.
func (r *run) numberRefs() []string {
	r.mu.Lock()
	defer r.mu.Unlock()
	refs := []string{r.emailRef}
	if r.reportRef != "" {
		refs = append(refs, r.reportRef)
	}
	for _, t := range r.results {
		refs = append(refs, t.ref)
	}
	return refs
}

func shortShip(name string) string {
	name = strings.TrimPrefix(strings.ToLower(name), "mv ")
	return strings.ReplaceAll(name, " ", "-")
}

func modelSlug(id string) string {
	return strings.NewReplacer("/", "-", ".", "-", ":", "-").Replace(id)
}

func firstLine(s string) string {
	s = strings.TrimSpace(s)
	if i := strings.IndexByte(s, '\n'); i >= 0 {
		s = s[:i]
	}
	if len(s) > 200 {
		s = s[:197] + "..."
	}
	return s
}

// AddLine returns a simulated operator who, for drafts to address, adds one
// line as a paragraph before the sign-off, then approves. Other drafts are
// approved unchanged.
func AddLine(address, line, note string) func(Draft) Decision {
	return func(d Draft) Decision {
		if !slices.ContainsFunc(d.To, func(a string) bool { return strings.EqualFold(strings.TrimSpace(a), address) }) {
			return Decision{Approve: true}
		}
		body := strings.TrimRight(d.Body, "\n")
		paras := strings.Split(body, "\n\n")
		if len(paras) > 1 {
			paras = slices.Insert(paras, len(paras)-1, line)
		} else {
			paras = append(paras, line)
		}
		return Decision{Approve: true, EditedBody: strings.Join(paras, "\n\n"), Note: note}
	}
}
