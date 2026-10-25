package sim

import (
	"fmt"
	"time"

	"github.com/captv89/seaRoute/route"
)

// Scene 2 counterparties.
var (
	s2Charterer = Contact{Role: "charterer", Name: "Laurentian Ore Chartering (chartering desk)", Email: "chartering@laurentian-ore.example", Domain: "laurentian-ore.example"}
	s2Agent     = Contact{Role: "agent", Name: "Baie Agencies, Sept-Iles", Email: "ops@baie-agencies.example", Domain: "baie-agencies.example"}
)

// SeptIles is the loading port of TORMAR SPRING's voyage, from seaRoute's port list.
var SeptIles = Port{Name: "Sept-Iles", Lat: 50.202675418655, Lon: -66.36136067705}

// Scene 2 timing, speeds and fuel. Chosen against the real ECMWF weather on
// the real seaRoute track (Scene 2 weather scan):
// leaving Rotterdam at 0300 UTC on 21 Jan, the day to noon 22 Jan is a good
// weather day in the western Channel, and the day to noon 24 Jan runs into
// Storm Éowyn's head gale west of Ireland.
var s2Departure = time.Date(2025, 1, 21, 3, 0, 0, 0, time.UTC)

const (
	s2PlannedKn    = 12.5   // CP warranted speed; the Master's ETA basis
	s2WarrantedT   = 24.0   // CP warranted VLSFO t/day, ballast, good weather
	s2MGOPerDay    = 0.1    // t/day
	s2ROBVLSFOFull = 1150.0 // on sailing
	s2ROBMGOFull   = 95.0
)

// s2Day is one noon-to-noon day of the voyage, newest last.
type s2Day struct {
	noon    time.Time
	hours   float64 // steaming hours covered by the report
	speedKn float64
	vlsfo   float64 // t used
	remarks string
	// stopFrom and stopTo bound a main engine stop within the day, if any.
	stopFrom, stopTo time.Time
}

func s2Days() []s2Day {
	noon := func(d int) time.Time { return time.Date(2025, 1, d, 12, 0, 0, 0, time.UTC) }
	return []s2Day{
		{noon: noon(21), hours: 9, speedKn: 12.5, vlsfo: 9.0, remarks: "First report after sailing Rotterdam 21 Jan 0300 UTC; figures cover 9 h."},
		// D1: good weather in the Channel, but 15% over the warranty.
		{noon: noon(22), hours: 24, speedKn: 12.5, vlsfo: 27.6},
		{noon: noon(23), hours: 24, speedKn: 12.0, vlsfo: 24.8, remarks: "Wind and sea increasing from W in the afternoon."},
		// D3: Storm Éowyn ahead; speed down, 18% over the warranty.
		{noon: noon(24), hours: 24, speedKn: 9.5, vlsfo: 28.3, remarks: "Heavy weather, strong head wind and very high head sea. Speed reduced for safety."},
	}
}

// Scene2 builds "Consumption is up: explained, then questioned": TORMAR
// SPRING westbound in ballast. It returns two episodes, D1 (22 Jan) and
// D3 (24 Jan), each with the voyage as the desk knows it at that report.
func Scene2(g *route.Graph) ([]Scene, error) {
	all, err := Scene2Days(g)
	if err != nil {
		return nil, err
	}
	return []Scene{all[0], all[2]}, nil // D1 and D3
}

// Scene2Days builds an episode for every noon report after the first: D1
// (22 Jan), D2 (23 Jan) and D3 (24 Jan). The eval uses all three.
func Scene2Days(g *route.Graph) ([]Scene, error) {
	v, reports, noticesBefore, err := tormarVoyage(g, s2Days())
	if err != nil {
		return nil, err
	}
	var scenes []Scene
	for i := 1; i < len(reports); i++ {
		ep := v
		ep.History = append([]NoonReport(nil), reports[:i]...)
		ep.Notices = noticesBefore(i)
		ep.Track = trackUpTo(v.Track, reports[i].Time)
		scenes = append(scenes, Scene{
			ID:      "s2-consumption",
			SimTime: reports[i].Time.Add(5 * time.Minute),
			Voyage:  ep,
			Report:  reports[i],
			Email:   reports[i].Render(ep),
		})
	}
	return scenes, nil
}

// tormarVoyage builds TORMAR SPRING's voyage from Rotterdam over the given
// days: the voyage with its AIS track to the last noon, every noon report,
// and the ETA notices sent before the report with a given index.
func tormarVoyage(g *route.Graph, days []s2Day) (Voyage, []NoonReport, func(int) []Notice, error) {
	avoid := []string{"belle_isle"}
	r, err := g.Route([]route.Position{{Rotterdam.Lon, Rotterdam.Lat}, {SeptIles.Lon, SeptIles.Lat}}, route.Options{Avoid: avoid})
	if err != nil {
		return Voyage{}, nil, nil, err
	}
	track := NewTrack(r.Path)

	// Distance along the track at time t, from the daily speeds.
	along := func(t time.Time) float64 {
		var nm float64
		start := s2Departure
		for _, d := range days {
			if !t.After(start) {
				break
			}
			end := d.noon
			if t.Before(end) {
				end = t
			}
			hours := end.Sub(start).Hours()
			// No way is made while the engine is stopped.
			if !d.stopFrom.IsZero() {
				from, to := d.stopFrom, d.stopTo
				if start.After(from) {
					from = start
				}
				if end.Before(to) {
					to = end
				}
				if to.After(from) {
					hours -= to.Sub(from).Hours()
				}
			}
			nm += hours * d.speedKn
			start = d.noon
		}
		if last := days[len(days)-1].noon; t.After(last) {
			nm += t.Sub(last).Hours() * s2PlannedKn
		}
		return nm * route.MetresPerNM
	}

	v := Voyage{
		Ship: Ship{
			ID: "SIM-0002", Name: "MV TORMAR SPRING", Type: "Panamax bulk carrier, ice class 1C, in ballast",
			DWT: 76000, LOAm: 225, BeamM: 32.26, LppM: 217, EmailLocal: "master@tormarspring.sim-fleet.example",
		},
		From: Rotterdam, To: SeptIles, Cargo: "In ballast, to load iron ore",
		Avoid: avoid,
		Contacts: []Contact{
			{Role: "ship", Name: "Master, MV TORMAR SPRING", Email: "master@tormarspring.sim-fleet.example", Domain: "tormarspring.sim-fleet.example"},
			s2Charterer, s2Agent,
		},
		CP:          DefaultCP(),
		ShipTimeUTC: "UTC",
		// Normal ballast, about 7 m mean draught: ∇ = Cb·Lpp·B·T.
		Loading: &Loading{Ballast: true, DisplacementM3: 39200, Cb: 0.80},
	}
	v.CP.WarrantedSpeedKn = s2PlannedKn
	v.CP.WarrantedVLSFOPerDayT = s2WarrantedT

	// Hourly AIS fixes from sailing to the last report.
	for t := s2Departure; !t.After(days[len(days)-1].noon); t = t.Add(time.Hour) {
		p := track.At(along(t))
		v.Track = append(v.Track, Fix{Time: t, Lat: roundTo(p[1], 1e-4), Lon: roundTo(p[0], 1e-4)})
	}

	toGo := func(t time.Time) float64 { return (track.LengthM() - along(t)) / route.MetresPerNM }
	etaAt := func(t time.Time) time.Time { return etaHour(t, toGo(t), s2PlannedKn) }

	// The ETA notice on sailing, then each day-threshold notice at the
	// first noon report at which it falls before the next report.
	v.Notices = []Notice{{SentAt: s2Departure.Add(time.Hour), DaysLabel: 10, ETA: etaAt(s2Departure)}}
	sent := map[int]bool{10: true}

	robV, robM := s2ROBVLSFOFull, s2ROBMGOFull
	reports := make([]NoonReport, len(days))
	for i, d := range days {
		robV -= d.vlsfo
		robM -= roundTo(s2MGOPerDay*d.hours/24, 0.1)
		p := track.At(along(d.noon))
		rep := NoonReport{
			ID:          fmt.Sprintf("nr-%04d", 101+i),
			Time:        d.noon,
			Lat:         roundMinute(p[1]),
			Lon:         roundMinute(p[0]),
			DistanceRun: roundTo(d.speedKn*d.hours, 0.1),
			SpeedKn:     d.speedKn,
			ROBVLSFO:    roundTo(robV, 0.1),
			ROBMGO:      roundTo(robM, 0.1),
			ConsVLSFO:   d.vlsfo,
			ConsMGO:     roundTo(s2MGOPerDay*d.hours/24, 0.1),
			NextPort:    SeptIles.Name,
			MasterETA:   etaAt(d.noon),
			ETABasisKn:  s2PlannedKn,
			Remarks:     d.remarks,
		}
		reports[i] = rep
	}

	// Notices sent at the reports before each episode.
	noticesBefore := func(upto int) []Notice {
		out := append([]Notice(nil), v.Notices...)
		done := map[int]bool{}
		for k := range sent {
			done[k] = true
		}
		for _, rep := range reports[:upto] {
			left := rep.MasterETA.Sub(rep.Time).Hours() / 24
			for _, nd := range v.CP.NoticeDays {
				if !done[nd] && left-1 < float64(nd) && float64(nd) <= left {
					done[nd] = true
					out = append(out, Notice{SentAt: rep.Time.Add(30 * time.Minute), DaysLabel: nd, ETA: rep.MasterETA})
				}
			}
		}
		return out
	}

	return v, reports, noticesBefore, nil
}

func trackUpTo(fixes []Fix, t time.Time) []Fix {
	var out []Fix
	for _, f := range fixes {
		if !f.Time.After(t) {
			out = append(out, f)
		}
	}
	return out
}
