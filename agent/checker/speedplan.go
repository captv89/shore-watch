package checker

import (
	"fmt"
	"math"
	"time"
)

// Tolerances for checking a speed advice against seaRoute. Fictional, like the CP.
const (
	SpeedPlanDistancePct   = 1.0 // the advice's distance against seaRoute's
	SpeedPlanArrivalHours  = 1.0 // the arrival at the advised speed against the advice's
	speedPlanMinSpeedKnots = 1.0
)

// SpeedPlanResult checks a voyage optimisation service's speed advice:
// does its distance agree with seaRoute, does distance divided by the
// advised speed give its arrival, and does that arrival come before the berth?
type SpeedPlanResult struct {
	DistanceNM       float64   `json:"distance_nm"` // seaRoute
	AdviceDistanceNM float64   `json:"advice_distance_nm"`
	DistanceDiffPct  float64   `json:"distance_diff_pct"`
	AdvisedSpeedKn   float64   `json:"advised_speed_kn"`
	HoursToGo        float64   `json:"hours_to_go"`
	ComputedArrival  time.Time `json:"computed_arrival_utc"`
	AdviceArrival    time.Time `json:"advice_arrival_utc"`
	ArrivalDiffHours float64   `json:"arrival_diff_hours"`
	BerthUTC         time.Time `json:"berth_utc"`
	BeforeBerth      bool      `json:"arrives_before_berth"`
	Consistent       bool      `json:"consistent"`
	Interpretation   string    `json:"interpretation"`
}

// CheckSpeedPlan recomputes the advice's arrival from seaRoute's distance
// to go and the advised speed, from the time of the advice's start position.
func CheckSpeedPlan(from time.Time, distanceNM, adviceDistanceNM, speedKn float64, adviceArrival, berth time.Time) (SpeedPlanResult, error) {
	if speedKn < speedPlanMinSpeedKnots || distanceNM <= 0 {
		return SpeedPlanResult{}, fmt.Errorf("speed %.1f kn over %.1f nm: cannot compute an arrival", speedKn, distanceNM)
	}
	hours := distanceNM / speedKn
	computed := from.Add(time.Duration(hours * float64(time.Hour))).Truncate(time.Minute)
	r := SpeedPlanResult{
		DistanceNM:       distanceNM,
		AdviceDistanceNM: adviceDistanceNM,
		DistanceDiffPct:  round1((adviceDistanceNM - distanceNM) / distanceNM * 100),
		AdvisedSpeedKn:   speedKn,
		HoursToGo:        round1(hours),
		ComputedArrival:  computed,
		AdviceArrival:    adviceArrival,
		ArrivalDiffHours: round1(computed.Sub(adviceArrival).Hours()),
		BerthUTC:         berth,
		BeforeBerth:      computed.Before(berth),
	}
	r.Consistent = math.Abs(r.DistanceDiffPct) <= SpeedPlanDistancePct && math.Abs(r.ArrivalDiffHours) <= SpeedPlanArrivalHours && r.BeforeBerth
	switch {
	case r.Consistent:
		r.Interpretation = fmt.Sprintf("The speed advice adds up: seaRoute's distance divided by the advised speed gives an arrival within %.0f h of the advice's, before the berthing time.", SpeedPlanArrivalHours)
	case !r.BeforeBerth:
		r.Interpretation = "At the advised speed the ship arrives after the berthing time. Do not use this advice."
	default:
		r.Interpretation = "The speed advice does not agree with seaRoute's distance or its own arrival. Do not use it until the service is asked again."
	}
	return r, nil
}
