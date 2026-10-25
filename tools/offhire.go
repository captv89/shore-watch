package tools

import (
	"fmt"
	"math"
	"slices"
	"time"

	"github.com/captv89/shorewatch/sim"
)

// OffHireNote goes with every off-hire statement, so neither the agent nor
// the page can take the figures for a real hire statement.
const OffHireNote = "Stand-in for the voyage management system's hire statement. The figures are made up for this simulation; the voyage management system was never part of this project, and a real one would give its own figures."

// OffHireStatement is the stand-in voyage management system's preview of an
// off-hire period on the hire statement: the hire that stops, and the
// bunkers used while off hire, charged to the owner. It enters nothing.
type OffHireStatement struct {
	ShipID    string `json:"ship_id"`
	Ship      string `json:"ship"`
	Service   string `json:"service"`
	Simulated bool   `json:"simulated"`
	Note      string `json:"note"`

	Owner string    `json:"owner"`
	From  time.Time `json:"from_utc"`
	To    time.Time `json:"to_utc"`
	Hours float64   `json:"hours"`
	// CranesOut of Cranes: hire is reduced in that proportion (a crane
	// breakdown); both 0 for a whole stop.
	CranesOut     int      `json:"cranes_out,omitempty"`
	Cranes        int      `json:"cranes,omitempty"`
	Share         float64  `json:"share_off_hire"`
	Days          float64  `json:"days"`
	RateUSDPerDay float64  `json:"hire_rate_usd_per_day"`
	HireUSD       float64  `json:"hire_deducted_usd"`
	Bunkers       []Bunker `json:"bunkers_owners_account"`
	BunkersUSD    float64  `json:"bunkers_usd"`
	TotalUSD      float64  `json:"total_owners_account_usd"`
	Entry         string   `json:"entry"`
}

// Bunker is one grade of fuel charged on the hire statement.
type Bunker struct {
	Grade     string  `json:"grade"`
	Tonnes    float64 `json:"tonnes"`
	USDPerT   float64 `json:"usd_per_t"`
	AmountUSD float64 `json:"amount_usd"`
}

// PreviewOffHire works out the stand-in hire statement lines for an
// off-hire period and the bunkers used in it. cranesOut, if more than 0, is
// the number of cargo cranes out of use: the hire stops pro rata.
func PreviewOffHire(v sim.Voyage, from, to time.Time, bunkers map[string]float64, cranesOut int) (OffHireStatement, error) {
	if v.VMS == nil {
		return OffHireStatement{}, fmt.Errorf("no voyage management system record for %s in this demo", v.Ship.ID)
	}
	if !to.After(from) {
		return OffHireStatement{}, fmt.Errorf("the period must end after it starts")
	}
	h := v.VMS.Hire
	if from.Before(h.DeliveredAt) {
		return OffHireStatement{}, fmt.Errorf("the period starts before delivery on %s UTC", h.DeliveredAt.Format("02 Jan 2006 1504"))
	}
	share := 1.0
	if cranesOut > 0 {
		if v.Ship.Cranes == 0 || cranesOut > v.Ship.Cranes {
			return OffHireStatement{}, fmt.Errorf("%s has %d cranes; %d cannot be out of use", v.Ship.Name, v.Ship.Cranes, cranesOut)
		}
		share = float64(cranesOut) / float64(v.Ship.Cranes)
	}
	hours := to.Sub(from).Hours()
	s := OffHireStatement{
		ShipID: v.Ship.ID, Ship: v.Ship.Name, Service: "Voyage management system hire statement (stand-in)", Simulated: true, Note: OffHireNote,
		Owner: h.Owner, From: from, To: to, Hours: round2(hours), Share: share, Days: math.Round(hours*share/24*1e4) / 1e4,
		RateUSDPerDay: h.RateUSDPerDay, HireUSD: round2(h.RateUSDPerDay * hours * share / 24), Bunkers: []Bunker{},
		Entry: "Preview only: nothing is entered. The off-hire period is proposed with propose_entry and waits for the operator.",
	}
	if cranesOut > 0 {
		s.CranesOut, s.Cranes = cranesOut, v.Ship.Cranes
	}
	grades := make([]string, 0, len(bunkers))
	for g := range bunkers {
		grades = append(grades, g)
	}
	slices.Sort(grades)
	for _, g := range grades {
		t := bunkers[g]
		if t <= 0 {
			continue
		}
		price, ok := v.VMS.BunkerPrices[g]
		if !ok {
			return OffHireStatement{}, fmt.Errorf("no bunker price for %s in the voyage management system", g)
		}
		b := Bunker{Grade: g, Tonnes: t, USDPerT: price, AmountUSD: round2(t * price)}
		s.Bunkers = append(s.Bunkers, b)
		s.BunkersUSD = round2(s.BunkersUSD + b.AmountUSD)
	}
	s.TotalUSD = round2(s.HireUSD + s.BunkersUSD)
	return s, nil
}

func round2(v float64) float64 { return math.Round(v*100) / 100 }
