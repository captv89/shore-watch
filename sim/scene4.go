package sim

import (
	"fmt"
	"time"

	"github.com/captv89/seaRoute/route"
)

// Scene 4 ("Heavy weather ahead"): ECMWF's 22 Jan 12z run
// is out at 2000 UTC (ECMWF open data dissemination schedule). No
// e-mail starts it. The desk scans its two ships at sea as it knows them
// then: VALDA CREST eight hours after her Scene 1 report, TORMAR SPRING
// after her first full day out of Rotterdam (Scene 2's D1).
var (
	s4Run       = time.Date(2025, 1, 22, 12, 0, 0, 0, time.UTC)
	s4Available = time.Date(2025, 1, 22, 20, 0, 0, 0, time.UTC)
)

// Scene4 builds the fleet scan. Each ship's history, notices and AIS track
// stop at the trigger: no hindsight.
func Scene4(g *route.Graph) (Scene, error) {
	s1, err := Scene1(g)
	if err != nil {
		return Scene{}, err
	}
	valda := s1.Voyage
	rep := s1.Report
	valda.History = append(append([]NoonReport(nil), valda.History...), rep)
	// Scene 1's 3-day notice went out after the noon report.
	valda.Notices = append(append([]Notice(nil), valda.Notices...), Notice{SentAt: rep.Time.Add(30 * time.Minute), DaysLabel: 3, ETA: rep.MasterETA})
	r, err := g.Route([]route.Position{{rep.Lon, rep.Lat}, {valda.To.Lon, valda.To.Lat}}, route.Options{})
	if err != nil {
		return Scene{}, err
	}
	tr := NewTrack(r.Path)
	for t := rep.Time; !t.After(s4Available); t = t.Add(time.Hour) {
		p := tr.At(t.Sub(rep.Time).Hours() * rep.SpeedKn * route.MetresPerNM)
		valda.Track = append(valda.Track, Fix{Time: t, Lat: roundTo(p[1], 1e-4), Lon: roundTo(p[0], 1e-4)})
	}

	s2, err := Scene2Days(g)
	if err != nil {
		return Scene{}, err
	}
	d1 := s2[0] // the report of 22 Jan
	tormar := d1.Voyage
	tormar.History = append(append([]NoonReport(nil), tormar.History...), d1.Report)
	// D1's track stops at its noon report; D3's runs on past the trigger.
	tormar.Track = trackUpTo(s2[len(s2)-1].Voyage.Track, s4Available)
	if last := tormar.Track[len(tormar.Track)-1]; !last.Time.Equal(s4Available) {
		return Scene{}, fmt.Errorf("TORMAR SPRING's track ends at %s, before the trigger", last.Time)
	}

	return Scene{
		ID:      "s4-weather",
		SimTime: s4Available,
		Voyage:  valda,
		Fleet:   []Voyage{valda, tormar},
		Forecast: &ForecastIssued{
			Model: "ECMWF IFS", Run: s4Run, Available: s4Available,
			NotScanned: []string{"MV IVELLE STAR (SIM-0003): no AIS track in this demo"},
		},
	}, nil
}
