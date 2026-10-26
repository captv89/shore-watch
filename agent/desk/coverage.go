package desk

import (
	"context"
	"slices"
	"strings"

	"github.com/captv89/shorewatch/agent/eventlog"
	"google.golang.org/adk/v2/agent"
	"google.golang.org/adk/v2/runner"
	"google.golang.org/genai"
)

// step is one check a noon report must get, and who does it.
type step struct{ tool, what string }

// unchecked lists the checks a noon report should have had and did not get.
// An agent can say its colleagues checked a report without calling them
// (Scene 2, run 1, decision 111); code looks at what actually ran.
func (r *run) unchecked() []step {
	if r.ep.Forecast != nil || r.ep.Report.ID == "" {
		return nil // not a noon report
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.report == nil {
		return []step{{"submit_extraction", "the Reports Agent's extraction of the report"}}
	}
	var out []step
	if !r.ran["check_eta"] {
		out = append(out, step{"check_eta", "the Voyage Agent's ETA check"})
	}
	// The Performance Agent works only on a report with all its figures,
	// against a CP that warrants a consumption.
	if len(MissingForDesk(*r.report)) == 0 && r.ep.Voyage.CP.WarrantedVLSFOPerDayT > 0 {
		if !r.ran["check_consumption"] {
			out = append(out, step{"check_consumption", "the Performance Agent's check against the warranty"})
		}
	}
	return out
}

// unqueued lists what the operator's yes to q asked for and did not reach
// the approval queue: for each topic, a draft or an entry that carries it;
// for an off-hire, both the entry and the e-mail to the owner. An agent can
// report an item done without making it (Scene 6, runs 3 and 4, decision 112).
func (r *run) unqueued(q Question) []step {
	r.mu.Lock()
	defer r.mu.Unlock()
	has := func(topic string, entry bool) bool {
		for _, d := range r.drafts {
			if (d.Entry != nil) == entry && slices.Contains(d.About, topic) {
				return true
			}
		}
		return false
	}
	var out []step
	for _, t := range q.About {
		switch {
		case t == "other":
		case t == "off_hire":
			if !has(t, true) {
				out = append(out, step{"propose_entry", "the off-hire entry for the voyage management system"})
			}
			if !has(t, false) {
				out = append(out, step{"draft_email", "the e-mail to the owner about the off-hire"})
			}
		case !has(t, false) && !has(t, true):
			out = append(out, step{"draft_email", "the draft about " + t})
		}
	}
	return out
}

func describe(s []step) string {
	var w []string
	for _, x := range s {
		w = append(w, x.what)
	}
	return strings.Join(w, "; ")
}

// sendBack is how both checks act on a gap: log it, send the Ops Agent back
// once naming it, then log that it is closed or alert the operator.
func (r *run) sendBack(ctx context.Context, rn *runner.Runner, sess, tool string, missing []step, gap, prompt, closed, alert string, recheck func() []step) error {
	r.emit(eventlog.Event{Actor: eventlog.ActorChecker, Type: eventlog.TypeCheck,
		Summary: gap + describe(missing) + ". The Ops Agent is sent back",
		Detail:  map[string]any{"tool": tool, "missing": missing, "passed": false}, SourceRefs: []string{r.emailRef}})
	// Only the missing steps: a live Ops Agent sent back for the off-hire
	// entry also asked for the owner's e-mail again, and a copy was queued.
	msg := genai.NewContentFromText(prompt+describe(missing)+". Nothing was done for them. Do only these now, following your procedure; everything else is already done, so do not ask for it again. Then finish with your summary.", genai.RoleUser)
	if err := r.consume(rn.Run(ctx, "operator", sess, msg, agent.RunConfig{})); err != nil {
		return err
	}
	if still := recheck(); len(still) > 0 {
		r.emit(eventlog.Event{Actor: eventlog.ActorChecker, Type: eventlog.TypeAlert,
			Summary: "Alert to the operator: " + alert + describe(still) + "). Please see to it by hand.",
			Detail:  map[string]any{"tool": tool, "missing": still, "about": []string{"other"}}, SourceRefs: []string{r.emailRef}})
		return nil
	}
	r.emit(eventlog.Event{Actor: eventlog.ActorChecker, Type: eventlog.TypeCheck, Summary: closed,
		Detail: map[string]any{"tool": tool, "passed": true}, SourceRefs: []string{r.emailRef}})
	return nil
}

// coverage is the Coverage check, plain code: after the Ops Agent says it is
// done with a noon report, code confirms the checks really ran.
func (r *run) coverage(ctx context.Context, rn *runner.Runner, sess string) error {
	missing := r.unchecked()
	if len(missing) == 0 {
		return nil
	}
	id := r.ep.Report.ID
	return r.sendBack(ctx, rn, sess, "coverage", missing,
		"Coverage check: the noon report was not fully checked. Not run: ",
		"Code check: for noon report "+id+" these steps did not run: ",
		"Coverage check: every check on the noon report has now run",
		"noon report "+id+" was not fully checked by the agents (not run: ", r.unchecked)
}

// followThrough is the Follow-through check, plain code: after the operator
// says yes, code confirms that what was asked for reached the approval queue.
func (r *run) followThrough(ctx context.Context, rn *runner.Runner, sess string, q Question) error {
	missing := r.unqueued(q)
	if len(missing) == 0 {
		return nil
	}
	return r.sendBack(ctx, rn, sess, "follow_through", missing,
		"Follow-through check: the operator said yes, but not in the approval queue: ",
		"Code check: the operator said yes, but these are not in the approval queue: ",
		"Follow-through check: everything the operator agreed to is now in the approval queue",
		"the work you agreed to did not reach the approval queue (missing: ", func() []step { return r.unqueued(q) })
}
