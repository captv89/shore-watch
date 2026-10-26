package eval

import (
	"fmt"
	"math"
	"slices"

	"github.com/captv89/seaRoute/route"

	"github.com/captv89/shorewatch/agent/checker"
	"github.com/captv89/shorewatch/agent/desk"
	"github.com/captv89/shorewatch/sim"
	"github.com/captv89/shorewatch/tools"
	"github.com/captv89/shorewatch/tools/weather"
)

// Weather files the noon report tasks on TORMAR SPRING use.
const (
	WeatherS2  = "data/weather/s2-tormar-spring.json"
	ForecastS2 = "data/weather/s2-forecast-20250124T00.json"
)

// reportTask is a noon report task before its key is worked out.
type reportTask struct {
	id, kind, planted string
	sc                sim.Scene
	// explained are report check IDs the report's remarks explain.
	explained []string
	// extra are topics the code key cannot work out (the forecast ETA).
	extra []string
	// offHire is the off-hire period the remarks give, "HHMM-HHMM" UTC, if any.
	offHire string
}

// Build makes the 40 Round 1 tasks, alternating tuning and held-out
// within each kind. root is the repository root, for the weather files.
func Build(g *route.Graph, root string) ([]Task, error) {
	s1, err := sim.Scene1(g)
	if err != nil {
		return nil, err
	}
	s2, err := sim.Scene2Days(g) // D1, D2, D3
	if err != nil {
		return nil, err
	}
	wx, err := weather.Load(root+"/"+WeatherS2, WeatherS2)
	if err != nil {
		return nil, err
	}

	// VALDA CREST, with the consumption the report leaves out in Scene 1.
	// Scene 1's CP gives no warranty; the eval adds her service figures,
	// about 12.5 kn on about 26 t VLSFO a day, for the Performance Agent.
	// Her 12.4 kn is inside the 0.5 kn "about" margin, so the clean tasks
	// stay clean.
	s1.Voyage.CP.WarrantedSpeedKn, s1.Voyage.CP.WarrantedVLSFOPerDayT = 12.5, 26.0
	vb := withReport(s1, func(r *sim.NoonReport) { r.ConsVLSFO, r.ConsMGO = 26.0, 0.2 })
	// The same, with the 3-day notice already sent at yesterday's report on
	// today's ETA: no notice due.
	vq := vb
	vq.Voyage.Notices = append(slices.Clone(vb.Voyage.Notices),
		sim.Notice{SentAt: prev(vb).Time.Add(hours(0.5)), DaysLabel: 3, ETA: vb.Report.MasterETA})
	vq = rerender(vq)
	d1, d2, d3 := s2[0], s2[1], s2[2]

	rt := []reportTask{
		// Clean reports.
		{id: "clean-valda", kind: "clean", planted: "nothing", sc: vq},
		{id: "clean-valda-remark", kind: "clean", planted: "nothing; a routine remark", sc: withReport(vq, func(r *sim.NoonReport) {
			r.Remarks = "Fresh water generator in service. Weekly fire and boat drill held 1000-1100."
		})},
		{id: "clean-tormar-d2", kind: "clean", planted: "nothing; consumption 3% over warranty, within tolerance", sc: d2},
		{id: "clean-valda-eta-1h", kind: "clean", planted: "Master's ETA 1 h later, within the 2 h tolerance", sc: withReport(vq, func(r *sim.NoonReport) { r.MasterETA = r.MasterETA.Add(hours(1)) })},
		{id: "clean-tormar-d1-on-warranty", kind: "clean", planted: "nothing; consumption on warranty", sc: withCons(d1, 24.0)},
		// ETA.
		{id: "eta-late-6h", kind: "eta_inconsistent", planted: "Master's ETA 6 h later than distance over speed", sc: withReport(vq, func(r *sim.NoonReport) { r.MasterETA = r.MasterETA.Add(hours(6)) })},
		{id: "eta-early-20h", kind: "eta_inconsistent", planted: "Master's ETA 20 h earlier than distance over speed", sc: withReport(vq, func(r *sim.NoonReport) { r.MasterETA = r.MasterETA.Add(hours(-20)) })},
		{id: "notice-3day-and-change", kind: "notice_due", planted: "3-day notice due and the ETA moved more than 12 h", sc: vb},
		{id: "notice-3day", kind: "notice_due", planted: "3-day notice due", sc: withChangeNotice(vb)},
		// ROB.
		{id: "rob-vlsfo-short", kind: "rob", planted: "VLSFO ROB 4.0 t below yesterday's less consumption; a routine remark", sc: withReport(vq, func(r *sim.NoonReport) {
			r.ROBVLSFO = round1(r.ROBVLSFO - 4.0)
			r.Remarks = "Weekly fire and boat drill held 1000-1100."
		})},
		{id: "rob-mgo-up", kind: "rob", planted: "MGO ROB 3.0 t higher than yesterday's less consumption", sc: withReport(vq, func(r *sim.NoonReport) { r.ROBMGO = round1(r.ROBMGO + 3.0) })},
		{id: "rob-vlsfo-short-tormar", kind: "rob", planted: "VLSFO ROB 6.0 t below yesterday's less consumption", sc: withReport(d2, func(r *sim.NoonReport) { r.ROBVLSFO = round1(r.ROBVLSFO - 6.0) })},
		// Speed and distance.
		{id: "speed-mismatch", kind: "speed", planted: "speed 1.0 kn above distance run over 24 h", sc: withReport(vq, func(r *sim.NoonReport) { r.SpeedKn = round1(r.SpeedKn + 1.0) })},
		{id: "speed-mismatch-tormar", kind: "speed", planted: "speed 13.4 kn against 288 nm in 24 h", sc: withReport(d2, func(r *sim.NoonReport) { r.SpeedKn = 13.4 })},
		{id: "speed-impossible", kind: "speed", planted: "distance run 432 nm and 18.0 kn, beyond a laden Supramax", sc: withReport(vq, func(r *sim.NoonReport) { r.DistanceRun, r.SpeedKn = 432.0, 18.0 })},
		// Position.
		{id: "position-lat-typo", kind: "position", planted: "latitude one degree out (a typed 1 for a 0)", sc: withReport(vq, func(r *sim.NoonReport) { r.Lat += 1 })},
		{id: "position-lon-typo-tormar", kind: "position", planted: "longitude one degree out", sc: withReport(d2, func(r *sim.NoonReport) { r.Lon -= 1 })},
		// Missing figures.
		{id: "missing-consumption", kind: "missing", planted: "no consumption figures", sc: withReport(vq, func(r *sim.NoonReport) { r.ConsVLSFO, r.ConsMGO = 0, 0 })},
		{id: "missing-consumption-tormar", kind: "missing", planted: "no consumption figures", sc: withReport(d2, func(r *sim.NoonReport) { r.ConsVLSFO, r.ConsMGO = 0, 0 })},
		// Remarks that explain what the checks flag.
		{id: "remark-engine-stop", kind: "remarks", planted: "4 h engine stop; speed given over steaming time, explained in remarks", explained: []string{"speed"},
			sc: engineStop(vq, 4, "Main engine stopped 0600-1000 for fuel pump repair. Average speed given over steaming time.")},
		{id: "remark-engine-stop-tormar", kind: "remarks", planted: "3 h engine stop; speed given over steaming time, explained in remarks", explained: []string{"speed"},
			sc: engineStop(d2, 3, "Main engine stopped 0300-0600 for exhaust valve change. Average speed given over steaming time. Wind and sea increasing from W in the afternoon.")},
		{id: "remark-rob-soundings", kind: "remarks", planted: "VLSFO ROB 4.2 t up; remarks: ROB corrected after soundings", explained: []string{"rob_vlsfo"}, sc: withReport(vq, func(r *sim.NoonReport) {
			r.ROBVLSFO = round1(r.ROBVLSFO + 4.2)
			r.Remarks = "All bunker tanks sounded today; VLSFO ROB corrected by +4.2 t."
		})},
		{id: "remark-rob-soundings-tormar", kind: "remarks", planted: "VLSFO ROB 3.5 t down; remarks: ROB corrected after soundings", explained: []string{"rob_vlsfo"}, sc: withReport(d2, func(r *sim.NoonReport) {
			r.ROBVLSFO = round1(r.ROBVLSFO - 3.5)
			r.Remarks = "All bunker tanks sounded today; VLSFO ROB corrected by -3.5 t. Wind and sea increasing from W in the afternoon."
		})},
		{id: "remark-mgo-soundings", kind: "remarks", planted: "MGO ROB 2.5 t up; remarks: ROB corrected after soundings", explained: []string{"rob_mgo"}, sc: withReport(vq, func(r *sim.NoonReport) {
			r.ROBMGO = round1(r.ROBMGO + 2.5)
			r.Remarks = "MGO tanks sounded today; MGO ROB corrected by +2.5 t."
		})},
		// Consumption and weather (real ECMWF data).
		{id: "weather-good-day-over", kind: "weather", planted: "consumption +15% on a good weather day (Scene 2 D1)", sc: d1},
		{id: "weather-gale-over", kind: "weather", planted: "consumption +18% in a gale; forecast puts the ETA at risk (Scene 2 D3)", sc: d3, extra: []string{"eta_forecast"}},
		{id: "weather-d2-over", kind: "weather", planted: "consumption +12.5% on 23 Jan", sc: withCons(d2, 27.0)},
		{id: "weather-gale-on-warranty", kind: "weather", planted: "consumption on warranty in a gale; forecast puts the ETA at risk", sc: withCons(d3, 24.0), extra: []string{"eta_forecast"}},
	}

	var tasks []Task
	for _, r := range rt {
		t, err := reportKey(r, g, wx)
		if err != nil {
			return nil, fmt.Errorf("%s: %w", r.id, err)
		}
		tasks = append(tasks, t)
	}
	tasks = append(tasks, emailTasks()...)
	r2, err := round2Tasks(g, root, wx)
	if err != nil {
		return nil, err
	}
	tasks = append(tasks, r2...)
	assignSets(tasks)
	return tasks, nil
}

// reportKey works the key out with the same Checker rules the tools use,
// then applies what was planted (remarks that explain, forecast ETA).
func reportKey(r reportTask, g *route.Graph, wx *weather.Grid) (Task, error) {
	sc := r.sc
	rep, v := sc.Report, sc.Voyage
	var topics, rules, coded []string
	if rep.ConsVLSFO <= 0 || rep.ConsMGO <= 0 {
		topics = append(topics, "missing_figures")
	}
	p := prev(sc)
	keywords := remarkRules(rep, p, v.VMS != nil)
	var flags []string
	for _, f := range desk.ReportChecks(asReport(rep), &p) {
		flags = append(flags, f.ID)
		// Keyword rules explain some flags; a coded report explains exactly
		// the ones the remarks do.
		if !slices.Contains(keywords.explained, f.ID) {
			rules = append(rules, f.ID)
		}
		if !slices.Contains(r.explained, f.ID) {
			topics = append(topics, f.ID)
			coded = append(coded, f.ID)
		}
	}
	for _, id := range r.explained {
		if !slices.ContainsFunc(desk.ReportChecks(asReport(rep), &p), func(f desk.ReportFlag) bool { return f.ID == id }) {
			return Task{}, fmt.Errorf("remarks explain %s, but the check does not flag it", id)
		}
	}

	leg, err := g.Route([]route.Position{{rep.Lon, rep.Lat}, {v.To.Lon, v.To.Lat}}, route.Options{Avoid: v.Avoid})
	if err != nil {
		return Task{}, err
	}
	speed := rep.SpeedKn
	if rep.ETABasisKn > 0 {
		speed = rep.ETABasisKn
	}
	eta, err := checker.CheckETA(rep.Time, rep.MasterETA, round1(leg.DistanceM/route.MetresPerNM), speed)
	if err != nil {
		return Task{}, err
	}
	var allowed []string
	switch {
	case !eta.Consistent && slices.Contains(topics, "speed"):
		// The ETA check divides by the reported speed. When that figure is
		// flagged, the ETA follows from it: one question about the speed is
		// enough, and asking about the ETA too is no false alarm.
		allowed = append(allowed, "eta")
	case !eta.Consistent:
		topics = append(topics, "eta")
	case checker.CheckNotices(rep.Time, rep.MasterETA, v.CP, v.Notices).Due:
		topics = append(topics, "eta_notice")
	}

	t := Task{ID: r.id, Kind: r.kind, Planted: r.planted, Episode: sc}
	if len(v.Track) > 0 {
		t.Weather = WeatherS2
	}
	if rep.ConsVLSFO > 0 && v.CP.WarrantedVLSFOPerDayT > 0 {
		c, err := checker.CheckConsumption(rep.ConsVLSFO, v.CP.WarrantedVLSFOPerDayT, rep.SpeedKn, v.CP.WarrantedSpeedKn)
		if err != nil {
			return Task{}, err
		}
		if c.Flagged {
			w, err := tools.WeatherOnTrack(wx, v, rep.Time.Add(hours(-24)), rep.Time)
			if err != nil {
				return Task{}, err
			}
			s := w.Summary
			day := checker.ClassifyWeatherDay(checker.WeatherDay{Hours: s.Samples * s.HoursPerSample, Samples: s.Samples,
				SamplesWithSea: s.SamplesWithSea, MaxBeaufort: s.MaxBeaufort, MaxDouglas: s.MaxDouglas}, v.CP)
			if !day.Classified {
				return Task{}, fmt.Errorf("weather day not classified: %s", day.Interpretation)
			}
			if day.GoodWeatherDay {
				topics = append(topics, "consumption")
			}
		}
	}
	if len(r.extra) > 0 {
		t.Forecast = ForecastS2
		topics = append(topics, r.extra...)
	}
	t.Key = Key{Topics: topics, Allowed: allowed}
	if r.offHire != "" {
		t.Key.Topics = append(t.Key.Topics, "off_hire")
		t.Key.OffHire = r.offHire
	}
	slices.Sort(t.Key.Topics)
	// Everything else in the key comes from Checker rules, which the
	// rules-only desk has too, including the forecast ETA check. Off-hire is
	// the rules' own call.
	finish := func(own []string, offHire string) Decisions {
		for _, k := range topics {
			// Report check flags are each baseline's own call, above.
			if !slices.Contains(own, k) && !slices.Contains(flags, k) {
				own = append(own, k)
			}
		}
		if !eta.Consistent && !slices.Contains(own, "eta") {
			// The rules-only desk raises every check that fails.
			own = append(own, "eta")
		}
		d := Decisions{Topics: own}
		if offHire != "" {
			d.Topics = append(d.Topics, "off_hire")
			d.OffHire = []string{offHire}
		}
		slices.Sort(d.Topics)
		return d
	}
	t.rules = finish(rules, keywords.offHire)
	t.coded = finish(coded, r.offHire)
	return t, nil
}

// assignSets alternates tuning and held-out down the list, which is grouped
// by kind, so both sets get 20 tasks and every kind.
func assignSets(tasks []Task) {
	for i := range tasks {
		tasks[i].Set = []string{Tuning, HeldOut}[i%2]
	}
}

func prev(sc sim.Scene) sim.NoonReport { return sc.Voyage.History[len(sc.Voyage.History)-1] }

func rerender(sc sim.Scene) sim.Scene {
	sc.Email = sc.Report.Render(sc.Voyage)
	return sc
}

// withReport changes the scene's noon report and writes the e-mail again.
func withReport(sc sim.Scene, change func(*sim.NoonReport)) sim.Scene {
	change(&sc.Report)
	return rerender(sc)
}

// withCons sets the day's VLSFO and keeps the ROB consistent with it.
func withCons(sc sim.Scene, vlsfo float64) sim.Scene {
	return withReport(sc, func(r *sim.NoonReport) {
		r.ROBVLSFO = round1(r.ROBVLSFO + r.ConsVLSFO - vlsfo)
		r.ConsVLSFO = vlsfo
	})
}

// withNotice adds an ETA-change notice sent at yesterday's report on
// today's ETA, so only the day-count notice falls due.
func withChangeNotice(sc sim.Scene) sim.Scene {
	sc.Voyage.Notices = append(slices.Clone(sc.Voyage.Notices),
		sim.Notice{SentAt: prev(sc).Time.Add(hours(0.5)), DaysLabel: 0, ETA: sc.Report.MasterETA})
	return rerender(sc)
}

// engineStop shortens the day by a stop: less distance and fuel, the
// position moved back along the leg, the speed over steaming time, and the
// Master's ETA later by the stop.
func engineStop(sc sim.Scene, stopH float64, remark string) sim.Scene {
	p := prev(sc)
	return withReport(sc, func(r *sim.NoonReport) {
		steaming := r.Time.Sub(p.Time).Hours() - stopH
		dist := round1(r.SpeedKn * steaming)
		f := dist / r.DistanceRun
		r.Lat = math.Round((p.Lat+(r.Lat-p.Lat)*f)*600) / 600
		r.Lon = math.Round((p.Lon+(r.Lon-p.Lon)*f)*600) / 600
		r.DistanceRun = dist
		used := round1(r.ConsVLSFO * steaming / (steaming + stopH))
		r.ROBVLSFO = round1(r.ROBVLSFO + r.ConsVLSFO - used)
		r.ConsVLSFO = used
		r.MasterETA = r.MasterETA.Add(hours(stopH))
		r.Remarks = remark
	})
}

// asReport is what a correct extraction of the report gives.
func asReport(n sim.NoonReport) desk.Report {
	return desk.Report{
		Extraction: desk.Extraction{DistanceRunNM: n.DistanceRun, AvgSpeedKn: n.SpeedKn, ROBVLSFOt: n.ROBVLSFO, ROBMGOt: n.ROBMGO,
			ConsVLSFOt: n.ConsVLSFO, ConsMGOt: n.ConsMGO, Remarks: n.Remarks},
		Lat: n.Lat, Lon: n.Lon, ReportTime: n.Time, MasterETA: n.MasterETA,
	}
}

func round1(v float64) float64 { return math.Round(v*10) / 10 }
