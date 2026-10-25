package sim

import (
	"fmt"
	"time"

	"github.com/captv89/seaRoute/route"
)

// Scene 6 ("Main engine stopped: off-hire"): TORMAR SPRING's
// noon report of 26 Jan, after Storm Éowyn. The main engine stopped for
// 5.5 h with fuel filters choked by sludge stirred up in the heavy weather. The Master gives the average speed over steaming time, so
// the day's distance run is short against it.
var (
	s6StopFrom = time.Date(2025, 1, 26, 6, 10, 0, 0, time.UTC)
	s6StopTo   = time.Date(2025, 1, 26, 11, 40, 0, 0, time.UTC)
)

// OffHireClause is the fictional CP's off-hire clause, modelled on NYPE 1993
// clause 17.
const OffHireClause = "Off hire (fictional, modelled on NYPE 1993 clause 17): in the event of loss of time from breakdown of, or damage to, hull, machinery or equipment, or any other similar cause preventing the full working of the vessel, the payment of hire shall cease for the time thereby lost. All bunkers used by the vessel while off hire shall be for the owners' account."

// Fictional owner, hire and bunker prices for TORMAR SPRING.
var (
	s6Owner = Contact{Role: "owner", Name: "Owner's vessel technical management (Tormar Bulk Carriers, fictional)", Email: "technical@tormar-bulk.example", Domain: "tormar-bulk.example"}
	// Made up for this simulation, like the hire statement that uses them.
	s6BunkerPrices = map[string]float64{"VLSFO": 600, "MGO": 750}
)

const s6HireUSDPerDay = 9500

// s6Days are the two days after Scene 2's last report.
func s6Days() []s2Day {
	noon := func(d int) time.Time { return time.Date(2025, 1, d, 12, 0, 0, 0, time.UTC) }
	stopH := s6StopTo.Sub(s6StopFrom).Hours()
	return []s2Day{
		{noon: noon(25), hours: 24, speedKn: 11.0, vlsfo: 22.5, remarks: "Weather moderating, wind and sea decreasing. Speed increased gradually."},
		{noon: noon(26), hours: 24 - stopH, speedKn: 12.5, vlsfo: 18.8, stopFrom: s6StopFrom, stopTo: s6StopTo,
			remarks: fmt.Sprintf("Main engine stopped %s-%s UTC: fuel oil filters choked with sludge stirred up in the heavy weather. Filters cleaned, engine restarted, vessel resumed passage. Average speed over %.1f h steaming. Fuel used during stoppage: VLSFO 1.1 t (generators and boiler).",
				s6StopFrom.Format("1504"), s6StopTo.Format("1504"), 24-stopH)},
	}
}

// Scene6 builds the engine-stop noon report.
func Scene6(g *route.Graph) (Scene, error) {
	v, reports, noticesBefore, err := tormarVoyage(g, append(s2Days(), s6Days()...))
	if err != nil {
		return Scene{}, err
	}
	i := len(reports) - 1
	rep := reports[i]
	v.History = append([]NoonReport(nil), reports[:i]...)
	v.Notices = noticesBefore(i)
	v.Track = trackUpTo(v.Track, rep.Time)
	// No sub-charters are modelled: ETA notices go to the
	// port agent; the owner is a contact for the off-hire.
	v.CP.NoticeRecipientRoles = []string{"agent"}
	v.CP.OffHire = OffHireClause
	v.Contacts = []Contact{v.Contacts[0], s2Agent, s6Owner}
	prev := reports[i-1]
	v.VMS = &VMS{
		PortCalls:    []PortCall{{Port: SeptIles.Name, Purpose: "load", ETA: prev.MasterETA}},
		Hire:         Hire{Owner: "Tormar Bulk Carriers (owner, fictional)", DeliveredAt: s2Departure, RateUSDPerDay: s6HireUSDPerDay, OffHire: []OffHire{}},
		BunkerPrices: s6BunkerPrices,
	}
	return Scene{
		ID:      "s6-offhire",
		SimTime: rep.Time.Add(5 * time.Minute),
		Voyage:  v,
		Report:  rep,
		Email:   rep.Render(v),
	}, nil
}
