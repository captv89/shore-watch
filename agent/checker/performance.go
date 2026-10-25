package checker

import (
	"fmt"

	"github.com/captv89/shorewatch/sim"
)

// ConsumptionTolerancePct is how far a day's fuel may run over the CP
// warranted figure before the desk looks into it. Fictional company rule.
const ConsumptionTolerancePct = 10.0

// ConsumptionResult compares a day's fuel with the CP warranty.
type ConsumptionResult struct {
	ReportedT      float64 `json:"reported_vlsfo_t"`
	WarrantedT     float64 `json:"warranted_vlsfo_t"`
	ExcessPct      float64 `json:"excess_pct"`
	TolerancePct   float64 `json:"tolerance_pct"`
	Flagged        bool    `json:"flagged"`
	Interpretation string  `json:"interpretation"`
}

// CheckConsumption compares the VLSFO used in the last 24 h with the CP
// warranted daily figure.
func CheckConsumption(reportedT, warrantedT float64) (ConsumptionResult, error) {
	if warrantedT <= 0 {
		return ConsumptionResult{}, fmt.Errorf("the CP gives no warranted consumption")
	}
	r := ConsumptionResult{
		ReportedT: reportedT, WarrantedT: warrantedT, TolerancePct: ConsumptionTolerancePct,
		ExcessPct: round1((reportedT - warrantedT) / warrantedT * 100),
	}
	r.Flagged = r.ExcessPct > ConsumptionTolerancePct
	if r.Flagged {
		r.Interpretation = fmt.Sprintf("Consumption is %.1f%% above the CP warranted figure, more than the %.0f%% the desk accepts. Look for a cause before accepting it.", r.ExcessPct, ConsumptionTolerancePct)
	} else {
		r.Interpretation = fmt.Sprintf("Consumption is within %.0f%% of the CP warranted figure.", ConsumptionTolerancePct)
	}
	return r, nil
}

// WeatherDay is the weather over one report day, from weather_on_track.
type WeatherDay struct {
	Hours          int `json:"hours"`
	Samples        int `json:"samples"`
	SamplesWithSea int `json:"samples_with_sea_state"`
	MaxBeaufort    int `json:"max_beaufort"`
	MaxDouglas     int `json:"max_douglas"`
}

// WeatherDayResult says whether a report day is a CP good weather day.
type WeatherDayResult struct {
	WeatherDay
	LimitBeaufort  int    `json:"limit_beaufort"`
	LimitDouglas   int    `json:"limit_douglas"`
	Classified     bool   `json:"classified"`
	GoodWeatherDay bool   `json:"good_weather_day"`
	Interpretation string `json:"interpretation"`
}

// ClassifyWeatherDay applies the CP good weather definition to a full
// noon-to-noon day: wind and sea must stay within the limits throughout.
func ClassifyWeatherDay(w WeatherDay, cp sim.CPTerms) WeatherDayResult {
	r := WeatherDayResult{WeatherDay: w, LimitBeaufort: cp.GoodWeatherMaxBeaufort, LimitDouglas: cp.GoodWeatherMaxDouglas}
	switch {
	case w.Hours < 24:
		r.Interpretation = fmt.Sprintf("Cannot classify: the weather covers %d h, not the full day.", w.Hours)
		return r
	case w.SamplesWithSea < w.Samples:
		r.Interpretation = "Cannot classify: the wave model has no sea state for part of the day."
		return r
	}
	r.Classified = true
	r.GoodWeatherDay = w.MaxBeaufort <= cp.GoodWeatherMaxBeaufort && w.MaxDouglas <= cp.GoodWeatherMaxDouglas
	if r.GoodWeatherDay {
		r.Interpretation = fmt.Sprintf("Good weather day under the CP: wind at most Beaufort %d and sea state at most %d (limits %d and %d). The day counts for the speed and consumption warranty, so weather does not explain extra fuel.",
			w.MaxBeaufort, w.MaxDouglas, cp.GoodWeatherMaxBeaufort, cp.GoodWeatherMaxDouglas)
	} else {
		r.Interpretation = fmt.Sprintf("Not a good weather day under the CP: wind reached Beaufort %d and sea state %d (limits %d and %d). The day is excluded from the speed and consumption assessment.",
			w.MaxBeaufort, w.MaxDouglas, cp.GoodWeatherMaxBeaufort, cp.GoodWeatherMaxDouglas)
	}
	return r
}
