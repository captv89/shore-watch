package checker

import (
	"fmt"
	"math"
	"time"

	"github.com/captv89/seaRoute/route"
	"github.com/captv89/shorewatch/sim"
	"github.com/captv89/shorewatch/tools/weather"
)

// Heavy weather, for a heads-up: wind of Beaufort 7 (near gale) or more,
// or a sea state of Douglas 6 (very rough, 4 m significant wave height) or more.
const (
	HeavyBeaufort = 7
	HeavyDouglas  = 6
)

// AheadSample is the forecast where the ship is expected to be at one hour,
// sailing the remaining route at a steady speed.
type AheadSample struct {
	Time        time.Time `json:"time"`
	Lat         float64   `json:"lat"`
	Lon         float64   `json:"lon"`
	HeadingDeg  float64   `json:"heading_deg"`
	WindFromDeg float64   `json:"wind_from_deg"`
	WindKn      int       `json:"wind_kn"`
	Beaufort    int       `json:"beaufort"`
	WindRelDeg  float64   `json:"wind_relative_deg"`
	WindSector  string    `json:"wind_sector"`
	SWHm        *float64  `json:"swh_m"`
	Douglas     *int      `json:"douglas"`
	WavesRelDeg *float64  `json:"waves_relative_deg"`
	SeaSector   string    `json:"sea_sector,omitempty"`
	Heavy       bool      `json:"heavy"`
}

// HeavyStretch is a run of consecutive hours of heavy weather on the route.
type HeavyStretch struct {
	From        time.Time `json:"from"`
	To          time.Time `json:"to"`
	StartsInH   int       `json:"starts_in_hours"`
	FromLat     float64   `json:"from_lat"`
	FromLon     float64   `json:"from_lon"`
	ToLat       float64   `json:"to_lat"`
	ToLon       float64   `json:"to_lon"`
	MaxBeaufort int       `json:"max_beaufort"`
	MaxWindKn   int       `json:"max_wind_kn"`
	WindSector  string    `json:"wind_sector"` // where the strongest wind comes from, relative to the heading
	MaxSWHm     float64   `json:"max_swh_m"`
	MaxDouglas  int       `json:"max_douglas"`
	SeaSector   string    `json:"sea_sector,omitempty"` // where the highest sea comes from
}

// WeatherAheadResult is what the forecast holds for the ship's remaining route.
type WeatherAheadResult struct {
	ForecastRun    time.Time      `json:"forecast_run"`
	From           time.Time      `json:"from"`
	FromLat        float64        `json:"from_lat"`
	FromLon        float64        `json:"from_lon"`
	SpeedKn        float64        `json:"speed_kn"`
	DistanceNM     float64        `json:"distance_to_go_nm"`
	Until          time.Time      `json:"until"` // end of the forecast, or arrival
	ArrivesInside  bool           `json:"arrives_within_forecast"`
	MaxBeaufort    int            `json:"max_beaufort"`
	MaxSWHm        float64        `json:"max_swh_m"`
	HeavyWeather   []HeavyStretch `json:"heavy_weather"`
	Threshold      string         `json:"threshold"`
	Interpretation string         `json:"interpretation"`
}

// WeatherAhead sails the remaining route hour by hour from the ship's last
// position at a steady speed, and reads the forecast where she is expected
// to be. Each forecast field stands for the hours until the next one; the
// scan stops when the forecast or the route ends. It reports the stretches
// of heavy weather. It gives no route, speed or ETA advice.
func WeatherAhead(fc *weather.Grid, path []route.Position, from time.Time, kn float64) (WeatherAheadResult, []AheadSample, error) {
	if len(path) < 2 {
		return WeatherAheadResult{}, nil, fmt.Errorf("no route to sail")
	}
	if kn <= 0 {
		return WeatherAheadResult{}, nil, fmt.Errorf("no speed to sail at")
	}
	run := ForecastRun(fc)
	if run.IsZero() || run.After(from) {
		return WeatherAheadResult{}, nil, fmt.Errorf("no forecast issued by %s", from.Format(time.RFC3339))
	}
	tr := sim.NewTrack(path)
	total := tr.LengthM()
	end := fc.Times[len(fc.Times)-1].Add(lastStepHours * time.Hour)
	res := WeatherAheadResult{
		ForecastRun: run, From: from, FromLat: round2(path[0][1]), FromLon: round2(path[0][0]),
		SpeedKn: kn, DistanceNM: round1(total / route.MetresPerNM), HeavyWeather: []HeavyStretch{},
		Threshold: fmt.Sprintf("wind Beaufort %d or more, or sea state Douglas %d (4 m) or more", HeavyBeaufort, HeavyDouglas),
	}
	var samples []AheadSample
	var cur *HeavyStretch
	t, d := from, 0.0
	for ; t.Before(end); t = t.Add(time.Hour) {
		if d > total {
			res.ArrivesInside = true
			break
		}
		ft, ok := fieldAt(fc, t)
		if !ok {
			return WeatherAheadResult{}, nil, fmt.Errorf("no forecast field at %s", t.Format(time.RFC3339))
		}
		p := tr.At(d)
		ahead := tr.At(math.Min(d+route.MetresPerNM, total))
		hdg := weather.Bearing(p[1], p[0], ahead[1], ahead[0])
		w, err := fc.At(ft, p[1], p[0])
		if err != nil {
			return WeatherAheadResult{}, nil, err
		}
		knots := int(math.Round(w.WindMS * weather.KnotsPerMS))
		s := AheadSample{Time: t, Lat: round2(p[1]), Lon: round2(p[0]), HeadingDeg: math.Round(hdg),
			WindFromDeg: math.Round(w.WindFromDeg), WindKn: knots, Beaufort: weather.Beaufort(knots)}
		s.WindRelDeg = math.Round(weather.Relative(w.WindFromDeg, hdg))
		s.WindSector = weather.Sector(s.WindRelDeg)
		if w.SWH != nil {
			h := round1(*w.SWH)
			dg := weather.Douglas(*w.SWH)
			rel := math.Round(weather.Relative(*w.WavesFrom, hdg))
			s.SWHm, s.Douglas, s.WavesRelDeg, s.SeaSector = &h, &dg, &rel, weather.Sector(rel)
			res.MaxSWHm = math.Max(res.MaxSWHm, h)
		}
		res.MaxBeaufort = max(res.MaxBeaufort, s.Beaufort)
		s.Heavy = s.Beaufort >= HeavyBeaufort || (s.Douglas != nil && *s.Douglas >= HeavyDouglas)
		samples = append(samples, s)

		switch {
		case s.Heavy && cur == nil:
			cur = &HeavyStretch{From: t, StartsInH: int(t.Sub(from).Hours()), FromLat: s.Lat, FromLon: s.Lon}
			fallthrough
		case s.Heavy:
			cur.To, cur.ToLat, cur.ToLon = t.Add(time.Hour), s.Lat, s.Lon
			if s.Beaufort > cur.MaxBeaufort {
				cur.MaxBeaufort, cur.MaxWindKn, cur.WindSector = s.Beaufort, s.WindKn, s.WindSector
			}
			if s.SWHm != nil && *s.SWHm > cur.MaxSWHm {
				cur.MaxSWHm, cur.MaxDouglas, cur.SeaSector = *s.SWHm, *s.Douglas, s.SeaSector
			}
		case cur != nil:
			res.HeavyWeather = append(res.HeavyWeather, *cur)
			cur = nil
		}
		d += kn * route.MetresPerNM
	}
	if cur != nil {
		res.HeavyWeather = append(res.HeavyWeather, *cur)
	}
	res.Until = t
	res.Interpretation = "No heavy weather on the remaining route while the forecast lasts."
	if n := len(res.HeavyWeather); n > 0 {
		first := res.HeavyWeather[0]
		res.Interpretation = fmt.Sprintf("Heavy weather on the remaining route: %d stretch(es), the first from %s UTC, wind up to Beaufort %d. The Master decides how to sail; this is not weather routing.",
			n, first.From.Format("02 Jan 1504"), res.MaxBeaufort)
	}
	return res, samples, nil
}
