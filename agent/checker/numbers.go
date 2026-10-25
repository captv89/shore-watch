package checker

import (
	"encoding/json"
	"math"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"sync"
	"time"
)

var (
	emailRe     = regexp.MustCompile(`[\w.+-]+@[\w.-]+`)
	thousandsRe = regexp.MustCompile(`(\d),(\d{3})`)
	numberRe    = regexp.MustCompile(`\d+(?:\.\d+)?`)
)

// NumericTokens returns the numbers written in text. E-mail addresses are
// ignored and thousands separators removed ("1,009.1" is 1009.1).
func NumericTokens(text string) []string {
	text = emailRe.ReplaceAllString(text, " ")
	for thousandsRe.MatchString(text) {
		text = thousandsRe.ReplaceAllString(text, "$1$2")
	}
	return numberRe.FindAllString(text, -1)
}

// Known is the set of numbers that tools, checks and incoming messages
// have produced in this run. The number guard allows only these in what
// the agent writes (principle 2: numbers come only from tools).
type Known struct {
	mu   sync.Mutex
	nums map[string]bool
}

// NewKnown returns an empty set.
func NewKnown() *Known { return &Known{nums: map[string]bool{}} }

// AddText records every number in text.
func (k *Known) AddText(text string) {
	k.mu.Lock()
	defer k.mu.Unlock()
	for _, t := range NumericTokens(text) {
		k.addToken(t)
	}
}

// AddValue records every number and time in a JSON-like value.
func (k *Known) AddValue(v any) {
	b, err := json.Marshal(v)
	if err != nil {
		return
	}
	var generic any
	if err := json.Unmarshal(b, &generic); err != nil {
		return
	}
	k.mu.Lock()
	defer k.mu.Unlock()
	k.walk(generic)
}

func (k *Known) walk(v any) {
	switch x := v.(type) {
	case map[string]any:
		for _, e := range x {
			k.walk(e)
		}
	case []any:
		for _, e := range x {
			k.walk(e)
		}
	case float64:
		// Text never carries the sign ("011-05.3W"), so store the magnitude.
		k.addToken(strconv.FormatFloat(math.Abs(x), 'f', -1, 64))
	case string:
		if t, err := time.Parse(time.RFC3339, x); err == nil {
			k.addTime(t)
		}
		for _, tok := range NumericTokens(x) {
			k.addToken(tok)
		}
	}
}

// addTime records the usual ways a time is written in an e-mail.
func (k *Known) addTime(t time.Time) {
	t = t.UTC()
	for _, layout := range []string{"2006-01-02 1504", "2006-01-02 15:04", "02 Jan 2006 1504", "02 Jan 2006 15:04", "02/01/2006", "2 January 2006", "January 2"} {
		for _, tok := range NumericTokens(t.Format(layout)) {
			k.addToken(tok)
		}
	}
}

// addToken records a number in its canonical form, so "13.0" and "13" match.
func (k *Known) addToken(tok string) {
	k.nums[canonical(tok)] = true
}

func canonical(tok string) string {
	f, err := strconv.ParseFloat(tok, 64)
	if err != nil {
		return tok
	}
	return strconv.FormatFloat(f, 'f', -1, 64)
}

// Unsupported returns the numbers in text that no tool or message produced.
func (k *Known) Unsupported(text string) []string {
	k.mu.Lock()
	defer k.mu.Unlock()
	var out []string
	for _, tok := range NumericTokens(text) {
		if !k.nums[canonical(tok)] && !slices.Contains(out, tok) {
			out = append(out, tok)
		}
	}
	return out
}

// Summary is a short description of a number-guard failure.
func Summary(unsupported []string) string {
	return "numbers no tool produced: " + strings.Join(unsupported, ", ")
}
