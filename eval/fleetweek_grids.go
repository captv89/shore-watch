package eval

import (
	"encoding/json"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"time"

	"github.com/captv89/shorewatch/sim"
)

// The fleet-week's forecast runs: the 00z run of each day,
// read when ECMWF open data has it out, about 0800 UTC.
const (
	FleetWeekGrids  = "data/weather/fleetweek"
	forecastLatency = 8 * time.Hour
	// aheadHours is how far ahead each ship's grid reaches: the run's last
	// step fetched, 120 h, less the latency.
	aheadHours = 112
)

// FleetRuns are the forecast runs of the week.
func FleetRuns() []time.Time {
	var runs []time.Time
	for t := sim.WeekStart; t.Before(sim.WeekEnd); t = t.Add(24 * time.Hour) {
		runs = append(runs, t)
	}
	return runs
}

// GridPath is the file a ship's cut of a forecast run is kept in.
func GridPath(run time.Time, shipID string) string {
	return filepath.Join(FleetWeekGrids, run.Format("20060102T15"), shipID+".json")
}

// gridJob is one box for data/ecmwf/prepare.py --batch.
type gridJob struct {
	Out  string     `json:"out"`
	Box  [4]float64 `json:"box"` // lat_min, lat_max, lon_min, lon_max
	From string     `json:"from"`
	To   string     `json:"to"`
}

// WriteGridJobs writes, for each run, the box around every ship at sea's
// track for the hours ahead, for prepare.py to cut. root is the repository root.
func WriteGridJobs(fleet []sim.FleetVoyage, root string) ([]string, error) {
	var files []string
	for _, run := range FleetRuns() {
		at := run.Add(forecastLatency)
		var jobs []gridJob
		for _, f := range fleet {
			if !f.AtSea(at) {
				continue
			}
			pts := f.Ahead(at, aheadHours)
			lat0, lat1, lon0, lon1 := 90.0, -90.0, 180.0, -180.0
			for _, p := range pts {
				lat0, lat1 = math.Min(lat0, p[1]), math.Max(lat1, p[1])
				lon0, lon1 = math.Min(lon0, p[0]), math.Max(lon1, p[0])
			}
			// A margin of 4°, snapped outwards to the 0.5° grid: the scan routes
			// afresh from the AIS position, which can run a little off the track.
			down := func(v float64) float64 { return math.Floor((v-4)*2) / 2 }
			up := func(v float64) float64 { return math.Ceil((v+4)*2) / 2 }
			jobs = append(jobs, gridJob{
				Out: GridPath(run, f.Spec.ID),
				Box: [4]float64{math.Max(-90, down(lat0)), math.Min(90, up(lat1)), math.Max(-180, down(lon0)), math.Min(179.75, up(lon1))},
				// From the step before the read, so the scan can start at once.
				From: at.Truncate(6 * time.Hour).Format("2006-01-02T15:04:05"), To: run.Add(120 * time.Hour).Format("2006-01-02T15:04:05"),
			})
		}
		b, err := json.MarshalIndent(jobs, "", " ")
		if err != nil {
			return nil, err
		}
		name := filepath.Join(FleetWeekGrids, "jobs-"+run.Format("20060102T15")+".json")
		if err := os.MkdirAll(filepath.Join(root, FleetWeekGrids), 0o755); err != nil {
			return nil, err
		}
		if err := os.WriteFile(filepath.Join(root, name), append(b, '\n'), 0o644); err != nil {
			return nil, err
		}
		files = append(files, fmt.Sprintf("%s (%d ships)", name, len(jobs)))
	}
	return files, nil
}
