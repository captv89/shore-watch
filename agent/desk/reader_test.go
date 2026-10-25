package desk

import (
	"math"
	"strings"
	"testing"
)

const body = `NOON REPORT - MV VALDA CREST
Date/time: 22 Jan 2025 1200 (ship's time UTC)
Position: 40-30.0N 011-05.3W
Distance run last 24h: 297.6 nm
Average speed last 24h: 12.4 kn
ROB VLSFO: 594.7 t
ROB MGO: 86.8 t
Next port: Rotterdam
ETA Rotterdam: 2025-01-25 2100 UTC`

func good() Extraction {
	return Extraction{
		ShipName: "MV VALDA CREST", ReportTimeText: "22 Jan 2025 1200", PositionText: "40-30.0N 011-05.3W",
		DistanceRunNM: 297.6, AvgSpeedKn: 12.4, ROBVLSFOt: 594.7, ROBMGOt: 86.8, NextPort: "Rotterdam",
		MasterETAText: "2025-01-25 2100 UTC",
	}
}

func TestValidateGood(t *testing.T) {
	r, problems := Validate(good(), body)
	if len(problems) > 0 {
		t.Fatal(problems)
	}
	if math.Abs(r.Lat-40.5) > 1e-9 || math.Abs(r.Lon-(-11-5.3/60)) > 1e-9 {
		t.Errorf("position %v, %v", r.Lat, r.Lon)
	}
	if r.MasterETA.Format("2006-01-02T15:04") != "2025-01-25T21:00" || r.ReportTime.Hour() != 12 {
		t.Errorf("times %v %v", r.ReportTime, r.MasterETA)
	}
}

func TestValidateCatchesInventedValues(t *testing.T) {
	cases := map[string]func(*Extraction){
		"invented speed":    func(x *Extraction) { x.AvgSpeedKn = 12.5 },
		"converted ETA":     func(x *Extraction) { x.MasterETAText = "2025-01-25 2200 UTC" },
		"decimal position":  func(x *Extraction) { x.PositionText = "40.5N 11.088W" },
		"missing next port": func(x *Extraction) { x.NextPort = "" },
	}
	for name, mutate := range cases {
		x := good()
		mutate(&x)
		if _, problems := Validate(x, body); len(problems) == 0 {
			t.Errorf("%s: no problem reported", name)
		} else {
			t.Logf("%s: %s", name, strings.Join(problems, "; "))
		}
	}
}
