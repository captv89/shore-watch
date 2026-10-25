package desk

import (
	"fmt"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/captv89/shorewatch/agent/checker"
	"github.com/captv89/shorewatch/sim"
)

// Extraction is what the Reader submits: values copied from the e-mail.
type Extraction struct {
	ShipName       string   `json:"ship_name"`
	ReportTimeText string   `json:"report_time" jsonschema:"date and time of the report as written"`
	PositionText   string   `json:"position" jsonschema:"position as written, e.g. 40-30.0N 011-05.3W"`
	DistanceRunNM  float64  `json:"distance_run_nm"`
	AvgSpeedKn     float64  `json:"avg_speed_kn"`
	ROBVLSFOt      float64  `json:"rob_vlsfo_t"`
	ROBMGOt        float64  `json:"rob_mgo_t"`
	ConsVLSFOt     float64  `json:"cons_vlsfo_t,omitempty" jsonschema:"VLSFO used in the last 24 h, as written; leave out if the report does not give it"`
	ConsMGOt       float64  `json:"cons_mgo_t,omitempty" jsonschema:"MGO used in the last 24 h, as written; leave out if the report does not give it"`
	ETABasisKn     float64  `json:"eta_basis_kn,omitempty" jsonschema:"speed the Master's ETA is based on, as written; leave out if the report does not give it"`
	NextPort       string   `json:"next_port"`
	MasterETAText  string   `json:"master_eta" jsonschema:"Master's ETA at the next port, as written"`
	Remarks        string   `json:"remarks"`
	MissingFields  []string `json:"missing_fields"`
}

// Report is a validated extraction with parsed values.
type Report struct {
	Extraction
	Lat        float64   `json:"lat"`
	Lon        float64   `json:"lon"`
	ReportTime time.Time `json:"report_time_utc"`
	MasterETA  time.Time `json:"master_eta_utc"`
}

var positionRe = regexp.MustCompile(`^\s*(\d{1,2})-(\d{1,2}(?:\.\d+)?)\s*([NS])[\s,]+(\d{1,3})-(\d{1,2}(?:\.\d+)?)\s*([EW])\s*$`)

// ParsePosition reads "40-30.0N 011-05.3W" into decimal degrees.
func ParsePosition(s string) (lat, lon float64, err error) {
	m := positionRe.FindStringSubmatch(strings.ToUpper(s))
	if m == nil {
		return 0, 0, fmt.Errorf("position %q is not in the form 40-30.0N 011-05.3W", s)
	}
	dm := func(d, mins, hemi string, neg string, limit float64) (float64, error) {
		deg, _ := strconv.ParseFloat(d, 64)
		mn, _ := strconv.ParseFloat(mins, 64)
		if mn >= 60 || deg > limit {
			return 0, fmt.Errorf("position %q out of range", s)
		}
		v := deg + mn/60
		if hemi == neg {
			v = -v
		}
		return v, nil
	}
	if lat, err = dm(m[1], m[2], m[3], "S", 90); err != nil {
		return 0, 0, err
	}
	if lon, err = dm(m[4], m[5], m[6], "W", 180); err != nil {
		return 0, 0, err
	}
	return lat, lon, nil
}

var timeLayouts = []string{
	"2006-01-02 1504 UTC", "2006-01-02 1504", "2006-01-02 15:04 UTC", "2006-01-02 15:04", time.RFC3339,
	"02 Jan 2006 1504", "02 Jan 2006 1504 UTC", "02 Jan 2006 15:04",
}

// ParseTime reads a report time as the ship writes it, in UTC.
func ParseTime(s string) (time.Time, error) {
	s = strings.TrimSpace(s)
	if i := strings.Index(s, " (ship's time"); i > 0 {
		s = s[:i]
	}
	for _, l := range timeLayouts {
		if t, err := time.Parse(l, s); err == nil {
			return t.UTC(), nil
		}
	}
	return time.Time{}, fmt.Errorf("time %q not understood; give it as written, e.g. 2025-01-25 2100 UTC", s)
}

// MissingForDesk lists what a noon report should give but this one leaves
// out. Plain code decides it, so no agent can overlook it.
func MissingForDesk(r Report) []string {
	var out []string
	if r.ConsVLSFOt <= 0 {
		out = append(out, "VLSFO consumption last 24 h")
	}
	if r.ConsMGOt <= 0 {
		out = append(out, "MGO consumption last 24 h")
	}
	return out
}

// Validate checks an extraction against the e-mail it came from. Every
// number must appear in the e-mail, so the Reader cannot invent values.
func Validate(x Extraction, emailBody string) (Report, []string) {
	var problems []string
	r := Report{Extraction: x}
	var err error
	if r.Lat, r.Lon, err = ParsePosition(x.PositionText); err != nil {
		problems = append(problems, err.Error())
	}
	if r.ReportTime, err = ParseTime(x.ReportTimeText); err != nil {
		problems = append(problems, "report time: "+err.Error())
	}
	if r.MasterETA, err = ParseTime(x.MasterETAText); err != nil {
		problems = append(problems, "Master's ETA: "+err.Error())
	}
	if strings.TrimSpace(x.NextPort) == "" {
		problems = append(problems, "next port is missing")
	}
	if x.AvgSpeedKn <= 0 || x.AvgSpeedKn > 25 {
		problems = append(problems, fmt.Sprintf("average speed %.1f kn is outside 0-25 kn", x.AvgSpeedKn))
	}

	inEmail := checker.NewKnown()
	inEmail.AddText(emailBody)
	for _, f := range []struct {
		name string
		v    float64
	}{{"distance run", x.DistanceRunNM}, {"average speed", x.AvgSpeedKn}, {"ROB VLSFO", x.ROBVLSFOt}, {"ROB MGO", x.ROBMGOt},
		{"VLSFO consumption", x.ConsVLSFOt}, {"MGO consumption", x.ConsMGOt}, {"ETA basis speed", x.ETABasisKn}} {
		if f.v == 0 && f.name != "distance run" && f.name != "average speed" && f.name != "ROB VLSFO" && f.name != "ROB MGO" {
			continue // optional field, not in this report
		}
		if u := inEmail.Unsupported(strconv.FormatFloat(f.v, 'f', -1, 64)); len(u) > 0 {
			problems = append(problems, fmt.Sprintf("%s %v does not appear in the e-mail", f.name, f.v))
		}
	}
	for _, f := range []struct{ name, text string }{{"position", x.PositionText}, {"Master's ETA", x.MasterETAText}} {
		if u := inEmail.Unsupported(f.text); len(u) > 0 {
			problems = append(problems, fmt.Sprintf("%s %q has numbers not in the e-mail: %v", f.name, f.text, u))
		}
	}
	if r.MasterETA.Before(r.ReportTime) && !slices.Contains(x.MissingFields, "master_eta") {
		problems = append(problems, "Master's ETA is before the report time")
	}
	return r, problems
}

// ReportFlag is one consistency check a noon report fails.
type ReportFlag struct {
	ID      string `json:"id"` // rob_vlsfo, rob_mgo, speed, position
	Finding string `json:"finding"`
	Result  any    `json:"result"`
}

// ReportChecks runs the Checker's consistency rules against the previous
// noon report. Plain code decides what is flagged; whether the remarks
// explain it is the Ops Agent's judgement.
func ReportChecks(r Report, prev *sim.NoonReport) []ReportFlag {
	if prev == nil {
		return nil
	}
	var out []ReportFlag
	if r.ConsVLSFOt > 0 {
		if c := checker.CheckROB("VLSFO", prev.ROBVLSFO, r.ConsVLSFOt, r.ROBVLSFOt); c.Flagged {
			out = append(out, ReportFlag{"rob_vlsfo", c.Interpretation, c})
		}
	}
	if r.ConsMGOt > 0 {
		if c := checker.CheckROB("MGO", prev.ROBMGO, r.ConsMGOt, r.ROBMGOt); c.Flagged {
			out = append(out, ReportFlag{"rob_mgo", c.Interpretation, c})
		}
	}
	if c, err := checker.CheckSpeedDistance(prev.Time, r.ReportTime, r.DistanceRunNM, r.AvgSpeedKn); err == nil && (c.Mismatch || c.TooFast) {
		out = append(out, ReportFlag{"speed", c.Interpretation, c})
	}
	if c := checker.CheckPosition(prev.Lat, prev.Lon, r.Lat, r.Lon, r.DistanceRunNM); c.Flagged {
		out = append(out, ReportFlag{"position", c.Interpretation, c})
	}
	return out
}
