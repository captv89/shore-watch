package desk

import (
	"testing"

	"github.com/captv89/shorewatch/agent/checker"
	"github.com/captv89/shorewatch/sim"
)

// TestScene6Story checks the facts Scene 6 depends on, on the real track:
// the report checks flag only the speed against the distance run (the
// remark explains it), consumption is within the warranty, and no ETA
// notice is due.
func TestScene6Story(t *testing.T) {
	scene1(t) // loads the graph
	sc, err := sim.Scene6(graph)
	if err != nil {
		t.Fatal(err)
	}
	rep, prev := sc.Report, sc.Voyage.History[len(sc.Voyage.History)-1]
	t.Logf("%s\n%s", sc.Email.Subject, sc.Email.Body)
	r := Report{Extraction: Extraction{DistanceRunNM: rep.DistanceRun, AvgSpeedKn: rep.SpeedKn, ROBVLSFOt: rep.ROBVLSFO, ROBMGOt: rep.ROBMGO, ConsVLSFOt: rep.ConsVLSFO, ConsMGOt: rep.ConsMGO},
		Lat: rep.Lat, Lon: rep.Lon, ReportTime: rep.Time, MasterETA: rep.MasterETA}
	var ids []string
	for _, f := range ReportChecks(r, &prev) {
		ids = append(ids, f.ID)
	}
	if len(ids) != 1 || ids[0] != "speed" {
		t.Errorf("report checks flag %v, want only speed", ids)
	}
	if c, _ := checker.CheckConsumption(rep.ConsVLSFO, sc.Voyage.CP.WarrantedVLSFOPerDayT, rep.SpeedKn, sc.Voyage.CP.WarrantedSpeedKn); c.Flagged {
		t.Errorf("consumption flagged: %+v", c)
	}
	n := checker.CheckNotices(rep.Time, rep.MasterETA, sc.Voyage.CP, sc.Voyage.Notices)
	t.Logf("ETA %s, last notice %d-day ETA %s, moved %.1f h, %.2f days to go", rep.MasterETA.Format("02 Jan 1504"), n.LastNoticeDays, n.LastNoticeETA.Format("02 Jan 1504"), n.ETAMovedHours, n.DaysToETA)
	if n.Due {
		t.Errorf("notice due: %v", n.Reasons)
	}
}
