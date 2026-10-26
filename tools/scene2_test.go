package tools

import (
	"sync"
	"testing"
	"time"

	"github.com/captv89/seaRoute/route"
	"github.com/captv89/shorewatch/agent/checker"
	"github.com/captv89/shorewatch/sim"
	"github.com/captv89/shorewatch/tools/weather"
)

var (
	graphOnce sync.Once
	graph     *route.Graph
	graphErr  error
)

func testGraph(t *testing.T) *route.Graph {
	t.Helper()
	graphOnce.Do(func() { graph, graphErr = route.Load("../third_party/seaRoute/dataset/marnet_plus_5km.geojson") })
	if graphErr != nil {
		t.Fatal(graphErr)
	}
	return graph
}

// TestScene2Story checks the facts Scene 2's story depends on, on the real
// track and the real ECMWF data: D1 is a good weather day with consumption
// flagged; D3 is a head gale, flagged and excluded; the ETA is consistent
// and no ETA notice is due on either day.
func TestScene2Story(t *testing.T) {
	g := testGraph(t)
	eps, err := sim.Scene2(g)
	if err != nil {
		t.Fatal(err)
	}
	if len(eps) != 2 {
		t.Fatalf("%d episodes, want 2", len(eps))
	}
	wx, err := weather.Load("../data/weather/s2-tormar-spring.json", "data/weather/s2-tormar-spring.json")
	if err != nil {
		t.Fatal(err)
	}
	fc, err := weather.Load("../data/weather/s2-forecast-20250124T00.json", "data/weather/s2-forecast-20250124T00.json")
	if err != nil {
		t.Fatal(err)
	}
	for i, ep := range eps {
		rep := ep.Report
		t.Logf("\n%s", ep.Email.Body)
		r, err := g.Route([]route.Position{{rep.Lon, rep.Lat}, {sim.SeptIles.Lon, sim.SeptIles.Lat}}, route.Options{Avoid: ep.Voyage.Avoid})
		if err != nil {
			t.Fatal(err)
		}
		eta, err := checker.CheckETA(rep.Time, rep.MasterETA, r.DistanceNM(), rep.ETABasisKn)
		if err != nil {
			t.Fatal(err)
		}
		if !eta.Consistent {
			t.Errorf("episode %d: ETA not consistent: %+v", i, eta)
		}
		n := checker.CheckNotices(rep.Time, rep.MasterETA, ep.Voyage.CP, ep.Voyage.Notices)
		if n.Due {
			t.Errorf("episode %d: notice due: %v", i, n.Reasons)
		}
		cons, err := checker.CheckConsumption(rep.ConsVLSFO, ep.Voyage.CP.WarrantedVLSFOPerDayT, rep.SpeedKn, ep.Voyage.CP.WarrantedSpeedKn)
		if err != nil {
			t.Fatal(err)
		}
		if !cons.Flagged {
			t.Errorf("episode %d: consumption %.1f%% not flagged", i, cons.ExcessPct)
		}
		w, err := WeatherOnTrack(wx, ep.Voyage, rep.Time.Add(-24*time.Hour), rep.Time)
		if err != nil {
			t.Fatal(err)
		}
		s := w.Summary
		t.Logf("episode %d: %.1f nm to go, ETA diff %+.1f h, %.1f days to ETA, consumption %+.1f%%; weather %+v", i, r.DistanceNM(), eta.DifferenceH, n.DaysToETA, cons.ExcessPct, s)
		lo := ep.Voyage.Loading
		hull := checker.Hull{LppM: ep.Voyage.Ship.LppM, Cb: lo.Cb, DisplacementM3: lo.DisplacementM3, Ballast: lo.Ballast}
		fe, _, ferr := checker.ForecastETA(fc, r.Path, rep.Time, rep.MasterETA, rep.ETABasisKn, ep.Voyage.CP.ETAChangeHours, hull)
		day := checker.ClassifyWeatherDay(checker.WeatherDay{Hours: s.Samples * s.HoursPerSample, Samples: s.Samples, SamplesWithSea: s.SamplesWithSea, MaxBeaufort: s.MaxBeaufort, MaxDouglas: s.MaxDouglas}, ep.Voyage.CP)
		switch i {
		case 0:
			if !day.GoodWeatherDay {
				t.Errorf("D1 is not a good weather day: %s", day.Interpretation)
			}
			if ferr == nil {
				t.Error("D1 must not use the 24 Jan forecast: it was not issued yet")
			}
		case 1:
			if !day.Classified || day.GoodWeatherDay {
				t.Errorf("D3 should be a bad weather day: %s", day.Interpretation)
			}
			if s.MaxBeaufort < 8 || s.HoursWindBf7AheadOrMore < 9 || s.HoursSeaD5AheadOrMore < 18 {
				t.Errorf("D3 is not a head gale: %+v", s)
			}
			// The forecast ETA is past the CP's 12 h, with the forecast
			// covering nearly all of the passage.
			t.Logf("D3 forecast ETA: %+v", fe)
			if ferr != nil || !fe.AtRisk || fe.HoursWithoutForecast > 6 {
				t.Errorf("D3 forecast ETA should be at risk: %v %+v", ferr, fe)
			}
		}
	}
}
