package eval

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"slices"
	"time"
)

// logEvent is the part of a §6 event the scorer reads.
type logEvent struct {
	Actor  string          `json:"actor"`
	Type   string          `json:"type"`
	Raw    json.RawMessage `json:"detail"`
	Detail struct {
		About    []string `json:"about"`
		Question struct {
			About []string `json:"about"`
		} `json:"question"`
		Draft struct {
			About []string `json:"about"`
			Entry *struct {
				Kind      string    `json:"kind"`
				From      time.Time `json:"from"`
				To        time.Time `json:"to"`
				CranesOut int       `json:"cranes_out"`
				Cranes    int       `json:"cranes"`
			} `json:"entry"`
		} `json:"draft"`
		Reasons []struct {
			Kind string `json:"kind"`
		} `json:"reasons"`
	} `json:"-"`
}

// ReadDecisions reads a run's event log: the topics of every question,
// draft, proposed entry and agent alert, and any block with its reason kinds. Alerts the
// guardrail raises are code, not a decision, and are not counted.
func ReadDecisions(path string) (Decisions, error) {
	f, err := os.Open(path)
	if err != nil {
		return Decisions{}, err
	}
	defer f.Close()
	var d Decisions
	add := func(list *[]string, items ...string) {
		for _, it := range items {
			if !slices.Contains(*list, it) {
				*list = append(*list, it)
			}
		}
	}
	sc := bufio.NewScanner(f)
	sc.Buffer(make([]byte, 0, 1<<20), 16<<20)
	for n := 1; sc.Scan(); n++ {
		var e logEvent
		if err := json.Unmarshal(sc.Bytes(), &e); err != nil {
			return Decisions{}, fmt.Errorf("%s line %d: %w", path, n, err)
		}
		// Detail's shape depends on the type (an answer's question is
		// text), so it is read only for the types scored.
		switch e.Type {
		case "question", "draft", "entry", "alert", "blocked":
			if err := json.Unmarshal(e.Raw, &e.Detail); err != nil {
				return Decisions{}, fmt.Errorf("%s line %d: %w", path, n, err)
			}
		default:
			continue
		}
		switch {
		case e.Type == "question":
			add(&d.Topics, e.Detail.Question.About...)
		case e.Type == "draft" || e.Type == "entry":
			add(&d.Topics, e.Detail.Draft.About...)
			if en := e.Detail.Draft.Entry; en != nil && en.Kind == "off_hire" {
				add(&d.OffHire, OffHirePeriod(en.From, en.To, en.CranesOut, en.Cranes))
			}
		case e.Type == "alert" && (e.Actor == "agent" || e.Actor == "subagent"):
			add(&d.Topics, e.Detail.About...)
		case e.Type == "blocked":
			d.Blocked = true
			for _, r := range e.Detail.Reasons {
				add(&d.Reasons, r.Kind)
			}
		}
	}
	slices.Sort(d.Topics)
	return d, sc.Err()
}

// OffHirePeriod writes an off-hire period as the key gives it: "HHMM-HHMM"
// UTC, and " 1/4" for one crane of four out of use.
func OffHirePeriod(from, to time.Time, cranesOut, cranes int) string {
	p := from.UTC().Format("1504") + "-" + to.UTC().Format("1504")
	if cranesOut > 0 && cranes > 0 {
		p += fmt.Sprintf(" %d/%d", cranesOut, cranes)
	}
	return p
}
