package sim

import (
	"fmt"
	"math"
	"time"

	"github.com/captv89/seaRoute/route"
)

// Scene is one storyboard scene: the voyage, and the e-mail that starts it.
type Scene struct {
	ID      string     `json:"id"`
	SimTime time.Time  `json:"sim_time"`
	Voyage  Voyage     `json:"voyage"`
	Report  NoonReport `json:"report"`
	Email   Email      `json:"email"`
	// Forecast is set for a scene that starts when a forecast run comes
	// out, not with an e-mail; Email and Report are then empty.
	Forecast *ForecastIssued `json:"forecast,omitempty"`
	// Fleet is every ship a fleet-wide scene covers; Voyage is the first.
	Fleet []Voyage `json:"fleet,omitempty"`
}

// ForecastIssued is a new forecast run becoming available to the desk.
type ForecastIssued struct {
	Model     string    `json:"model"`
	Run       time.Time `json:"run"`
	Available time.Time `json:"available"`
	// NotScanned names the ships left out of the scan, and why.
	NotScanned []string `json:"not_scanned"`
}

// Fictional counterparties. Domains use .example (RFC 2606), so they can never be real.
var (
	charterer = Contact{Role: "charterer", Name: "Northwind Chartering (chartering desk)", Email: "chartering@northwind-chartering.example", Domain: "northwind-chartering.example"}
	rtmAgent  = Contact{Role: "agent", Name: "Maasport Agencies, Rotterdam", Email: "ops@maasport-agencies.example", Domain: "maasport-agencies.example"}
	receiver  = Contact{Role: "receiver", Name: "Rijnmond Energy Coal Terminal", Email: "receiving@rijnmond-coal.example", Domain: "rijnmond-coal.example"}
)

// Ports, positions from seaRoute's port list (UN/LOCODE).
var (
	RichardsBay = Port{Name: "Richards Bay", Lat: -28.771113820687, Lon: 32.083520507812}
	Rotterdam   = Port{Name: "Rotterdam", Lat: 51.904383352111097, Lon: 4.4424468168432654}
)

// Scene 1 timing and speeds. Chosen so that, on the real seaRoute track:
// the Master's ETA agrees with distance / speed within 2 h, the ETA has
// moved more than 12 h since the last notice (an 8 h engine stop on 21 Jan),
// and the 3-day notice falls due before the next noon report.
const (
	s1ReportLat     = 40.5 // the report is made where the track crosses 40°30'N
	s1SpeedKn       = 12.4 // last 24 h
	s1PrevSpeedKn   = 13.0 // service speed before the stop
	s1StopHours     = 8.0  // main engine stopped on 21 Jan
	s1VLSFOPerDay   = 26.0 // t/day at sea, laden
	s1MGOPerDay     = 0.2  // t/day
	s1HistoryDays   = 5
	s1ROBVLSFOStart = 742.0
	s1ROBMGOStart   = 88.0
)

// Scene1 builds "A normal noon report": VALDA CREST off Portugal, D1 noon.
func Scene1(g *route.Graph) (Scene, error) {
	reportTime := time.Date(2025, 1, 22, 12, 0, 0, 0, time.UTC)
	r, err := g.Route([]route.Position{{RichardsBay.Lon, RichardsBay.Lat}, {Rotterdam.Lon, Rotterdam.Lat}}, route.Options{})
	if err != nil {
		return Scene{}, err
	}
	track := NewTrack(r.Path)
	here := track.NorthboundCrossing(s1ReportLat)
	if here < 0 {
		return Scene{}, fmt.Errorf("track never reaches %.1f°N", s1ReportLat)
	}

	v := Voyage{
		Ship: Ship{
			ID: "SIM-0001", Name: "MV VALDA CREST", Type: "Supramax bulk carrier, laden",
			DWT: 58000, LOAm: 190, BeamM: 32.26, EmailLocal: "master@valdacrest.sim-fleet.example",
		},
		From: RichardsBay, To: Rotterdam, Cargo: "Steam coal in bulk",
		Contacts: []Contact{
			{Role: "ship", Name: "Master, MV VALDA CREST", Email: "master@valdacrest.sim-fleet.example", Domain: "valdacrest.sim-fleet.example"},
			charterer, rtmAgent, receiver,
		},
		CP:          DefaultCP(),
		ShipTimeUTC: "UTC",
	}

	// Daily distances, newest last: service speed, then the stop day, then today.
	daily := make([]float64, s1HistoryDays+1)
	for i := range daily {
		daily[i] = s1PrevSpeedKn * 24
	}
	stopDay := s1HistoryDays - 1 // the report of 21 Jan covers the stop
	daily[stopDay] = s1PrevSpeedKn * (24 - s1StopHours)
	daily[s1HistoryDays] = s1SpeedKn * 24

	// Walk back along the track from today's position.
	positions := make([]route.Position, len(daily))
	d := here
	for i := len(daily) - 1; i >= 0; i-- {
		p := track.At(d)
		positions[i] = route.Position{roundMinute(p[0]), roundMinute(p[1])}
		d -= daily[i] * route.MetresPerNM
	}

	robV, robM := s1ROBVLSFOStart, s1ROBMGOStart
	reports := make([]NoonReport, len(daily))
	for i := range daily {
		t := reportTime.Add(time.Duration(i-s1HistoryDays) * 24 * time.Hour)
		hours := 24.0
		if i == stopDay {
			hours = 24 - s1StopHours
		}
		robV -= roundTo(s1VLSFOPerDay*hours/24, 0.1)
		robM -= s1MGOPerDay
		rep := NoonReport{
			ID:          fmt.Sprintf("nr-%04d", 1+i),
			Time:        t,
			Lat:         positions[i][1],
			Lon:         positions[i][0],
			DistanceRun: roundTo(daily[i], 0.1),
			SpeedKn:     roundTo(daily[i]/24, 0.1),
			ROBVLSFO:    roundTo(robV, 0.1),
			ROBMGO:      roundTo(robM, 0.1),
			NextPort:    Rotterdam.Name,
		}
		if i == stopDay {
			rep.Remarks = fmt.Sprintf("Main engine stopped 0400-1200 for fuel valve replacement (%.0f h). Resumed passage.", s1StopHours)
		}
		reports[i] = rep
	}

	// The Master's ETAs: distance to go along the track at the speed he expects.
	remaining := func(i int) float64 {
		var ahead float64
		for j := i + 1; j < len(daily); j++ {
			ahead += daily[j]
		}
		return (track.LengthM()-here)/route.MetresPerNM + ahead
	}
	today := &reports[s1HistoryDays]
	today.MasterETA = etaHour(today.Time, remaining(s1HistoryDays), s1SpeedKn)
	today.Remarks = "ETA revised after main engine stop on 21 Jan."
	for i := range reports[:s1HistoryDays] {
		reports[i].MasterETA = etaHour(reports[i].Time, remaining(i), s1PrevSpeedKn)
	}

	// Notices already sent: each at the first noon report at which the
	// threshold falls before the next report. Before the history window the
	// ship ran at service speed.
	dtgToday := remaining(s1HistoryDays)
	dtgDaysAgo := func(k int) float64 {
		nm := dtgToday
		for j := 0; j < k; j++ {
			switch idx := s1HistoryDays - j; {
			case idx >= 0:
				nm += daily[idx]
			default:
				nm += s1PrevSpeedKn * 24
			}
		}
		return nm
	}
	sent := map[int]bool{}
	for k := 20; k >= 1; k-- {
		t := reportTime.Add(-time.Duration(k) * 24 * time.Hour)
		eta := etaHour(t, dtgDaysAgo(k), s1PrevSpeedKn)
		left := eta.Sub(t).Hours() / 24
		for _, nd := range v.CP.NoticeDays {
			if !sent[nd] && left-1 < float64(nd) {
				sent[nd] = true
				v.Notices = append(v.Notices, Notice{SentAt: t.Add(30 * time.Minute), DaysLabel: nd, ETA: eta})
			}
		}
	}
	v.History = reports[:s1HistoryDays]

	return Scene{
		ID:      "s1-routine",
		SimTime: today.Time.Add(5 * time.Minute),
		Voyage:  v,
		Report:  *today,
		Email:   today.Render(v),
	}, nil
}

// etaHour is t + nm / kn, rounded to the nearest hour, as a Master would give it.
func etaHour(t time.Time, nm, kn float64) time.Time {
	h := math.Round(nm / kn)
	return t.Add(time.Duration(h) * time.Hour)
}
