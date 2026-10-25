package sim

import (
	"math"

	"github.com/captv89/seaRoute/route"
)

// Track is a route path with cumulative distances, for placing ships on it.
type Track struct {
	Path []route.Position
	cum  []float64 // metres from the start to each point
}

// NewTrack measures a path.
func NewTrack(path []route.Position) Track {
	t := Track{Path: path, cum: make([]float64, len(path))}
	for i := 1; i < len(path); i++ {
		t.cum[i] = t.cum[i-1] + route.Distance(path[i-1], path[i])
	}
	return t
}

// LengthM is the track length in metres.
func (t Track) LengthM() float64 { return t.cum[len(t.cum)-1] }

// At returns the position d metres from the start, clamped to the ends.
// Between points it interpolates linearly, which is close enough on
// network segments of a few tens of miles.
func (t Track) At(d float64) route.Position {
	if d <= 0 {
		return t.Path[0]
	}
	for i := 1; i < len(t.Path); i++ {
		if t.cum[i] >= d {
			seg := t.cum[i] - t.cum[i-1]
			f := 0.0
			if seg > 0 {
				f = (d - t.cum[i-1]) / seg
			}
			a, b := t.Path[i-1], t.Path[i]
			return route.Position{a[0] + f*(b[0]-a[0]), a[1] + f*(b[1]-a[1])}
		}
	}
	return t.Path[len(t.Path)-1]
}

// NorthboundCrossing returns the distance in metres at which the track
// first reaches latitude lat going north, or -1 if it never does.
func (t Track) NorthboundCrossing(lat float64) float64 {
	for i := 1; i < len(t.Path); i++ {
		a, b := t.Path[i-1][1], t.Path[i][1]
		if a < lat && b >= lat {
			return t.cum[i-1] + (lat-a)/(b-a)*(t.cum[i]-t.cum[i-1])
		}
	}
	return -1
}

// roundTo rounds v to the nearest step.
// roundTo rounds v to a step of 1/n (0.1, 1/600). Dividing by n, rather
// than multiplying by the step, avoids values like 87.80000000000001.
func roundTo(v, step float64) float64 { return math.Round(v/step) / math.Round(1/step) }

// roundMinute rounds a coordinate to the 0.1 minute a Master reports.
func roundMinute(v float64) float64 { return roundTo(v, 1.0/600) }

// Cut returns the path from the start to d metres along it, ending at the
// position there.
func (t Track) Cut(d float64) []route.Position {
	var out []route.Position
	for i := range t.Path {
		if t.cum[i] >= d {
			break
		}
		out = append(out, t.Path[i])
	}
	return append(out, t.At(d))
}
