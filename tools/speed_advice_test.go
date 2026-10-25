package tools

import (
	"strings"
	"testing"
	"time"

	"github.com/captv89/seaRoute/route"
	"github.com/captv89/shorewatch/sim"
)

// TestScene5SpeedAdvice checks the facts Scene 5 depends on: the stand-in
// aims for pilot boarding at 27 Jan 0400, two hours before the berth, at a
// believable slow speed, and it always says its figures are made up.
func TestScene5SpeedAdvice(t *testing.T) {
	g := testGraph(t)
	sc, err := sim.Scene5(g)
	if err != nil {
		t.Fatal(err)
	}
	v := sc.Voyage
	f := v.Track[len(v.Track)-1]
	r, err := g.Route([]route.Position{{f.Lon, f.Lat}, {v.To.Lon, v.To.Lat}}, route.Options{})
	if err != nil {
		t.Fatal(err)
	}
	berth := time.Date(2025, 1, 27, 6, 0, 0, 0, time.UTC)
	a, err := AdviseSpeed(v, r.Path, berth)
	if err != nil {
		t.Fatal(err)
	}
	t.Logf("from %s %.4f %.4f: %.1f nm; now %.1f kn to %s; advised %.1f kn, arrive %s; %.1f h at anchor and %.1f t VLSFO saved (base %.1f t/d at %.1f kn)",
		a.FromTime.Format("02 Jan 1504"), a.FromLat, a.FromLon, a.DistanceNM, a.CurrentSpeedKn, a.CurrentETA.Format("02 Jan 1504"),
		a.AdvisedSpeedKn, a.AdvisedArrival.Format("02 Jan 1504"), a.AnchorHoursSaved, a.VLSFOSavedT, a.BaseVLSFOPerDayT, a.BaseSpeedKn)

	if want := time.Date(2025, 1, 27, 4, 0, 0, 0, time.UTC); !a.TargetArrival.Equal(want) {
		t.Errorf("target arrival %s, want %s", a.TargetArrival, want)
	}
	if d := a.AdvisedArrival.Sub(a.TargetArrival); d < -time.Hour || d > time.Hour {
		t.Errorf("advised arrival %s is %s off the target", a.AdvisedArrival, d)
	}
	if a.CurrentSpeedKn < 11.9 || a.CurrentSpeedKn > 12.9 {
		t.Errorf("current speed %.1f kn: the voyage system ETA does not fit her reported 12.4 kn", a.CurrentSpeedKn)
	}
	if a.AdvisedSpeedKn < 7 || a.AdvisedSpeedKn > 10 {
		t.Errorf("advised speed %.1f kn", a.AdvisedSpeedKn)
	}
	if a.VLSFOSavedT <= 0 || a.AnchorHoursSaved <= 0 {
		t.Errorf("saves %.1f t and %.1f h", a.VLSFOSavedT, a.AnchorHoursSaved)
	}
	if !a.Simulated || !strings.Contains(a.Note, "made up for this simulation") {
		t.Error("speed advice not labelled as made up")
	}

	if _, err := AdviseSpeed(v, r.Path, v.VMS.PortCalls[0].ETA); err == nil {
		t.Error("a berth at the current ETA should leave nothing to advise")
	}
}
