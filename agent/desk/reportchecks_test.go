package desk

import (
	"testing"

	"google.golang.org/genai"

	"github.com/captv89/shorewatch/sim"
)

// asReport turns a simulated noon report into what a correct extraction gives.
func asReport(n sim.NoonReport) Report {
	return Report{
		Extraction: Extraction{DistanceRunNM: n.DistanceRun, AvgSpeedKn: n.SpeedKn, ROBVLSFOt: n.ROBVLSFO, ROBMGOt: n.ROBMGO,
			ConsVLSFOt: n.ConsVLSFO, ConsMGOt: n.ConsMGO, Remarks: n.Remarks},
		Lat: n.Lat, Lon: n.Lon, ReportTime: n.Time, MasterETA: n.MasterETA,
	}
}

// The recorded scenes have no planted inconsistencies: the new checks must
// leave them as they are.
func TestReportChecksLeaveScenesClean(t *testing.T) {
	scenes := []sim.Scene{scene1(t)}
	s2, err := sim.Scene2(graph)
	if err != nil {
		t.Fatal(err)
	}
	scenes = append(scenes, s2...)
	for _, sc := range scenes {
		h := sc.Voyage.History
		if len(h) == 0 {
			t.Fatalf("%s: no history", sc.ID)
		}
		if f := ReportChecks(asReport(sc.Report), &h[len(h)-1]); len(f) > 0 {
			t.Errorf("%s %s: flagged %+v", sc.ID, sc.Report.ID, f)
		}
	}
}

func TestReportChecksFlagPlantedErrors(t *testing.T) {
	sc := scene1(t)
	prev := sc.Voyage.History[len(sc.Voyage.History)-1]
	r := asReport(sc.Report)
	r.ConsVLSFOt = 26.0
	r.ROBVLSFOt = prev.ROBVLSFO - 26.0 - 4.0 // 4 t unaccounted for
	r.AvgSpeedKn += 1.0
	r.Lat += 1.0 // one degree typed wrong
	got := map[string]bool{}
	for _, f := range ReportChecks(r, &prev) {
		got[f.ID] = true
	}
	for _, id := range []string{"rob_vlsfo", "speed", "position"} {
		if !got[id] {
			t.Errorf("%s not flagged", id)
		}
	}
}

func TestBadTopics(t *testing.T) {
	for _, c := range []struct {
		about []string
		ok    bool
	}{{nil, false}, {[]string{"eta_notice"}, true}, {[]string{"rob_vlsfo", "speed"}, true}, {[]string{"fuel"}, false}} {
		if got := badTopics(c.about) == ""; got != c.ok {
			t.Errorf("%v: ok %v, want %v", c.about, got, c.ok)
		}
	}
}

// An empty stated reason is refused and not logged: §6 needs a reason on
// every rationale event (tuning round 2).
func TestEmptyReasonRefusedNotLogged(t *testing.T) {
	r := &run{} // no log: emitting would panic
	r.logReason("agent", opsName, &genai.FunctionCall{Name: "state_reason", Args: map[string]any{"reason": "  "}})
	out, err := r.stateReason(nil, reasonIn{Reason: ""})
	if err != nil || out["accepted"] != false {
		t.Errorf("got %v, %v", out, err)
	}
}
