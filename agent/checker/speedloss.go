package checker

import (
	"fmt"
	"math"
)

// Kwon's (2008) approximate method for the speed a ship loses in wind and
// waves at constant engine power: loss % = Cβ · CU · CForm. The tables are
// as reproduced in Yang et al. (2020), Sustainability 12(9) 3649, Tables
// 2-4. Shore Watch uses it for learning only: real
// operators use ship-specific performance models.

// KwonMaxBeaufort is the highest force the model is used at. Above it,
// CForm grows with BN^6.5 and for TORMAR SPRING gives a head-sea loss of
// 75% at Beaufort 8 and more than 100% at 9, and a negative beam-sea loss
// at 10: the method no longer describes a ship, which by then is slowed by
// her Master, not by constant power. Stronger wind is counted at this force
// and reported as beyond the model.
const KwonMaxBeaufort = 7

// Hull is what Kwon's method needs about a ship in her loading condition.
type Hull struct {
	LppM           float64 `json:"lpp_m"`
	Cb             float64 `json:"block_coefficient"`
	DisplacementM3 float64 `json:"displacement_m3"`
	Ballast        bool    `json:"ballast"`
}

// cuRow is one row of Kwon's speed reduction coefficient table:
// CU = a + b·Fn + c·Fn².
type cuRow struct{ a, b, c float64 }

// kwonCU is Kwon's CU table by block coefficient. Rows up to 0.70 are for
// the normal condition; from 0.75 there is one row for loaded and one for
// ballast.
var kwonCU = map[float64][2]cuRow{ // [loaded or normal, ballast]
	0.55: {{1.7, -1.4, -7.4}, {1.7, -1.4, -7.4}},
	0.60: {{2.2, -2.5, -9.7}, {2.2, -2.5, -9.7}},
	0.65: {{2.6, -3.7, -11.6}, {2.6, -3.7, -11.6}},
	0.70: {{3.1, -5.3, -12.4}, {3.1, -5.3, -12.4}},
	0.75: {{2.4, -10.6, -9.5}, {2.6, -12.5, -13.5}},
	0.80: {{2.6, -13.1, -15.1}, {3.0, -16.3, -21.6}},
	0.85: {{3.1, -18.7, 28.0}, {3.4, -20.9, 31.8}},
}

// KwonSector is Kwon's weather direction for an angle off the bow. His
// beam sector runs to 150°, so what a deck officer calls "on the quarter"
// counts as beam here.
func KwonSector(relDeg float64) string {
	switch {
	case relDeg <= 30:
		return "head"
	case relDeg <= 60:
		return "bow"
	case relDeg <= 150:
		return "beam"
	}
	return "following"
}

// SpeedLoss is the model's answer for one wind.
type SpeedLoss struct {
	Pct           float64 // loss of speed, % of calm-water speed; never below 0
	BeyondModel   bool    // the wind was above KwonMaxBeaufort and counted at it
	Sector        string  // Kwon's weather direction
	BeaufortUsed  int
	FroudeNumber  float64
	CU, CForm, Cβ float64
}

// KwonLoss returns the speed loss for a ship at calm-water speed speedKn in
// wind of force bf coming from relDeg off the bow (0 is dead ahead).
func KwonLoss(h Hull, speedKn float64, bf int, relDeg float64) (SpeedLoss, error) {
	rows, ok := kwonCU[h.Cb]
	if !ok {
		return SpeedLoss{}, fmt.Errorf("block coefficient %.2f is not a row of Kwon's table (0.55 to 0.85 in steps of 0.05)", h.Cb)
	}
	if h.LppM <= 0 || h.DisplacementM3 <= 0 || speedKn <= 0 {
		return SpeedLoss{}, fmt.Errorf("Kwon's method needs Lpp, displacement and speed")
	}
	row := rows[0]
	if h.Ballast {
		row = rows[1]
	}
	s := SpeedLoss{BeaufortUsed: min(bf, KwonMaxBeaufort), BeyondModel: bf > KwonMaxBeaufort, Sector: KwonSector(relDeg)}
	bn := float64(s.BeaufortUsed)
	s.FroudeNumber = speedKn * 1852 / 3600 / math.Sqrt(9.81*h.LppM)
	s.CU = row.a + row.b*s.FroudeNumber + row.c*s.FroudeNumber*s.FroudeNumber
	k := 0.5 // laden
	if h.Ballast {
		k = 0.7
	}
	s.CForm = k*bn + math.Pow(bn, 6.5)/(2.7*math.Pow(h.DisplacementM3, 2.0/3))
	twice := map[string]float64{ // 2Cβ
		"head":      2,
		"bow":       1.7 - 0.03*(bn-4)*(bn-4),
		"beam":      0.9 - 0.06*(bn-6)*(bn-6),
		"following": 0.4 - 0.03*(bn-8)*(bn-8),
	}[s.Sector]
	s.Cβ = twice / 2
	s.Pct = math.Max(0, s.Cβ*s.CU*s.CForm)
	return s, nil
}
