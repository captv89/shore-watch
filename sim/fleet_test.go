package sim

import (
	"testing"

	"github.com/captv89/seaRoute/route"
)

func TestBuildFleet(t *testing.T) {
	g, err := route.Load("../third_party/seaRoute/dataset/marnet_plus_5km.geojson")
	if err != nil {
		t.Fatal(err)
	}
	fleet, err := BuildFleet(g)
	if err != nil {
		t.Fatal(err)
	}
	if len(fleet) != 30 {
		t.Fatalf("%d ships, want 30", len(fleet))
	}
	for _, f := range fleet {
		in := 0
		for _, r := range f.Reports {
			if !r.Time.Before(WeekStart) {
				in++
			}
		}
		last := f.Reports[len(f.Reports)-1]
		t.Logf("%s %-20s reports %2d (week %d) arrived %v last %s %.2f,%.2f ROB %.0f ETA %s notices %d",
			f.Spec.ID, f.Spec.Name, len(f.Reports), in, f.ArrivedAt.Format("02 Jan 15h"), last.Time.Format("02 1504"), last.Lat, last.Lon, last.ROBVLSFO, last.MasterETA.Format("02 Jan 15h"), len(f.SentAfter[len(f.SentAfter)-1]))
	}
}
