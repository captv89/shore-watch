package checker

import (
	"fmt"
	"math"
	"time"

	"github.com/captv89/seaRoute/route"
	"github.com/captv89/shorewatch/sim"
	"github.com/captv89/shorewatch/tools/weather"
)

// ForecastSample is the forecast where the ship is expected to be at one
// time, and the speed the model gives her there.
type ForecastSample struct {
	Time        time.Time `json:"time"`
	Lat         float64   `json:"lat"`
	Lon         float64   `json:"lon"`
	HeadingDeg  float64   `json:"heading_deg"`
	WindFromDeg float64   `json:"wind_from_deg"`
	WindKn      int       `json:"wind_kn"`
	Beaufort    int       `json:"beaufort"`
	WindRelDeg  float64   `json:"wind_relative_deg"`
	Sector      string    `json:"kwon_sector"`
	LossPct     float64   `json:"speed_loss_pct"`
	SpeedKn     float64   `json:"speed_kn"`
	Forecast    bool      `json:"forecast"` // false once the forecast has run out
}

// ForecastETAResult compares the Master's ETA with the ETA the forecast
// gives through Kwon's speed-loss model.
type ForecastETAResult struct {
	ForecastRun          time.Time `json:"forecast_run"`
	PlannedSpeedKn       float64   `json:"planned_speed_kn"`
	DistanceNM           float64   `json:"distance_to_go_nm"`
	MasterETA            time.Time `json:"master_eta"`
	ForecastETA          time.Time `json:"forecast_eta"`
	DifferenceH          float64   `json:"difference_hours"` // forecast minus Master
	LimitH               float64   `json:"limit_hours"`
	AtRisk               bool      `json:"at_risk"`
	MeanSpeedKn          float64   `json:"mean_speed_kn"`
	HoursLossOver10Pct   int       `json:"hours_speed_loss_over_10pct"`
	MaxBeaufort          int       `json:"max_beaufort"`
	HoursBeyondModel     int       `json:"hours_wind_above_model_range"`
	HoursWithoutForecast int       `json:"hours_without_forecast"`
	Model                string    `json:"model"`
	Interpretation       string    `json:"interpretation"`
}

// lastStepHours is how long the forecast's last field is held: the
// run's step there.
const lastStepHours = 6

// ForecastETA sails the remaining route hour by hour from the report
// position at the planned speed less Kwon's loss in the forecast wind, and
// compares the arrival with the Master's ETA. Each forecast field stands
// for the hours until the next one. Once the forecast runs out the ship is
// given her planned speed. The limit is the CP's ETA change limit: beyond
// it the charterer would need a revised notice.
func ForecastETA(fc *weather.Grid, path []route.Position, from, masterETA time.Time, plannedKn, limitH float64, h Hull) (ForecastETAResult, []ForecastSample, error) {
	if len(path) < 2 {
		return ForecastETAResult{}, nil, fmt.Errorf("no route to sail")
	}
	if run := ForecastRun(fc); run.IsZero() || run.After(from) {
		return ForecastETAResult{}, nil, fmt.Errorf("no forecast issued by %s", from.Format(time.RFC3339))
	}
	tr := sim.NewTrack(path)
	total := tr.LengthM()
	end := fc.Times[len(fc.Times)-1].Add(lastStepHours * time.Hour)
	res := ForecastETAResult{
		ForecastRun: ForecastRun(fc), PlannedSpeedKn: plannedKn, DistanceNM: round1(total / route.MetresPerNM),
		MasterETA: masterETA, LimitH: limitH,
		Model: "Kwon (2008) speed loss, simplified, for learning; not a ship-specific performance model",
	}
	var samples []ForecastSample
	t, d := from, 0.0
	for d < total {
		p := tr.At(d)
		ahead := tr.At(d + route.MetresPerNM)
		hdg := weather.Bearing(p[1], p[0], ahead[1], ahead[0])
		s := ForecastSample{Time: t, Lat: round2(p[1]), Lon: round2(p[0]), HeadingDeg: math.Round(hdg), SpeedKn: plannedKn}
		if ft, ok := fieldAt(fc, t); ok && t.Before(end) {
			w, err := fc.At(ft, p[1], p[0])
			if err != nil {
				return ForecastETAResult{}, nil, err
			}
			kn := int(math.Round(w.WindMS * weather.KnotsPerMS))
			s.Forecast, s.WindFromDeg, s.WindKn, s.Beaufort = true, math.Round(w.WindFromDeg), kn, weather.Beaufort(kn)
			s.WindRelDeg = math.Round(weather.Relative(w.WindFromDeg, hdg))
			loss, err := KwonLoss(h, plannedKn, s.Beaufort, s.WindRelDeg)
			if err != nil {
				return ForecastETAResult{}, nil, err
			}
			s.Sector, s.LossPct, s.SpeedKn = loss.Sector, round1(loss.Pct), plannedKn*(1-loss.Pct/100)
			res.MaxBeaufort = max(res.MaxBeaufort, s.Beaufort)
			if loss.BeyondModel {
				res.HoursBeyondModel++
			}
			if loss.Pct > 10 {
				res.HoursLossOver10Pct++
			}
		} else {
			res.HoursWithoutForecast++
		}
		// One hour, or the part of it needed to arrive.
		step := s.SpeedKn * route.MetresPerNM
		frac := math.Min(1, (total-d)/step)
		d += step * frac
		t = t.Add(time.Duration(frac * float64(time.Hour)))
		s.SpeedKn = round1(s.SpeedKn)
		samples = append(samples, s)
	}
	res.ForecastETA = t.Truncate(time.Minute)
	res.MeanSpeedKn = round1(total / route.MetresPerNM / t.Sub(from).Hours())
	res.DifferenceH = round1(res.ForecastETA.Sub(masterETA).Hours())
	res.AtRisk = res.DifferenceH > limitH
	if res.AtRisk {
		res.Interpretation = fmt.Sprintf("The forecast puts arrival %.0f h after the Master's ETA, more than the CP's %.0f h. The Master owns the ETA: ask the Master to review it in view of the forecast, and tell the operator.", res.DifferenceH, limitH)
	} else {
		res.Interpretation = fmt.Sprintf("The forecast does not put the Master's ETA at risk: within the CP's %.0f h.", limitH)
	}
	return res, samples, nil
}

// fieldAt returns the time of the forecast field in force at t: the latest
// at or before it.
func fieldAt(fc *weather.Grid, t time.Time) (time.Time, bool) {
	var ft time.Time
	for _, gt := range fc.Times {
		if !gt.After(t) {
			ft = gt
		}
	}
	return ft, !ft.IsZero()
}

// ForecastRun reads the run time from the grid's source line, e.g.
// "... forecast run 2025-01-24 00z ...". It is zero for a grid that is not
// one forecast run.
func ForecastRun(fc *weather.Grid) time.Time {
	var y, mo, d, h int
	for i := range fc.Source {
		if n, _ := fmt.Sscanf(fc.Source[i:], "forecast run %d-%d-%d %dz", &y, &mo, &d, &h); n == 4 {
			return time.Date(y, time.Month(mo), d, h, 0, 0, 0, time.UTC)
		}
	}
	return time.Time{}
}
