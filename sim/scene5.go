package sim

import (
	"fmt"
	"time"

	"github.com/captv89/seaRoute/route"
)

// Scene 5 ("Berth not ready: slow down?"): on 23 Jan the
// Rotterdam agent writes that the coal berth is occupied and VALDA CREST
// berths about 27 Jan 0600 UTC, subject to the vessel ahead completing as
// planned. The desk knows the ship as it did after Scene 4: her noon
// reports and notices to 22 Jan, Scene 4's bad-weather note, and her AIS
// track up to the e-mail.
var s5EmailAt = time.Date(2025, 1, 23, 8, 0, 0, 0, time.UTC)

// Fictional owner and hire for VALDA CREST. Delivery into the
// time charter is set before the voyage started at Richards Bay.
var (
	s5Owner       = "Valda Shipping Ltd (owner, fictional)"
	s5DeliveredAt = time.Date(2024, 12, 20, 0, 0, 0, 0, time.UTC)
)

const s5HireUSDPerDay = 8500

// Scene5 builds the berth-delay e-mail scene.
func Scene5(g *route.Graph) (Scene, error) {
	s1, err := Scene1(g)
	if err != nil {
		return Scene{}, err
	}
	v := s1.Voyage
	rep := s1.Report
	v.History = append(append([]NoonReport(nil), v.History...), rep)
	// Scene 1's 3-day notice went out after the noon report.
	v.Notices = append(append([]Notice(nil), v.Notices...), Notice{SentAt: rep.Time.Add(30 * time.Minute), DaysLabel: 3, ETA: rep.MasterETA})
	// No sub-charters are modelled: ETA notices go to the
	// port agent and the receiver.
	v.CP.NoticeRecipientRoles = []string{"agent", "receiver"}
	v.Contacts = []Contact{v.Contacts[0], rtmAgent, receiver}
	v.Notes = []string{"Heavy weather expected on the route to Rotterdam from 22 Jan evening to 24 Jan: wind up to Beaufort 8 from astern, seas up to 6.1 m (ECMWF 22 Jan 12z). Bad weather days, to be left out of any performance assessment."}
	v.VMS = &VMS{
		PortCalls: []PortCall{{Port: Rotterdam.Name, Purpose: "discharge", ETA: rep.MasterETA, Berth: receiver.Name}},
		Hire:      Hire{Owner: s5Owner, DeliveredAt: s5DeliveredAt, RateUSDPerDay: s5HireUSDPerDay, OffHire: []OffHire{}},
	}

	r, err := g.Route([]route.Position{{rep.Lon, rep.Lat}, {v.To.Lon, v.To.Lat}}, route.Options{})
	if err != nil {
		return Scene{}, err
	}
	tr := NewTrack(r.Path)
	for t := rep.Time; !t.After(s5EmailAt); t = t.Add(time.Hour) {
		p := tr.At(t.Sub(rep.Time).Hours() * rep.SpeedKn * route.MetresPerNM)
		v.Track = append(v.Track, Fix{Time: t, Lat: roundTo(p[1], 1e-4), Lon: roundTo(p[0], 1e-4)})
	}
	if last := v.Track[len(v.Track)-1]; !last.Time.Equal(s5EmailAt) {
		return Scene{}, fmt.Errorf("VALDA CREST's track ends at %s, before the e-mail", last.Time)
	}

	body := `Dear Operations,

MV VALDA CREST - Rotterdam, Rijnmond Energy Coal Terminal

The coal berth is occupied. The vessel ahead is expected to complete discharge late on 26 Jan. VALDA CREST is next in line, berthing about 27 Jan 0600 UTC, subject to the vessel ahead completing as planned.

We will advise at once if the line-up changes. Please keep us informed of the vessel's ETA.

Best regards,
Operations desk
Maasport Agencies, Rotterdam
`
	return Scene{
		ID: "s5-berth", SimTime: s5EmailAt.Add(5 * time.Minute), Voyage: v,
		Email: Email{
			ID: "em-0501", From: "Maasport Agencies - Operations <" + rtmAgent.Email + ">", To: []string{"ops@shorewatch-demo.example"}, Time: s5EmailAt,
			Subject: "MV VALDA CREST - Rotterdam line-up: berth occupied", Body: body,
		},
	}, nil
}
