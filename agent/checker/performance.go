package checker

import (
	"fmt"
	"strings"

	"github.com/captv89/shorewatch/sim"
)

// AboutSpeedKn and AboutConsumptionPct are the margins English law reads
// into "about" in a speed and consumption warranty: 0.5 kn off the speed,
// 5% on top of the consumption (The "Al Bida" [1987] 1 Lloyd's Rep 124).
// A tribunal may allow other margins; these are the usual ones.
const (
	AboutSpeedKn        = 0.5
	AboutConsumptionPct = 5.0
)

// ConsumptionResult compares a day's fuel and speed with the CP warranty
// of about this speed on about this much fuel, "about" both times.
type ConsumptionResult struct {
	ReportedT  float64 `json:"reported_vlsfo_t"`
	WarrantedT float64 `json:"warranted_vlsfo_t"`
	// ExcessPct is against the warranted figure itself.
	ExcessPct    float64 `json:"excess_pct"`
	TolerancePct float64 `json:"tolerance_pct"`
	// AllowedT is the warranted figure with the "about" margin; a claim
	// starts from it.
	AllowedT       float64 `json:"allowed_vlsfo_t"`
	OverAllowedT   float64 `json:"over_allowed_t"`
	OverAllowedPct float64 `json:"over_allowed_pct"`
	FuelFlagged    bool    `json:"fuel_flagged"`
	// Speed, when the report and the CP both give one.
	ReportedKn     float64 `json:"reported_speed_kn,omitempty"`
	WarrantedKn    float64 `json:"warranted_speed_kn,omitempty"`
	MinSpeedKn     float64 `json:"min_speed_kn,omitempty"`
	SpeedFlagged   bool    `json:"speed_flagged"`
	Flagged        bool    `json:"flagged"`
	Interpretation string  `json:"interpretation"`
}

// CheckConsumption compares the VLSFO used and the speed made in the last
// 24 h with the CP warranty, allowing the "about" margins. The warranty
// holds on good weather days only; classify_weather_day settles that.
func CheckConsumption(reportedT, warrantedT, reportedKn, warrantedKn float64) (ConsumptionResult, error) {
	if warrantedT <= 0 {
		return ConsumptionResult{}, fmt.Errorf("the CP gives no warranted consumption")
	}
	allowed := round1(warrantedT * (1 + AboutConsumptionPct/100))
	r := ConsumptionResult{
		ReportedT: reportedT, WarrantedT: warrantedT, TolerancePct: AboutConsumptionPct,
		ExcessPct: round1((reportedT - warrantedT) / warrantedT * 100),
		AllowedT:  allowed,
	}
	if reportedT > allowed {
		r.FuelFlagged = true
		r.OverAllowedT = round1(reportedT - allowed)
		r.OverAllowedPct = round1((reportedT - allowed) / allowed * 100)
	}
	if reportedKn > 0 && warrantedKn > 0 {
		r.ReportedKn, r.WarrantedKn, r.MinSpeedKn = reportedKn, warrantedKn, round1(warrantedKn-AboutSpeedKn)
		r.SpeedFlagged = reportedKn < r.MinSpeedKn
	}
	r.Flagged = r.FuelFlagged || r.SpeedFlagged

	var parts []string
	if r.FuelFlagged {
		parts = append(parts, fmt.Sprintf("Consumption is %.1f t, %+.1f%% on the CP warranted %.1f t and %.1f t (%.1f%%) over the %.1f t that \"about\" allows (+%.0f%%).",
			reportedT, r.ExcessPct, warrantedT, r.OverAllowedT, r.OverAllowedPct, allowed, AboutConsumptionPct))
	} else {
		parts = append(parts, fmt.Sprintf("Consumption is %.1f t, within the %.1f t that \"about\" allows on the CP warranted %.1f t (+%.0f%%).", reportedT, allowed, warrantedT, AboutConsumptionPct))
	}
	if r.SpeedFlagged {
		parts = append(parts, fmt.Sprintf("Speed %.1f kn is below the %.1f kn that \"about\" allows on the CP warranted %.1f kn (-%.1f kn).", reportedKn, r.MinSpeedKn, warrantedKn, AboutSpeedKn))
	} else if r.MinSpeedKn > 0 {
		parts = append(parts, fmt.Sprintf("Speed %.1f kn is within the warranty (at least %.1f kn).", reportedKn, r.MinSpeedKn))
	}
	if r.Flagged {
		parts = append(parts, "The warranty holds on good weather days only. Look for a cause before accepting it.")
	}
	r.Interpretation = strings.Join(parts, " ")
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
