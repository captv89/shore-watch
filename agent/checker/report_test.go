package checker

import (
	"testing"
	"time"
)

func TestCheckROB(t *testing.T) {
	cases := []struct {
		name            string
		prev, cons, now float64
		gap             float64
		flagged         bool
	}{
		{"adds up", 742.0, 26.0, 716.0, 0, false},
		{"within 1 t", 742.0, 26.0, 716.8, -0.8, false},
		{"1.5 t short", 742.0, 26.0, 714.5, 1.5, true},
		{"ROB went up (bunkered?)", 400.0, 26.0, 724.0, -350.0, true},
		{"2% of a large figure", 1000.0, 80.0, 921.4, -1.4, false},
	}
	for _, c := range cases {
		r := CheckROB("VLSFO", c.prev, c.cons, c.now)
		if r.GapT != c.gap || r.Flagged != c.flagged {
			t.Errorf("%s: gap %.1f flagged %v, want %.1f %v", c.name, r.GapT, r.Flagged, c.gap, c.flagged)
		}
	}
}

func TestCheckSpeedDistance(t *testing.T) {
	prev := time.Date(2025, 1, 21, 12, 0, 0, 0, time.UTC)
	cases := []struct {
		name     string
		hours    float64
		run, kn  float64
		implied  float64
		mismatch bool
		tooFast  bool
	}{
		{"24 h, consistent", 24, 297.6, 12.4, 12.4, false, false},
		{"25 h day (clocks back)", 25, 310.0, 12.4, 12.4, false, false},
		{"speed does not match", 24, 297.6, 13.4, 12.4, true, false},
		{"impossible speed", 24, 432.0, 18.0, 18.0, false, true},
	}
	for _, c := range cases {
		now := prev.Add(time.Duration(c.hours * float64(time.Hour)))
		r, err := CheckSpeedDistance(prev, now, c.run, c.kn)
		if err != nil {
			t.Fatal(err)
		}
		if r.ImpliedKn != c.implied || r.Mismatch != c.mismatch || r.TooFast != c.tooFast {
			t.Errorf("%s: implied %.1f mismatch %v too fast %v, want %.1f %v %v", c.name, r.ImpliedKn, r.Mismatch, r.TooFast, c.implied, c.mismatch, c.tooFast)
		}
	}
	if _, err := CheckSpeedDistance(prev, prev, 100, 12); err == nil {
		t.Error("want error when the reports are not in time order")
	}
}

func TestCheckPosition(t *testing.T) {
	// 300 nm due north along a meridian is 5 degrees of latitude.
	cases := []struct {
		name    string
		lat     float64
		run     float64
		flagged bool
	}{
		{"matches", 45.0, 300, false},
		{"route longer than the straight line", 44.5, 300, false},
		{"one degree typed wrong", 46.0, 300, true},
		{"ship hardly moved", 41.0, 300, true},
	}
	for _, c := range cases {
		r := CheckPosition(40.0, -10.0, c.lat, -10.0, c.run)
		if r.Flagged != c.flagged {
			t.Errorf("%s: straight line %.1f nm flagged %v, want %v", c.name, r.StraightLineNM, r.Flagged, c.flagged)
		}
	}
}
