package checker

import (
	"testing"
	"time"

	"github.com/captv89/seaRoute/route"
	"github.com/captv89/shorewatch/sim"
	"github.com/captv89/shorewatch/tools/weather"
)

// TestWeatherAheadScene4 checks the scan on the real ECMWF 22 Jan 12z run
// against the Scene 4 story: VALDA CREST meets a following gale in Biscay
// about a day ahead; TORMAR SPRING sails into Storm Éowyn head on.
func TestWeatherAheadScene4(t *testing.T) {
	g, err := route.Load("../../third_party/seaRoute/dataset/marnet_plus_5km.geojson")
	if err != nil {
		t.Fatal(err)
	}
	sc, err := sim.Scene4(g)
	if err != nil {
		t.Fatal(err)
	}
	fc, err := weather.Load("../../data/weather/s4-forecast-20250122T12.json", "s4")
	if err != nil {
		t.Fatal(err)
	}
	scan := func(v sim.Voyage, from time.Time) (WeatherAheadResult, error) {
		last := v.Track[len(v.Track)-1]
		r, err := g.Route([]route.Position{{last.Lon, last.Lat}, {v.To.Lon, v.To.Lat}}, route.Options{Avoid: v.Avoid})
		if err != nil {
			t.Fatal(err)
		}
		rep := v.History[len(v.History)-1]
		kn := rep.ETABasisKn
		if kn == 0 {
			kn = rep.SpeedKn
		}
		res, _, err := WeatherAhead(fc, r.Path, from, kn)
		return res, err
	}
	at := sc.Forecast.Available

	valda, err := scan(sc.Fleet[0], at)
	if err != nil {
		t.Fatal(err)
	}
	var gale *HeavyStretch
	for i, h := range valda.HeavyWeather {
		if h.MaxBeaufort >= 8 {
			gale = &valda.HeavyWeather[i]
		}
	}
	if gale == nil || gale.WindSector != "following" || gale.StartsInH < 20 || gale.StartsInH > 30 || gale.MaxSWHm < 6 {
		t.Errorf("VALDA CREST: want a following Beaufort 8 gale with 6 m seas about a day ahead, got %+v", valda.HeavyWeather)
	}
	if !valda.ArrivesInside {
		t.Error("VALDA CREST should reach Rotterdam within the forecast")
	}

	tormar, err := scan(sc.Fleet[1], at)
	if err != nil {
		t.Fatal(err)
	}
	if len(tormar.HeavyWeather) == 0 {
		t.Fatal("TORMAR SPRING: no heavy weather ahead")
	}
	if h := tormar.HeavyWeather[0]; h.StartsInH > 12 || h.WindSector != "head" || h.MaxBeaufort < 10 || h.MaxSWHm < 9 {
		t.Errorf("TORMAR SPRING: want storm-force head wind and 9 m seas within 12 h, got %+v", h)
	}

	// A forecast issued after the scan's start is refused: no hindsight.
	if _, err := scan(sc.Fleet[0], at.Add(-9*time.Hour)); err == nil {
		t.Error("a scan from before the run was issued should be refused")
	}
}
