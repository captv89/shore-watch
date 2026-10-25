package eval

import (
	"regexp"

	"github.com/captv89/shorewatch/agent/checker"
	"github.com/captv89/shorewatch/sim"
)

// Baselines. DoNothing raises nothing. RulesOnly is the
// desk with every Checker rule and keyword rules for e-mail, and no AI: it
// is what a programmed if/else desk would do. It is meant to
// be a fair opponent, not a straw man: its keyword rules on the remarks were
// written from the tuning tasks only, as the prompts were.
// RulesCoded is the same desk given a coded report: the extra fields in
// CodedFields, which the crew would have to fill in.
const (
	DoNothing  = "do-nothing"
	RulesOnly  = "rules-only"
	RulesCoded = "rules-coded"
)

// CodedFields are the fields a coded report adds to the noon report so that
// rules can do what the agents do from the Master's remarks. Shore Watch's
// report has 14 fields; DNV OVD 3.13's Log Abstract has 409.
var CodedFields = []string{
	"Main engine stop from (UTC)",
	"Main engine stop to (UTC)",
	"Stop reason (code)",
	"Stop ordered by (code)",
	"Speed given over steaming time (yes/no)",
	"ROB correction VLSFO (t)",
	"ROB correction MGO (t)",
	"Heavy weather precautions taken (yes/no)",
}

// Keyword rules on the remarks, from the tuning tasks' wording.
var (
	speedOverSteamingRe = regexp.MustCompile(`(?i)average speed (given )?over ([\d.]+ h )?steaming`)
	robCorrectedRe      = regexp.MustCompile(`(?i)ROB corrected`)
	stopRe              = regexp.MustCompile(`(?i)main engine stopped (\d{4})-(\d{4})`)
	breakdownRe         = regexp.MustCompile(`(?i)choked|repair|defect|failure|breakdown`)
	chartererOrderRe    = regexp.MustCompile(`(?i)charterers'? (orders|instructions)|awaiting orders`)
)

// Keyword rules a programmer would write for e-mail.
var (
	bankChangeRe = regexp.MustCompile(`(?i)\bIBAN\b|bank (details|account)[^.]*(changed|new)|new (bank )?account|bank has changed`)
	toAIRe       = regexp.MustCompile(`(?i)\b(AI|automated) assistant\s*:`)
)

// Baseline returns a baseline's decisions on a task.
func Baseline(name string, t Task) Decisions {
	switch name {
	case RulesOnly:
		return t.rules
	case RulesCoded:
		return t.coded
	}
	return Decisions{Topics: []string{}}
}

// rulesEmail screens an e-mail that is not a noon report: the guardrail's
// sender check blocks unknown and lookalike senders; from a known sender, a
// bank change is raised with the operator.
func rulesEmail(sc sim.Scene) Decisions {
	d := Decisions{Topics: []string{}}
	s := checker.CheckSender(sc.Email.From, sc.Voyage.Contacts)
	bank := bankChangeRe.MatchString(sc.Email.Body)
	if s.Known {
		if bank {
			d.Topics = append(d.Topics, "payment_change")
		}
		return d
	}
	d.Blocked = true
	if s.LookalikeOf != nil {
		d.Reasons = append(d.Reasons, "lookalike_sender")
	} else {
		d.Reasons = append(d.Reasons, "unknown_sender")
	}
	if bank {
		d.Reasons = append(d.Reasons, "bank_details_change")
	}
	if toAIRe.MatchString(sc.Email.Body) {
		d.Reasons = append(d.Reasons, "instruction_to_ai")
	}
	return d
}
