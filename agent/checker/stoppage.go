package checker

import (
	"fmt"
	"math"
	"time"
)

// StoppageDistancePct is how far the distance run may differ from the
// reported speed over the steaming hours. Fictional, like the CP.
const StoppageDistancePct = 2.0

// StoppageResult checks a main engine stop as the Master reported it: the
// stop lies within the report day, and the reported speed over the hours
// left for steaming gives the distance run.
type StoppageResult struct {
	From               time.Time `json:"from_utc"`
	To                 time.Time `json:"to_utc"`
	StopHours          float64   `json:"stop_hours"`
	SteamingHours      float64   `json:"steaming_hours"`
	AvgSpeedKn         float64   `json:"avg_speed_kn"`
	ExpectedDistanceNM float64   `json:"expected_distance_nm"`
	DistanceRunNM      float64   `json:"distance_run_nm"`
	DistanceDiffPct    float64   `json:"distance_diff_pct"`
	WithinReportDay    bool      `json:"within_report_day"`
	Consistent         bool      `json:"consistent"`
	Interpretation     string    `json:"interpretation"`
}

// CheckStoppage checks a stop from..to against the report made at
// reportTime, the previous one at prevTime.
func CheckStoppage(prevTime, reportTime, from, to time.Time, distanceRunNM, avgSpeedKn float64) (StoppageResult, error) {
	if !to.After(from) {
		return StoppageResult{}, fmt.Errorf("the stop must end after it starts")
	}
	stop := to.Sub(from).Hours()
	steaming := reportTime.Sub(prevTime).Hours() - stop
	r := StoppageResult{
		From: from, To: to, StopHours: round2(stop), SteamingHours: round2(steaming), AvgSpeedKn: avgSpeedKn,
		ExpectedDistanceNM: round1(avgSpeedKn * steaming), DistanceRunNM: distanceRunNM,
		WithinReportDay: !from.Before(prevTime) && !to.After(reportTime),
	}
	if r.ExpectedDistanceNM > 0 {
		r.DistanceDiffPct = round1((distanceRunNM - r.ExpectedDistanceNM) / r.ExpectedDistanceNM * 100)
	}
	r.Consistent = r.WithinReportDay && steaming > 0 && math.Abs(r.DistanceDiffPct) <= StoppageDistancePct
	switch {
	case r.Consistent:
		r.Interpretation = fmt.Sprintf("The stop agrees with the report: the reported speed over the %.1f h left for steaming gives the distance run within %.0f%%.", steaming, StoppageDistancePct)
	case !r.WithinReportDay:
		r.Interpretation = "The stop is not within this report's day. Ask the Master to confirm the times."
	default:
		r.Interpretation = "The stop does not agree with the distance run and the speed. Ask the Master to confirm the times and figures."
	}
	return r, nil
}
