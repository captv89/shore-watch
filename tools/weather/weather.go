// Package weather reads a prepared ECMWF grid (data/ecmwf/prepare.py) and
// samples it along a ship's track. All conversions are plain code: the
// agent never works out a Beaufort force or a sea state itself.
package weather

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"time"
)

// Grid is one scene's weather: 10 m wind and waves on a regular grid,
// every 3 hours.
type Grid struct {
	LatNorth float64        `json:"lat_north"`
	LonWest  float64        `json:"lon_west"`
	Step     float64        `json:"step_deg"`
	Rows     int            `json:"rows"`
	Cols     int            `json:"cols"`
	Times    []time.Time    `json:"times"`
	U10      [][][]float64  `json:"u10"`
	V10      [][][]float64  `json:"v10"`
	SWH      [][][]*float64 `json:"swh"`
	MWD      [][][]*float64 `json:"mwd"`
	Source   string         `json:"source"`
	Licence  string         `json:"licence"`

	// Ref names the file and its SHA-256, so a log can say exactly which
	// data a result came from.
	Ref Ref `json:"-"`
}

// Ref identifies a grid file.
type Ref struct {
	File   string `json:"file"`
	SHA256 string `json:"sha256"`
}

// Load reads a grid file. name is how the file is referred to in logs,
// e.g. "data/weather/s2-tormar-spring.json".
func Load(path, name string) (*Grid, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var g Grid
	if err := json.Unmarshal(b, &g); err != nil {
		return nil, fmt.Errorf("weather grid %s: %w", filepath.Base(path), err)
	}
	if len(g.Times) == 0 || len(g.U10) != len(g.Times) || len(g.SWH) != len(g.Times) {
		return nil, fmt.Errorf("weather grid %s: fields and times differ in length", filepath.Base(path))
	}
	sum := sha256.Sum256(b)
	g.Ref = Ref{File: name, SHA256: hex.EncodeToString(sum[:])}
	return &g, nil
}

// Point is the weather at one place and time.
type Point struct {
	WindFromDeg float64  // direction the wind blows from, true
	WindMS      float64  // 10 m mean wind
	SWH         *float64 // significant wave height, m; nil where the wave model has no value
	WavesFrom   *float64 // mean wave direction, from, true
}

// At samples the grid at one of its times. Wind and wave height are
// interpolated bilinearly between the four surrounding points; wave
// direction is averaged as a vector. Points without wave values (near
// land) are left out of the wave average.
func (g *Grid) At(t time.Time, lat, lon float64) (Point, error) {
	k := -1
	for i, gt := range g.Times {
		if gt.Equal(t) {
			k = i
		}
	}
	if k < 0 {
		return Point{}, fmt.Errorf("no weather field at %s", t.Format(time.RFC3339))
	}
	fi := (g.LatNorth - lat) / g.Step
	fj := (lon - g.LonWest) / g.Step
	i0, j0 := int(math.Floor(fi)), int(math.Floor(fj))
	if i0 < 0 || j0 < 0 || i0+1 >= g.Rows || j0+1 >= g.Cols {
		return Point{}, fmt.Errorf("position %.2f, %.2f is outside the weather grid", lat, lon)
	}
	di, dj := fi-float64(i0), fj-float64(j0)
	type corner struct {
		i, j int
		w    float64
	}
	corners := []corner{{i0, j0, (1 - di) * (1 - dj)}, {i0, j0 + 1, (1 - di) * dj}, {i0 + 1, j0, di * (1 - dj)}, {i0 + 1, j0 + 1, di * dj}}

	var u, v, h, hw, wx, wy float64
	for _, c := range corners {
		u += c.w * g.U10[k][c.i][c.j]
		v += c.w * g.V10[k][c.i][c.j]
		if s, d := g.SWH[k][c.i][c.j], g.MWD[k][c.i][c.j]; s != nil && d != nil {
			h += c.w * *s
			hw += c.w
			r := *d * math.Pi / 180
			wx += c.w * math.Sin(r)
			wy += c.w * math.Cos(r)
		}
	}
	p := Point{WindMS: math.Hypot(u, v), WindFromDeg: norm360(math.Atan2(-u, -v) * 180 / math.Pi)}
	if hw > 0 {
		swh := h / hw
		from := norm360(math.Atan2(wx, wy) * 180 / math.Pi)
		p.SWH, p.WavesFrom = &swh, &from
	}
	return p, nil
}

// KnotsPerMS converts metres per second to knots.
const KnotsPerMS = 3600.0 / 1852.0

// beaufortFloor is the lowest whole-knot wind speed of each force from 1,
// per the WMO scale as published by the US National Weather Service.
var beaufortFloor = []int{1, 4, 7, 11, 17, 22, 28, 34, 41, 48, 56, 64}

// Beaufort returns the Beaufort force for a wind speed in whole knots.
func Beaufort(kn int) int {
	f := 0
	for i, lo := range beaufortFloor {
		if kn >= lo {
			f = i + 1
		}
	}
	return f
}

// douglasCeil is the highest significant wave height of each Douglas sea
// state from 0 (WMO code table 3700). A height on a boundary takes the
// lower code.
var douglasCeil = []float64{0, 0.1, 0.5, 1.25, 2.5, 4, 6, 9, 14}

// Douglas returns the sea state for a significant wave height in metres.
func Douglas(swh float64) int {
	for code, hi := range douglasCeil {
		if swh <= hi {
			return code
		}
	}
	return 9
}

// Relative returns the angle between where something comes from and the
// ship's heading: 0 is dead ahead, 180 is from astern.
func Relative(fromDeg, headingDeg float64) float64 {
	d := math.Abs(math.Mod(fromDeg-headingDeg+540, 360) - 180)
	return d
}

// Sector names a relative angle the way a deck officer would.
func Sector(rel float64) string {
	switch {
	case rel <= 30:
		return "head"
	case rel <= 60:
		return "on the bow"
	case rel <= 120:
		return "on the beam"
	case rel <= 150:
		return "on the quarter"
	}
	return "following"
}

func norm360(d float64) float64 { return math.Mod(d+360, 360) }

// Bearing is the initial great-circle bearing from a to b, in degrees true.
func Bearing(latA, lonA, latB, lonB float64) float64 {
	φ1, φ2 := latA*math.Pi/180, latB*math.Pi/180
	dλ := (lonB - lonA) * math.Pi / 180
	y := math.Sin(dλ) * math.Cos(φ2)
	x := math.Cos(φ1)*math.Sin(φ2) - math.Sin(φ1)*math.Cos(φ2)*math.Cos(dλ)
	return norm360(math.Atan2(y, x) * 180 / math.Pi)
}
