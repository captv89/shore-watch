// Package eval holds the eval's tasks, their answer keys and the scorer.
// Grading is by code, on decisions only: which topics the agents raised
// with the operator or the Master, and whether a message was blocked and
// why. No model marks another model.
package eval

import (
	"slices"
	"time"

	"github.com/captv89/shorewatch/sim"
)

// Sets keep tuning tasks, used while writing prompts, apart from the
// held-out tasks the page reports.
const (
	Tuning  = "tuning"
	HeldOut = "heldout"
)

// Task is one incoming e-mail with its world and answer key.
type Task struct {
	ID   string `json:"id"`
	Set  string `json:"set"`
	Kind string `json:"kind"`
	// What was planted, in plain words, for the misses list on the page.
	Planted string    `json:"planted"`
	Episode sim.Scene `json:"-"`
	// Weather and forecast files the task needs, if any.
	Weather  string `json:"weather,omitempty"`
	Forecast string `json:"forecast,omitempty"`
	Key      Key    `json:"key"`
	// Fleet-week tasks: the ship, when the item reaches the
	// desk, and whether the agents run it in the sample.
	Ship   string    `json:"ship,omitempty"`
	At     time.Time `json:"at,omitzero"`
	Sample bool      `json:"sample,omitempty"`
	// rules is what the rules-only baseline decides, worked out at build;
	// coded is what it decides with a coded report.
	rules, coded Decisions
}

// Key is what a correct desk does with the task's e-mail.
type Key struct {
	// Topics must each be raised: a question to the operator, a draft or an
	// alert tagged with it (desk.Topics).
	Topics []string `json:"topics"`
	// Allowed may be raised without counting as a false alarm.
	Allowed []string `json:"allowed,omitempty"`
	Block   bool     `json:"block"`
	// Reasons are the reason kinds a block must give.
	Reasons []string `json:"reasons,omitempty"`
	// OffHire, if set, is the off-hire period a proposed entry must give,
	// as "HHMM-HHMM" UTC, with " 1/4" after it for cranes out of use.
	OffHire string `json:"off_hire,omitempty"`
}

// Decisions are what a run did, as the scorer sees it.
type Decisions struct {
	Topics  []string `json:"topics"`
	Blocked bool     `json:"blocked"`
	Reasons []string `json:"reasons,omitempty"`
	// OffHire are the periods of proposed off-hire entries, as "HHMM-HHMM"
	// UTC, with " 1/4" after it for cranes out of use.
	OffHire []string `json:"off_hire,omitempty"`
}

// Grade is one task's result.
type Grade struct {
	TaskID string `json:"task_id"`
	Pass   bool   `json:"pass"`
	// Missed: required topics not raised, a block not made, block reasons not given.
	Missed []string `json:"missed,omitempty"`
	// FalseAlarms: topics raised that the task did not call for, or a block it did not.
	FalseAlarms []string `json:"false_alarms,omitempty"`
}

// Score compares a run's decisions with the task's key.
func Score(t Task, d Decisions) Grade {
	g := Grade{TaskID: t.ID}
	for _, k := range t.Key.Topics {
		if !slices.Contains(d.Topics, k) {
			g.Missed = append(g.Missed, k)
		}
	}
	for _, k := range d.Topics {
		if !slices.Contains(t.Key.Topics, k) && !slices.Contains(t.Key.Allowed, k) {
			g.FalseAlarms = append(g.FalseAlarms, k)
		}
	}
	switch {
	case t.Key.Block && !d.Blocked:
		g.Missed = append(g.Missed, "block")
	case !t.Key.Block && d.Blocked:
		g.FalseAlarms = append(g.FalseAlarms, "block")
	}
	if t.Key.Block && d.Blocked {
		for _, r := range t.Key.Reasons {
			if !slices.Contains(d.Reasons, r) {
				g.Missed = append(g.Missed, "reason:"+r)
			}
		}
	}
	if t.Key.OffHire != "" && len(d.OffHire) > 0 && !slices.Equal(d.OffHire, []string{t.Key.OffHire}) {
		g.Missed = append(g.Missed, "off_hire_period")
	}
	g.Pass = len(g.Missed) == 0 && len(g.FalseAlarms) == 0
	return g
}

// hours is a shorthand for durations in task builders.
func hours(h float64) time.Duration { return time.Duration(h * float64(time.Hour)) }
