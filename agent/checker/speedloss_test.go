package checker

import (
	"math"
	"testing"
)

// tormar is TORMAR SPRING in ballast, as in sim.Scene2.
var tormar = Hull{LppM: 217, Cb: 0.80, DisplacementM3: 39200, Ballast: true}

func TestKwonLoss(t *testing.T) {
	// Expected values worked by hand from Yang et al. (2020) Tables 2-4
	// for Lpp 217 m, Cb 0.80 ballast, ∇ 39,200 m³, 12.5 kn (Fn 0.139).
	for _, c := range []struct {
		bf     int
		rel    float64
		want   float64
		beyond bool
	}{
		{4, 0, 1.7, false},
		{6, 10, 12.6, false},
		{7, 0, 32.3, false},
		{7, 45, 23.1, false},
		{7, 90, 13.6, false},
		{7, 135, 13.6, false}, // on the quarter is Kwon's beam
		{7, 170, 6.0, false},
		{10, 0, 32.3, true}, // counted at Beaufort 7
		{3, 180, 0, false},  // a following breeze gains a little; not modelled
	} {
		got, err := KwonLoss(tormar, 12.5, c.bf, c.rel)
		if err != nil {
			t.Fatal(err)
		}
		if math.Abs(got.Pct-c.want) > 0.05 || got.BeyondModel != c.beyond {
			t.Errorf("Bf %d at %.0f°: %.2f%% beyond=%v, want %.1f%% beyond=%v", c.bf, c.rel, got.Pct, got.BeyondModel, c.want, c.beyond)
		}
	}
	if _, err := KwonLoss(Hull{LppM: 217, Cb: 0.82, DisplacementM3: 39200, Ballast: true}, 12.5, 5, 0); err == nil {
		t.Error("a block coefficient between table rows should be refused")
	}
}
