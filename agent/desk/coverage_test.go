package desk

import (
	"slices"
	"strings"
	"testing"

	"github.com/captv89/shorewatch/agent/eventlog"
	"github.com/captv89/shorewatch/sim"
)

// Scene 2, run 1 (2026-10-26): the Ops Agent said its colleagues had
// checked a noon report without calling them. Code now sees the gap.
func TestUnchecked(t *testing.T) {
	ep := sim.Scene{Report: sim.NoonReport{ID: "nr-0102"}}
	ep.Voyage.CP.WarrantedVLSFOPerDayT = 24
	withCons := &Report{Extraction: Extraction{ConsVLSFOt: 27.6, ConsMGOt: 0.1}}
	noCons := &Report{Extraction: Extraction{}}
	done := func(names ...string) map[string]bool {
		m := map[string]bool{}
		for _, n := range names {
			m[n] = true
		}
		return m
	}
	cases := []struct {
		name   string
		ep     sim.Scene
		report *Report
		ran    map[string]bool
		want   []string
	}{
		{"nothing called", ep, nil, done(), []string{"submit_extraction"}},
		{"report only", ep, withCons, done(), []string{"check_eta", "check_consumption"}},
		{"all done", ep, withCons, done("check_eta", "check_consumption"), nil},
		{"figures missing: no consumption check", ep, noCons, done("check_eta"), nil},
		{"not a noon report", sim.Scene{}, nil, done(), nil},
	}
	for _, c := range cases {
		r := &run{ep: c.ep, report: c.report, ran: c.ran}
		var got []string
		for _, s := range r.unchecked() {
			got = append(got, s.tool)
		}
		if !slices.Equal(got, c.want) {
			t.Errorf("%s: unchecked %v, want %v", c.name, got, c.want)
		}
	}
}

// TestScene2SkippedColleagues: an Ops Agent that says the report was checked
// without calling anyone is sent back by code, and the work gets done.
func TestScene2SkippedColleagues(t *testing.T) {
	events, res, _ := recordScene2(t, &staff{skipColleagues: true})
	var sentBack, nowRun int
	for _, e := range events {
		switch {
		case strings.HasPrefix(e.Summary, "Coverage check: the noon report was not fully checked"):
			sentBack++
		case e.Summary == "Coverage check: every check on the noon report has now run":
			nowRun++
		case e.Type == eventlog.TypeAlert && e.Actor == eventlog.ActorChecker:
			t.Errorf("unexpected alert: %s", e.Summary)
		}
	}
	if sentBack != 2 || nowRun != 2 {
		t.Errorf("sent back %d times, checks run after %d; want 2 and 2 (D1 and D3)", sentBack, nowRun)
	}
	if len(res.Drafts) != 2 {
		t.Errorf("drafts: %+v", res.Drafts)
	}
}

func TestUnqueued(t *testing.T) {
	mail := Draft{About: []string{"off_hire"}}
	entry := Draft{About: []string{"off_hire"}, Entry: &Entry{Kind: "off_hire"}}
	eta := Draft{About: []string{"eta_forecast"}}
	cases := []struct {
		name   string
		q      Question
		drafts []Draft
		want   []string
	}{
		{"off-hire, both queued", Question{About: []string{"off_hire"}}, []Draft{mail, entry}, nil},
		{"off-hire, entry missing", Question{About: []string{"off_hire"}}, []Draft{mail}, []string{"propose_entry"}},
		{"off-hire, nothing", Question{About: []string{"off_hire"}}, nil, []string{"propose_entry", "draft_email"}},
		{"ETA review queued", Question{About: []string{"eta_forecast"}}, []Draft{eta}, nil},
		{"ETA review missing", Question{About: []string{"eta_forecast"}}, []Draft{mail}, []string{"draft_email"}},
		{"other needs nothing", Question{About: []string{"other"}}, nil, nil},
	}
	for _, c := range cases {
		r := &run{drafts: c.drafts}
		var got []string
		for _, s := range r.unqueued(c.q) {
			got = append(got, s.tool)
		}
		if !slices.Equal(got, c.want) {
			t.Errorf("%s: unqueued %v, want %v", c.name, got, c.want)
		}
	}
}
