package tools

import (
	"errors"
	"fmt"
	"math"
	"time"

	"github.com/captv89/seaRoute/route"
	"github.com/captv89/shorewatch/sim"
)

// PilotAllowanceHours is the stand-in's time from pilot boarding to the
// berth, taken off the berthing time to get the arrival it aims for.
// Made up for this simulation.
const PilotAllowanceHours = 2.0

// SpeedAdviceNote goes with every speed advice, so neither the agent nor
// the page can take the figures for real optimisation.
const SpeedAdviceNote = "Stand-in for the ship's voyage optimisation and routing service. The figures are made up for this simulation by a simple model (one steady speed, fuel by the cube law from the last noon reports, main engine only). In practice this advice comes from the voyage optimisation service."

// SpeedAdvice is the stand-in voyage optimisation service's answer: one
// steady speed from the ship's AIS position to arrive at the pilot station
// in time for the berth.
type SpeedAdvice struct {
	ShipID    string `json:"ship_id"`
	Ship      string `json:"ship"`
	Service   string `json:"service"`
	Simulated bool   `json:"simulated"`
	Note      string `json:"note"`

	FromTime   time.Time `json:"from_time"`
	FromLat    float64   `json:"from_lat"`
	FromLon    float64   `json:"from_lon"`
	To         string    `json:"to"`
	DistanceNM float64   `json:"distance_nm"`

	BerthUTC            time.Time `json:"berth_utc"`
	PilotAllowanceHours float64   `json:"pilot_allowance_hours"`
	TargetArrival       time.Time `json:"target_arrival_utc"`

	CurrentETA       time.Time `json:"current_eta_utc"` // the port call ETA in the voyage management system
	CurrentSpeedKn   float64   `json:"current_speed_kn"`
	AdvisedSpeedKn   float64   `json:"advised_speed_kn"`
	AdvisedArrival   time.Time `json:"advised_arrival_utc"`
	AnchorHoursSaved float64   `json:"anchor_hours_saved"` // the wait at anchor the current ETA would mean
	BaseVLSFOPerDayT float64   `json:"base_vlsfo_t_per_day"`
	BaseSpeedKn      float64   `json:"base_speed_kn"`
	VLSFOSavedT      float64   `json:"vlsfo_saved_t"`
	Advice           string    `json:"advice"`
}

// AdviseSpeed works out the stand-in speed advice for a voyage, from its
// last AIS fix along path to its next port.
func AdviseSpeed(v sim.Voyage, path []route.Position, berth time.Time) (SpeedAdvice, error) {
	if len(v.Track) == 0 || len(v.History) < 2 || v.VMS == nil || len(v.VMS.PortCalls) == 0 {
		return SpeedAdvice{}, fmt.Errorf("no AIS position, noon reports or voyage system record for %s", v.Ship.ID)
	}
	fix := v.Track[len(v.Track)-1]
	eta := v.VMS.PortCalls[0].ETA
	target := berth.Add(-time.Duration(PilotAllowanceHours * float64(time.Hour)))
	if !target.After(eta) {
		return SpeedAdvice{}, fmt.Errorf("berthing at %s UTC leaves no time to save: the current ETA is %s UTC", berth.Format("02 Jan 1504"), eta.Format("02 Jan 1504"))
	}
	var nm float64
	for i := 1; i < len(path); i++ {
		nm += route.Distance(path[i-1], path[i]) / route.MetresPerNM
	}
	if nm <= 0 {
		return SpeedAdvice{}, errors.New("no route to go")
	}
	// The daily VLSFO the ship burns at her last reported speed, from the
	// fall in ROB between the last two noon reports.
	prev, last := v.History[len(v.History)-2], v.History[len(v.History)-1]
	base := (prev.ROBVLSFO - last.ROBVLSFO) * 24 / last.Time.Sub(prev.Time).Hours()
	fuel := func(kn, hours float64) float64 { return base * math.Pow(kn/last.SpeedKn, 3) * hours / 24 }

	nowH := eta.Sub(fix.Time).Hours()
	advised := round1(nm / target.Sub(fix.Time).Hours())
	advH := nm / advised
	arrive := fix.Time.Add(time.Duration(advH * float64(time.Hour))).Truncate(time.Minute)
	return SpeedAdvice{
		ShipID: v.Ship.ID, Ship: v.Ship.Name, Service: "Voyage optimisation service (stand-in)", Simulated: true, Note: SpeedAdviceNote,
		FromTime: fix.Time, FromLat: fix.Lat, FromLon: fix.Lon, To: v.To.Name, DistanceNM: round1(nm),
		BerthUTC: berth, PilotAllowanceHours: PilotAllowanceHours, TargetArrival: target,
		CurrentETA: eta, CurrentSpeedKn: round1(nm / nowH),
		AdvisedSpeedKn: advised, AdvisedArrival: arrive, AnchorHoursSaved: round1(target.Sub(eta).Hours()),
		BaseVLSFOPerDayT: round1(base), BaseSpeedKn: last.SpeedKn,
		VLSFOSavedT: round1(fuel(nm/nowH, nowH) - fuel(advised, advH)),
		Advice:      "Advice only. The Master decides the speed for the safety of the ship, and the operator decides whether to order it.",
	}, nil
}

func round1(v float64) float64 { return math.Round(v*10) / 10 }
