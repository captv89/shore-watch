package checker

import (
	"testing"
	"time"
)

func TestCheckStoppage(t *testing.T) {
	prev := time.Date(2025, 1, 25, 12, 0, 0, 0, time.UTC)
	now := prev.Add(24 * time.Hour)
	from := time.Date(2025, 1, 26, 6, 10, 0, 0, time.UTC)
	to := time.Date(2025, 1, 26, 11, 40, 0, 0, time.UTC)
	for name, c := range map[string]struct {
		from, to time.Time
		dist     float64
		want     bool
	}{
		"agrees":            {from, to, 231.3, true},
		"distance too long": {from, to, 280, false},
		"before the day":    {from.Add(-24 * time.Hour), to.Add(-24 * time.Hour), 231.3, false},
	} {
		r, err := CheckStoppage(prev, now, c.from, c.to, c.dist, 12.5)
		if err != nil {
			t.Fatalf("%s: %v", name, err)
		}
		if r.Consistent != c.want {
			t.Errorf("%s: consistent %v, want %v (%+v)", name, r.Consistent, c.want, r)
		}
	}
	if r, _ := CheckStoppage(prev, now, from, to, 231.3, 12.5); r.StopHours != 5.5 || r.SteamingHours != 18.5 {
		t.Errorf("stop %.2f h, steaming %.2f h; want 5.5 and 18.5", r.StopHours, r.SteamingHours)
	}
	if _, err := CheckStoppage(prev, now, to, from, 231.3, 12.5); err == nil {
		t.Error("backwards stop accepted")
	}
}
