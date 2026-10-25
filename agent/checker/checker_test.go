package checker

import (
	"slices"
	"testing"
	"time"

	"github.com/captv89/shorewatch/sim"
)

var noon = time.Date(2025, 1, 22, 12, 0, 0, 0, time.UTC)

func TestCheckETA(t *testing.T) {
	cases := []struct {
		name       string
		master     time.Time
		nm, kn     float64
		consistent bool
	}{
		{"agrees", time.Date(2025, 1, 25, 21, 0, 0, 0, time.UTC), 1009.1, 12.4, true},
		{"3 h early", time.Date(2025, 1, 25, 18, 0, 0, 0, time.UTC), 1009.1, 12.4, false},
		{"at tolerance", noon.Add(100*time.Hour + 2*time.Hour), 1000, 10, true},
	}
	for _, c := range cases {
		r, err := CheckETA(noon, c.master, c.nm, c.kn)
		if err != nil {
			t.Fatal(err)
		}
		if r.Consistent != c.consistent {
			t.Errorf("%s: consistent %v (diff %.1f h), want %v", c.name, r.Consistent, r.DifferenceH, c.consistent)
		}
	}
	if _, err := CheckETA(noon, noon, 100, 0); err == nil {
		t.Error("want error for zero speed")
	}
}

func TestCheckNotices(t *testing.T) {
	cp := sim.DefaultCP()
	eta := time.Date(2025, 1, 25, 21, 0, 0, 0, time.UTC)
	sent := []sim.Notice{
		{DaysLabel: 10, ETA: time.Date(2025, 1, 25, 8, 0, 0, 0, time.UTC)},
		{DaysLabel: 7, ETA: time.Date(2025, 1, 25, 8, 0, 0, 0, time.UTC)},
		{DaysLabel: 5, ETA: time.Date(2025, 1, 25, 8, 0, 0, 0, time.UTC)},
	}
	r := CheckNotices(noon, eta, cp, sent)
	if !r.Due || r.ThresholdDue != 3 || r.ETAMovedHours != 13 || len(r.Reasons) != 2 {
		t.Errorf("scene 1: got %+v; want 3-day due and 13 h change", r)
	}

	// 4.5 days out, 5-day already sent, ETA unchanged: nothing due.
	quiet := CheckNotices(noon, noon.Add(108*time.Hour), cp, []sim.Notice{{DaysLabel: 5, ETA: noon.Add(108 * time.Hour)}})
	if quiet.Due {
		t.Errorf("want no notice due, got %v", quiet.Reasons)
	}

	// Change of exactly the limit is not "more than".
	limit := CheckNotices(noon, noon.Add(200*time.Hour), cp, []sim.Notice{{DaysLabel: 10, ETA: noon.Add(188 * time.Hour)}})
	if slices.ContainsFunc(limit.Reasons, func(s string) bool { return s[:3] == "ETA" }) {
		t.Errorf("12 h change should not trigger a notice: %v", limit.Reasons)
	}
}

func TestSenderAndRecipients(t *testing.T) {
	contacts := []sim.Contact{{Role: "agent", Email: "ops@maasport-agencies.example", Domain: "maasport-agencies.example"}}
	if !CheckSender("Ops <ops@maasport-agencies.example>", contacts).Known {
		t.Error("known sender not recognised")
	}
	if CheckSender("ops@maasp0rt-agencies.example", contacts).Known {
		t.Error("lookalike domain accepted")
	}
	r := CheckRecipients([]string{"ops@maasport-agencies.example", "x@unknown.example"}, contacts)
	if r.OK || !slices.Equal(r.Unknown, []string{"x@unknown.example"}) {
		t.Errorf("recipients: %+v", r)
	}
	if CheckRecipients(nil, contacts).OK {
		t.Error("empty recipient list accepted")
	}
}

func TestNumberGuard(t *testing.T) {
	k := NewKnown()
	k.AddText("Position: 40-30.0N 011-05.3W\nAverage speed last 24h: 12.4 kn\nmaster@valdacrest.sim-fleet.example")
	k.AddValue(map[string]any{"distance_nm": 1009.1, "eta": "2025-01-25T21:00:00Z", "moved_hours": 13.0, "days": []int{10, 7, 5, 3}})

	ok := "ETA Rotterdam 25 Jan 2025 2100 UTC (2025-01-25 21:00). Distance to go 1,009.1 nm at 12.4 kn; ETA moved 13 h. This is the 3-day notice. Position 40-30.0N 011-05.3W."
	if u := k.Unsupported(ok); len(u) > 0 {
		t.Errorf("supported text flagged: %v", u)
	}
	bad := "Steaming time approx. 79.2 hours."
	if u := k.Unsupported(bad); !slices.Equal(u, []string{"79.2"}) {
		t.Errorf("got %v, want [79.2]", u)
	}
	if u := k.Unsupported("Contact ops via master@valdacrest.sim-fleet.example"); len(u) > 0 {
		t.Errorf("digits inside e-mail addresses flagged: %v", u)
	}
}

func TestCheckSenderLookalike(t *testing.T) {
	real := sim.Contact{Role: "bunker_supplier", Domain: "solentbunker.example"}
	cases := []struct {
		from string
		at   int
		hit  bool
	}{
		{"accounts@so1entbunker.example", 2, true},
		{"Accounts <accounts@solentbunkers.example>", 12, true},
		{"accounts@solentbunker.example", 0, false},
		{"accounts@northwind-chartering.example", 0, false},
	}
	for _, c := range cases {
		r := CheckSender(c.from, []sim.Contact{real})
		if got := r.LookalikeOf != nil; got != c.hit || (c.hit && *r.DiffAt != c.at) {
			t.Errorf("%s: lookalike %v at %v, want %v at %d", c.from, got, r.DiffAt, c.hit, c.at)
		}
	}
}
