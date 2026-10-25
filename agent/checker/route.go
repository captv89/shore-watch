package checker

import (
	"fmt"
	"math"
	"time"
)

// BunkerReserveDays is the company's minimum ROB on arrival: this many
// days' consumption at sea. A fictional company rule.
const BunkerReserveDays = 3.0

// RouteChangeResult compares a new route with the planned one from the
// same position, and checks the fuel on board for the new one.
type RouteChangeResult struct {
	From          time.Time `json:"from_utc"`
	PlannedNM     float64   `json:"planned_distance_nm"`
	NewNM         float64   `json:"new_distance_nm"`
	ExtraNM       float64   `json:"extra_distance_nm"`
	SpeedKn       float64   `json:"speed_kn"`
	ExtraDays     float64   `json:"extra_days"`
	NewETA        time.Time `json:"new_eta_utc"`
	PlannedETA    time.Time `json:"planned_eta_utc"`
	ETAMovedHours float64   `json:"eta_moved_hours"`

	ROBVLSFOt     float64 `json:"rob_vlsfo_t"` // estimated now, from the last report
	VLSFOPerDayT  float64 `json:"vlsfo_t_per_day"`
	NeededT       float64 `json:"vlsfo_needed_t"`
	ReserveDays   float64 `json:"reserve_days"`
	ReserveT      float64 `json:"reserve_t"`
	ArrivalROBt   float64 `json:"arrival_rob_vlsfo_t"`
	BunkersEnough bool    `json:"bunkers_enough"`
	ShortT        float64 `json:"short_t"`

	Interpretation string `json:"interpretation"`
}

// CheckRouteChange works out what a new route means at the Master's ETA
// speed: the extra distance and days, the new ETA (to the hour, as a
// Master gives it), and whether the VLSFO on board covers the new route
// with the company's reserve. robT and perDayT are the last noon report's
// ROB and 24 h consumption; hoursSinceReport is from that report to from.
func CheckRouteChange(from time.Time, plannedNM, newNM, speedKn float64, plannedETA time.Time, robT, perDayT, hoursSinceReport float64) (RouteChangeResult, error) {
	if speedKn < 1 || newNM <= 0 || plannedNM <= 0 {
		return RouteChangeResult{}, fmt.Errorf("speed %.1f kn over %.1f nm: cannot compute an ETA", speedKn, newNM)
	}
	if perDayT <= 0 {
		return RouteChangeResult{}, fmt.Errorf("no daily consumption in the last noon report")
	}
	r := RouteChangeResult{From: from, PlannedNM: round1(plannedNM), NewNM: round1(newNM), SpeedKn: speedKn, PlannedETA: plannedETA,
		VLSFOPerDayT: perDayT, ReserveDays: BunkerReserveDays}
	r.ExtraNM = round1(newNM - plannedNM)
	r.ExtraDays = round1(r.ExtraNM / speedKn / 24)
	r.NewETA = from.Add(time.Duration(math.Round(newNM/speedKn)) * time.Hour)
	r.ETAMovedHours = round1(r.NewETA.Sub(plannedETA).Hours())

	r.ROBVLSFOt = round1(robT - perDayT*hoursSinceReport/24)
	r.NeededT = round1(newNM / speedKn / 24 * perDayT)
	r.ReserveT = round1(BunkerReserveDays * perDayT)
	r.ArrivalROBt = round1(r.ROBVLSFOt - r.NeededT)
	r.BunkersEnough = r.ArrivalROBt >= r.ReserveT
	if !r.BunkersEnough {
		r.ShortT = round1(r.ReserveT - r.ArrivalROBt)
	}
	r.Interpretation = fmt.Sprintf("The new route is %.1f nm against %.1f nm planned (%+.1f nm, %+.1f days at %.1f kn). New ETA %s UTC, %+.0f h on the planned ETA.",
		r.NewNM, r.PlannedNM, r.ExtraNM, r.ExtraDays, speedKn, r.NewETA.Format("02 Jan 1504"), r.ETAMovedHours)
	if r.BunkersEnough {
		r.Interpretation += fmt.Sprintf(" VLSFO on board, about %.1f t, covers it: about %.1f t on arrival, above the %.0f-day reserve of %.1f t.", r.ROBVLSFOt, r.ArrivalROBt, BunkerReserveDays, r.ReserveT)
	} else {
		r.Interpretation += fmt.Sprintf(" VLSFO on board, about %.1f t, does NOT cover it with the %.0f-day reserve of %.1f t: about %.1f t on arrival, %.1f t short. Bunkers are needed on the way.", r.ROBVLSFOt, BunkerReserveDays, r.ReserveT, r.ArrivalROBt, r.ShortT)
	}
	return r, nil
}
