package eval

import (
	"errors"
	"io/fs"
	"os"
	"testing"

	"github.com/captv89/seaRoute/route"
)

// TestBuildFleetWeek needs the fleet-week grids (cmd/fleetweek -jobs, then
// data/ecmwf/prepare.py --batch); without them, or with only the
// committed sample grids, it is skipped.
func TestBuildFleetWeek(t *testing.T) {
	if _, err := os.Stat("../" + FleetWeekGrids); err != nil {
		t.Skip("no fleet-week grids")
	}
	g, err := route.Load("../third_party/seaRoute/dataset/marnet_plus_5km.geojson")
	if err != nil {
		t.Fatal(err)
	}
	tasks, err := BuildFleetWeek(g, "..")
	if errors.Is(err, fs.ErrNotExist) {
		t.Skip("fleet-week grids not all built")
	}
	if err != nil {
		t.Fatal(err)
	}
	count := map[string]int{}
	rules := map[string][2]int{}
	for _, tk := range tasks {
		if tk.Set != FleetWeek {
			continue
		}
		count[tk.Kind]++
		r := rules[tk.Kind]
		if !Score(tk, tk.rules).Pass {
			r[0]++
		}
		if !Score(tk, tk.coded).Pass {
			r[1]++
		}
		rules[tk.Kind] = r
		if tk.Kind != kindReport && tk.Kind != kindForecastQ {
			t.Logf("%-32s %-15s key %v %s rules %v %v", tk.ID, tk.Kind, tk.Key.Topics, tk.Key.OffHire, tk.rules.Topics, tk.rules.OffHire)
		}
	}
	for _, k := range FleetKinds {
		t.Logf("%-15s %3d items; rules-only wrong on %d, rules-coded on %d", k, count[k], rules[k][0], rules[k][1])
	}
}

func TestListFleetWeek(t *testing.T) {
	if _, err := os.Stat("../" + FleetWeekGrids); err != nil || os.Getenv("FLEET_LIST") == "" {
		t.Skip()
	}
	g, _ := route.Load("../third_party/seaRoute/dataset/marnet_plus_5km.geojson")
	tasks, err := BuildFleetWeek(g, "..")
	if err != nil {
		t.Fatal(err)
	}
	for _, tk := range tasks {
		t.Logf("%s|%s|%s|%v|%s", tk.Kind, tk.ID, tk.Set, tk.Key.Topics, tk.Planted)
	}
}

func TestFleetSampleExists(t *testing.T) {
	if _, err := os.Stat("../" + FleetWeekGrids); err != nil {
		t.Skip("no fleet-week grids")
	}
	g, _ := route.Load("../third_party/seaRoute/dataset/marnet_plus_5km.geojson")
	tasks, err := BuildFleetWeek(g, "..")
	if errors.Is(err, fs.ErrNotExist) {
		t.Skip("fleet-week grids not all built")
	}
	if err != nil {
		t.Fatal(err)
	}
	n := 0
	for _, tk := range tasks {
		if tk.Sample {
			n++
		}
	}
	if n != len(fleetSample) {
		t.Fatalf("%d sample tasks found, %d listed", n, len(fleetSample))
	}
}
