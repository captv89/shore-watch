package desk

import (
	"encoding/json"
	"slices"

	"errors"
	"fmt"
	"math"
	"strconv"
	"strings"
	"time"

	"google.golang.org/genai"

	"google.golang.org/adk/v2/agent"
	"google.golang.org/adk/v2/tool"
	"google.golang.org/adk/v2/tool/functiontool"

	"github.com/captv89/seaRoute/route"
	"github.com/captv89/shorewatch/agent/checker"
	"github.com/captv89/shorewatch/agent/eventlog"
	"github.com/captv89/shorewatch/sim"
	"github.com/captv89/shorewatch/tools"
)

type reasonIn struct {
	Reason string `json:"reason" jsonschema:"one or two plain sentences: why you take your next step"`
}

type reportIn struct {
	ReportID string `json:"report_id" jsonschema:"the e-mail ID of the noon report"`
}

type draftIn struct {
	To      []string `json:"to" jsonschema:"recipient e-mail addresses, from the contacts tool"`
	Subject string   `json:"subject"`
	Body    string   `json:"body"`
	Purpose string   `json:"purpose" jsonschema:"one line: why this message is needed"`
	About   []string `json:"about" jsonschema:"topic IDs this is about, from: eta, eta_notice, eta_forecast, consumption, rob_vlsfo, rob_mgo, speed, position, missing_figures, payment_change, heavy_weather, berth_delay, off_hire, route_change, bunkers, other"`
}

type entryIn struct {
	Kind      string   `json:"kind" jsonschema:"one of: port_call_eta (a new ETA for a port call), off_hire (a period the ship was off hire, billed against the owner)"`
	Port      string   `json:"port,omitempty" jsonschema:"for port_call_eta: the port, as in voyage_system"`
	ETAUTC    string   `json:"eta_utc,omitempty" jsonschema:"for port_call_eta: the new ETA, RFC 3339, e.g. 2025-01-27T06:00:00Z"`
	FromUTC   string   `json:"from_utc,omitempty" jsonschema:"for off_hire: when the ship went off hire, RFC 3339"`
	ToUTC     string   `json:"to_utc,omitempty" jsonschema:"for off_hire: when she came back on hire, RFC 3339"`
	CranesOut int      `json:"cranes_out,omitempty" jsonschema:"for off_hire after a cargo crane breakdown only: how many of the ship's cranes were out of use; the hire stops pro rata"`
	Reason    string   `json:"reason" jsonschema:"one plain sentence for the operator: why this entry; numbers only as given by tools"`
	About     []string `json:"about" jsonschema:"topic IDs this is about, from: eta, eta_notice, eta_forecast, consumption, rob_vlsfo, rob_mgo, speed, position, missing_figures, payment_change, heavy_weather, berth_delay, off_hire, route_change, bunkers, other"`
}

type questionIn struct {
	Question string   `json:"question" jsonschema:"one plain question the operator can answer yes or no; numbers only as given by tools"`
	Proposal string   `json:"proposal" jsonschema:"one sentence: what would be done if the operator says yes"`
	About    []string `json:"about" jsonschema:"topic IDs this is about, from: eta, eta_notice, eta_forecast, consumption, rob_vlsfo, rob_mgo, speed, position, missing_figures, payment_change, heavy_weather, berth_delay, off_hire, route_change, bunkers, other"`
}

type shipIn struct {
	ShipID string `json:"ship_id" jsonschema:"ship ID, e.g. SIM-0001"`
}

type emailIn struct {
	EmailID string `json:"email_id" jsonschema:"the e-mail ID, e.g. nr-0006"`
}

// instruments returns every function tool by name: actions and checks in
// plain code. Each agent is given only the ones its job needs.
func (r *run) instruments() (map[string]tool.Tool, error) {
	out := map[string]tool.Tool{}
	add := func(t tool.Tool, err error) error {
		if err != nil {
			return err
		}
		out[t.Name()] = t
		return nil
	}
	return out, errors.Join(
		add(functiontool.New(functiontool.Config{Name: "read_email", Description: "Read an e-mail from the operations inbox by its ID (stand-in for the mail connector)."}, r.readEmail)),
		add(functiontool.New(functiontool.Config{Name: "submit_extraction", Description: "Submit the fields extracted from the noon report. They are checked against the e-mail."}, r.submitExtraction)),
		add(functiontool.New(functiontool.Config{Name: "state_reason", Description: "Record why you take your next step. Call before each step."}, r.stateReason)),
		add(functiontool.New(functiontool.Config{Name: "checked_report", Description: "The noon report as extracted and checked by the Reports Agent (stand-in for the voyage file)."}, r.checkedReport)),
		add(functiontool.New(functiontool.Config{Name: "check_eta", Description: "Checker (plain code): compares the Master's ETA with distance to go divided by speed. Needs the extracted report and a distance_to_go result."}, r.checkETA)),
		add(functiontool.New(functiontool.Config{Name: "check_notices", Description: "Checker (plain code): applies the CP ETA-notice rules. Needs the extracted report (or, for an e-mail that is not a noon report, a speed_advice result) and voyage_history and cp_terms results."}, r.checkNotices)),
		add(functiontool.New(functiontool.Config{Name: "check_stoppage", Description: "Checker (plain code): checks a main engine stop against the extracted noon report: the times and fuel as the Master wrote them in the remarks, the stop within the report day, and the reported speed over the steaming hours against the distance run."}, r.checkStoppage)),
		add(functiontool.New(functiontool.Config{Name: "check_speed_plan", Description: "Checker (plain code): checks the voyage optimisation service's speed advice against seaRoute: the distance, the arrival at the advised speed, and that it comes before the berth. Needs ais_position, distance_to_go from that position and speed_advice results."}, r.checkSpeedPlan)),
		add(functiontool.New(functiontool.Config{Name: "check_route_change", Description: "Checker (plain code): compares the new route in distance_to_go (called from the AIS position, avoiding the passages the ship will not use) with the planned route from the same position, at the Master's ETA speed: the extra distance and days, the new ETA, and whether the VLSFO on board covers the new route with the company's reserve. Needs ais_position and distance_to_go results."}, r.checkRouteChange)),
		add(functiontool.New(functiontool.Config{Name: "check_forecast_eta", Description: "Checker (plain code): sails the remaining route through the ECMWF forecast as issued, with a simplified speed-loss model (Kwon), and compares the arrival with the Master's ETA. Needs the extracted report, a distance_to_go result and cp_terms."}, r.checkForecastETA)),
		add(functiontool.New(functiontool.Config{Name: "weather_ahead", Description: "Weather connector (plain code): sails the ship's remaining route from her last AIS position, at the Master's ETA speed, through the new forecast run, and lists the stretches of heavy weather (Beaufort 7 or more, or seas of 4 m or more). Gives no route, speed or ETA advice."}, r.weatherAhead)),
		add(functiontool.New(functiontool.Config{Name: "check_consumption", Description: "Checker (plain code): compares the report's VLSFO consumption with the CP warranted daily figure. Needs the extracted report and a cp_terms result."}, r.checkConsumption)),
		add(functiontool.New(functiontool.Config{Name: "classify_weather_day", Description: "Checker (plain code): applies the CP good weather definition to the report day. Needs cp_terms and a weather_on_track result for the 24 h up to the report time."}, r.classifyWeatherDay)),
		add(functiontool.New(functiontool.Config{Name: "add_voyage_note", Description: "File a short note in the voyage file for the operator. Not an e-mail; nothing is sent."}, r.addVoyageNote)),
		add(functiontool.New(functiontool.Config{Name: "alert_operator", Description: "Raise an alert on the operator's desk: something needs their attention. Not an e-mail; nothing is sent."}, r.alertOperator)),
		add(functiontool.New(functiontool.Config{Name: "ask_operator", Description: "Put a yes-or-no question on the operator's desk, for a decision that is the operator's to make. The operator answers after you finish."}, r.askOperator)),
		add(functiontool.New(functiontool.Config{Name: "report_findings", Description: "Report what is wrong with the e-mail. Each quote is checked against the e-mail. Changes nothing; nothing is sent."}, r.reportFindings)),
		add(functiontool.New(functiontool.Config{Name: "propose_entry", Description: "Propose an entry in the voyage management system: a new port call ETA, or an off-hire period. It waits for operator approval; nothing is posted."}, r.proposeEntry)),
		add(functiontool.New(functiontool.Config{Name: "draft_email", Description: "Draft an outgoing e-mail. The guardrail checks recipients and numbers; then it waits for operator approval. Nothing is ever sent."}, r.draftEmail)),
	)
}

func (r *run) readEmail(ctx agent.Context, in emailIn) (map[string]any, error) {
	e := r.ep.Email
	if !strings.EqualFold(strings.TrimSpace(in.EmailID), e.ID) {
		return nil, fmt.Errorf("no e-mail %q in the inbox; the current one is %s", in.EmailID, e.ID)
	}
	r.emit(eventlog.Event{Actor: eventlog.ActorSubagent, Agent: ctx.AgentName(), Type: eventlog.TypeToolCall, Summary: agentTitle[ctx.AgentName()] + " opens the e-mail",
		Detail: map[string]any{"tool": "read_email", "email_id": e.ID}, SourceRefs: []string{eventlog.ToolRef("read_email", ctx.FunctionCallID()), r.emailRef}})
	return map[string]any{"email_id": e.ID, "from": e.From, "subject": e.Subject, "body": e.Body}, nil
}

// stateReason only answers the agent. The rationale (or its refusal) is
// logged when the call is seen, so a reason always comes before the step it explains.
func (r *run) stateReason(_ agent.Context, in reasonIn) (map[string]any, error) {
	if strings.TrimSpace(in.Reason) == "" {
		return map[string]any{"accepted": false, "problem": "The reason is empty. Give one or two plain sentences."}, nil
	}
	if u := r.known.Unsupported(in.Reason); len(u) > 0 {
		return map[string]any{"accepted": false, "problem": "Your reason contains " + checker.Summary(u) + ". Restate it without numbers of your own; refer to tool results instead."}, nil
	}
	return map[string]any{"accepted": true}, nil
}

// logReason writes the agent's stated reason, or the number guard's refusal of it.
func (r *run) logReason(actor, author string, fc *genai.FunctionCall) {
	ref := eventlog.ToolRef(fc.Name, fc.ID)
	reason, _ := fc.Args["reason"].(string)
	if strings.TrimSpace(reason) == "" {
		return // stateReason refuses it; there is no reason to log
	}
	if u := r.known.Unsupported(reason); len(u) > 0 {
		r.emit(eventlog.Event{Actor: eventlog.ActorChecker, Type: eventlog.TypeCheck,
			Summary: "Number guard: stated reason refused, it contains " + checker.Summary(u),
			Detail:  map[string]any{"reason": reason, "unsupported": u, "passed": false}, SourceRefs: []string{ref}})
		return
	}
	r.emit(eventlog.Event{Actor: actor, Agent: author, Type: eventlog.TypeRationale, Summary: agentTitle[author] + "'s stated reason: " + reason,
		Detail: map[string]any{"reason": reason}, SourceRefs: []string{ref}})
}

func (r *run) submitExtraction(ctx agent.Context, x Extraction) (map[string]any, error) {
	ref := eventlog.ToolRef("submit_extraction", ctx.FunctionCallID())
	rep, problems := Validate(x, r.ep.Email.Body)
	if len(problems) > 0 {
		r.emit(eventlog.Event{Actor: eventlog.ActorSubagent, Agent: reportsName, Type: eventlog.TypeCheck,
			Summary: "Reports Agent's extraction fails the schema check: " + problems[0],
			Detail:  map[string]any{"extraction": x, "problems": problems, "passed": false}, SourceRefs: []string{ref, r.emailRef}})
		return map[string]any{"status": "invalid", "problems": problems}, nil
	}
	r.mu.Lock()
	r.report, r.reportRef = &rep, ref
	r.mu.Unlock()
	r.known.AddValue(rep)
	cons := ""
	if rep.ConsVLSFOt > 0 {
		cons = fmt.Sprintf(", VLSFO used %.1f t", rep.ConsVLSFOt)
	}
	missing := MissingForDesk(rep)
	var prev *sim.NoonReport
	if h := r.ep.Voyage.History; len(h) > 0 {
		prev = &h[len(h)-1]
	}
	flags := ReportChecks(rep, prev)
	check := "Schema check passed"
	if len(missing) > 0 {
		check += ". Not in the report: " + strings.Join(missing, ", ")
	}
	r.emit(eventlog.Event{Actor: eventlog.ActorSubagent, Agent: reportsName, Type: eventlog.TypeExtracted,
		Summary: fmt.Sprintf("Reports Agent extracts the report: position %s, speed %.1f kn, ROB VLSFO %.1f t%s, Master's ETA %s %s. %s",
			rep.PositionText, rep.AvgSpeedKn, rep.ROBVLSFOt, cons, rep.NextPort, rep.MasterETA.Format("02 Jan 1504")+" UTC", check),
		Detail: map[string]any{"report": rep, "missing_for_desk": nonNilStrings(missing), "passed": true}, SourceRefs: []string{ref, r.emailRef}})
	out := map[string]any{"status": "ok", "report_id": r.ep.Email.ID, "lat": rep.Lat, "lon": rep.Lon,
		"next_port": rep.NextPort, "master_eta_utc": rep.MasterETA.Format(time.RFC3339), "report_time_utc": rep.ReportTime.Format(time.RFC3339)}
	if len(missing) > 0 {
		out["missing_for_desk"] = missing
	}
	if prev != nil {
		r.emitReportChecks(ref, flags)
	}
	if len(flags) > 0 {
		out["report_checks"] = flags
		out["remarks"] = rep.Remarks
	}
	return out, nil
}

// emitReportChecks logs the Checker's consistency rules on the report, so
// every figure they produce has a source on the page.
func (r *run) emitReportChecks(extractRef string, flags []ReportFlag) {
	ref := "check:report#" + extractRef
	summary := "Checker: the report's ROB, speed and position agree with the previous noon report"
	ids := make([]string, 0, len(flags))
	for _, f := range flags {
		ids = append(ids, f.ID)
		r.known.AddValue(f.Result)
	}
	if len(flags) > 0 {
		summary = "Checker: " + flags[0].Finding
		if len(flags) > 1 {
			summary += fmt.Sprintf(" (and %d more)", len(flags)-1)
		}
	}
	r.emit(eventlog.Event{Actor: eventlog.ActorChecker, Type: eventlog.TypeCheck, Summary: summary,
		Detail:     map[string]any{"tool": "report_checks", "flags": flags, "flagged": nonNilStrings(ids), "passed": len(flags) == 0},
		SourceRefs: []string{ref, extractRef, r.emailRef}})
}

func (r *run) checkedReport(_ agent.Context, _ reportIn) (map[string]any, error) {
	rep, err := r.needReport()
	if err != nil {
		return nil, err
	}
	return toMap(rep), nil
}

func (r *run) checkETA(ctx agent.Context, _ reportIn) (map[string]any, error) {
	ref := eventlog.ToolRef("check_eta", ctx.FunctionCallID())
	rep, err := r.needReport()
	if err != nil {
		return nil, err
	}
	dtg, ok := r.latest("distance_to_go")
	if !ok {
		return nil, errors.New("call distance_to_go from the reported position first")
	}
	var d struct {
		DistanceNM float64 `json:"distance_nm"`
	}
	if err := decode(dtg.output, &d); err != nil {
		return nil, err
	}
	if err := sameLeg(dtg.args, rep); err != nil {
		return nil, err
	}
	// The Master's stated ETA basis speed, when the report gives one; else
	// the last 24 h average. A Master slowed by weather plans on resuming.
	speed, basis := rep.AvgSpeedKn, "reported speed"
	if rep.ETABasisKn > 0 {
		speed, basis = rep.ETABasisKn, "Master's ETA basis speed"
	}
	res, err := checker.CheckETA(rep.ReportTime, rep.MasterETA, d.DistanceNM, speed)
	if err != nil {
		return nil, err
	}
	r.known.AddValue(res)
	verdict := "consistent"
	if !res.Consistent {
		verdict = "NOT consistent"
	}
	r.emit(eventlog.Event{Actor: eventlog.ActorChecker, Type: eventlog.TypeCheck,
		Summary: fmt.Sprintf("ETA %s: Master %s, distance %.1f nm ÷ %.1f kn (%s) gives %s (%+.1f h; limit ±%.0f h)",
			verdict, res.MasterETA.Format("02 Jan 1504"), res.DistanceNM, res.SpeedKn, basis, res.ComputedETA.Format("02 Jan 1504"), res.DifferenceH, res.ToleranceH),
		Detail: map[string]any{"result": res, "speed_basis": basis, "passed": res.Consistent}, SourceRefs: []string{ref, dtg.ref, r.reportRef}})
	return toMap(res), nil
}

func (r *run) checkNotices(ctx agent.Context, _ reportIn) (map[string]any, error) {
	ref := eventlog.ToolRef("check_notices", ctx.FunctionCallID())
	// At a noon report: the Master's ETA at the report time. For an e-mail
	// that is not a noon report: the speed advice's arrival, now. Models call
	// this alongside check_speed_plan, so it reads the advice itself; the
	// operator's decision, which follows the plan check, still gates every draft.
	var at, eta time.Time
	var basisRef, prefix string
	if rep, err := r.needReport(); err == nil {
		at, eta, basisRef = rep.ReportTime, rep.MasterETA, r.reportRef
	} else if rc, ok := r.latest("check_route_change"); ok {
		// A new route: its ETA, now.
		var res checker.RouteChangeResult
		if err := decode(rc.output, &res); err != nil {
			return nil, err
		}
		at, eta, basisRef = r.ep.SimTime, res.NewETA, rc.ref
		prefix = fmt.Sprintf("On the new route, arriving %s UTC: ", eta.Format("02 Jan 1504"))
	} else {
		adv, ok := r.latest("speed_advice")
		if !ok {
			return nil, errors.New("no noon report in this work: call speed_advice (or check_route_change) first, or wait for the Reports Agent's extraction")
		}
		var a tools.SpeedAdvice
		if err := decode(adv.output, &a); err != nil {
			return nil, err
		}
		at, eta, basisRef = r.ep.SimTime, a.AdvisedArrival, adv.ref
		prefix = fmt.Sprintf("If the arrival moves to %s UTC: ", eta.Format("02 Jan 1504"))
	}
	cpRes, ok := r.latest("cp_terms")
	if !ok {
		return nil, errors.New("call cp_terms first")
	}
	histRes, ok := r.latest("voyage_history")
	if !ok {
		return nil, errors.New("call voyage_history first")
	}
	var cp sim.CPTerms
	var hist struct {
		Notices []sim.Notice `json:"eta_notices_sent"`
	}
	if err := errors.Join(decode(cpRes.output, &cp), decode(histRes.output, &hist)); err != nil {
		return nil, err
	}
	res := checker.CheckNotices(at, eta, cp, hist.Notices)
	r.known.AddValue(res)
	summary := "No ETA notice due"
	if res.Due {
		summary = "Notice due: " + strings.Join(res.Reasons, " ")
	}
	r.emit(eventlog.Event{Actor: eventlog.ActorChecker, Type: eventlog.TypeCheck, Summary: prefix + summary,
		Detail: map[string]any{"result": res, "eta": eta, "recipients": cp.NoticeRecipientRoles, "passed": true}, SourceRefs: []string{ref, cpRes.ref, histRes.ref, basisRef}})
	out := toMap(res)
	out["eta_utc"], out["recipient_roles"] = eta.Format(time.RFC3339), cp.NoticeRecipientRoles
	return out, nil
}

type stoppageIn struct {
	FromUTC string  `json:"from_utc" jsonschema:"when the main engine stopped, RFC 3339, as the Master wrote it, e.g. 2025-01-26T06:10:00Z"`
	ToUTC   string  `json:"to_utc" jsonschema:"when it restarted, RFC 3339, as the Master wrote it"`
	VLSFOt  float64 `json:"vlsfo_t,omitempty" jsonschema:"VLSFO used during the stop, tonnes, as the Master wrote it"`
	MGOt    float64 `json:"mgo_t,omitempty" jsonschema:"MGO used during the stop, tonnes, as the Master wrote it"`
}

// checkStoppage checks a main engine stop against the Master's report. The
// times and fuel must be the ones in his remarks. A stop that agrees with
// the report leaves the off-hire decision to the operator; what it costs is
// the voyage management system's to work out (off_hire_statement).
func (r *run) checkStoppage(ctx agent.Context, in stoppageIn) (map[string]any, error) {
	ref := eventlog.ToolRef("check_stoppage", ctx.FunctionCallID())
	rep, err := r.needReport()
	if err != nil {
		return nil, err
	}
	from, err1 := time.Parse(time.RFC3339, in.FromUTC)
	to, err2 := time.Parse(time.RFC3339, in.ToUTC)
	if err1 != nil || err2 != nil {
		return nil, errors.New("give from_utc and to_utc as RFC 3339 times, e.g. 2025-01-26T06:10:00Z")
	}
	from, to = from.UTC(), to.UTC()
	remarks := rep.Remarks
	for _, t := range []time.Time{from, to} {
		if !strings.Contains(remarks, t.Format("1504")) {
			return nil, fmt.Errorf("%s UTC is not a time in the Master's remarks; copy the stop times as he wrote them", t.Format("02 Jan 1504"))
		}
	}
	for grade, t := range map[string]float64{"VLSFO": in.VLSFOt, "MGO": in.MGOt} {
		if t > 0 && !slices.Contains(checker.NumericTokens(remarks), strconv.FormatFloat(t, 'f', -1, 64)) && !slices.Contains(checker.NumericTokens(remarks), strconv.FormatFloat(t, 'f', 1, 64)) {
			return nil, fmt.Errorf("%s %.1f t is not in the Master's remarks; copy the fuel used during the stop as he wrote it", grade, t)
		}
	}
	var prev time.Time
	if h := r.ep.Voyage.History; len(h) > 0 {
		prev = h[len(h)-1].Time
	} else {
		return nil, errors.New("no previous noon report to check the day against")
	}
	res, err := checker.CheckStoppage(prev, rep.ReportTime, from, to, rep.DistanceRunNM, rep.AvgSpeedKn)
	if err != nil {
		return nil, err
	}
	r.known.AddValue(res)
	if res.Consistent {
		r.mu.Lock()
		r.decide = "whether to put the ship off hire and notify the owner"
		r.mu.Unlock()
	}
	summary := fmt.Sprintf("Stoppage agrees with the report: %s to %s UTC is %.1f h; %.1f kn over %.1f h steaming gives %.1f nm, distance run %.1f nm (%+.1f%%; limit ±%.0f%%)",
		from.Format("02 Jan 1504"), to.Format("1504"), res.StopHours, res.AvgSpeedKn, res.SteamingHours, res.ExpectedDistanceNM, res.DistanceRunNM, res.DistanceDiffPct, checker.StoppageDistancePct)
	if !res.Consistent {
		summary = "Stoppage does NOT agree with the report: " + res.Interpretation
	}
	r.emit(eventlog.Event{Actor: eventlog.ActorChecker, Type: eventlog.TypeCheck, Summary: summary,
		Detail: map[string]any{"tool": "check_stoppage", "result": res, "fuel_during_stop": map[string]float64{"VLSFO": in.VLSFOt, "MGO": in.MGOt}, "passed": res.Consistent}, SourceRefs: []string{ref, r.reportRef}})
	return toMap(res), nil
}

// checkRouteChange works out what a new route means: seaRoute's planned
// route from the same AIS position, the new ETA at the Master's ETA speed,
// and the bunkers. A route that adds up leaves the decision to the operator.
func (r *run) checkRouteChange(ctx agent.Context, in shipIn) (map[string]any, error) {
	ref := eventlog.ToolRef("check_route_change", ctx.FunctionCallID())
	ais, ok1 := r.latest("ais_position")
	dtg, ok2 := r.latest("distance_to_go")
	if !ok1 || !ok2 {
		return nil, errors.New("call ais_position, then distance_to_go from that position avoiding the passages the ship will not use, first")
	}
	v, ok := r.voyage(in.ShipID)
	if !ok {
		return nil, fmt.Errorf("unknown ship ID %q", in.ShipID)
	}
	var pos struct {
		Time time.Time `json:"time"`
		Lat  float64   `json:"lat"`
		Lon  float64   `json:"lon"`
	}
	var d struct {
		DistanceNM float64 `json:"distance_nm"`
	}
	var args struct {
		From  struct{ Lat, Lon *float64 } `json:"from"`
		Avoid []string                    `json:"avoid"`
	}
	if err := errors.Join(decode(ais.output, &pos), decode(dtg.output, &d), decode(dtg.args, &args)); err != nil {
		return nil, err
	}
	if args.From.Lat == nil || args.From.Lon == nil || math.Abs(*args.From.Lat-pos.Lat) > 0.01 || math.Abs(*args.From.Lon-pos.Lon) > 0.01 {
		return nil, errors.New("distance_to_go was not called from the AIS position; call it again with ais_position's lat and lon")
	}
	planned, err := r.cfg.Graph.Route([]route.Position{{pos.Lon, pos.Lat}, {v.To.Lon, v.To.Lat}}, route.Options{Avoid: v.Avoid})
	if err != nil {
		return nil, err
	}
	if len(v.History) == 0 || v.VMS == nil || len(v.VMS.PortCalls) == 0 {
		return nil, errors.New("no noon report or port call on file for this ship")
	}
	last := v.History[len(v.History)-1]
	speed := last.ETABasisKn
	if speed <= 0 {
		speed = last.SpeedKn
	}
	res, err := checker.CheckRouteChange(pos.Time, planned.DistanceNM(), d.DistanceNM, speed, v.VMS.PortCalls[0].ETA, last.ROBVLSFO, last.ConsVLSFO, pos.Time.Sub(last.Time).Hours())
	if err != nil {
		return nil, err
	}
	r.known.AddValue(res)
	r.mu.Lock()
	r.results["check_route_change"] = toolResult{ref: ref, args: map[string]any{"ship_id": v.Ship.ID, "avoid": args.Avoid}, output: res}
	r.decide = "whether to confirm the new route"
	r.mu.Unlock()
	r.emit(eventlog.Event{Actor: eventlog.ActorChecker, Type: eventlog.TypeCheck, Summary: "Route change: " + res.Interpretation,
		Detail: map[string]any{"tool": "check_route_change", "result": res, "avoid": args.Avoid, "passed": true}, SourceRefs: []string{ref, ais.ref, dtg.ref}})
	return toMap(res), nil
}

// checkSpeedPlan checks the voyage optimisation service's speed advice:
// seaRoute's distance from the same AIS position, the arrival at the
// advised speed, the berth time as the agent's e-mail gives it. A plan that
// adds up leaves the decision to slow down to the operator.
func (r *run) checkSpeedPlan(ctx agent.Context, _ shipIn) (map[string]any, error) {
	ref := eventlog.ToolRef("check_speed_plan", ctx.FunctionCallID())
	ais, ok1 := r.latest("ais_position")
	dtg, ok2 := r.latest("distance_to_go")
	adv, ok3 := r.latest("speed_advice")
	if !ok1 || !ok2 || !ok3 {
		return nil, errors.New("call ais_position, then distance_to_go from that position, then speed_advice first")
	}
	var pos struct {
		Time time.Time `json:"time"`
		Lat  float64   `json:"lat"`
		Lon  float64   `json:"lon"`
	}
	var d struct {
		DistanceNM float64 `json:"distance_nm"`
	}
	var a tools.SpeedAdvice
	if err := errors.Join(decode(ais.output, &pos), decode(dtg.output, &d), decode(adv.output, &a)); err != nil {
		return nil, err
	}
	var args struct {
		From struct{ Lat, Lon *float64 } `json:"from"`
		To   struct {
			Port string `json:"port"`
		} `json:"to"`
	}
	if err := decode(dtg.args, &args); err != nil {
		return nil, err
	}
	if args.From.Lat == nil || args.From.Lon == nil || math.Abs(*args.From.Lat-pos.Lat) > 0.01 || math.Abs(*args.From.Lon-pos.Lon) > 0.01 {
		return nil, errors.New("distance_to_go was not called from the AIS position; call it again with ais_position's lat and lon")
	}
	if !strings.EqualFold(args.To.Port, a.To) {
		return nil, fmt.Errorf("distance_to_go was called to %q, but the speed advice is to %q", args.To.Port, a.To)
	}
	if !a.FromTime.Equal(pos.Time) {
		return nil, errors.New("the speed advice starts from another AIS position; call speed_advice again")
	}
	// The berth time must be the one in the e-mail, not one the agent made up.
	body := r.ep.Email.Body
	if !strings.Contains(body, a.BerthUTC.Format("02 Jan")) && !strings.Contains(body, a.BerthUTC.Format("2 Jan")) || !strings.Contains(body, a.BerthUTC.Format("1504")) {
		return nil, fmt.Errorf("the berthing time %s UTC is not the one in the e-mail; call speed_advice with the e-mail's berthing time", a.BerthUTC.Format("02 Jan 1504"))
	}
	res, err := checker.CheckSpeedPlan(pos.Time, d.DistanceNM, a.DistanceNM, a.AdvisedSpeedKn, a.AdvisedArrival, a.BerthUTC)
	if err != nil {
		return nil, err
	}
	r.known.AddValue(res)
	r.mu.Lock()
	r.results["check_speed_plan"] = toolResult{ref: ref, output: res}
	if res.Consistent {
		r.decide = "whether to reduce speed for the later berth"
	}
	r.mu.Unlock()
	summary := fmt.Sprintf("Speed advice adds up: seaRoute %.1f nm ÷ %.1f kn gives arrival %s UTC (advice %s, %+.1f h; limit ±%.0f h), before the berth at %s",
		res.DistanceNM, res.AdvisedSpeedKn, res.ComputedArrival.Format("02 Jan 1504"), res.AdviceArrival.Format("02 Jan 1504"), res.ArrivalDiffHours, checker.SpeedPlanArrivalHours, res.BerthUTC.Format("02 Jan 1504"))
	if !res.Consistent {
		summary = "Speed advice does NOT add up: " + res.Interpretation
	}
	r.emit(eventlog.Event{Actor: eventlog.ActorChecker, Type: eventlog.TypeCheck, Summary: summary,
		Detail: map[string]any{"tool": "check_speed_plan", "result": res, "passed": res.Consistent}, SourceRefs: []string{ref, ais.ref, dtg.ref, adv.ref, r.emailRef}})
	return toMap(res), nil
}

func (r *run) checkForecastETA(ctx agent.Context, _ reportIn) (map[string]any, error) {
	ref := eventlog.ToolRef("check_forecast_eta", ctx.FunctionCallID())
	rep, err := r.needReport()
	if err != nil {
		return nil, err
	}
	v := r.ep.Voyage
	if r.cfg.Forecast == nil || v.Loading == nil {
		return nil, errors.New("no forecast or ship particulars for this voyage")
	}
	dtg, ok := r.latest("distance_to_go")
	if !ok {
		return nil, errors.New("call distance_to_go from the reported position first")
	}
	if err := sameLeg(dtg.args, rep); err != nil {
		return nil, err
	}
	cpRes, ok := r.latest("cp_terms")
	if !ok {
		return nil, errors.New("call cp_terms first")
	}
	var cp sim.CPTerms
	var path struct {
		Path []route.Position `json:"path"`
	}
	r.mu.Lock()
	kept := r.forLog[strings.TrimPrefix(dtg.ref, "tool:distance_to_go#")]
	r.mu.Unlock()
	if err := errors.Join(decode(cpRes.output, &cp), decode(kept, &path)); err != nil {
		return nil, err
	}
	speed := rep.ETABasisKn
	if speed <= 0 {
		speed = rep.AvgSpeedKn
	}
	hull := checker.Hull{LppM: v.Ship.LppM, Cb: v.Loading.Cb, DisplacementM3: v.Loading.DisplacementM3, Ballast: v.Loading.Ballast}
	res, samples, err := checker.ForecastETA(r.cfg.Forecast, path.Path, rep.ReportTime, rep.MasterETA, speed, cp.ETAChangeHours, hull)
	if err != nil {
		return nil, err
	}
	r.known.AddValue(res)
	if res.AtRisk {
		r.mu.Lock()
		r.decide = "whether to ask the Master to review the ETA in view of the forecast"
		r.mu.Unlock()
	}
	summary := fmt.Sprintf("Forecast ETA %s: the ECMWF forecast of %s through the speed-loss model gives %s, %+.1f h on the Master's %s (CP limit %.0f h). Simplified model, for learning",
		map[bool]string{true: "at risk", false: "holds"}[res.AtRisk], res.ForecastRun.Format("02 Jan 15z"), res.ForecastETA.Format("02 Jan 1504"), res.DifferenceH, res.MasterETA.Format("02 Jan 1504"), res.LimitH)
	r.emit(eventlog.Event{Actor: eventlog.ActorChecker, Type: eventlog.TypeCheck, Summary: summary,
		Detail:     map[string]any{"tool": "check_forecast_eta", "result": res, "hull": hull, "samples": samples, "grid_ref": r.cfg.Forecast.Ref, "passed": !res.AtRisk},
		SourceRefs: []string{ref, dtg.ref, cpRes.ref, r.reportRef}})
	return toMap(res), nil
}

// weatherAhead scans one ship's remaining route through the forecast run
// that started the work. It starts from her AIS position at the time the
// run became available, so it never uses a forecast from the future.
func (r *run) weatherAhead(ctx agent.Context, in shipIn) (map[string]any, error) {
	ref := eventlog.ToolRef("weather_ahead", ctx.FunctionCallID())
	fi := r.ep.Forecast
	if fi == nil || r.cfg.Forecast == nil || !checker.ForecastRun(r.cfg.Forecast).Equal(fi.Run) {
		return nil, errors.New("no new forecast run at hand")
	}
	v, ok := r.voyage(in.ShipID)
	if !ok {
		return nil, fmt.Errorf("ship %q is not in the current work", in.ShipID)
	}
	if len(v.Track) == 0 || len(v.History) == 0 {
		return nil, fmt.Errorf("no AIS position or noon report for %s", v.Ship.ID)
	}
	last := v.Track[len(v.Track)-1]
	rep := v.History[len(v.History)-1]
	speed, basis := rep.ETABasisKn, "Master's ETA basis speed"
	if speed <= 0 {
		speed, basis = rep.SpeedKn, "last reported speed"
	}
	rt, err := r.cfg.Graph.Route([]route.Position{{last.Lon, last.Lat}, {v.To.Lon, v.To.Lat}}, route.Options{Avoid: v.Avoid})
	if err != nil {
		return nil, err
	}
	res, samples, err := checker.WeatherAhead(r.cfg.Forecast, rt.Path, fi.Available, speed)
	if err != nil {
		return nil, err
	}
	out := toMap(res)
	out["ship_id"], out["ship"], out["to"], out["speed_basis"] = v.Ship.ID, v.Ship.Name, v.To.Name, basis
	r.known.AddValue(out)
	summary := fmt.Sprintf("Weather ahead for %s (ECMWF %s): no heavy weather on the route while the forecast lasts", v.Ship.Name, res.ForecastRun.Format("02 Jan 15z"))
	if n := len(res.HeavyWeather); n > 0 {
		worst := res.HeavyWeather[0]
		for _, h := range res.HeavyWeather {
			if h.MaxBeaufort > worst.MaxBeaufort {
				worst = h
			}
		}
		summary = fmt.Sprintf("Weather ahead for %s (ECMWF %s): heavy weather from %s UTC; the worst, from %s UTC, Beaufort %d (%s), seas up to %.1f m",
			v.Ship.Name, res.ForecastRun.Format("02 Jan 15z"), res.HeavyWeather[0].From.Format("02 Jan 1504"), worst.From.Format("02 Jan 1504"), worst.MaxBeaufort, worst.WindSector, worst.MaxSWHm)
	}
	r.emit(eventlog.Event{Actor: eventlog.ActorChecker, Type: eventlog.TypeCheck, Summary: summary,
		Detail:     map[string]any{"tool": "weather_ahead", "ship_id": v.Ship.ID, "result": out, "samples": samples, "path": rt.Path, "grid_ref": r.cfg.Forecast.Ref, "passed": len(res.HeavyWeather) == 0},
		SourceRefs: []string{ref, r.emailRef}})
	return out, nil
}

func (r *run) checkConsumption(ctx agent.Context, _ reportIn) (map[string]any, error) {
	ref := eventlog.ToolRef("check_consumption", ctx.FunctionCallID())
	rep, err := r.needReport()
	if err != nil {
		return nil, err
	}
	if rep.ConsVLSFOt <= 0 {
		return nil, errors.New("the report gives no VLSFO consumption")
	}
	cpRes, ok := r.latest("cp_terms")
	if !ok {
		return nil, errors.New("call cp_terms first")
	}
	var cp sim.CPTerms
	if err := decode(cpRes.output, &cp); err != nil {
		return nil, err
	}
	res, err := checker.CheckConsumption(rep.ConsVLSFOt, cp.WarrantedVLSFOPerDayT, rep.AvgSpeedKn, cp.WarrantedSpeedKn)
	if err != nil {
		return nil, err
	}
	r.known.AddValue(res)
	summary := fmt.Sprintf("Within the warranty: %.1f t VLSFO against %.1f t allowed (about %.1f t)", res.ReportedT, res.AllowedT, res.WarrantedT)
	switch {
	case res.FuelFlagged && res.SpeedFlagged:
		summary = fmt.Sprintf("Fuel and speed flagged: %.1f t VLSFO is %.1f t over the %.1f t allowed (%+.1f%% on the CP warranted %.1f t); %.1f kn is below the %.1f kn allowed", res.ReportedT, res.OverAllowedT, res.AllowedT, res.ExcessPct, res.WarrantedT, res.ReportedKn, res.MinSpeedKn)
	case res.FuelFlagged:
		summary = fmt.Sprintf("Consumption flagged: %.1f t VLSFO is %.1f t over the %.1f t allowed (%+.1f%% on the CP warranted %.1f t)", res.ReportedT, res.OverAllowedT, res.AllowedT, res.ExcessPct, res.WarrantedT)
	case res.SpeedFlagged:
		summary = fmt.Sprintf("Speed flagged: %.1f kn is below the %.1f kn allowed (CP warranted about %.1f kn)", res.ReportedKn, res.MinSpeedKn, res.WarrantedKn)
	}
	r.emit(eventlog.Event{Actor: eventlog.ActorChecker, Type: eventlog.TypeCheck, Summary: summary,
		Detail: map[string]any{"result": res, "passed": !res.Flagged}, SourceRefs: []string{ref, cpRes.ref, r.reportRef}})
	return toMap(res), nil
}

func (r *run) classifyWeatherDay(ctx agent.Context, _ reportIn) (map[string]any, error) {
	ref := eventlog.ToolRef("classify_weather_day", ctx.FunctionCallID())
	rep, err := r.needReport()
	if err != nil {
		return nil, err
	}
	cpRes, ok := r.latest("cp_terms")
	if !ok {
		return nil, errors.New("call cp_terms first")
	}
	wxRes, ok := r.latest("weather_on_track")
	if !ok {
		return nil, errors.New("call weather_on_track for the 24 h up to the report time first")
	}
	var cp sim.CPTerms
	var wx struct {
		Summary tools.WeatherSummary `json:"summary"`
	}
	if err := errors.Join(decode(cpRes.output, &cp), decode(wxRes.output, &wx)); err != nil {
		return nil, err
	}
	s := wx.Summary
	if want := rep.ReportTime.Add(-24 * time.Hour); !s.From.Equal(want) || !s.To.Equal(rep.ReportTime) {
		return nil, fmt.Errorf("weather_on_track covers %s to %s; call it for the report day, %s to %s",
			s.From.Format(time.RFC3339), s.To.Format(time.RFC3339), want.Format(time.RFC3339), rep.ReportTime.Format(time.RFC3339))
	}
	res := checker.ClassifyWeatherDay(checker.WeatherDay{Hours: s.Samples * s.HoursPerSample, Samples: s.Samples, SamplesWithSea: s.SamplesWithSea, MaxBeaufort: s.MaxBeaufort, MaxDouglas: s.MaxDouglas}, cp)
	r.known.AddValue(res)
	summary := "Weather day not classified: " + res.Interpretation
	if res.Classified && res.GoodWeatherDay {
		summary = fmt.Sprintf("Good weather day under the CP: up to Beaufort %d, sea state %d (limits %d and %d). Weather does not explain the extra fuel", res.MaxBeaufort, res.MaxDouglas, res.LimitBeaufort, res.LimitDouglas)
	} else if res.Classified {
		summary = fmt.Sprintf("Not a good weather day under the CP: up to Beaufort %d, sea state %d (limits %d and %d). The day is excluded from the warranty", res.MaxBeaufort, res.MaxDouglas, res.LimitBeaufort, res.LimitDouglas)
	}
	r.emit(eventlog.Event{Actor: eventlog.ActorChecker, Type: eventlog.TypeCheck, Summary: summary,
		Detail: map[string]any{"result": res, "passed": res.Classified}, SourceRefs: []string{ref, wxRes.ref, cpRes.ref, r.reportRef}})
	return toMap(res), nil
}

type noteIn struct {
	ShipID string `json:"ship_id,omitempty" jsonschema:"the ship the note is for, e.g. SIM-0001; required when the work covers more than one ship"`
	Text   string `json:"text" jsonschema:"one or two plain sentences; numbers only as given by tools"`
}

type alertIn struct {
	Text  string   `json:"text" jsonschema:"one or two plain sentences; numbers only as given by tools"`
	About []string `json:"about" jsonschema:"topic IDs this is about, from: eta, eta_notice, eta_forecast, consumption, rob_vlsfo, rob_mgo, speed, position, missing_figures, payment_change, heavy_weather, berth_delay, off_hire, route_change, bunkers, other"`
}

// Topics tag every question, draft and alert with what it is about, the way
// a desk system files a case under a subject. The eval scores on them.
var Topics = []string{"eta", "eta_notice", "eta_forecast", "consumption", "rob_vlsfo", "rob_mgo", "speed", "position", "missing_figures", "payment_change", "heavy_weather", "berth_delay", "off_hire", "route_change", "bunkers", "other"}

// badTopics returns the refusal for a missing or unknown topic, or "".
func badTopics(about []string) string {
	if len(about) == 0 {
		return "about is empty. Give the topic IDs: " + strings.Join(Topics, ", ") + "."
	}
	for _, a := range about {
		if !slices.Contains(Topics, a) {
			return fmt.Sprintf("about has %q, which is not a topic. Use: %s.", a, strings.Join(Topics, ", "))
		}
	}
	return ""
}

// noteShip is the ship a voyage-file note is for. With one ship in the work
// the ID may be left out; in a fleet the note must name its ship.
func (r *run) noteShip(id string) (sim.Voyage, error) {
	if strings.TrimSpace(id) == "" {
		if f := r.fleet(); len(f) == 1 {
			return f[0], nil
		}
		return sim.Voyage{}, errors.New("the work covers more than one ship: give the ship_id the note is for")
	}
	v, ok := r.voyage(id)
	if !ok {
		return sim.Voyage{}, fmt.Errorf("ship %q is not in the current work", id)
	}
	return v, nil
}

// addVoyageNote files a note for the operator in one ship's voyage file. Its
// numbers must come from tools.
func (r *run) addVoyageNote(ctx agent.Context, in noteIn) (map[string]any, error) {
	ref := eventlog.ToolRef("add_voyage_note", ctx.FunctionCallID())
	v, err := r.noteShip(in.ShipID)
	if err != nil {
		return nil, err
	}
	if u := r.known.Unsupported(in.Text); len(u) > 0 {
		r.emit(eventlog.Event{Actor: eventlog.ActorChecker, Type: eventlog.TypeCheck, Summary: "Number guard: note refused, it contains " + checker.Summary(u),
			Detail: map[string]any{"text": in.Text, "unsupported": u, "passed": false}, SourceRefs: []string{ref}})
		return map[string]any{"status": "refused", "problem": "The note contains " + checker.Summary(u) + ". Use only numbers from tool results."}, nil
	}
	r.emit(eventlog.Event{Actor: actorFor(ctx), Agent: ctx.AgentName(), Type: eventlog.TypeToolCall, Summary: agentTitle[ctx.AgentName()] + " files a note in " + v.Ship.Name + "'s voyage file",
		Detail: map[string]any{"tool": "add_voyage_note", "ship_id": v.Ship.ID, "text": in.Text}, SourceRefs: []string{ref}})
	r.emit(eventlog.Event{Actor: eventlog.ActorTool, Type: eventlog.TypeToolResult, Summary: "Note in " + v.Ship.Name + "'s voyage file: " + in.Text,
		Detail: map[string]any{"tool": "add_voyage_note", "ship_id": v.Ship.ID, "note": in.Text}, SourceRefs: append([]string{ref}, r.numberRefs()...)})
	return map[string]any{"status": "filed"}, nil
}

// alertOperator raises an alert on the desk. Its numbers must come from tools.
func (r *run) alertOperator(ctx agent.Context, in alertIn) (map[string]any, error) {
	ref := eventlog.ToolRef("alert_operator", ctx.FunctionCallID())
	if p := badTopics(in.About); p != "" {
		return map[string]any{"status": "refused", "problem": p}, nil
	}
	if u := r.known.Unsupported(in.Text); len(u) > 0 {
		r.emit(eventlog.Event{Actor: eventlog.ActorChecker, Type: eventlog.TypeCheck, Summary: "Number guard: alert refused, it contains " + checker.Summary(u),
			Detail: map[string]any{"text": in.Text, "unsupported": u, "passed": false}, SourceRefs: []string{ref}})
		return map[string]any{"status": "refused", "problem": "The alert contains " + checker.Summary(u) + ". Use only numbers from tool results."}, nil
	}
	r.emit(eventlog.Event{Actor: actorFor(ctx), Agent: ctx.AgentName(), Type: eventlog.TypeAlert, Summary: "Alert to the operator: " + in.Text,
		Detail: map[string]any{"text": in.Text, "about": in.About}, SourceRefs: append([]string{ref}, r.numberRefs()...)})
	return map[string]any{"status": "raised"}, nil
}

type finding struct {
	Kind  string `json:"kind" jsonschema:"one of: lookalike_sender, bank_details_change, instruction_to_ai, pressure, other"`
	Quote string `json:"quote" jsonschema:"exact words copied from the e-mail or its From line"`
	Why   string `json:"why" jsonschema:"one plain sentence: why it matters"`
}

type findingsIn struct {
	Findings []finding `json:"findings"`
}

var findingKinds = []string{"lookalike_sender", "bank_details_change", "instruction_to_ai", "pressure", "other"}

// reportFindings records the Security Agent's findings. Every quote must be
// in the e-mail, so the agent cannot put words into it.
func (r *run) reportFindings(ctx agent.Context, in findingsIn) (map[string]any, error) {
	ref := eventlog.ToolRef("report_findings", ctx.FunctionCallID())
	e := r.ep.Email
	norm := func(s string) string { return strings.ToLower(strings.Join(strings.Fields(s), " ")) }
	text := norm("From: " + e.From + "\nSubject: " + e.Subject + "\n" + e.Body)
	var problems []string
	for i, f := range in.Findings {
		if !slices.Contains(findingKinds, f.Kind) {
			problems = append(problems, fmt.Sprintf("finding %d: kind %q is not one of %s", i+1, f.Kind, strings.Join(findingKinds, ", ")))
		}
		if q := norm(f.Quote); q == "" || !strings.Contains(text, q) {
			problems = append(problems, fmt.Sprintf("finding %d: the quote %q is not in the e-mail; copy it exactly", i+1, f.Quote))
		}
	}
	if len(problems) > 0 {
		r.emit(eventlog.Event{Actor: actorFor(ctx), Agent: ctx.AgentName(), Type: eventlog.TypeCheck, Summary: "Security Agent's findings refused: " + problems[0],
			Detail: map[string]any{"findings": in.Findings, "problems": problems, "passed": false}, SourceRefs: []string{ref, r.emailRef}})
		return map[string]any{"status": "refused", "problems": problems}, nil
	}
	r.mu.Lock()
	r.findings, r.findingsRef = in.Findings, ref
	r.mu.Unlock()
	kinds := make([]string, len(in.Findings))
	for i, f := range in.Findings {
		kinds[i] = findingLabel[f.Kind]
	}
	summary := "Security Agent finds nothing wrong"
	if len(kinds) > 0 {
		summary = "Security Agent finds: " + strings.Join(kinds, "; ")
	}
	r.emit(eventlog.Event{Actor: actorFor(ctx), Agent: ctx.AgentName(), Type: eventlog.TypeCheck, Summary: summary,
		Detail: map[string]any{"tool": "report_findings", "findings": in.Findings, "passed": len(in.Findings) == 0}, SourceRefs: []string{ref, r.emailRef}})
	return map[string]any{"status": "recorded"}, nil
}

var findingLabel = map[string]string{
	"lookalike_sender":    "sender imitates a known contact",
	"bank_details_change": "request to change bank details",
	"instruction_to_ai":   "instruction addressed to an AI",
	"pressure":            "pressure to skip checks",
	"other":               "other concern",
}

// askOperator puts a decision on the operator's desk. Its numbers must come from tools.
func (r *run) askOperator(ctx agent.Context, in questionIn) (map[string]any, error) {
	ref := eventlog.ToolRef("ask_operator", ctx.FunctionCallID())
	if p := badTopics(in.About); p != "" {
		return map[string]any{"status": "refused", "problem": p}, nil
	}
	if u := r.known.Unsupported(in.Question + "\n" + in.Proposal); len(u) > 0 {
		r.emit(eventlog.Event{Actor: eventlog.ActorChecker, Type: eventlog.TypeCheck, Summary: "Number guard: question refused, it contains " + checker.Summary(u),
			Detail: map[string]any{"text": in.Question, "unsupported": u, "passed": false}, SourceRefs: []string{ref}})
		return map[string]any{"status": "refused", "problem": "The question contains " + checker.Summary(u) + ". Use only numbers from tool results."}, nil
	}
	q := Question{CallID: ctx.FunctionCallID(), Question: in.Question, Proposal: in.Proposal, About: in.About}
	r.mu.Lock()
	r.questions = append(r.questions, q)
	r.mu.Unlock()
	r.emit(eventlog.Event{Actor: actorFor(ctx), Agent: ctx.AgentName(), Type: eventlog.TypeQuestion, Summary: "Question to the operator: " + in.Question,
		Detail: map[string]any{"question": q}, SourceRefs: append([]string{"question:" + q.CallID, ref}, r.numberRefs()...)})
	return map[string]any{"status": "on the operator's desk. Do nothing about it until the operator answers; finish with your summary."}, nil
}

func (r *run) draftEmail(ctx agent.Context, in draftIn) (map[string]any, error) {
	ref := eventlog.ToolRef("draft_email", ctx.FunctionCallID())
	if p := badTopics(in.About); p != "" {
		return map[string]any{"status": "refused", "problem": p}, nil
	}

	v := r.ep.Voyage
	// A decision that is the operator's comes before any draft.
	r.mu.Lock()
	pending := r.decide
	if r.decided {
		pending = ""
	}
	r.mu.Unlock()
	if pending != "" {
		r.emit(eventlog.Event{Actor: eventlog.ActorGuardrail, Type: eventlog.TypeCheck, Summary: "Guardrail: draft stopped, the operator has not yet decided " + pending,
			Detail: map[string]any{"pending_decision": pending, "passed": false}, SourceRefs: []string{ref}})
		return map[string]any{"status": "stopped by guardrail", "problems": []string{"The operator has not yet decided " + pending + ". Write nothing now. Reply that the Ops Agent must ask the operator first (ask_operator)."}}, nil
	}
	var contacts []sim.Contact
	for _, fv := range r.fleet() {
		contacts = append(contacts, fv.Contacts...)
	}
	rec := checker.CheckRecipients(in.To, contacts)
	recSummary := "Guardrail: all recipients are verified contacts"
	if !rec.OK {
		recSummary = "Guardrail: draft stopped, recipients not in the verified contacts: " + strings.Join(rec.Unknown, ", ")
	}
	r.emit(eventlog.Event{Actor: eventlog.ActorGuardrail, Type: eventlog.TypeCheck, Summary: recSummary,
		Detail: map[string]any{"recipients": rec, "passed": rec.OK}, SourceRefs: []string{ref, "contacts:" + v.Ship.ID}})

	unsupported := r.known.Unsupported(in.Subject + "\n" + in.Body)
	numSummary := "Number guard: every number in the draft comes from the e-mail or a tool"
	if len(unsupported) > 0 {
		numSummary = "Number guard: draft stopped, it contains " + checker.Summary(unsupported)
	}
	r.emit(eventlog.Event{Actor: eventlog.ActorChecker, Type: eventlog.TypeCheck, Summary: numSummary,
		Detail: map[string]any{"unsupported": nonNilStrings(unsupported), "passed": len(unsupported) == 0}, SourceRefs: append([]string{ref}, r.numberRefs()...)})

	if !rec.OK || len(unsupported) > 0 {
		var problems []string
		if !rec.OK {
			problems = append(problems, "unknown recipients: "+strings.Join(rec.Unknown, ", ")+"; use addresses from the contacts tool")
		}
		if len(unsupported) > 0 {
			problems = append(problems, "the draft contains "+checker.Summary(unsupported)+"; copy numbers exactly from the e-mail or tool results")
		}
		return map[string]any{"status": "stopped by guardrail", "problems": problems}, nil
	}

	d := Draft{CallID: ctx.FunctionCallID(), To: in.To, Subject: in.Subject, Body: in.Body, Purpose: in.Purpose, About: in.About, Status: statusQueued}
	r.mu.Lock()
	r.drafts = append(r.drafts, d)
	r.mu.Unlock()
	r.emit(eventlog.Event{Actor: actorFor(ctx), Agent: ctx.AgentName(), Type: eventlog.TypeDraft, Summary: agentTitle[ctx.AgentName()] + " drafts: " + in.Subject,
		Detail: map[string]any{"draft": d}, SourceRefs: append([]string{ref}, r.numberRefs()...)})
	r.emit(eventlog.Event{Actor: actorFor(ctx), Agent: ctx.AgentName(), Type: eventlog.TypeApprovalRequested, Summary: "Draft waits in the approval queue for the operator",
		Detail: map[string]any{"call_id": d.CallID}, SourceRefs: []string{"approval:" + d.CallID, ref}})
	return map[string]any{"status": "in the approval queue; the operator approves, edits or rejects it. Nothing is sent."}, nil
}

// proposeEntry puts a proposed voyage management system entry in the
// approval queue. Like a draft: the operator's pending decision comes
// first, and every number and time must come from a tool or the e-mail.
func (r *run) proposeEntry(ctx agent.Context, in entryIn) (map[string]any, error) {
	ref := eventlog.ToolRef("propose_entry", ctx.FunctionCallID())
	// Every refusal is logged: an agent may still say it proposed the entry.
	refuse := func(p string) (map[string]any, error) {
		r.emit(eventlog.Event{Actor: eventlog.ActorGuardrail, Type: eventlog.TypeCheck, Summary: "Entry refused: " + p,
			Detail: map[string]any{"tool": "propose_entry", "args": in, "problem": p, "passed": false}, SourceRefs: []string{ref}})
		return map[string]any{"status": "refused", "problem": p}, nil
	}
	if p := badTopics(in.About); p != "" {
		return refuse(p)
	}
	if strings.TrimSpace(in.Reason) == "" {
		return refuse("give a reason the operator can read")
	}
	parse := func(name, v string) (time.Time, string) {
		t, err := time.Parse(time.RFC3339, v)
		if err != nil {
			return time.Time{}, fmt.Sprintf("%s must be an RFC 3339 time, e.g. 2025-01-27T06:00:00Z", name)
		}
		return t.UTC(), ""
	}
	e := Entry{Kind: in.Kind, Reason: in.Reason}
	var p string
	switch in.Kind {
	case "port_call_eta":
		if r.ep.Voyage.VMS == nil || !slices.ContainsFunc(r.ep.Voyage.VMS.PortCalls, func(c sim.PortCall) bool { return c.Port == in.Port }) {
			return refuse(fmt.Sprintf("port %q is not a port call in voyage_system", in.Port))
		}
		e.Port = in.Port
		e.ETA, p = parse("eta_utc", in.ETAUTC)
	case "off_hire":
		if e.From, p = parse("from_utc", in.FromUTC); p == "" {
			e.To, p = parse("to_utc", in.ToUTC)
		}
		if p == "" && !e.To.After(e.From) {
			p = "to_utc must be after from_utc"
		}
		if p == "" && in.CranesOut > 0 {
			v := r.ep.Voyage
			if v.Ship.Cranes == 0 || in.CranesOut > v.Ship.Cranes {
				p = fmt.Sprintf("%s has %d cranes; cranes_out must be 1 to %d", v.Ship.Name, v.Ship.Cranes, v.Ship.Cranes)
			} else {
				e.CranesOut, e.Cranes = in.CranesOut, v.Ship.Cranes
			}
		}
	default:
		p = fmt.Sprintf("kind %q is not port_call_eta or off_hire", in.Kind)
	}
	if p != "" {
		return refuse(p)
	}

	r.mu.Lock()
	pending := r.decide
	if r.decided {
		pending = ""
	}
	r.mu.Unlock()
	if pending != "" {
		r.emit(eventlog.Event{Actor: eventlog.ActorGuardrail, Type: eventlog.TypeCheck, Summary: "Guardrail: entry stopped, the operator has not yet decided " + pending,
			Detail: map[string]any{"pending_decision": pending, "passed": false}, SourceRefs: []string{ref}})
		return map[string]any{"status": "stopped by guardrail", "problems": []string{"The operator has not yet decided " + pending + ". Propose nothing now."}}, nil
	}
	// Times are checked as numbers too: an ETA the agent made up is refused.
	var times []string
	for _, t := range []time.Time{e.ETA, e.From, e.To} {
		if !t.IsZero() {
			times = append(times, t.Format("2006-01-02 1504"))
		}
	}
	unsupported := r.known.Unsupported(strings.Join(append(times, in.Reason), "\n"))
	numSummary := "Number guard: every number and time in the entry comes from the e-mail or a tool"
	if len(unsupported) > 0 {
		numSummary = "Number guard: entry stopped, it contains " + checker.Summary(unsupported)
	}
	r.emit(eventlog.Event{Actor: eventlog.ActorChecker, Type: eventlog.TypeCheck, Summary: numSummary,
		Detail: map[string]any{"unsupported": nonNilStrings(unsupported), "passed": len(unsupported) == 0}, SourceRefs: append([]string{ref}, r.numberRefs()...)})
	if len(unsupported) > 0 {
		return map[string]any{"status": "stopped by guardrail", "problems": []string{"the entry contains " + checker.Summary(unsupported) + "; copy numbers and times exactly from tool results"}}, nil
	}

	d := Draft{CallID: ctx.FunctionCallID(), Purpose: in.Reason, About: in.About, Status: statusQueued, Entry: &e}
	r.mu.Lock()
	r.drafts = append(r.drafts, d)
	r.mu.Unlock()
	r.emit(eventlog.Event{Actor: actorFor(ctx), Agent: ctx.AgentName(), Type: eventlog.TypeEntry, Summary: agentTitle[ctx.AgentName()] + " proposes an entry: " + entryLabel(e),
		Detail: map[string]any{"draft": d}, SourceRefs: append([]string{ref}, r.numberRefs()...)})
	r.emit(eventlog.Event{Actor: actorFor(ctx), Agent: ctx.AgentName(), Type: eventlog.TypeApprovalRequested, Summary: "Entry waits in the approval queue for the operator",
		Detail: map[string]any{"call_id": d.CallID}, SourceRefs: []string{"approval:" + d.CallID, ref}})
	return map[string]any{"status": "in the approval queue; the operator approves or rejects it. Nothing is posted."}, nil
}

func entryLabel(e Entry) string {
	if e.Kind == "off_hire" && e.CranesOut > 0 {
		return fmt.Sprintf("off hire pro rata, %d of %d cranes, %s to %s UTC", e.CranesOut, e.Cranes, e.From.Format("02 Jan 1504"), e.To.Format("02 Jan 1504"))
	}
	if e.Kind == "off_hire" {
		return fmt.Sprintf("off hire %s to %s UTC", e.From.Format("02 Jan 1504"), e.To.Format("02 Jan 1504"))
	}
	return fmt.Sprintf("%s ETA %s UTC", e.Port, e.ETA.Format("02 Jan 1504"))
}

// statusQueued marks a draft waiting at the Approval gate.
const statusQueued = "awaiting approval"

func (r *run) needReport() (Report, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.report == nil {
		return Report{}, errors.New("no checked report yet: the Reports Agent must extract it first")
	}
	return *r.report, nil
}

// sameLeg confirms distance_to_go was asked from the reported position to the next port.
func sameLeg(args map[string]any, rep Report) error {
	var a struct {
		From struct {
			Lat, Lon *float64
			Port     string
		} `json:"from"`
		To struct {
			Port string `json:"port"`
		} `json:"to"`
	}
	if err := decode(args, &a); err != nil {
		return err
	}
	if a.From.Lat == nil || a.From.Lon == nil || math.Abs(*a.From.Lat-rep.Lat) > 0.01 || math.Abs(*a.From.Lon-rep.Lon) > 0.01 {
		return errors.New("distance_to_go was not called from the reported position; call it again with the extracted lat and lon")
	}
	if !strings.EqualFold(a.To.Port, rep.NextPort) {
		return fmt.Errorf("distance_to_go was called to %q, but the next port is %q", a.To.Port, rep.NextPort)
	}
	return nil
}

func actorFor(ctx agent.Context) string {
	if ctx.AgentName() != opsName {
		return eventlog.ActorSubagent
	}
	return eventlog.ActorAgent
}

func decode(in, out any) error {
	b, err := json.Marshal(in)
	if err != nil {
		return err
	}
	return json.Unmarshal(b, out)
}

func toMap(v any) map[string]any {
	var m map[string]any
	_ = decode(v, &m)
	return m
}

func nonNilStrings(s []string) []string {
	if s == nil {
		return []string{}
	}
	return s
}

func callSummary(name string, args map[string]any) string {
	switch name {
	case "distance_to_go":
		var a struct {
			To    struct{ Port string } `json:"to"`
			Avoid []string              `json:"avoid"`
		}
		_ = decode(args, &a)
		s := "Asks seaRoute for the distance to go to " + a.To.Port
		if len(a.Avoid) > 0 {
			s += ", avoiding " + strings.Join(a.Avoid, ", ")
		}
		return s
	case "voyage_history":
		return "Looks up this voyage's previous reports and notices"
	case "checked_report":
		return "Reads the checked noon report from the voyage file"
	case "cp_terms":
		return "Looks up the charter party terms"
	case "contacts":
		return "Looks up the verified contacts for this voyage"
	case "voyage_system":
		return "Reads the voyage in the voyage management system"
	case "ais_position":
		return "Reads the ship's latest AIS position"
	case "off_hire_statement":
		from, _ := args["from_utc"].(string)
		to, _ := args["to_utc"].(string)
		return fmt.Sprintf("Asks the voyage management system (stand-in) for the hire statement lines for off hire %s to %s UTC", shortTime(from), shortTime(to))
	case "speed_advice":
		berth, _ := args["berth_utc"].(string)
		return fmt.Sprintf("Asks the voyage optimisation service (stand-in) for a speed to berth at %s UTC", shortTime(berth))
	case "weather_on_track":
		var a struct {
			From string `json:"from_utc"`
			To   string `json:"to_utc"`
		}
		_ = decode(args, &a)
		return fmt.Sprintf("Asks for the weather on the ship's track from %s to %s (ECMWF)", shortTime(a.From), shortTime(a.To))
	}
	return "Calls " + name
}

func resultSummary(name string, resp map[string]any) string {
	if e, ok := resp["error"]; ok {
		return fmt.Sprintf("%s failed: %v", name, e)
	}
	out := resp["output"]
	switch name {
	case "distance_to_go":
		var d struct {
			DistanceNM float64  `json:"distance_nm"`
			Passages   []string `json:"passages"`
		}
		_ = decode(out, &d)
		s := fmt.Sprintf("seaRoute: %.1f nm to go", d.DistanceNM)
		if len(d.Passages) > 0 {
			s += " (via " + strings.Join(d.Passages, ", ") + ")"
		}
		return s
	case "voyage_history":
		var h struct {
			Reports []sim.NoonReport `json:"previous_noon_reports"`
			Notices []sim.Notice     `json:"eta_notices_sent"`
		}
		_ = decode(out, &h)
		s := fmt.Sprintf("Voyage history: %d previous noon reports", len(h.Reports))
		if n := len(h.Notices); n > 0 {
			last := h.Notices[n-1]
			s += fmt.Sprintf("; last notice was the %d-day, ETA %s UTC", last.DaysLabel, last.ETA.Format("02 Jan 1504"))
		}
		return s
	case "cp_terms":
		var c sim.CPTerms
		_ = decode(out, &c)
		days := make([]string, len(c.NoticeDays))
		for i, d := range c.NoticeDays {
			days[i] = fmt.Sprint(d)
		}
		return fmt.Sprintf("CP (fictional): ETA notices at %s days, and when the ETA moves more than %.0f h", strings.Join(days, ", "), c.ETAChangeHours)
	case "checked_report":
		return "Checked noon report from the voyage file"
	case "contacts":
		var c struct {
			Contacts []sim.Contact `json:"contacts"`
		}
		_ = decode(out, &c)
		return fmt.Sprintf("%d verified contacts for this voyage", len(c.Contacts))
	case "voyage_system":
		var v sim.VMS
		_ = decode(out, &v)
		if len(v.PortCalls) == 0 {
			return "Voyage management system: no port calls"
		}
		c := v.PortCalls[0]
		return fmt.Sprintf("Voyage management system: %s (%s), ETA %s UTC; hire USD %.0f a day", c.Port, c.Purpose, c.ETA.Format("02 Jan 1504"), v.Hire.RateUSDPerDay)
	case "ais_position":
		var a struct {
			Time     time.Time `json:"time"`
			Lat, Lon float64
		}
		_ = decode(out, &a)
		return fmt.Sprintf("AIS: %s %s at %s UTC", sim.FormatLat(a.Lat), sim.FormatLon(a.Lon), a.Time.Format("02 Jan 1504"))
	case "off_hire_statement":
		var o tools.OffHireStatement
		_ = decode(out, &o)
		return fmt.Sprintf("Hire statement preview (stand-in for the voyage management system; figures made up for this simulation): %.2f h off hire, hire USD %.2f and bunkers USD %.2f for owner's account, USD %.2f in all",
			o.Hours, o.HireUSD, o.BunkersUSD, o.TotalUSD)
	case "speed_advice":
		var a tools.SpeedAdvice
		_ = decode(out, &a)
		return fmt.Sprintf("Speed advice (stand-in for the voyage optimisation service; figures made up for this simulation): %.1f kn to arrive %s UTC, saving %.1f h at anchor and about %.1f t VLSFO",
			a.AdvisedSpeedKn, a.AdvisedArrival.Format("02 Jan 1504"), a.AnchorHoursSaved, a.VLSFOSavedT)
	case "weather_on_track":
		var w struct {
			Summary tools.WeatherSummary `json:"summary"`
		}
		_ = decode(out, &w)
		s := w.Summary
		return fmt.Sprintf("Weather on track: wind Beaufort %d to %d, sea state %d to %d (%.1f to %.1f m); %d h of Beaufort 7+ and %d h of sea state 5+ from ahead",
			s.MinBeaufort, s.MaxBeaufort, s.MinDouglas, s.MaxDouglas, s.MinSWHm, s.MaxSWHm, s.HoursWindBf7AheadOrMore, s.HoursSeaD5AheadOrMore)
	}
	return name + " returns"
}

// shortTime writes an RFC 3339 time as "21 Jan 1200", or returns it as given.
func shortTime(s string) string {
	t, err := time.Parse(time.RFC3339, s)
	if err != nil {
		return s
	}
	return t.UTC().Format("02 Jan 1504")
}
