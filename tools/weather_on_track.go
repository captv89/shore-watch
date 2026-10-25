package tools

import (
	"fmt"
	"math"
	"time"

	"github.com/captv89/shorewatch/sim"
	"github.com/captv89/shorewatch/tools/weather"
)

type weatherIn struct {
	ShipID  string `json:"ship_id" jsonschema:"ship ID, e.g. SIM-0002"`
	FromUTC string `json:"from_utc" jsonschema:"start of the period, RFC 3339, e.g. 2025-01-21T12:00:00Z"`
	ToUTC   string `json:"to_utc" jsonschema:"end of the period, RFC 3339"`
}

// Sample is the weather where the ship was at one time.
type Sample struct {
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
}

// WeatherSummary is what the agent reads: the period's extremes, and how
// long strong wind and high sea came from ahead.
type WeatherSummary struct {
	ShipID         string    `json:"ship_id"`
	From           time.Time `json:"from"`
	To             time.Time `json:"to"`
	Samples        int       `json:"samples"`
	SamplesWithSea int       `json:"samples_with_sea_state"`
	HoursPerSample int       `json:"hours_per_sample"`
	MinBeaufort    int       `json:"min_beaufort"`
	MaxBeaufort    int       `json:"max_beaufort"`
	MaxWindKn      int       `json:"max_wind_kn"`
	MinDouglas     int       `json:"min_douglas"`
	MaxDouglas     int       `json:"max_douglas"`
	MinSWHm        float64   `json:"min_swh_m"`
	MaxSWHm        float64   `json:"max_swh_m"`
	// Hours with Beaufort 7 or more, and with Douglas 5 or more, within 60° of the bow.
	HoursWindBf7AheadOrMore int    `json:"hours_wind_bf7_or_more_ahead"`
	HoursSeaD5AheadOrMore   int    `json:"hours_sea_douglas5_or_more_ahead"`
	Source                  string `json:"source"`
}

// WeatherOut is the tool's result. Samples and Grid go into the event log
// for the chart; the agent is shown only the summary.
type WeatherOut struct {
	Summary WeatherSummary `json:"summary"`
	Samples []Sample       `json:"samples"`
	Grid    weather.Ref    `json:"grid_ref"`
}

const sampleHours = 3

// WeatherOnTrack samples the grid where the ship was, every 3 hours over
// [from, to): each sample stands for the 3 hours that follow it.
func WeatherOnTrack(g *weather.Grid, v sim.Voyage, from, to time.Time) (WeatherOut, error) {
	if !to.After(from) {
		return WeatherOut{}, fmt.Errorf("the period must end after it starts")
	}
	if len(v.Track) < 2 {
		return WeatherOut{}, fmt.Errorf("no AIS track for %s", v.Ship.ID)
	}
	fix := map[time.Time]sim.Fix{}
	for _, f := range v.Track {
		fix[f.Time] = f
	}
	out := WeatherOut{Grid: g.Ref, Samples: []Sample{}}
	s := &out.Summary
	*s = WeatherSummary{ShipID: v.Ship.ID, From: from, To: to, HoursPerSample: sampleHours, MinBeaufort: 99, MinDouglas: 99, MinSWHm: math.Inf(1), Source: "ECMWF IFS open data (CC BY 4.0), forecasts as issued, at most 3 h lead time"}
	for t := from; t.Before(to); t = t.Add(sampleHours * time.Hour) {
		here, ok := fix[t]
		if !ok {
			return WeatherOut{}, fmt.Errorf("no AIS position for %s at %s", v.Ship.ID, t.Format(time.RFC3339))
		}
		// Heading over the next hour, or the last hour at the track's end.
		a, b := here, fix[t.Add(time.Hour)]
		if b.Time.IsZero() {
			a, b = fix[t.Add(-time.Hour)], here
		}
		hdg := weather.Bearing(a.Lat, a.Lon, b.Lat, b.Lon)
		p, err := g.At(t, here.Lat, here.Lon)
		if err != nil {
			return WeatherOut{}, err
		}
		kn := int(math.Round(p.WindMS * weather.KnotsPerMS))
		smp := Sample{
			Time: t, Lat: here.Lat, Lon: here.Lon, HeadingDeg: math.Round(hdg),
			WindFromDeg: math.Round(p.WindFromDeg), WindKn: kn, Beaufort: weather.Beaufort(kn),
		}
		smp.WindRelDeg = math.Round(weather.Relative(p.WindFromDeg, hdg))
		smp.WindSector = weather.Sector(smp.WindRelDeg)
		if p.SWH != nil {
			h := math.Round(*p.SWH*10) / 10
			d := weather.Douglas(*p.SWH)
			rel := math.Round(weather.Relative(*p.WavesFrom, hdg))
			smp.SWHm, smp.Douglas, smp.WavesRelDeg, smp.SeaSector = &h, &d, &rel, weather.Sector(rel)
			s.SamplesWithSea++
			s.MinDouglas, s.MaxDouglas = min(s.MinDouglas, d), max(s.MaxDouglas, d)
			s.MinSWHm, s.MaxSWHm = math.Min(s.MinSWHm, h), math.Max(s.MaxSWHm, h)
			if d >= 5 && rel <= 60 {
				s.HoursSeaD5AheadOrMore += sampleHours
			}
		}
		s.MinBeaufort, s.MaxBeaufort = min(s.MinBeaufort, smp.Beaufort), max(s.MaxBeaufort, smp.Beaufort)
		s.MaxWindKn = max(s.MaxWindKn, kn)
		if smp.Beaufort >= 7 && smp.WindRelDeg <= 60 {
			s.HoursWindBf7AheadOrMore += sampleHours
		}
		out.Samples = append(out.Samples, smp)
	}
	s.Samples = len(out.Samples)
	if math.IsInf(s.MinSWHm, 1) {
		s.MinSWHm, s.MinDouglas = 0, 0
	}
	return out, nil
}
