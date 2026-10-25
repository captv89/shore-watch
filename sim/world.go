// Package sim holds the simulated world: fictional ships, voyages,
// charter party terms, counterparties and the noon reports they send.
// Everything here is synthetic and deterministic.
package sim

import (
	"fmt"
	"math"
	"strings"
	"time"
)

// Ship is a fictional vessel. IDs use SIM-000x, never IMO numbers.
type Ship struct {
	ID         string  `json:"id"`
	Name       string  `json:"name"`
	Type       string  `json:"type"`
	DWT        int     `json:"dwt"`
	LOAm       float64 `json:"loa_m"`
	BeamM      float64 `json:"beam_m"`
	LppM       float64 `json:"lpp_m,omitempty"`
	EmailLocal string  `json:"email"`
	// Cranes is the number of the ship's own cargo cranes; 0 for a gearless ship.
	Cranes int `json:"cranes,omitempty"`
}

// Loading is the ship's condition on this voyage, as a speed-loss model
// needs it.
type Loading struct {
	Ballast        bool    `json:"ballast"`
	DisplacementM3 float64 `json:"displacement_m3"`
	Cb             float64 `json:"block_coefficient"` // at this draught
}

// Port is a named port with a position.
type Port struct {
	Name string  `json:"name"`
	Lat  float64 `json:"lat"`
	Lon  float64 `json:"lon"`
}

// Contact is a known counterparty with its verified e-mail domain.
type Contact struct {
	Role   string `json:"role"`
	Name   string `json:"name"`
	Email  string `json:"email"`
	Domain string `json:"domain"`
	// Phone is the number on file, for checking a request by telephone.
	Phone string `json:"phone,omitempty"`
}

// CPTerms are the fictional charter party terms.
type CPTerms struct {
	Fictional bool `json:"fictional"`
	// ETA notices are due this many days before arrival.
	NoticeDays []int `json:"eta_notice_days"`
	// A new notice is due when the ETA moves more than this since the last one.
	ETAChangeHours float64 `json:"eta_change_hours"`
	// Good weather day: wind up to this Beaufort force and Douglas sea state.
	GoodWeatherMaxBeaufort int      `json:"good_weather_max_beaufort"`
	GoodWeatherMaxDouglas  int      `json:"good_weather_max_douglas"`
	NoticeRecipientRoles   []string `json:"eta_notice_recipients"`
	// Speed and consumption warranty, good weather days only:
	// about this speed on about this much VLSFO a day.
	WarrantedSpeedKn      float64 `json:"warranted_speed_kn,omitempty"`
	WarrantedVLSFOPerDayT float64 `json:"warranted_vlsfo_t_per_day,omitempty"`
	// OffHire is the off-hire clause, for scenes that need it.
	OffHire string `json:"off_hire_clause,omitempty"`
	// WarRisks is the war risks clause, for the fleet-week.
	WarRisks string `json:"war_risks_clause,omitempty"`
}

// Notice is an ETA notice already sent on this voyage.
type Notice struct {
	SentAt    time.Time `json:"sent_at"`
	DaysLabel int       `json:"days_before_arrival"` // 0 for an ETA-change notice
	ETA       time.Time `json:"eta"`
}

// NoonReport is what the Master reports each day at noon.
type NoonReport struct {
	ID          string    `json:"id"`
	Time        time.Time `json:"time"`
	Lat         float64   `json:"lat"`
	Lon         float64   `json:"lon"`
	DistanceRun float64   `json:"distance_run_nm"` // last 24 h
	SpeedKn     float64   `json:"avg_speed_kn"`    // last 24 h
	ROBVLSFO    float64   `json:"rob_vlsfo_t"`
	ROBMGO      float64   `json:"rob_mgo_t"`
	// Fuel used in the last 24 h. Zero means the report does not give it.
	ConsVLSFO float64   `json:"cons_vlsfo_t,omitempty"`
	ConsMGO   float64   `json:"cons_mgo_t,omitempty"`
	NextPort  string    `json:"next_port"`
	MasterETA time.Time `json:"master_eta"`
	// ETABasisKn is the speed the Master's ETA assumes, when the report gives it.
	ETABasisKn float64 `json:"eta_basis_kn,omitempty"`
	Remarks    string  `json:"remarks,omitempty"`
}

// Email is an incoming or outgoing message.
type Email struct {
	ID      string    `json:"id"`
	From    string    `json:"from"`
	To      []string  `json:"to"`
	Time    time.Time `json:"time"`
	Subject string    `json:"subject"`
	Body    string    `json:"body"`
}

// Voyage is one ship's current voyage with its history.
type Voyage struct {
	Ship        Ship         `json:"ship"`
	From        Port         `json:"from"`
	To          Port         `json:"to"`
	Cargo       string       `json:"cargo"`
	Avoid       []string     `json:"avoid_passages"`
	History     []NoonReport `json:"history"` // oldest first, excluding the scene's report
	Notices     []Notice     `json:"notices_sent"`
	Contacts    []Contact    `json:"contacts"`
	CP          CPTerms      `json:"cp_terms"`
	ShipTimeUTC string       `json:"ship_time"`
	// Track is the ship's synthetic AIS track, oldest first. Only scenes
	// that need it (weather on the past track) fill it in.
	Track []Fix `json:"track,omitempty"`
	// Loading is set for scenes that run the forecast ETA check.
	Loading *Loading `json:"loading,omitempty"`
	// VMS is the voyage as the company's voyage management system holds it.
	// Round 2 scenes fill it in; Round 1 scenes leave it empty.
	VMS *VMS `json:"vms,omitempty"`
	// Notes are earlier notes in the voyage file, oldest first, such as a
	// previous scene's bad-weather note. Only scenes that carry one on fill it in.
	Notes []string `json:"voyage_notes,omitempty"`
}

// VMS is what the operator's company has entered for the voyage in its
// voyage management system: the port calls and the hire. Every ship is on
// time charter to the company.
type VMS struct {
	PortCalls []PortCall `json:"port_calls"`
	Hire      Hire       `json:"hire"`
	// BunkerPrices are the prices the hire statement charges bunkers at,
	// USD a tonne by grade. Made up for this simulation.
	BunkerPrices map[string]float64 `json:"bunker_prices_usd_per_t,omitempty"`
}

// PortCall is one call in the itinerary.
type PortCall struct {
	Port    string    `json:"port"`
	Purpose string    `json:"purpose"` // e.g. "discharge"
	ETA     time.Time `json:"eta"`
	Berth   string    `json:"berth,omitempty"`
}

// Hire is the time charter hire as entered: the daily rate, and the
// off-hire periods already entered against the owner.
type Hire struct {
	Owner         string    `json:"owner"`
	DeliveredAt   time.Time `json:"delivered_at"`
	RateUSDPerDay float64   `json:"rate_usd_per_day"`
	OffHire       []OffHire `json:"off_hire"`
}

// OffHire is one off-hire period, billed against the owner in the hire statement.
type OffHire struct {
	From   time.Time `json:"from"`
	To     time.Time `json:"to"`
	Reason string    `json:"reason"`
}

// Fix is one position of the ship at a time, like an AIS position report.
type Fix struct {
	Time time.Time `json:"time"`
	Lat  float64   `json:"lat"`
	Lon  float64   `json:"lon"`
}

// DefaultCP returns the fictional CP terms.
func DefaultCP() CPTerms {
	return CPTerms{
		Fictional:              true,
		NoticeDays:             []int{10, 7, 5, 3, 2, 1},
		ETAChangeHours:         12,
		GoodWeatherMaxBeaufort: 4,
		GoodWeatherMaxDouglas:  3,
		NoticeRecipientRoles:   []string{"charterer", "agent"},
	}
}

// FormatLat writes a latitude as the Master would: 40-30.0N.
func FormatLat(lat float64) string { return formatDM(lat, 2, "N", "S") }

// FormatLon writes a longitude as the Master would: 010-00.0W.
func FormatLon(lon float64) string { return formatDM(lon, 3, "E", "W") }

func formatDM(v float64, width int, pos, neg string) string {
	hemi := pos
	if v < 0 {
		hemi, v = neg, -v
	}
	tenths := int(math.Round(v * 600)) // minutes x 10
	deg, min := tenths/600, float64(tenths%600)/10
	return fmt.Sprintf("%0*d-%04.1f%s", width, deg, min, hemi)
}

// Render writes the noon report e-mail as the ship would send it.
func (r NoonReport) Render(v Voyage) Email {
	var b strings.Builder
	fmt.Fprintf(&b, "NOON REPORT - %s\n", v.Ship.Name)
	if off, ok := zoneOffset(v.ShipTimeUTC); ok && off != 0 {
		// Ship's time is not UTC: the report gives the time in UTC, then
		// the ship's time, as many companies ask.
		fmt.Fprintf(&b, "Date/time: %s UTC (ship's time %s: %s)\n", r.Time.Format("02 Jan 2006 1504"), v.ShipTimeUTC, r.Time.Add(off).Format("02 Jan 1504"))
	} else {
		fmt.Fprintf(&b, "Date/time: %s (ship's time %s)\n", r.Time.Format("02 Jan 2006 1504"), v.ShipTimeUTC)
	}
	fmt.Fprintf(&b, "Position: %s %s\n", FormatLat(r.Lat), FormatLon(r.Lon))
	fmt.Fprintf(&b, "Distance run last 24h: %.1f nm\n", r.DistanceRun)
	fmt.Fprintf(&b, "Average speed last 24h: %.1f kn\n", r.SpeedKn)
	fmt.Fprintf(&b, "ROB VLSFO: %.1f t\n", r.ROBVLSFO)
	fmt.Fprintf(&b, "ROB MGO: %.1f t\n", r.ROBMGO)
	if r.ConsVLSFO > 0 {
		fmt.Fprintf(&b, "Consumption last 24h: VLSFO %.1f t, MGO %.1f t\n", r.ConsVLSFO, r.ConsMGO)
	}
	fmt.Fprintf(&b, "Next port: %s\n", r.NextPort)
	fmt.Fprintf(&b, "ETA %s: %s\n", r.NextPort, r.MasterETA.Format("2006-01-02 1504")+" UTC")
	if r.ETABasisKn > 0 {
		fmt.Fprintf(&b, "ETA based on: %.1f kn\n", r.ETABasisKn)
	}
	if r.Remarks != "" {
		fmt.Fprintf(&b, "Remarks: %s\n", r.Remarks)
	}
	fmt.Fprintf(&b, "\nMaster %s\n", v.Ship.Name)
	return Email{
		ID:      r.ID,
		From:    v.Ship.EmailLocal,
		To:      []string{"ops@shorewatch-demo.example"},
		Time:    r.Time,
		Subject: fmt.Sprintf("NOON REPORT %s %s", v.Ship.Name, r.Time.Format("02/01/2006")),
		Body:    b.String(),
	}
}

// zoneOffset reads a ship's time such as "UTC+8" or "UTC-5".
func zoneOffset(z string) (time.Duration, bool) {
	rest, ok := strings.CutPrefix(z, "UTC")
	if !ok {
		return 0, false
	}
	if rest == "" {
		return 0, true
	}
	var h int
	if _, err := fmt.Sscanf(rest, "%d", &h); err != nil {
		return 0, false
	}
	return time.Duration(h) * time.Hour, true
}
