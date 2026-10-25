package eval

import (
	"fmt"
	"regexp"
	"slices"
	"time"

	"github.com/captv89/seaRoute/route"

	"github.com/captv89/shorewatch/agent/checker"
	"github.com/captv89/shorewatch/sim"
	"github.com/captv89/shorewatch/tools/weather"
)

// Round 2 tasks: six each for a new forecast, berth news
// from the port agent and a main engine stop. In each kind the tuning tasks
// come first in each pair, so the keyword rules and the prompts were written
// from them only.

// The forecast runs the forecast tasks use (real ECMWF data).
const (
	Forecast22 = "data/weather/s4-forecast-20250122T12.json"
	Forecast24 = ForecastS2
)

// keywordResult is what the keyword rules make of a report's remarks.
type keywordResult struct {
	explained []string // report check IDs the remarks explain
	offHire   string   // the off-hire period, "HHMM-HHMM" UTC, if the rules put one
}

// remarkRules are the keyword rules a programmer would write on the remarks,
// from the tuning tasks' wording. withHire is false for a voyage with no
// voyage management system record: no hire to put off.
func remarkRules(rep, prev sim.NoonReport, withHire bool) keywordResult {
	var k keywordResult
	if robCorrectedRe.MatchString(rep.Remarks) {
		k.explained = append(k.explained, "rob_vlsfo", "rob_mgo")
	}
	m := stopRe.FindStringSubmatch(rep.Remarks)
	if m == nil {
		if speedOverSteamingRe.MatchString(rep.Remarks) {
			k.explained = append(k.explained, "speed")
		}
		return k
	}
	from, to := atHHMM(rep.Time, m[1]), atHHMM(rep.Time, m[2])
	st, err := checker.CheckStoppage(prev.Time, rep.Time, from, to, rep.DistanceRun, rep.SpeedKn)
	if err != nil || !st.Consistent {
		// The stop does not explain the figures: the speed flag stands.
		return k
	}
	if speedOverSteamingRe.MatchString(rep.Remarks) {
		k.explained = append(k.explained, "speed")
	}
	if withHire && breakdownRe.MatchString(rep.Remarks) && !chartererOrderRe.MatchString(rep.Remarks) {
		k.offHire = m[1] + "-" + m[2]
	}
	return k
}

// atHHMM is the time HHMM UTC on the day of t.
func atHHMM(t time.Time, hhmm string) time.Time {
	h, _ := time.Parse("1504", hhmm)
	return time.Date(t.Year(), t.Month(), t.Day(), h.Hour(), h.Minute(), 0, 0, time.UTC)
}

func round2Tasks(g *route.Graph, root string, wx *weather.Grid) ([]Task, error) {
	var tasks []Task
	fc, err := forecastTasks(g, root)
	if err != nil {
		return nil, err
	}
	tasks = append(tasks, fc...)
	bt, err := berthTasks(g)
	if err != nil {
		return nil, err
	}
	tasks = append(tasks, bt...)
	st, err := stopTasks(g, wx)
	if err != nil {
		return nil, err
	}
	return append(tasks, st...), nil
}

// told is what the office already told a Master about heavy weather, as
// its sent-messages log holds it.
type told struct {
	at, from, until time.Time
	beaufort        int
	swhM            float64
	run             string
}

func (t told) note(ship string) string {
	return fmt.Sprintf("Office e-mail to the Master of %s sent %s UTC: heavy weather expected on the route from %s to %s UTC, wind up to Beaufort %d, seas up to %.1f m (ECMWF %s). The Master acknowledged.",
		ship, t.at.Format("02 Jan 1504"), t.from.Format("02 Jan 1504"), t.until.Format("02 Jan 1504"), t.beaufort, t.swhM, t.run)
}

type forecastTask struct {
	id, planted string
	file        string
	ship        string // "valda" or "tormar"
	told        *told
	// prepared is a remark in the Master's last noon report that he has the
	// weather in hand.
	prepared string
	heavy    bool // the key: the office should write to the Master
}

func forecastTasks(g *route.Graph, root string) ([]Task, error) {
	day := func(d, h, m int) time.Time { return time.Date(2025, 1, d, h, m, 0, 0, time.UTC) }
	ft := []forecastTask{
		{id: "forecast-gale-valda", planted: "a following gale in Biscay; the office has not written", file: Forecast22, ship: "valda", heavy: true},
		{id: "forecast-storm-tormar", planted: "Storm Éowyn head on; the office has not written", file: Forecast22, ship: "tormar", heavy: true},
		{id: "forecast-told-same-valda", planted: "the same gale the office already told the Master about", file: Forecast22, ship: "valda",
			told: &told{at: day(22, 8, 30), from: day(22, 20, 0), until: day(24, 20, 0), beaufort: 8, swhM: 6.1, run: "22 Jan 00z"}},
		{id: "forecast-master-prepared-tormar", planted: "Storm Éowyn ahead; the Master's report says he has it in hand", file: Forecast22, ship: "tormar",
			prepared: "Weather routing service advises on Storm Eowyn ahead from 23 Jan. Vessel secured for heavy weather; speed will be reduced as necessary. Will advise any change in ETA."},
		{id: "forecast-worse-than-told-tormar", planted: "Storm Éowyn much worse than the office told the Master", file: Forecast22, ship: "tormar", heavy: true,
			told: &told{at: day(22, 8, 30), from: day(23, 4, 0), until: day(26, 3, 0), beaufort: 9, swhM: 6.0, run: "22 Jan 00z"}},
		{id: "forecast-new-low-tormar", planted: "Éowyn easing as told; a new low on 27 and 28 Jan", file: Forecast24, ship: "tormar", heavy: true,
			told: &told{at: day(22, 20, 30), from: day(23, 4, 0), until: day(26, 3, 0), beaufort: 12, swhM: 9.9, run: "22 Jan 12z"}},
	}
	s4, err := sim.Scene4(g)
	if err != nil {
		return nil, err
	}
	s2, err := sim.Scene2Days(g)
	if err != nil {
		return nil, err
	}
	grids := map[string]*weather.Grid{}
	var tasks []Task
	for _, f := range ft {
		if grids[f.file] == nil {
			if grids[f.file], err = weather.Load(root+"/"+f.file, f.file); err != nil {
				return nil, err
			}
		}
		grid := grids[f.file]
		run := checker.ForecastRun(grid)
		var v sim.Voyage
		var available time.Time
		switch {
		case f.file == Forecast24:
			// The 24 Jan 00z run, as the office reads it at noon: TORMAR
			// SPRING's reports to 23 Jan and her AIS track to noon.
			v, available = s2[2].Voyage, day(24, 12, 0)
			v.Track = trackTo(v.Track, available)
		case f.ship == "valda":
			v, available = s4.Fleet[0], s4.Forecast.Available
		default:
			v, available = s4.Fleet[1], s4.Forecast.Available
		}
		v.History = slices.Clone(v.History)
		v.Notes = slices.Clone(v.Notes)
		if f.told != nil {
			v.Notes = append(v.Notes, f.told.note(v.Ship.Name))
		}
		if f.prepared != "" {
			v.History[len(v.History)-1].Remarks = f.prepared
		}
		res, err := aheadOf(g, grid, v, available)
		if err != nil {
			return nil, fmt.Errorf("%s: %w", f.id, err)
		}
		key := Key{Topics: []string{}}
		if f.heavy {
			key.Topics = []string{"heavy_weather"}
		}
		t := Task{ID: f.id, Kind: "forecast", Planted: f.planted, Forecast: f.file, Key: key,
			Episode: sim.Scene{ID: f.id, SimTime: available, Voyage: v, Fleet: []sim.Voyage{v},
				Forecast: &sim.ForecastIssued{Model: "ECMWF IFS", Run: run, Available: available}},
			rules: forecastRules(res, f.told, false), coded: forecastRules(res, f.told, f.prepared != "")}
		tasks = append(tasks, t)
	}
	return tasks, nil
}

// aheadOf is the weather_ahead tool's scan for one ship.
func aheadOf(g *route.Graph, grid *weather.Grid, v sim.Voyage, from time.Time) (checker.WeatherAheadResult, error) {
	last, rep := v.Track[len(v.Track)-1], v.History[len(v.History)-1]
	speed := rep.ETABasisKn
	if speed <= 0 {
		speed = rep.SpeedKn
	}
	rt, err := g.Route([]route.Position{{last.Lon, last.Lat}, {v.To.Lon, v.To.Lat}}, route.Options{Avoid: v.Avoid})
	if err != nil {
		return checker.WeatherAheadResult{}, err
	}
	res, _, err := checker.WeatherAhead(grid, rt.Path, from, speed)
	return res, err
}

// forecastRules writes to the Master when heavy weather lies ahead that the
// office has not already told him about: outside the period told, or worse
// than told. With a coded report, a Master who has reported his heavy
// weather precautions is not written to.
func forecastRules(res checker.WeatherAheadResult, t *told, prepared bool) Decisions {
	d := Decisions{Topics: []string{}}
	if prepared {
		return d
	}
	for _, h := range res.HeavyWeather {
		if t == nil || h.From.Before(t.from) || h.To.After(t.until) || h.MaxBeaufort > t.beaufort || h.MaxSWHm > t.swhM+0.5 {
			d.Topics = []string{"heavy_weather"}
			break
		}
	}
	return d
}

func trackTo(fixes []sim.Fix, t time.Time) []sim.Fix {
	var out []sim.Fix
	for _, f := range fixes {
		if !f.Time.After(t) {
			out = append(out, f)
		}
	}
	return out
}

// berthTimeRe finds a berthing time as the port agent writes it.
var berthTimeRe = regexp.MustCompile(`(\d{1,2}) Jan (\d{4}) UTC`)

// berthDelayHours is the rules' threshold: a berth this much after the ETA
// is worth asking the operator about slowing down for.
const berthDelayHours = 12

func berthTasks(g *route.Graph) ([]Task, error) {
	s5, err := sim.Scene5(g)
	if err != nil {
		return nil, err
	}
	eta := s5.Voyage.VMS.PortCalls[0].ETA
	sign := "\n\nBest regards,\nOperations desk\nMaasport Agencies, Rotterdam\n"
	head := "Dear Operations,\n\nMV VALDA CREST - Rotterdam, Rijnmond Energy Coal Terminal\n\n"
	soon := eta.Add(4 * time.Hour)
	bt := []struct {
		id, planted, subject, body string
		ask                        bool
	}{
		{"berth-later-subject", "berth about 27 Jan 0600, subject to the vessel ahead (Scene 5)", s5.Email.Subject, "", true},
		{"berth-later-confirmed", "berth confirmed for 27 Jan 0600", "MV VALDA CREST - Rotterdam line-up: berth confirmed",
			"The coal berth is occupied by a vessel discharging. Her completion is confirmed for late on 26 Jan, and the terminal has confirmed VALDA CREST's berth for 27 Jan 0600 UTC.\n\nPlease keep us informed of the vessel's ETA.", true},
		{"berth-unchanged", "line-up unchanged, berth on arrival", "MV VALDA CREST - Rotterdam line-up",
			"The line-up is unchanged. The berth will be ready on VALDA CREST's arrival, and the terminal expects to berth her on arrival as planned.\n\nPlease keep us informed of the vessel's ETA.", false},
		{"berth-delay-no-time", "berth occupied, delay of 2 to 3 days, no berthing time yet", "MV VALDA CREST - Rotterdam line-up: delay expected",
			"The coal berth is occupied and the vessel ahead is discharging slowly. We expect a delay of possibly 2 to 3 days, but cannot give a berthing time yet. We will advise one as soon as the terminal confirms the vessel ahead's discharge rate.", false},
		{"berth-later-4h", "berth free 4 h after the ETA", "MV VALDA CREST - Rotterdam line-up",
			fmt.Sprintf("The coal berth will be free from %s UTC, a few hours after the vessel's ETA, once the vessel ahead has sailed.\n\nPlease keep us informed of the vessel's ETA.", soon.Format("2 Jan 1504")), false},
		{"berth-may-shift", "berth about 27 Jan 0600, but she may berth on arrival if the vessel ahead shifts", "MV VALDA CREST - Rotterdam line-up: berth occupied",
			"The coal berth is occupied. The vessel ahead is expected to complete late on 26 Jan, which would give VALDA CREST a berth about 27 Jan 0600 UTC. However, the terminal is considering shifting the vessel ahead to the lay-by berth to finish there; if it does, VALDA CREST will berth on arrival. A decision is expected within 24 hours.\n\nWe will advise at once. Please keep us informed of the vessel's ETA.", false},
	}
	var tasks []Task
	for _, b := range bt {
		sc := s5
		if b.body != "" {
			sc.Email.Subject, sc.Email.Body = b.subject, head+b.body+sign
		}
		key := Key{Topics: []string{}}
		if b.ask {
			key = Key{Topics: []string{"berth_delay"}, Allowed: []string{"eta_notice"}}
		}
		r := berthRules(sc.Email.Body, eta)
		tasks = append(tasks, Task{ID: b.id, Kind: "berth", Planted: b.planted, Episode: sc, Key: key, rules: r, coded: r})
	}
	return tasks, nil
}

// berthRules ask the operator about slowing down when the e-mail gives a
// berthing time at least berthDelayHours after the port call ETA. A port
// agent's e-mail has no coded form, so the coded baseline is the same.
func berthRules(body string, eta time.Time) Decisions {
	d := Decisions{Topics: []string{}}
	for _, m := range berthTimeRe.FindAllStringSubmatch(body, -1) {
		t, err := time.Parse("2 Jan 2006 1504", m[1]+" Jan "+fmt.Sprint(eta.Year())+" "+m[2])
		if err == nil && t.Sub(eta).Hours() >= berthDelayHours {
			d.Topics = []string{"berth_delay"}
		}
	}
	return d
}

func stopTasks(g *route.Graph, wx *weather.Grid) ([]Task, error) {
	s6, err := sim.Scene6(g)
	if err != nil {
		return nil, err
	}
	const fuel = "Average speed over 18.5 h steaming. Fuel used during stoppage: VLSFO 1.1 t (generators and boiler)."
	rt := []reportTask{
		{id: "stop-filters-choked", kind: "engine_stop", planted: "5.5 h stop, fuel filters choked (Scene 6)", sc: s6, explained: []string{"speed"}, offHire: "0610-1140"},
		{id: "stop-lube-oil", kind: "engine_stop", planted: "5.5 h stop for low lube oil pressure, worded without the usual words", explained: []string{"speed"}, offHire: "0610-1140",
			sc: withReport(s6, func(r *sim.NoonReport) {
				r.Remarks = "Main engine stopped 0610-1140 UTC: low lube oil pressure alarm. Cause traced to a blocked strainer; strainer cleaned, pressure normal, engine restarted and passage resumed. " + fuel
			})},
		{id: "stop-figures-disagree", kind: "engine_stop", planted: "5.5 h stop, but the speed over steaming time does not give the distance run",
			sc: withReport(s6, func(r *sim.NoonReport) { r.SpeedKn = 13.5 })},
		{id: "stop-split", kind: "engine_stop", planted: "stop on charterers' orders, then a breakdown on restart: off hire only for the breakdown", explained: []string{"speed"}, offHire: "0900-1140",
			sc: withReport(s6, func(r *sim.NoonReport) {
				r.Remarks = "Main engine stopped 0610-1140 UTC. 0610-0900 drifting on charterers' instructions, awaiting voyage orders. Orders received 0900; on restart the main engine air start valve was found defective, repaired, engine restarted 1140 and passage resumed. Average speed over 18.5 h steaming. Fuel used: VLSFO 0.6 t while drifting 0610-0900, VLSFO 0.5 t during repair 0900-1140 (generators and boiler)."
			})},
		{id: "stop-charterers-orders", kind: "engine_stop", planted: "5.5 h stop drifting on charterers' orders: not off hire", explained: []string{"speed"},
			sc: withReport(s6, func(r *sim.NoonReport) {
				r.Remarks = "Main engine stopped 0610-1140 UTC on charterers' instructions: vessel drifting awaiting voyage orders. Orders received 1140, passage resumed. " + fuel
			})},
		{id: "stop-steering-gear", kind: "engine_stop", planted: "5.5 h stop to repair a steering gear leak", explained: []string{"speed"}, offHire: "0610-1140",
			sc: withReport(s6, func(r *sim.NoonReport) {
				r.Remarks = "Main engine stopped 0610-1140 UTC to repair a hydraulic oil leak on the steering gear. Leak repaired, steering tested, passage resumed. " + fuel
			})},
	}
	var tasks []Task
	for _, r := range rt {
		t, err := reportKey(r, g, wx)
		if err != nil {
			return nil, fmt.Errorf("%s: %w", r.id, err)
		}
		tasks = append(tasks, t)
	}
	return tasks, nil
}
