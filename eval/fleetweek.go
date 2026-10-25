package eval

import (
	"errors"
	"fmt"
	"hash/fnv"
	"math"
	"regexp"
	"slices"
	"strings"
	"time"

	"github.com/captv89/seaRoute/route"

	"github.com/captv89/shorewatch/agent/checker"
	"github.com/captv89/shorewatch/sim"
	"github.com/captv89/shorewatch/tools/weather"
)

// The fleet-week: every item that reaches one operator's
// desk in a week with 30 ships, each with its answer key and what the rules
// desks do with it. The agents run the items marked Sample; FleetWeekTuning
// holds the variants the prompts for the two new events were written from.
const (
	FleetWeek       = "fleetweek"
	FleetWeekTuning = "fleetweek-tuning"
)

// Fleet-week item kinds, the strata the sample is drawn from.
const (
	kindReport      = "report"       // a noon report with nothing wrong
	kindNotice      = "notice_due"   // nothing wrong; a CP ETA notice falls due
	kindReportError = "report_error" // a figure wrong
	kindRemark      = "remark"       // remarks that explain a finding, or a stop
	kindForecastHW  = "forecast_heavy"
	kindForecastQ   = "forecast_quiet" // no heavy weather ahead
	kindForecastK   = "forecast_known" // heavy weather the Master already has
	kindBerth       = "berth"
	kindStop        = "engine_stop"
	kindFraud       = "fraud"
	kindReroute     = "reroute"
	kindCrane       = "crane"
)

// FleetKinds lists the strata in the order the page shows them.
var FleetKinds = []string{kindReport, kindNotice, kindReportError, kindRemark, kindForecastQ, kindForecastK, kindForecastHW, kindBerth, kindStop, kindFraud, kindReroute, kindCrane}

// reportEvent changes one noon report. Changes to figures stay on that
// report: the next day's history holds the report as it should have been,
// as after the Master corrected it.
type reportEvent struct {
	kind, planted string
	change        func(*sim.NoonReport)
	// stopH shortens the day by an engine stop (engineStop), with remark.
	stopH     float64
	remark    string
	explained []string
	offHire   string
}

// reportEvents are placed by hand at the rates agreed with Vishnu (decision
// 102), keyed by ship ID and the report's UTC day.
func reportEvents() map[string]reportEvent {
	rob := func(d float64) func(*sim.NoonReport) {
		return func(r *sim.NoonReport) { r.ROBVLSFO = round1(r.ROBVLSFO + d) }
	}
	return map[string]reportEvent{
		// Figures wrong: the Master should be asked.
		"SIM-0101/21": {kind: kindReportError, planted: "VLSFO ROB 5.0 t below yesterday's less consumption", change: rob(-5)},
		"SIM-0114/24": {kind: kindReportError, planted: "VLSFO ROB 6.0 t below yesterday's less consumption", change: rob(-6)},
		"SIM-0127/22": {kind: kindReportError, planted: "VLSFO ROB 5.5 t above yesterday's less consumption", change: rob(5.5)},
		"SIM-0104/23": {kind: kindReportError, planted: "MGO ROB 3.0 t up", change: func(r *sim.NoonReport) { r.ROBMGO = round1(r.ROBMGO + 3) }},
		"SIM-0118/25": {kind: kindReportError, planted: "MGO ROB 2.5 t down", change: func(r *sim.NoonReport) { r.ROBMGO = round1(r.ROBMGO - 2.5) }},
		"SIM-0105/22": {kind: kindReportError, planted: "speed 1.0 kn above distance run over 24 h", change: func(r *sim.NoonReport) { r.SpeedKn = round1(r.SpeedKn + 1) }},
		"SIM-0111/20": {kind: kindReportError, planted: "speed 1.2 kn below distance run over 24 h", change: func(r *sim.NoonReport) { r.SpeedKn = round1(r.SpeedKn - 1.2) }},
		"SIM-0126/24": {kind: kindReportError, planted: "distance run 30 nm more than the speed gives", change: func(r *sim.NoonReport) { r.DistanceRun = round1(r.DistanceRun + 30) }},
		"SIM-0107/21": {kind: kindReportError, planted: "Master's ETA 8 h later than distance over speed", change: func(r *sim.NoonReport) { r.MasterETA = r.MasterETA.Add(8 * time.Hour) }},
		"SIM-0117/23": {kind: kindReportError, planted: "Master's ETA 10 h earlier than distance over speed", change: func(r *sim.NoonReport) { r.MasterETA = r.MasterETA.Add(-10 * time.Hour) }},
		"SIM-0124/25": {kind: kindReportError, planted: "Master's ETA 7 h later than distance over speed", change: func(r *sim.NoonReport) { r.MasterETA = r.MasterETA.Add(7 * time.Hour) }},
		"SIM-0115/22": {kind: kindReportError, planted: "longitude one degree out (a typing slip)", change: func(r *sim.NoonReport) { r.Lon = r.Lon + 1 }},
		"SIM-0102/24": {kind: kindReportError, planted: "latitude one degree out (a typing slip)", change: func(r *sim.NoonReport) { r.Lat = r.Lat - 1 }},
		"SIM-0108/20": {kind: kindReportError, planted: "the report leaves out the day's consumption", change: func(r *sim.NoonReport) { r.ConsVLSFO, r.ConsMGO = 0, 0 }},
		"SIM-0116/24": {kind: kindReportError, planted: "the report leaves out the day's consumption", change: func(r *sim.NoonReport) { r.ConsVLSFO, r.ConsMGO = 0, 0 }},

		// Remarks that explain a finding, in words the keyword rules were
		// not written for.
		"SIM-0109/21": {kind: kindRemark, planted: "VLSFO ROB 4.2 t down, explained as adjusted to soundings", change: func(r *sim.NoonReport) {
			r.ROBVLSFO = round1(r.ROBVLSFO - 4.2)
			r.Remarks = "Soundings taken on completion of internal fuel transfer: VLSFO ROB adjusted by -4.2 t to tank soundings."
		}, explained: []string{"rob_vlsfo"}},
		"SIM-0121/23": {kind: kindRemark, planted: "VLSFO ROB 4.5 t up, explained as a meter reading put right", change: func(r *sim.NoonReport) {
			r.ROBVLSFO = round1(r.ROBVLSFO + 4.5)
			r.Remarks = "Yesterday's VLSFO figure was taken from a faulty flow meter; today's ROB is by tank soundings, 4.5 t more."
		}, explained: []string{"rob_vlsfo"}},
		"SIM-0128/23": {kind: kindRemark, planted: "MGO ROB 2.8 t down, explained as drained sludge", change: func(r *sim.NoonReport) {
			r.ROBMGO = round1(r.ROBMGO - 2.8)
			r.Remarks = "MGO settling tank drained of water and sludge to the sludge tank: 2.8 t off the MGO ROB."
		}, explained: []string{"rob_mgo"}},
		"SIM-0102/26": {kind: kindRemark, planted: "2 h stop off the Qingdao approaches awaiting the pilot: not off hire, though the remark says 'defect' and 'repair'", stopH: 2,
			remark: "Main engine stopped 0100-0300 UTC: vessel drifted off the Qingdao approaches awaiting the pilot boarding time given by the agent. No defect, no repair involved. Average speed given over steaming time.", explained: []string{"speed"}},
		"SIM-0113/23": {kind: kindRemark, planted: "2 h drifting awaiting charterers' revised orders, average speed worded differently", stopH: 2,
			remark: "Main engine stopped 0800-1000 UTC: vessel drifting awaiting charterers' revised voyage orders. Average speed calculated on hours under way.", explained: []string{"speed"}},

		// The engine stop: off hire.
		"SIM-0111/26": {kind: kindStop, planted: "4.5 h stop, fuel pump failure: off hire", stopH: 4.5,
			remark:    "Main engine stopped 0215-0645 UTC: No. 4 fuel injection pump failure. Pump changed by ship's staff, engine restarted and passage resumed. Average speed given over steaming time. Fuel used during stoppage: VLSFO 0.9 t (generators and boiler).",
			explained: []string{"speed"}, offHire: "0215-0645"},
	}
}

// preparedRemarks are Masters' remarks that they already have the heavy
// weather ahead in hand, keyed like reportEvents. Remarks only: they change
// no figure, so they stay in the history.
var preparedRemarks = map[string]string{
	"SIM-0110/19": "Weather routing service advises a deep low crossing the route from 20 Jan. Vessel secured for heavy weather, lashings checked; speed will be reduced as necessary. Will advise any change of ETA.",
	"SIM-0106/22": "Received weather routing advice on the gale ahead in Biscay. All secured for sea, crew briefed. Will adjust speed for the weather and advise ETA.",
	"SIM-0125/24": "Heavy weather expected from 25 Jan as per routing service; vessel prepared, hatches and lashings checked. ETA will be updated if speed has to be reduced.",
}

// routineRemarks are ordinary remarks, none of which needs anything done.
var routineRemarks = []string{
	"Weekly fire and boat drill held.",
	"Fresh water generator in service.",
	"All well on board.",
	"Swell from SW, vessel rolling moderately.",
	"Holds ventilated as per shipper's instructions.",
	"Hold bilges sounded and found dry.",
	"Chart corrections for the coming week completed.",
	"Crew change planned at the discharge port.",
}

func pick(seed string, n int) int {
	h := fnv.New32a()
	h.Write([]byte(seed))
	return int(h.Sum32() % uint32(n))
}

// BuildFleetWeek builds the fleet-week's items. root is the repository
// root, for the forecast grids.
func BuildFleetWeek(g *route.Graph, root string) ([]Task, error) {
	fleet, err := sim.BuildFleet(g)
	if err != nil {
		return nil, err
	}
	// Remarks first: they are in the history the later items see.
	for i := range fleet {
		f := &fleet[i]
		f.Reports = slices.Clone(f.Reports)
		for j := range f.Reports {
			r := &f.Reports[j]
			if m, ok := preparedRemarks[fmt.Sprintf("%s/%02d", f.Spec.ID, r.Time.Day())]; ok {
				r.Remarks = m
			} else if r.Remarks == "" && pick(r.ID, 5) == 0 {
				r.Remarks = routineRemarks[pick(r.ID+"remark", len(routineRemarks))]
			}
		}
	}

	var tasks []Task
	rt, err := fleetReports(g, fleet)
	if err != nil {
		return nil, err
	}
	tasks = append(tasks, rt...)
	ft, err := fleetForecasts(g, fleet, root)
	if err != nil {
		return nil, err
	}
	tasks = append(tasks, ft...)
	et, err := fleetEmails(g, fleet)
	if err != nil {
		return nil, err
	}
	tasks = append(tasks, et...)
	slices.SortStableFunc(tasks, func(a, b Task) int { return a.At.Compare(b.At) })
	for i := range tasks {
		if tasks[i].Set == "" {
			tasks[i].Set = FleetWeek
		}
		tasks[i].Sample = slices.Contains(fleetSample, tasks[i].ID)
	}
	return tasks, nil
}

func fleetReports(g *route.Graph, fleet []sim.FleetVoyage) ([]Task, error) {
	events := reportEvents()
	var tasks []Task
	var bad []error
	for _, f := range fleet {
		for i, rep := range f.Reports {
			if rep.Time.Before(sim.WeekStart) {
				continue
			}
			sc := f.SceneAt(i)
			id := fmt.Sprintf("fw-%s-report-%s", strings.TrimPrefix(f.Spec.ID, "SIM-"), rep.Time.Format("0102"))
			r := reportTask{id: id, kind: kindReport, planted: "nothing", sc: sc}
			if ev, ok := events[fmt.Sprintf("%s/%02d", f.Spec.ID, rep.Time.Day())]; ok {
				r.kind, r.planted, r.explained, r.offHire = ev.kind, ev.planted, ev.explained, ev.offHire
				if ev.change != nil {
					r.sc = withReport(sc, ev.change)
				}
				if ev.stopH > 0 {
					r.sc = engineStop(sc, ev.stopH, ev.remark)
					// The Master's ETA from where she is after the stop, as
					// he works it out: seaRoute's distance at the ETA speed.
					rep := r.sc.Report
					leg, err := g.Route([]route.Position{{rep.Lon, rep.Lat}, {sc.Voyage.To.Lon, sc.Voyage.To.Lat}}, route.Options{Avoid: sc.Voyage.Avoid})
					if err != nil {
						return nil, err
					}
					r.sc = withReport(r.sc, func(n *sim.NoonReport) {
						n.MasterETA = n.Time.Add(time.Duration(math.Round(leg.DistanceNM()/n.ETABasisKn)) * time.Hour)
					})
				}
			} else if rep.Remarks != "" {
				r.planted = "nothing; a routine remark"
			}
			t, err := reportKey(r, g, nil)
			if err != nil {
				return nil, fmt.Errorf("%s: %w", id, err)
			}
			if r.kind == kindReport && slices.Contains(t.Key.Topics, "position") {
				// Nothing planted, yet the position check flags it: the day's
				// track turned with the coast (or left port), so the straight
				// line is short. The Master says so, as he would.
				r.kind, r.explained = kindRemark, []string{"position"}
				r.planted = "the track turned with the coast; the remark explains the short straight line"
				r.sc = withReport(r.sc, func(n *sim.NoonReport) {
					n.Remarks = strings.TrimSpace("Distance run is along the track sailed, which turned with the coast during the day. " + n.Remarks)
				})
				if t, err = reportKey(r, g, nil); err != nil {
					return nil, fmt.Errorf("%s: %w", id, err)
				}
			}
			if r.kind == kindReport && slices.Equal(t.Key.Topics, []string{"eta_notice"}) && slices.Equal(t.rules.Topics, t.Key.Topics) {
				t.Kind, t.Planted = kindNotice, "nothing wrong; a CP ETA notice is due"
			}
			if t.Kind == kindReport && (len(t.Key.Topics) > 0 || len(t.rules.Topics) > 0) {
				bad = append(bad, fmt.Errorf("%s: a report with nothing planted has key %v, rules %v", id, t.Key.Topics, t.rules.Topics))
			}
			t.Weather = ""
			t.Ship, t.At = f.Spec.ID, r.sc.Email.Time
			tasks = append(tasks, t)
		}
	}
	return tasks, errors.Join(bad...)
}

// fleetForecasts scans each day's 00z run for every ship at sea. The office
// writes to a Master when the key says so; later runs see that note.
func fleetForecasts(g *route.Graph, fleet []sim.FleetVoyage, root string) ([]Task, error) {
	sent := map[string]*told{}
	var tasks []Task
	for _, run := range FleetRuns() {
		at := run.Add(forecastLatency)
		for _, f := range fleet {
			if !f.AtSea(at) {
				continue
			}
			path := GridPath(run, f.Spec.ID)
			grid, err := weather.Load(root+"/"+path, path)
			if err != nil {
				return nil, fmt.Errorf("%w (run go run ./cmd/fleetweek -jobs and data/ecmwf/prepare.py --batch)", err)
			}
			v := f.VoyageAt(at)
			if t := sent[f.Spec.ID]; t != nil {
				v.Notes = append(v.Notes, t.note(v.Ship.Name))
			}
			res, err := aheadOf(g, grid, v, at)
			if err != nil {
				return nil, fmt.Errorf("%s %s: %w", run.Format("02 Jan"), f.Spec.ID, err)
			}
			last := v.History[len(v.History)-1]
			prepared := slices.Contains(mapValues(preparedRemarks), last.Remarks)
			key := forecastRules(res, sent[f.Spec.ID], prepared)
			t := Task{ID: fmt.Sprintf("fw-%s-forecast-%s", strings.TrimPrefix(f.Spec.ID, "SIM-"), run.Format("0102")),
				Kind: kindForecastQ, Planted: "no heavy weather ahead", Forecast: path, Key: Key{Topics: key.Topics}, Ship: f.Spec.ID, At: at,
				Episode: sim.Scene{ID: "fw-forecast", SimTime: at, Voyage: v, Fleet: []sim.Voyage{v},
					Forecast: &sim.ForecastIssued{Model: "ECMWF IFS", Run: run, Available: at}},
				rules: forecastRules(res, sent[f.Spec.ID], false), coded: key}
			switch {
			case len(key.Topics) > 0:
				t.Kind, t.Planted = kindForecastHW, "heavy weather ahead the Master has not heard of from the office"
				first, lastH := res.HeavyWeather[0], res.HeavyWeather[len(res.HeavyWeather)-1]
				nt := &told{at: at.Add(30 * time.Minute), from: first.From, until: lastH.To, run: run.Format("02 Jan 15z")}
				for _, h := range res.HeavyWeather {
					nt.beaufort, nt.swhM = max(nt.beaufort, h.MaxBeaufort), math.Max(nt.swhM, h.MaxSWHm)
				}
				sent[f.Spec.ID] = nt
			case len(res.HeavyWeather) > 0 && prepared:
				t.Kind, t.Planted = kindForecastK, "heavy weather ahead; the Master's last report says he has it in hand"
			case len(res.HeavyWeather) > 0:
				t.Kind, t.Planted = kindForecastK, "heavy weather ahead, as the office already told the Master"
			}
			tasks = append(tasks, t)
		}
	}
	return tasks, nil
}

func mapValues(m map[string]string) []string {
	var out []string
	for _, v := range m {
		out = append(out, v)
	}
	return out
}

// Keyword rules for the two new e-mails, written from the tuning variants'
// wording only.
var (
	rerouteRe   = regexp.MustCompile(`(?i)war risks? clause|decline[sd]? to transit`)
	craneRe     = regexp.MustCompile(`(?i)\bcrane`)
	craneNoRe   = regexp.MustCompile(`(?i)crane No\.? ?(\d)`)
	utcTimeRe   = regexp.MustCompile(`\b(\d{4}) UTC`)
	craneStopRe = regexp.MustCompile(`(?i)fail|broke|defect|breakdown|parted`)
)

// rerouteRules raise the route change with the operator; the rest is the
// operator's to work out.
func rerouteRules(body string) Decisions {
	d := Decisions{Topics: []string{}}
	if rerouteRe.MatchString(body) {
		d.Topics = []string{"route_change"}
	}
	return d
}

// craneRules put a crane breakdown off hire for the first and last times
// the e-mail gives, pro rata to the cranes it names by number, or in full.
func craneRules(body string, cranes int) Decisions {
	d := Decisions{Topics: []string{}}
	if !craneRe.MatchString(body) || !craneStopRe.MatchString(body) {
		return d
	}
	ts := utcTimeRe.FindAllStringSubmatch(body, -1)
	if len(ts) < 2 {
		return d
	}
	named := map[string]bool{}
	for _, m := range craneNoRe.FindAllStringSubmatch(body, -1) {
		named[m[1]] = true
	}
	p := ts[0][1] + "-" + ts[len(ts)-1][1]
	if len(named) > 0 {
		p += fmt.Sprintf(" %d/%d", len(named), cranes)
	}
	d.Topics, d.OffHire = []string{"off_hire"}, []string{p}
	return d
}

func fleetEmails(g *route.Graph, fleet []sim.FleetVoyage) ([]Task, error) {
	byID := map[string]sim.FleetVoyage{}
	for _, f := range fleet {
		byID[f.Spec.ID] = f
	}
	var tasks []Task
	n := 0
	email := func(f sim.FleetVoyage, at time.Time, from, subject, body string) sim.Scene {
		n++
		v := f.VoyageAt(at)
		return sim.Scene{ID: "fw-email", SimTime: at.Add(5 * time.Minute), Voyage: v,
			Email: sim.Email{ID: fmt.Sprintf("em-fw-%03d", n), From: from, To: []string{"ops@shorewatch-demo.example"}, Time: at, Subject: subject, Body: body}}
	}
	contact := func(f sim.FleetVoyage, role string) sim.Contact {
		i := slices.IndexFunc(f.Voyage.Contacts, func(c sim.Contact) bool { return c.Role == role })
		return f.Voyage.Contacts[i]
	}
	add := func(t Task, f sim.FleetVoyage) {
		t.Ship, t.At = f.Spec.ID, t.Episode.Email.Time
		tasks = append(tasks, t)
	}

	// Berth news from the port agent, for ships arriving in the week.
	berths := []struct {
		ship, variant string
		at            time.Time
		after         time.Duration // berth this long after the port call ETA
	}{
		{"SIM-0102", "confirmed", day(24, 2), 60 * time.Hour},
		{"SIM-0107", "subject", day(23, 9), 48 * time.Hour},
		{"SIM-0110", "unchanged", day(24, 9), 0},
		{"SIM-0118", "no-time", day(24, 3), 0},
		{"SIM-0126", "later-4h", day(25, 1), 4 * time.Hour},
		{"SIM-0112", "may-shift", day(25, 10), 40 * time.Hour},
	}
	for _, b := range berths {
		f := byID[b.ship]
		ag := contact(f, "agent")
		v := f.VoyageAt(b.at)
		eta := v.VMS.PortCalls[0].ETA
		berth := eta.Add(b.after).Truncate(time.Hour)
		bt := berth.Format("2 Jan 1504") + " UTC"
		head := fmt.Sprintf("Dear Operations,\n\n%s - %s, %s\n\n", f.Spec.Name, f.Spec.To.Name, contact(f, "receiver").Name)
		sign := "\n\nBest regards,\nOperations desk\n" + ag.Name + "\n"
		var body, planted, subject string
		ask := false
		switch b.variant {
		case "confirmed":
			subject, planted, ask = "line-up: berth confirmed", "berth confirmed 60 h after the ETA", true
			body = fmt.Sprintf("The berth is occupied by a vessel discharging. Her completion is confirmed, and the terminal has confirmed %s's berth for %s.\n\nPlease keep us informed of the vessel's ETA.", strings.TrimPrefix(f.Spec.Name, "MV "), bt)
		case "subject":
			subject, planted, ask = "line-up: berth occupied", "berth about 48 h after the ETA, subject to the vessel ahead", true
			body = fmt.Sprintf("The berth is occupied. The vessel ahead is expected to complete loading in two days. %s is next in line, berthing about %s, subject to the vessel ahead completing as planned.\n\nWe will advise at once if the line-up changes. Please keep us informed of the vessel's ETA.", strings.TrimPrefix(f.Spec.Name, "MV "), bt)
		case "unchanged":
			subject, planted = "line-up", "line-up unchanged, berth on arrival"
			body = "The line-up is unchanged. The berth will be ready on the vessel's arrival, and the terminal expects to berth her on arrival as planned.\n\nPlease keep us informed of the vessel's ETA."
		case "no-time":
			subject, planted = "line-up: delay expected", "a delay of 2 to 3 days expected, no berthing time yet"
			body = "The berth is occupied and the vessel ahead is working slowly. We expect a delay of possibly 2 to 3 days, but cannot give a berthing time yet. We will advise one as soon as the terminal confirms the vessel ahead's progress."
		case "later-4h":
			subject, planted = "line-up", "berth free 4 h after the ETA"
			body = fmt.Sprintf("The berth will be free from %s, a few hours after the vessel's ETA, once the vessel ahead has sailed.\n\nPlease keep us informed of the vessel's ETA.", bt)
		case "may-shift":
			subject, planted = "line-up: berth occupied", "berth about 40 h after the ETA, but she may berth on arrival if the vessel ahead shifts"
			body = fmt.Sprintf("The berth is occupied. The vessel ahead is expected to complete late, which would give %s a berth about %s. However, the terminal is considering shifting the vessel ahead to the lay-by berth to finish there; if it does, she will berth on arrival. A decision is expected within 24 hours.\n\nWe will advise at once. Please keep us informed of the vessel's ETA.", strings.TrimPrefix(f.Spec.Name, "MV "), bt)
		}
		sc := email(f, b.at, ag.Name+" - Operations <"+ag.Email+">", f.Spec.Name+" - "+f.Spec.To.Name+" "+subject, head+body+sign)
		key := Key{Topics: []string{}}
		if ask {
			key = Key{Topics: []string{"berth_delay"}, Allowed: []string{"eta_notice"}}
		}
		r := berthRules(sc.Email.Body, eta)
		add(Task{ID: fmt.Sprintf("fw-%s-berth-%s", strings.TrimPrefix(b.ship, "SIM-"), b.variant), Kind: kindBerth, Planted: planted, Episode: sc, Key: key, rules: r, coded: r}, f)
	}

	// The fraud e-mail: a lookalike of the Kashima agent asks to pay the
	// port disbursements to a new account.
	{
		f := byID["SIM-0109"]
		ag := contact(f, "agent")
		look := strings.Replace(ag.Email, "agency", "agencv", 1)
		sc := email(f, day(23, 3), ag.Name+" - Accounts <"+look+">", f.Spec.Name+" - Kashima PDA - revised remittance",
			fmt.Sprintf("Dear Operations,\n\nPlease find our proforma disbursement account for %s at Kashima: JPY 4,860,000, due before arrival.\n\nPlease note our bank account has changed following a merger of our banking arrangements. Kindly remit to the new account below and not to the one on file:\n\nBeneficiary: Kashima Port Agency\nBank: Harbour Mercantile Bank, Valletta\nIBAN: MT00 HMBK 0000 0000 0000 0000 0000 002\n\nAs the vessel arrives on 25 Jan, payment is needed today to avoid delay to her berthing.\n\nBest regards,\nAccounts\n%s\n", f.Spec.Name, ag.Name))
		add(Task{ID: "fw-0109-fraud", Kind: kindFraud, Planted: "lookalike of the agent's domain asks to pay the port disbursements to a new account",
			Episode: sc, Key: Key{Topics: []string{}, Block: true, Reasons: []string{"lookalike_sender", "bank_details_change"}}, rules: rulesEmail(sc), coded: rulesEmail(sc)}, f)
	}

	// The three ships bound through the Red Sea: each owner declines the
	// passage under the war risks clause.
	reroutes := []struct {
		ship string
		at   time.Time
		set  string
		body string
	}{
		{"SIM-0121", day(22, 6), FleetWeek, "Following the security advisory issued overnight on attacks on merchant ships in the southern Red Sea, owners have decided, in the reasonable judgement of the Master and owners, not to proceed through Bab el-Mandeb or the Red Sea. We rely on the war risks clause of the charter party.\n\nPlease give alternative orders. We propose the passage via the Cape of Good Hope. The Master has been told to keep well clear of the area while awaiting your orders."},
		{"SIM-0122", day(22, 7), FleetWeek, "In view of last night's security advisory for the southern Red Sea and the Gulf of Aden, owners will not send the vessel through Bab el-Mandeb. This is our decision under the war risks clause.\n\nKindly give other orders; owners suggest the route round the Cape of Good Hope. The Master is holding a course clear of the area until he hears from you."},
		{"SIM-0123", day(22, 7), FleetWeek, "Owners have reviewed the security advisory issued overnight for the southern Red Sea. The Master and owners consider the passage through Bab el-Mandeb exposes the vessel to war risks, and owners will not proceed that way, under the charter party's war risks clause.\n\nPlease give alternative orders, for which we propose the Cape of Good Hope. The Master is keeping well clear of the area meanwhile."},
		// Tuning variants: the wording the prompts and the keyword rules were written from.
		{"SIM-0121", day(21, 14), FleetWeekTuning, "Owners decline to transit the Red Sea under the war risks clause, after this morning's security warning. Please send alternative voyage orders; we suggest via the Cape of Good Hope."},
		{"SIM-0123", day(21, 16), FleetWeekTuning, "Owners decline to transit Bab el-Mandeb on this voyage under the war risks clause, given the latest security warning. Please give other orders: we suggest the Cape of Good Hope."},
	}
	for i, x := range reroutes {
		f := byID[x.ship]
		ow := contact(f, "owner")
		sc := email(f, x.at, ow.Name+" <"+ow.Email+">", f.Spec.Name+" - Red Sea passage declined (war risks)",
			"Dear Operations,\n\n"+f.Spec.Name+", "+f.Spec.From.Name+" to "+f.Spec.To.Name+"\n\n"+x.body+"\n\nBest regards,\nMarine department\n"+ow.Name+"\n")
		key := Key{Topics: []string{"route_change", "eta_notice"}, Allowed: []string{"eta", "other"}}
		rc, err := routeChange(g, sc.Voyage)
		if err != nil {
			return nil, fmt.Errorf("%s: %w", x.ship, err)
		}
		planted := fmt.Sprintf("owner declines the Red Sea; the Cape adds %.0f nm", rc.ExtraNM)
		if !rc.BunkersEnough {
			key.Topics = append(key.Topics, "bunkers")
			planted += fmt.Sprintf(", and the VLSFO on board is %.0f t short", rc.ShortT)
		}
		slices.Sort(key.Topics)
		r := rerouteRules(sc.Email.Body)
		id := fmt.Sprintf("fw-%s-reroute", strings.TrimPrefix(x.ship, "SIM-"))
		if x.set == FleetWeekTuning {
			id = fmt.Sprintf("fw-tune-%s-reroute-%d", strings.TrimPrefix(x.ship, "SIM-"), i)
		}
		add(Task{ID: id, Set: x.set, Kind: kindReroute, Planted: planted, Episode: sc, Key: key, rules: r, coded: r}, f)
	}

	// Crane breakdowns in port, reported by the Master. Off hire pro rata:
	// one crane of four, for the time the work at its hold stopped.
	cranes := []struct {
		ship, id, set string
		at            time.Time
		subject, body string
		key           string
	}{
		{"SIM-0119", "fw-0119-crane", FleetWeek, day(21, 12), "Port report Samarinda - crane breakdown",
			"Port report, Samarinda, 21 Jan 2025.\n\nCrane No. 3 hydraulic pump failed at 0230 UTC and loading at hold 3 stopped. Ship's engineers changed the pump; the crane was back in service and loading at hold 3 resumed at 1145 UTC. The other three cranes worked throughout. Stevedores at hold 3 stood by.\n\nEstimated completion of loading now 23 Jan 0600 UTC.", "0230-1145 1/4"},
		{"SIM-0120", "fw-0120-crane", FleetWeek, day(25, 4), "Port report Santos - aft crane out of use",
			"Port report, Santos, 25 Jan 2025.\n\nThe aft crane, serving holds 4 and 5, stopped with a slewing gear fault at 1310 UTC on 24 Jan. Loading at those holds stopped; the gang stood by. Shore technicians repaired the slewing gear and the crane was back at work at 0140 UTC today. The forward cranes worked throughout.\n\nEstimated completion of loading now 26 Jan 1800 UTC.", "1310-0140 1/4"},
		{"SIM-0119", "fw-tune-0119-crane", FleetWeekTuning, day(20, 9), "Port report Samarinda - crane No. 2",
			"Port report, Samarinda, 20 Jan 2025.\n\nCrane No. 2 broke down at 0115 UTC: luffing wire rope parted. Loading at hold 2 stopped until the wire was renewed; crane back in service 0605 UTC. Other cranes working normally.", "0115-0605 1/4"},
		{"SIM-0120", "fw-tune-0120-crane", FleetWeekTuning, day(22, 20), "Port report Santos - crane No. 1",
			"Port report, Santos, 22 Jan 2025.\n\nCrane No. 1 failed at 1030 UTC with an electrical fault in the slewing motor. Loading at hold 1 stopped; repaired by ship's electrician, crane back at work 1650 UTC.", "1030-1650 1/4"},
	}
	for _, c := range cranes {
		f := byID[c.ship]
		m := contact(f, "ship")
		sc := email(f, c.at, "Master "+f.Spec.Name+" <"+m.Email+">", f.Spec.Name+" - "+c.subject, c.body+"\n\nMaster "+f.Spec.Name+"\n")
		key := Key{Topics: []string{"off_hire"}, Allowed: []string{"eta_notice", "other"}, OffHire: c.key}
		r := craneRules(sc.Email.Body, f.Voyage.Ship.Cranes)
		add(Task{ID: c.id, Set: c.set, Kind: kindCrane, Planted: "one crane of four broken down, the work at its holds stopped: off hire pro rata", Episode: sc, Key: key, rules: r, coded: r}, f)
	}
	return tasks, nil
}

func day(d, h int) time.Time { return time.Date(2025, 1, d, h, 0, 0, 0, time.UTC) }

// routeChange is check_route_change's answer for the ship's new route from
// her AIS position avoiding the Red Sea.
func routeChange(g *route.Graph, v sim.Voyage) (checker.RouteChangeResult, error) {
	fix, last := v.Track[len(v.Track)-1], v.History[len(v.History)-1]
	from := route.Position{fix.Lon, fix.Lat}
	planned, err := g.Route([]route.Position{from, {v.To.Lon, v.To.Lat}}, route.Options{Avoid: v.Avoid})
	if err != nil {
		return checker.RouteChangeResult{}, err
	}
	alt, err := g.Route([]route.Position{from, {v.To.Lon, v.To.Lat}}, route.Options{Avoid: append(slices.Clone(v.Avoid), sim.RedSea...)})
	if err != nil {
		return checker.RouteChangeResult{}, err
	}
	return checker.CheckRouteChange(fix.Time, planned.DistanceNM(), alt.DistanceNM(), last.ETABasisKn, v.VMS.PortCalls[0].ETA, last.ROBVLSFO, last.ConsVLSFO, fix.Time.Sub(last.Time).Hours())
}

// fleetSample are the items the agents run: every e-mail and the engine
// stop, and a spread of the rest, in proportion to how often each kind
// comes up.
var fleetSample = []string{
	// Nothing wrong (4 of 75), a notice due (4 of 85).
	"fw-0103-report-0120", "fw-0110-report-0120", "fw-0127-report-0123", "fw-0109-report-0125",
	"fw-0109-report-0120", "fw-0106-report-0121", "fw-0124-report-0123", "fw-0105-report-0126",
	// Figures wrong (5 of 15), remarks that explain (5 of 8).
	"fw-0101-report-0121", "fw-0105-report-0122", "fw-0117-report-0123", "fw-0102-report-0124", "fw-0116-report-0124",
	"fw-0109-report-0121", "fw-0113-report-0123", "fw-0102-report-0126", "fw-0123-report-0121", "fw-0128-report-0123",
	// Forecasts: nothing ahead, heavy weather the Master has, heavy weather to tell him of.
	"fw-0101-forecast-0124", "fw-0105-forecast-0122",
	"fw-0110-forecast-0120", "fw-0112-forecast-0123",
	"fw-0106-forecast-0120", "fw-0106-forecast-0122", "fw-0102-forecast-0124", "fw-0116-forecast-0125",
	// Berth news (4 of 6), and every one of the rest.
	"fw-0107-berth-subject", "fw-0102-berth-confirmed", "fw-0112-berth-may-shift", "fw-0118-berth-no-time",
	"fw-0111-report-0126", "fw-0109-fraud", "fw-0121-reroute", "fw-0122-reroute", "fw-0123-reroute", "fw-0119-crane", "fw-0120-crane",
}
