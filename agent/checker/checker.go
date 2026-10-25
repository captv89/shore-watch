// Package checker holds the deterministic checks: plain code, no LLM.
// Every number the agent may show comes from a tool or from these checks.
package checker

import (
	"fmt"
	"math"
	"slices"
	"strings"
	"time"

	"github.com/captv89/shorewatch/sim"
)

// ETAToleranceHours is the company's tolerance for a Master's ETA against
// distance to go divided by the reported speed. Fictional, like the CP.
const ETAToleranceHours = 2.0

// ETAResult compares the Master's ETA with distance / speed.
type ETAResult struct {
	DistanceNM     float64   `json:"distance_to_go_nm"`
	SpeedKn        float64   `json:"speed_kn"`
	HoursToGo      float64   `json:"hours_to_go"`
	ComputedETA    time.Time `json:"computed_eta"`
	MasterETA      time.Time `json:"master_eta"`
	DifferenceH    float64   `json:"difference_hours"` // Master minus computed
	ToleranceH     float64   `json:"tolerance_hours"`
	Consistent     bool      `json:"consistent"`
	Interpretation string    `json:"interpretation"`
}

// CheckETA computes the ETA from distance and speed and compares it with the Master's.
func CheckETA(reportTime, masterETA time.Time, distanceNM, speedKn float64) (ETAResult, error) {
	if speedKn <= 0 {
		return ETAResult{}, fmt.Errorf("speed %.1f kn: cannot compute an ETA", speedKn)
	}
	hours := distanceNM / speedKn
	computed := reportTime.Add(time.Duration(hours * float64(time.Hour))).Truncate(time.Minute)
	diff := masterETA.Sub(computed).Hours()
	r := ETAResult{
		DistanceNM:  distanceNM,
		SpeedKn:     speedKn,
		HoursToGo:   round1(hours),
		ComputedETA: computed,
		MasterETA:   masterETA,
		DifferenceH: round1(diff),
		ToleranceH:  ETAToleranceHours,
		Consistent:  math.Abs(diff) <= ETAToleranceHours,
	}
	if r.Consistent {
		r.Interpretation = fmt.Sprintf("Master's ETA is consistent: within %.0f h of distance to go divided by reported speed.", ETAToleranceHours)
	} else {
		r.Interpretation = fmt.Sprintf("Master's ETA differs by more than %.0f h from distance to go divided by reported speed. Ask the Master to confirm.", ETAToleranceHours)
	}
	return r, nil
}

// NoticeResult says whether an ETA notice is due under the CP terms.
type NoticeResult struct {
	DaysToETA        float64   `json:"days_to_eta"`
	ThresholdDue     int       `json:"threshold_due_days"` // 0 if none
	LastNoticeETA    time.Time `json:"last_notice_eta"`
	LastNoticeDays   int       `json:"last_notice_days"`
	ETAMovedHours    float64   `json:"eta_moved_hours"`
	ChangeLimitHours float64   `json:"change_limit_hours"`
	Due              bool      `json:"due"`
	Reasons          []string  `json:"reasons"`
}

// CheckNotices applies the CP notice rules at a noon report.
//
// A day-threshold notice is due at the noon report before which it passes:
// the threshold lies between now and the next report (24 h later), and that
// notice has not been sent. Thresholds already passed are not due again. A change notice is due when the ETA has moved more than the CP limit
// since the last notice. One e-mail covers both.
func CheckNotices(reportTime, eta time.Time, cp sim.CPTerms, sent []sim.Notice) NoticeResult {
	days := eta.Sub(reportTime).Hours() / 24
	r := NoticeResult{DaysToETA: round2(days), ChangeLimitHours: cp.ETAChangeHours, Reasons: []string{}}
	already := map[int]bool{}
	for _, n := range sent {
		already[n.DaysLabel] = true
	}
	for _, nd := range cp.NoticeDays { // largest first
		if !already[nd] && days-1 < float64(nd) && float64(nd) <= days {
			r.ThresholdDue = nd
			r.Reasons = append(r.Reasons, fmt.Sprintf("%d-day ETA notice due: %.1f days to ETA, so the %d-day point passes before the next noon report.", nd, days, nd))
			break
		}
	}
	if len(sent) > 0 {
		last := sent[len(sent)-1]
		r.LastNoticeETA, r.LastNoticeDays = last.ETA, last.DaysLabel
		r.ETAMovedHours = round1(eta.Sub(last.ETA).Hours())
		if math.Abs(r.ETAMovedHours) > cp.ETAChangeHours {
			r.Reasons = append(r.Reasons, fmt.Sprintf("ETA has moved %.0f h since the last notice; the CP limit is %.0f h.", r.ETAMovedHours, cp.ETAChangeHours))
		}
	}
	r.Due = len(r.Reasons) > 0
	return r
}

// SenderResult is the inbound guardrail's view of a sender.
type SenderResult struct {
	From    string       `json:"from"`
	Domain  string       `json:"domain"`
	Known   bool         `json:"known"`
	Contact *sim.Contact `json:"contact,omitempty"`
	// LookalikeOf is the known contact whose domain the sender's imitates:
	// one character changed, added or removed.
	LookalikeOf *sim.Contact `json:"lookalike_of,omitempty"`
	// DiffAt is the index in Domain where it first differs from the real one.
	DiffAt *int `json:"diff_at,omitempty"`
}

// CheckSender matches the sender's domain against the verified contact list,
// and flags a domain one character away from a known one.
func CheckSender(from string, contacts []sim.Contact) SenderResult {
	r := SenderResult{From: from, Domain: domain(from)}
	for i, c := range contacts {
		if strings.EqualFold(r.Domain, c.Domain) {
			r.Known, r.Contact = true, &contacts[i]
			return r
		}
	}
	for i, c := range contacts {
		if at, ok := oneEditAway(r.Domain, strings.ToLower(c.Domain)); ok {
			r.LookalikeOf, r.DiffAt = &contacts[i], &at
			return r
		}
	}
	return r
}

// oneEditAway reports whether a and b differ by exactly one character
// changed, added or removed, and where in a the difference starts.
func oneEditAway(a, b string) (int, bool) {
	ra, rb := []rune(a), []rune(b)
	if len(ra)-len(rb) > 1 || len(rb)-len(ra) > 1 || a == b {
		return 0, false
	}
	i := 0
	for i < len(ra) && i < len(rb) && ra[i] == rb[i] {
		i++
	}
	switch {
	case len(ra) == len(rb):
		return i, string(ra[i+1:]) == string(rb[i+1:])
	case len(ra) > len(rb):
		return i, string(ra[i+1:]) == string(rb[i:])
	default:
		return i, string(ra[i:]) == string(rb[i+1:])
	}
}

// RecipientResult lists recipients not in the verified contact list.
type RecipientResult struct {
	To      []string `json:"to"`
	Unknown []string `json:"unknown"`
	OK      bool     `json:"ok"`
}

// CheckRecipients requires every recipient address to be a known contact.
func CheckRecipients(to []string, contacts []sim.Contact) RecipientResult {
	r := RecipientResult{To: to, Unknown: []string{}}
	for _, addr := range to {
		if !slices.ContainsFunc(contacts, func(c sim.Contact) bool { return strings.EqualFold(c.Email, strings.TrimSpace(addr)) }) {
			r.Unknown = append(r.Unknown, addr)
		}
	}
	r.OK = len(to) > 0 && len(r.Unknown) == 0
	return r
}

func domain(addr string) string {
	addr = strings.TrimSpace(addr)
	if i := strings.LastIndexByte(addr, '<'); i >= 0 {
		addr = strings.TrimSuffix(addr[i+1:], ">")
	}
	if i := strings.LastIndexByte(addr, '@'); i >= 0 {
		return strings.ToLower(addr[i+1:])
	}
	return ""
}

func round1(v float64) float64 { return math.Round(v*10) / 10 }
func round2(v float64) float64 { return math.Round(v*100) / 100 }
