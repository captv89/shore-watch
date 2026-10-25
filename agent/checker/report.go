package checker

import (
	"fmt"
	"math"
	"time"

	"github.com/captv89/seaRoute/route"
)

// Noon report consistency rules. Fictional company rules,
// agreed with Vishnu; the position tolerances are for him to check.
const (
	ROBToleranceT       = 1.0 // tonnes, or ROBTolerancePct of the day's consumption if larger
	ROBTolerancePct     = 2.0
	SpeedToleranceKn    = 0.3  // reported speed against distance run over the time between reports
	MaxPlausibleSpeedKn = 16.5 // these ship types, laden or in ballast
	PositionOverPct     = 5.0  // straight line from the last noon position longer than the distance run by more than this, plus PositionSlackNM
	PositionUnderPct    = 15.0 // or shorter by more than this, plus PositionSlackNM
	PositionSlackNM     = 5.0
)

// ROBResult checks that remaining fuel follows from yesterday's figure and
// the day's consumption.
type ROBResult struct {
	Fuel           string  `json:"fuel"`
	PrevROBT       float64 `json:"previous_rob_t"`
	ConsumptionT   float64 `json:"consumption_t"`
	ExpectedROBT   float64 `json:"expected_rob_t"`
	ReportedROBT   float64 `json:"reported_rob_t"`
	GapT           float64 `json:"gap_t"` // expected minus reported; negative means more fuel on board than expected
	ToleranceT     float64 `json:"tolerance_t"`
	Flagged        bool    `json:"flagged"`
	Interpretation string  `json:"interpretation"`
}

// CheckROB compares today's ROB with the previous ROB less the day's consumption.
func CheckROB(fuel string, prevROB, consumption, rob float64) ROBResult {
	r := ROBResult{
		Fuel: fuel, PrevROBT: prevROB, ConsumptionT: consumption, ReportedROBT: rob,
		ExpectedROBT: round1(prevROB - consumption),
		GapT:         round1(prevROB - consumption - rob),
		ToleranceT:   round1(math.Max(ROBToleranceT, consumption*ROBTolerancePct/100)),
	}
	r.Flagged = math.Abs(r.GapT) > r.ToleranceT
	switch {
	case !r.Flagged:
		r.Interpretation = fmt.Sprintf("%s ROB follows from the previous report and the day's consumption, within %.1f t.", fuel, r.ToleranceT)
	case r.GapT < 0:
		r.Interpretation = fmt.Sprintf("%s ROB is %.1f t more than the previous report less the day's consumption. Fuel came on board, or a figure is wrong.", fuel, -r.GapT)
	default:
		r.Interpretation = fmt.Sprintf("%s ROB is %.1f t less than the previous report less the day's consumption. Fuel is unaccounted for, or a figure is wrong.", fuel, r.GapT)
	}
	return r
}

// SpeedDistanceResult checks the reported speed against the distance run.
type SpeedDistanceResult struct {
	Hours          float64 `json:"hours_between_reports"`
	DistanceRunNM  float64 `json:"distance_run_nm"`
	ReportedKn     float64 `json:"reported_speed_kn"`
	ImpliedKn      float64 `json:"implied_speed_kn"`
	ToleranceKn    float64 `json:"tolerance_kn"`
	MaxKn          float64 `json:"max_plausible_kn"`
	Mismatch       bool    `json:"mismatch"`
	TooFast        bool    `json:"too_fast"`
	Interpretation string  `json:"interpretation"`
}

// CheckSpeedDistance divides the distance run by the time since the last
// report (not always 24 h: clocks change on passage) and compares it with
// the reported average speed.
func CheckSpeedDistance(prev, now time.Time, distanceRunNM, speedKn float64) (SpeedDistanceResult, error) {
	h := now.Sub(prev).Hours()
	if h <= 0 {
		return SpeedDistanceResult{}, fmt.Errorf("the report is not after the previous one")
	}
	r := SpeedDistanceResult{
		Hours: round1(h), DistanceRunNM: distanceRunNM, ReportedKn: speedKn,
		ImpliedKn: round1(distanceRunNM / h), ToleranceKn: SpeedToleranceKn, MaxKn: MaxPlausibleSpeedKn,
	}
	r.Mismatch = math.Abs(r.ImpliedKn-speedKn) > SpeedToleranceKn+1e-9
	r.TooFast = math.Max(r.ImpliedKn, speedKn) > MaxPlausibleSpeedKn
	switch {
	case r.TooFast:
		r.Interpretation = fmt.Sprintf("A speed of %.1f kn is above the %.1f kn this ship can make. A figure is wrong.", math.Max(r.ImpliedKn, speedKn), MaxPlausibleSpeedKn)
	case r.Mismatch:
		r.Interpretation = fmt.Sprintf("%.1f nm in %.1f h is %.1f kn, but the report gives %.1f kn.", distanceRunNM, r.Hours, r.ImpliedKn, speedKn)
	default:
		r.Interpretation = fmt.Sprintf("The reported speed matches the distance run within %.1f kn.", SpeedToleranceKn)
	}
	return r, nil
}

// PositionResult checks the noon position against the distance run.
type PositionResult struct {
	StraightLineNM float64 `json:"straight_line_nm"`
	DistanceRunNM  float64 `json:"distance_run_nm"`
	MaxNM          float64 `json:"max_nm"`
	MinNM          float64 `json:"min_nm"`
	Flagged        bool    `json:"flagged"`
	Interpretation string  `json:"interpretation"`
}

// CheckPosition compares the great-circle distance from the previous noon
// position with the distance run. The track may be longer than the straight
// line (coasts, traffic schemes, weather), never much shorter: a position
// far off either way is most often a mistyped figure.
func CheckPosition(prevLat, prevLon, lat, lon, distanceRunNM float64) PositionResult {
	r := PositionResult{
		StraightLineNM: round1(route.Distance(route.Position{prevLon, prevLat}, route.Position{lon, lat}) / route.MetresPerNM),
		DistanceRunNM:  distanceRunNM,
		MaxNM:          round1(distanceRunNM*(1+PositionOverPct/100) + PositionSlackNM),
		MinNM:          round1(math.Max(0, distanceRunNM*(1-PositionUnderPct/100)-PositionSlackNM)),
	}
	r.Flagged = r.StraightLineNM > r.MaxNM || r.StraightLineNM < r.MinNM
	switch {
	case r.StraightLineNM > r.MaxNM:
		r.Interpretation = fmt.Sprintf("The position is %.1f nm in a straight line from the last noon position, more than the %.1f nm run. The position or the distance run is wrong.", r.StraightLineNM, distanceRunNM)
	case r.Flagged:
		r.Interpretation = fmt.Sprintf("The position is only %.1f nm in a straight line from the last noon position, against %.1f nm run. The position or the distance run may be wrong.", r.StraightLineNM, distanceRunNM)
	default:
		r.Interpretation = "The position agrees with the distance run."
	}
	return r
}
