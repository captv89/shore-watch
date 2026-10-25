package sim

import (
	"fmt"
	"hash/fnv"
	"math"
	"slices"
	"strings"
	"time"

	"github.com/captv89/seaRoute/route"
)

// The fleet-week: one operator's 30 dry bulk carriers, all
// on time charter to the company, over 20–26 Jan 2025. The ships and their
// people are fictional; the ports and routes are real (seaRoute).
var (
	WeekStart = time.Date(2025, 1, 20, 0, 0, 0, 0, time.UTC)
	WeekEnd   = time.Date(2025, 1, 27, 0, 0, 0, 0, time.UTC)
)

// Ports, positions from seaRoute's port list (UN/LOCODE).
var (
	Tubarao       = Port{Name: "Tubarao", Lat: -20.279468560907962, Lon: -40.244505948279802}
	Qingdao       = Port{Name: "Qingdao", Lat: 36.064986836649354, Lon: 120.38533529151967}
	PortHedland   = Port{Name: "Port Hedland", Lat: -20.32442828111957, Lon: 118.54248195155853}
	Dampier       = Port{Name: "Dampier", Lat: -20.661527144963671, Lon: 116.72746846265436}
	Kaohsiung     = Port{Name: "Kaohsiung", Lat: 22.642017334437032, Lon: 120.27196781167579}
	Itaqui        = Port{Name: "Itaqui", Lat: -2.5696142825110582, Lon: -44.341946676654203}
	Krishnapatnam = Port{Name: "Krishnapatnam", Lat: 14.297338730132442, Lon: 80.149250966486491}
	Gladstone     = Port{Name: "Gladstone", Lat: -23.842464062201792, Lon: 151.25960717029653}
	Pohang        = Port{Name: "Pohang", Lat: 36.0333333333, Lon: 129.37215983072}
	Newcastle     = Port{Name: "Newcastle (NSW)", Lat: -32.933334856914755, Lon: 151.75019233762185}
	Kashima       = Port{Name: "Kashima", Lat: 35.9, Lon: 140.016666667}
	Mobile        = Port{Name: "Mobile", Lat: 30.6666666667, Lon: -88.0333333333}
	Santos        = Port{Name: "Santos", Lat: -23.973772741447995, Lon: -46.320785667895713}
	Kamsar        = Port{Name: "Kamsar", Lat: 10.65, Lon: -14.6166666667}
	Ghent         = Port{Name: "Ghent", Lat: 51.080430590693, Lon: 3.7345349629686}
	Tianjin       = Port{Name: "Tianjin", Lat: 38.986801744702824, Lon: 117.74485246028443}
	Paradip       = Port{Name: "Paradip", Lat: 20.25, Lon: 86.6666666667}
	Balikpapan    = Port{Name: "Balikpapan", Lat: -1.2731382706431, Lon: 116.80582530697}
	JorfLasfar    = Port{Name: "Jorf Lasfar", Lat: 33.089056339801871, Lon: -8.6113607791267981}
	Mundra        = Port{Name: "Mundra", Lat: 22.795047722791168, Lon: 69.746702105827822}
	Samarinda     = Port{Name: "Samarinda", Lat: -0.56699081167141907, Lon: 117.57980776918025}
	Gijon         = Port{Name: "Gijon", Lat: 43.539597524663634, Lon: -5.6696320276613337}
	Ravenna       = Port{Name: "Ravenna", Lat: 44.424175324071989, Lon: 12.222976982027365}
	Fujairah      = Port{Name: "Fujairah", Lat: 25.176610035422261, Lon: 56.357553992838106}
	Antwerp       = Port{Name: "Antwerp", Lat: 51.202366263933435, Lon: 4.3828961050172168}
	Houston       = Port{Name: "Houston", Lat: 29.745752378204, Lon: -95.2871564229}
	Singapore     = Port{Name: "Singapore", Lat: 1.2392074109743487, Lon: 103.83246063045297}
	Mizushima     = Port{Name: "Mizushima", Lat: 34.5166666667, Lon: 133.733333333}
	Hamburg       = Port{Name: "Hamburg", Lat: 53.547720036227, Lon: 9.9694064380267875}
	Durban        = Port{Name: "Durban", Lat: -29.8666666667, Lon: 31.0166666667}
	PortLouis     = Port{Name: "Port Louis", Lat: -20.143648541714967, Lon: 57.516292170436188}
)

// Class is a bulk carrier size with typical figures, made up for this
// simulation from the usual ranges for each size.
type Class struct {
	Name         string
	DWT          int
	LOAm, BeamM  float64
	LppM         float64
	Cranes       int
	LadenKn      float64
	BallastKn    float64
	LadenVLSFO   float64 // t/day at the laden speed
	BallastVLSFO float64
	// Displaced volume and block coefficient, ∇ = Cb·Lpp·B·T, at the
	// laden and ballast draughts.
	LadenM3, LadenCb     float64
	BallastM3, BallastCb float64
	HireUSD              float64
}

var (
	Capesize  = Class{Name: "Capesize bulk carrier", DWT: 180000, LOAm: 292, BeamM: 45, LppM: 282, LadenKn: 12.5, BallastKn: 13.0, LadenVLSFO: 42, BallastVLSFO: 38, LadenM3: 196000, LadenCb: 0.85, BallastM3: 98900, BallastCb: 0.82, HireUSD: 18500}
	Kamsarmax = Class{Name: "Kamsarmax bulk carrier", DWT: 82000, LOAm: 229, BeamM: 32.26, LppM: 222, LadenKn: 12.5, BallastKn: 13.0, LadenVLSFO: 28, BallastVLSFO: 25, LadenM3: 88700, LadenCb: 0.86, BallastM3: 44000, BallastCb: 0.82, HireUSD: 12500}
	Ultramax  = Class{Name: "Ultramax bulk carrier, four 30 t cranes", DWT: 63000, LOAm: 200, BeamM: 32.26, LppM: 195, Cranes: 4, LadenKn: 12.5, BallastKn: 13.0, LadenVLSFO: 25, BallastVLSFO: 22, LadenM3: 71100, LadenCb: 0.85, BallastM3: 34200, BallastCb: 0.80, HireUSD: 12000}
	Handysize = Class{Name: "Handysize bulk carrier, four 30 t cranes", DWT: 38000, LOAm: 180, BeamM: 30, LppM: 172, Cranes: 4, LadenKn: 12.0, BallastKn: 12.5, LadenVLSFO: 20, BallastVLSFO: 18, LadenM3: 44400, LadenCb: 0.82, BallastM3: 23300, BallastCb: 0.78, HireUSD: 10500}
)

// FleetShip is one ship of the fleet-week and the voyage she is on.
type FleetShip struct {
	ID, Name string
	Class    Class
	From, To Port
	Depart   time.Time
	Laden    bool
	Cargo    string
	// Zone is the ship's time, hours from UTC; she reports at local noon.
	Zone     int
	ROBVLSFO float64 // at departure
	ROBMGO   float64
	Owner    string
	// RerouteAt, if set, is when she turns from the Red Sea for the Cape
	// of Good Hope (the security advisory).
	RerouteAt time.Time
}

// rerouteConfirmed is how long after the advisory the operator has
// confirmed the new route and sent the revised ETA notice.
const rerouteConfirmed = 2 * time.Hour

// RedSea are the passages the rerouted ships stop using.
var RedSea = []string{"babelmandeb", "suez"}

// fleetAvoid are passages no ship of this fleet uses: the Corinth Canal
// takes only small ships.
var fleetAvoid = []string{"corinth"}

func day(d, h int) time.Time { return time.Date(2025, 1, d, h, 0, 0, 0, time.UTC) }

// Fleet lists the 30 ships. Departures are set so that most are at sea
// all week; a few arrive or sail during it, and two are in port loading.
func Fleet() []FleetShip {
	s := []FleetShip{
		{Name: "MV CORAL HARROW", Class: Capesize, From: Tubarao, To: Qingdao, Depart: day(1, 18), Laden: true, Cargo: "Iron ore fines", Zone: 0, ROBVLSFO: 2600},
		{Name: "MV KESTREL MOOR", Class: Capesize, From: PortHedland, To: Qingdao, Depart: day(15, 6), Laden: true, Cargo: "Iron ore fines", Zone: 8, ROBVLSFO: 1500},
		{Name: "MV ASHBY FELL", Class: Capesize, From: Dampier, To: Kaohsiung, Depart: day(18, 4), Laden: true, Cargo: "Iron ore lump", Zone: 8, ROBVLSFO: 1200},
		{Name: "MV TARN BRIGHT", Class: Capesize, From: Qingdao, To: PortHedland, Depart: day(17, 2), Cargo: "In ballast", Zone: 8, ROBVLSFO: 1400},
		{Name: "MV HOLLIN REACH", Class: Capesize, From: Rotterdam, To: Tubarao, Depart: day(12, 14), Cargo: "In ballast", Zone: -1, ROBVLSFO: 1900},
		{Name: "MV MARRAM STAR", Class: Capesize, From: Itaqui, To: Rotterdam, Depart: day(14, 9), Laden: true, Cargo: "Iron ore pellets", Zone: -2, ROBVLSFO: 1800},
		{Name: "MV SKERRY DAWN", Class: Capesize, From: RichardsBay, To: Krishnapatnam, Depart: day(13, 10), Laden: true, Cargo: "Steam coal", Zone: 4, ROBVLSFO: 1600},
		{Name: "MV WENDING GLEN", Class: Capesize, From: Gladstone, To: Pohang, Depart: day(12, 0), Laden: true, Cargo: "Coking coal", Zone: 10, ROBVLSFO: 1700},
		{Name: "MV BRACKEN ISLE", Class: Kamsarmax, From: Newcastle, To: Kashima, Depart: day(10, 22), Laden: true, Cargo: "Steam coal", Zone: 10, ROBVLSFO: 1100},
		{Name: "MV GANNET RIDGE", Class: Kamsarmax, From: Mobile, To: Rotterdam, Depart: day(11, 16), Laden: true, Cargo: "Steam coal", Zone: -4, ROBVLSFO: 1150},
		{Name: "MV LINNET BAY", Class: Kamsarmax, From: Santos, To: Qingdao, Depart: day(5, 12), Laden: true, Cargo: "Soya beans", Zone: 3, ROBVLSFO: 1500},
		{Name: "MV ORRIN WATER", Class: Kamsarmax, From: Kamsar, To: Ghent, Depart: day(16, 20), Laden: true, Cargo: "Bauxite", Zone: -1, ROBVLSFO: 800},
		{Name: "MV PELL HAVEN", Class: Kamsarmax, From: Rotterdam, To: Mobile, Depart: day(15, 8), Cargo: "In ballast", Zone: -3, ROBVLSFO: 1000},
		{Name: "MV QUILLAN ROCK", Class: Kamsarmax, From: Tianjin, To: Newcastle, Depart: day(14, 3), Cargo: "In ballast", Zone: 9, ROBVLSFO: 1050},
		{Name: "MV RUNNEL POINT", Class: Kamsarmax, From: Paradip, To: Balikpapan, Depart: day(18, 6), Cargo: "In ballast", Zone: 6, ROBVLSFO: 700},
		{Name: "MV SOLWAY FIRTH", Class: Kamsarmax, From: Santos, To: JorfLasfar, Depart: day(13, 15), Laden: true, Cargo: "Maize", Zone: -1, ROBVLSFO: 900},
		{Name: "MV TEAL HOLLOW", Class: Kamsarmax, From: RichardsBay, To: Mundra, Depart: day(12, 7), Laden: true, Cargo: "Steam coal", Zone: 4, ROBVLSFO: 900},
		{Name: "MV ALDER CROSS", Class: Ultramax, From: Samarinda, To: Krishnapatnam, Depart: day(17, 3), Laden: true, Cargo: "Steam coal", Zone: 7, ROBVLSFO: 700},
		// In port at Samarinda, loading coal; a crane breaks down on 21 Jan.
		{Name: "MV BRYONY HEAD", Class: Ultramax, From: Balikpapan, To: Samarinda, Depart: day(17, 6), Cargo: "In ballast, to load steam coal", Zone: 8, ROBVLSFO: 650},
		// In port at Santos, loading sugar; a crane breaks down on 24 Jan.
		{Name: "MV CALDER SOUND", Class: Ultramax, From: Itaqui, To: Santos, Depart: day(11, 2), Cargo: "In ballast, to load bagged sugar", Zone: -3, ROBVLSFO: 600},
		// Bound through the Red Sea when the advisory comes, 22 Jan.
		{Name: "MV DUNLIN GATE", Class: Ultramax, From: Mundra, To: Gijon, Depart: day(19, 18), Laden: true, Cargo: "Bagged salt", Zone: 4, ROBVLSFO: 1500, RerouteAt: day(22, 6)},
		{Name: "MV EYOT SPRING", Class: Ultramax, From: Krishnapatnam, To: Ravenna, Depart: day(17, 0), Laden: true, Cargo: "Iron ore pellets", Zone: 4, ROBVLSFO: 1650, RerouteAt: day(22, 6)},
		{Name: "MV FERN ESTUARY", Class: Ultramax, From: Fujairah, To: Antwerp, Depart: day(19, 22), Laden: true, Cargo: "Limestone aggregate", Zone: 4, ROBVLSFO: 900, RerouteAt: day(22, 6)},
		{Name: "MV GORSE LANDING", Class: Ultramax, From: Antwerp, To: Houston, Depart: day(12, 18), Laden: true, Cargo: "Steel products", Zone: -2, ROBVLSFO: 700},
		{Name: "MV HEATH WARREN", Class: Ultramax, From: Houston, To: JorfLasfar, Depart: day(14, 20), Laden: true, Cargo: "Petroleum coke", Zone: -4, ROBVLSFO: 700},
		{Name: "MV INGLE NESS", Class: Ultramax, From: Tianjin, To: Singapore, Depart: day(18, 1), Laden: true, Cargo: "Steel coils", Zone: 8, ROBVLSFO: 500},
		{Name: "MV JUNIPER TOR", Class: Ultramax, From: Mizushima, To: Newcastle, Depart: day(13, 4), Cargo: "In ballast", Zone: 10, ROBVLSFO: 650},
		{Name: "MV KITTIWAKE SHOAL", Class: Handysize, From: Hamburg, To: JorfLasfar, Depart: day(20, 9), Cargo: "In ballast", Zone: 0, ROBVLSFO: 350},
		{Name: "MV LARCH QUAY", Class: Handysize, From: Durban, To: PortLouis, Depart: day(19, 13), Laden: true, Cargo: "Bagged rice", Zone: 3, ROBVLSFO: 330},
		{Name: "MV MERLIN STRAND", Class: Handysize, From: Ghent, To: Gdynia, Depart: day(21, 15), Laden: true, Cargo: "Fertiliser", Zone: 1, ROBVLSFO: 250},
	}
	for i := range s {
		s[i].ID = fmt.Sprintf("SIM-%04d", 101+i)
		s[i].ROBMGO = 80
		s[i].Owner = strings.TrimPrefix(s[i].Name, "MV ")
	}
	return s
}

// FleetVoyage is a fleet ship's voyage as built: the voyage record without
// history, every noon report she makes up to the end of the week, the ETA
// notices sent after each, and her AIS track.
type FleetVoyage struct {
	Spec    FleetShip
	Voyage  Voyage
	Reports []NoonReport
	// SentAfter[i] are the notices sent up to and including report i;
	// SentBefore(i) those sent before it.
	SentAfter [][]Notice
	Sailing   Notice
	// Departure is the departure report, the first report the desk has.
	Departure NoonReport
	Track     []Fix
	// ArrivedAt is when she reached the port, if within the week.
	ArrivedAt time.Time
	path      Track
	planned   Track // the route before any reroute
	speedKn   func(time.Time) float64
	along     func(time.Time) float64
}

// SentBefore returns the notices sent before report i.
func (f FleetVoyage) SentBefore(i int) []Notice {
	if i == 0 {
		return []Notice{f.Sailing}
	}
	return f.SentAfter[i-1]
}

// Fixed figures for every fleet voyage, made up for this simulation.
const (
	fleetMGOPerDay = 0.2
	// Bunker prices the hire statement charges, USD a tonne.
	fleetVLSFOPrice = 600
	fleetMGOPrice   = 750
)

// FleetOffHireClause adds a fictional crane rider to the off-hire clause.
const FleetOffHireClause = OffHireClause + " Rider (fictional): in the event of a breakdown of the vessel's cargo cranes, hire shall be reduced pro rata for the time lost, in the proportion that the number of cranes out of use bears to the total number of cranes."

// WarRisksClause is the fictional war risks clause, modelled on BIMCO
// CONWARTIME 2013.
const WarRisksClause = "War risks (fictional, modelled on BIMCO CONWARTIME 2013): the vessel shall not be obliged to proceed through any area where, in the reasonable judgement of the Master and/or the owners, she may be exposed to war risks. If the owners so decide, the charterers shall give other orders, and the vessel shall proceed by an alternative route. Extra time and bunkers on that route are for the charterers' account."

// jitter is a small, repeatable daily change in speed, -0.3 to +0.3 kn.
func jitter(id string, d time.Time) float64 {
	h := fnv.New32a()
	fmt.Fprintf(h, "%s %s", id, d.Format("2006-01-02"))
	return float64(int(h.Sum32()%7)-3) / 10
}

// BuildFleet builds every fleet voyage on the seaRoute network.
func BuildFleet(g *route.Graph) ([]FleetVoyage, error) {
	var out []FleetVoyage
	for _, s := range Fleet() {
		f, err := buildFleetVoyage(g, s)
		if err != nil {
			return nil, fmt.Errorf("%s %s: %w", s.ID, s.Name, err)
		}
		out = append(out, f)
	}
	return out, nil
}

func buildFleetVoyage(g *route.Graph, s FleetShip) (FleetVoyage, error) {
	c := s.Class
	service, perDay := c.BallastKn, c.BallastVLSFO
	load := &Loading{Ballast: true, DisplacementM3: c.BallastM3, Cb: c.BallastCb}
	if s.Laden {
		service, perDay = c.LadenKn, c.LadenVLSFO
		load = &Loading{DisplacementM3: c.LadenM3, Cb: c.LadenCb}
	}
	r, err := g.Route([]route.Position{{s.From.Lon, s.From.Lat}, {s.To.Lon, s.To.Lat}}, route.Options{Avoid: fleetAvoid})
	if err != nil {
		return FleetVoyage{}, err
	}
	path := NewTrack(r.Path)
	avoid := slices.Clone(fleetAvoid)

	// Report times: local noon each day from the first after sailing.
	noon := func(d time.Time) time.Time {
		return time.Date(d.Year(), d.Month(), d.Day(), 12-s.Zone, 0, 0, 0, time.UTC)
	}
	first := noon(s.Depart)
	for !first.After(s.Depart.Add(6 * time.Hour)) {
		first = first.Add(24 * time.Hour)
	}
	// The speed through each day, ending at its noon report.
	speed := func(t time.Time) float64 {
		end := first
		for end.Before(t) {
			end = end.Add(24 * time.Hour)
		}
		return service + jitter(s.ID, end)
	}
	// Distance along the track at time t.
	along := func(t time.Time) float64 {
		var nm float64
		for a := s.Depart; a.Before(t); {
			b := first
			for !b.After(a) {
				b = b.Add(24 * time.Hour)
			}
			if b.After(t) {
				b = t
			}
			nm += b.Sub(a).Hours() * speed(b)
			a = b
		}
		return nm * route.MetresPerNM
	}

	ship := Ship{ID: s.ID, Name: s.Name, Type: c.Name, DWT: c.DWT, LOAm: c.LOAm, BeamM: c.BeamM, LppM: c.LppM, Cranes: c.Cranes,
		EmailLocal: "master@" + strings.ToLower(strings.ReplaceAll(strings.TrimPrefix(s.Name, "MV "), " ", "")) + ".sim-fleet.example"}
	if s.Laden {
		ship.Type += ", laden"
	} else {
		ship.Type += ", in ballast"
	}
	v := Voyage{
		Ship: ship, From: s.From, To: s.To, Cargo: s.Cargo, Avoid: avoid,
		Contacts:    fleetContacts(ship, s),
		CP:          DefaultCP(),
		ShipTimeUTC: fmt.Sprintf("UTC%+d", s.Zone),
		Loading:     load,
	}
	if s.Zone == 0 {
		v.ShipTimeUTC = "UTC"
	}
	v.CP.NoticeRecipientRoles = []string{"agent", "receiver"}
	v.CP.OffHire = FleetOffHireClause
	v.CP.WarRisks = WarRisksClause

	f := FleetVoyage{Spec: s, Voyage: v, path: path, planned: path, speedKn: speed, along: along}

	// The reroute: from her position at the advisory, the shortest route
	// that avoids the Red Sea. The track ahead is replaced from there.
	if !s.RerouteAt.IsZero() {
		at := along(s.RerouteAt)
		here := path.At(at)
		r2, err := g.Route([]route.Position{here, {s.To.Lon, s.To.Lat}}, route.Options{Avoid: append(slices.Clone(fleetAvoid), RedSea...)})
		if err != nil {
			return FleetVoyage{}, err
		}
		if !slices.Contains(r.Passages, "babelmandeb") {
			return FleetVoyage{}, fmt.Errorf("not bound through Bab el-Mandeb")
		}
		if aheadOfStrait(path, at) <= 0 {
			return FleetVoyage{}, fmt.Errorf("already past Bab el-Mandeb at the advisory")
		}
		f.path = NewTrack(append(path.Cut(at), r2.Path[1:]...))
	}

	// Distance to go as the desk measures it: seaRoute from the position,
	// so the Master's ETA agrees with distance_to_go.
	toGo := func(p route.Position, t time.Time) (float64, error) {
		avoid := fleetAvoid
		if !s.RerouteAt.IsZero() && !t.Before(s.RerouteAt) {
			avoid = append(slices.Clone(fleetAvoid), RedSea...)
		}
		r, err := g.Route([]route.Position{p, {s.To.Lon, s.To.Lat}}, route.Options{Avoid: avoid})
		return r.DistanceNM(), err
	}

	robV, robM := s.ROBVLSFO, s.ROBMGO
	prev := s.Depart
	sailNM, err := toGo(route.Position{s.From.Lon, s.From.Lat}, s.Depart)
	if err != nil {
		return FleetVoyage{}, err
	}
	f.Departure = NoonReport{
		ID: fmt.Sprintf("dep-%s", strings.TrimPrefix(s.ID, "SIM-")), Time: s.Depart, Lat: roundMinute(s.From.Lat), Lon: roundMinute(s.From.Lon),
		ROBVLSFO: robV, ROBMGO: robM, NextPort: s.To.Name, MasterETA: etaHour(s.Depart, sailNM, service), ETABasisKn: service,
		Remarks: "Departure report: all fast off, pilot away.",
	}
	for t := first; t.Before(WeekEnd); t = t.Add(24 * time.Hour) {
		d := f.along(t)
		if d >= f.path.LengthM() {
			// Arrived: the hour she reaches the port, at the day's speed.
			left := (f.path.LengthM() - f.along(prev)) / route.MetresPerNM
			f.ArrivedAt = prev.Add(time.Duration(left / speed(t) * float64(time.Hour))).Truncate(time.Hour)
			break
		}
		hours := t.Sub(prev).Hours()
		kn := speed(t)
		used := roundTo(perDay*math.Pow(kn/service, 3)*hours/24, 0.1)
		robV, robM = roundTo(robV-used, 0.1), roundTo(robM-fleetMGOPerDay*hours/24, 0.1)
		p := f.path.At(d)
		p = route.Position{roundMinute(p[0]), roundMinute(p[1])}
		left, err := toGo(p, t)
		if err != nil {
			return FleetVoyage{}, err
		}
		rep := NoonReport{
			ID: fmt.Sprintf("nr-%s-%s", strings.TrimPrefix(s.ID, "SIM-"), t.Format("0102")), Time: t,
			Lat: p[1], Lon: p[0],
			DistanceRun: roundTo(kn*hours, 0.1), SpeedKn: roundTo(kn, 0.1),
			ROBVLSFO: robV, ROBMGO: robM, ConsVLSFO: used, ConsMGO: roundTo(fleetMGOPerDay*hours/24, 0.1),
			NextPort: s.To.Name, MasterETA: etaHour(t, left, service), ETABasisKn: service,
		}
		f.Reports = append(f.Reports, rep)
		prev = t
	}
	if len(f.Reports) == 0 {
		return FleetVoyage{}, fmt.Errorf("no noon reports")
	}

	// ETA notices: the sailing notice, then each due at a report, sent half
	// an hour after it. At the reroute the operator sends a change notice.
	f.Sailing = Notice{SentAt: s.Depart.Add(time.Hour), ETA: f.Departure.MasterETA}
	sent := []Notice{f.Sailing}
	for i, rep := range f.Reports {
		if !s.RerouteAt.IsZero() && i > 0 && f.Reports[i-1].Time.Before(s.RerouteAt) && !rep.Time.Before(s.RerouteAt) {
			// The revised ETA the advisory e-mail's handling sent.
			left, err := toGo(f.path.At(f.along(s.RerouteAt)), s.RerouteAt)
			if err != nil {
				return FleetVoyage{}, err
			}
			sent = append(slices.Clone(sent), Notice{SentAt: s.RerouteAt.Add(rerouteConfirmed), ETA: etaHour(s.RerouteAt, left, service)})
		}
		if n := checkNoticeDue(rep, v.CP, sent); n != nil {
			sent = append(slices.Clone(sent), *n)
		}
		f.SentAfter = append(f.SentAfter, sent)
	}

	// Hourly AIS fixes from sailing to the end of the week or arrival.
	end := WeekEnd
	if !f.ArrivedAt.IsZero() {
		end = f.ArrivedAt
	}
	for t := s.Depart.Truncate(time.Hour).Add(time.Hour); !t.After(end); t = t.Add(time.Hour) {
		p := f.path.At(f.along(t))
		f.Track = append(f.Track, Fix{Time: t, Lat: roundTo(p[1], 1e-4), Lon: roundTo(p[0], 1e-4)})
	}
	return f, nil
}

// aheadOfStrait is how far ahead of d the track passes closest to Bab
// el-Mandeb (about 12.6°N 43.4°E), in metres; 0 or less if behind.
func aheadOfStrait(t Track, d float64) float64 {
	best, at := math.Inf(1), 0.0
	for i := range t.Path {
		if dd := route.Distance(t.Path[i], route.Position{43.4, 12.6}); dd < best {
			best, at = dd, t.cum[i]
		}
	}
	return at - d
}

// checkNoticeDue is the CP notice rule as Checker applies it
// (checker.CheckNotices; the sim package cannot import it). It returns the
// notice the operator sends after the report, if one is due.
func checkNoticeDue(rep NoonReport, cp CPTerms, sent []Notice) *Notice {
	days := rep.MasterETA.Sub(rep.Time).Hours() / 24
	already := map[int]bool{}
	for _, n := range sent {
		already[n.DaysLabel] = true
	}
	n := Notice{SentAt: rep.Time.Add(30 * time.Minute), ETA: rep.MasterETA}
	due := false
	for _, nd := range cp.NoticeDays {
		if !already[nd] && days-1 < float64(nd) && float64(nd) <= days {
			n.DaysLabel, due = nd, true
			break
		}
	}
	if last := sent[len(sent)-1]; math.Abs(rep.MasterETA.Sub(last.ETA).Hours()) > cp.ETAChangeHours {
		due = true
	}
	if !due {
		return nil
	}
	return &n
}

// fleetContacts are a fleet ship's counterparties: her Master, the agent and
// the receiver at the port she is bound for, and her owner's technical
// management. Domains use .example (RFC 2606).
func fleetContacts(ship Ship, s FleetShip) []Contact {
	slug := strings.ToLower(strings.NewReplacer(" ", "", "(", "", ")", "").Replace(s.To.Name))
	owner := strings.ToLower(strings.ReplaceAll(s.Owner, " ", ""))
	return []Contact{
		{Role: "ship", Name: "Master, " + ship.Name, Email: ship.EmailLocal, Domain: strings.TrimPrefix(ship.EmailLocal, "master@")},
		{Role: "agent", Name: s.To.Name + " Port Agency", Email: "ops@" + slug + "-agency.example", Domain: slug + "-agency.example"},
		{Role: "receiver", Name: s.To.Name + " Bulk Terminal", Email: "shipping@" + slug + "-bulk.example", Domain: slug + "-bulk.example"},
		{Role: "owner", Name: "Owner's vessel technical management (" + titleCase(s.Owner) + " Shipping, fictional)", Email: "technical@" + owner + "-shipping.example", Domain: owner + "-shipping.example"},
	}
}

func titleCase(s string) string {
	w := strings.Fields(strings.ToLower(s))
	for i := range w {
		w[i] = strings.ToUpper(w[i][:1]) + w[i][1:]
	}
	return strings.Join(w, " ")
}

// SceneAt is the noon report scene for report i: the voyage as the desk
// knows it then, with the last five reports, the notices sent, the AIS
// track of the last three days and the voyage management system record.
func (f FleetVoyage) SceneAt(i int) Scene {
	rep := f.Reports[i]
	v := f.VoyageAt(rep.Time)
	v.History = f.history(i)
	v.Notices = slices.Clone(f.SentBefore(i))
	return Scene{ID: "fw-" + rep.ID, SimTime: rep.Time.Add(5 * time.Minute), Voyage: v, Report: rep, Email: rep.Render(v)}
}

// VoyageAt is the voyage as the desk knows it at time t, between reports:
// the reports and notices before t, the AIS track to t and the voyage
// management system record, with the port call ETA as last reported.
func (f FleetVoyage) VoyageAt(t time.Time) Voyage {
	v := f.Voyage
	n := 0
	for n < len(f.Reports) && f.Reports[n].Time.Before(t) {
		n++
	}
	v.History = f.history(n)
	v.Notices = slices.Clone(f.SentBefore(n))
	// The voyage record keeps the planned route until the operator has
	// confirmed the new one, when the revised ETA notice goes (2 h on).
	if !f.Spec.RerouteAt.IsZero() && !t.Before(f.Spec.RerouteAt.Add(rerouteConfirmed)) {
		v.Avoid = append(slices.Clone(v.Avoid), RedSea...)
	}
	for _, x := range f.Track {
		if !x.Time.After(t) && x.Time.After(t.Add(-72*time.Hour)) {
			v.Track = append(v.Track, x)
		}
	}
	eta := f.Sailing.ETA
	if n > 0 {
		eta = f.Reports[n-1].MasterETA
	}
	purpose := "discharge"
	if !f.Spec.Laden {
		purpose = "load"
	}
	v.VMS = &VMS{
		PortCalls:    []PortCall{{Port: f.Spec.To.Name, Purpose: purpose, ETA: eta}},
		Hire:         Hire{Owner: titleCase(f.Spec.Owner) + " Shipping (owner, fictional)", DeliveredAt: f.Spec.Depart.Add(-10 * 24 * time.Hour), RateUSDPerDay: f.Spec.Class.HireUSD, OffHire: []OffHire{}},
		BunkerPrices: map[string]float64{"VLSFO": fleetVLSFOPrice, "MGO": fleetMGOPrice},
	}
	return v
}

// AtSea reports whether the ship is on passage at t, with at least one
// noon report made: a forecast scan has her last report to go on.
func (f FleetVoyage) AtSea(t time.Time) bool {
	return f.Reports[0].Time.Before(t) && (f.ArrivedAt.IsZero() || t.Before(f.ArrivedAt)) && t.Before(WeekEnd)
}

// Ahead is where she will be every 3 h for the next h hours from t, on her
// track, stopping at the port.
func (f FleetVoyage) Ahead(t time.Time, h float64) []route.Position {
	path := f.path
	if !f.Spec.RerouteAt.IsZero() && t.Before(f.Spec.RerouteAt) {
		path = f.planned
	}
	var out []route.Position
	for dt := 0.0; dt <= h; dt += 3 {
		d := f.along(t.Add(time.Duration(dt * float64(time.Hour))))
		out = append(out, path.At(d))
		if d >= path.LengthM() {
			break
		}
	}
	return out
}

// history is the last five reports before report i, the departure report first.
func (f FleetVoyage) history(i int) []NoonReport {
	all := append([]NoonReport{f.Departure}, f.Reports[:i]...)
	return slices.Clone(all[max(0, len(all)-5):])
}
