package checker

import (
	"testing"
	"time"
)

func TestCheckSpeedPlan(t *testing.T) {
	from := time.Date(2025, 1, 23, 8, 0, 0, 0, time.UTC)
	berth := time.Date(2025, 1, 27, 6, 0, 0, 0, time.UTC)
	arrive := time.Date(2025, 1, 27, 4, 0, 0, 0, time.UTC)
	for name, c := range map[string]struct {
		dist, adviceDist, kn float64
		arrive               time.Time
		want                 bool
	}{
		"adds up":            {763.6, 763.6, 8.3, arrive, true},
		"distance disagrees": {763.6, 700, 8.3, arrive, false},
		"arrival disagrees":  {763.6, 763.6, 8.3, arrive.Add(-3 * time.Hour), false},
		"after the berth":    {763.6, 763.6, 7.9, berth.Add(4 * time.Hour), false},
	} {
		r, err := CheckSpeedPlan(from, c.dist, c.adviceDist, c.kn, c.arrive, berth)
		if err != nil {
			t.Fatalf("%s: %v", name, err)
		}
		if r.Consistent != c.want {
			t.Errorf("%s: consistent %v, want %v (%+v)", name, r.Consistent, c.want, r)
		}
	}
	if _, err := CheckSpeedPlan(from, 763.6, 763.6, 0, arrive, berth); err == nil {
		t.Error("zero speed accepted")
	}
}
