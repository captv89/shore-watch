package sim

import (
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/captv89/seaRoute/route"
)

var (
	graphOnce sync.Once
	graph     *route.Graph
	graphErr  error
)

func testGraph(t *testing.T) *route.Graph {
	t.Helper()
	graphOnce.Do(func() { graph, graphErr = route.Load("../third_party/seaRoute/dataset/marnet_plus_5km.geojson") })
	if graphErr != nil {
		t.Fatal(graphErr)
	}
	return graph
}

// TestScene1Story checks the facts Scene 1's story depends on.
func TestScene1Story(t *testing.T) {
	g := testGraph(t)
	s, err := Scene1(g)
	if err != nil {
		t.Fatal(err)
	}
	rep := s.Report
	t.Logf("\n%s", s.Email.Body)
	for _, n := range s.Voyage.Notices {
		t.Logf("notice sent %s: %d-day, ETA %s", n.SentAt.Format(time.RFC3339), n.DaysLabel, n.ETA.Format(time.RFC3339))
	}

	r, err := g.Route([]route.Position{{rep.Lon, rep.Lat}, {Rotterdam.Lon, Rotterdam.Lat}}, route.Options{})
	if err != nil {
		t.Fatal(err)
	}
	expected := rep.Time.Add(time.Duration(r.DistanceNM() / rep.SpeedKn * float64(time.Hour)))
	diff := rep.MasterETA.Sub(expected).Hours()
	t.Logf("distance to go %.1f nm; distance/speed ETA %s; Master's %s; diff %.2f h", r.DistanceNM(), expected.Format(time.RFC3339), rep.MasterETA.Format(time.RFC3339), diff)
	if diff > 2 || diff < -2 {
		t.Errorf("Master's ETA is %.2f h from distance/speed; story needs within 2 h", diff)
	}

	if len(s.Voyage.Notices) == 0 {
		t.Fatal("no notices sent before the scene")
	}
	last := s.Voyage.Notices[len(s.Voyage.Notices)-1]
	if moved := rep.MasterETA.Sub(last.ETA).Hours(); moved <= s.Voyage.CP.ETAChangeHours {
		t.Errorf("ETA moved %.1f h since last notice; story needs > %.0f h", moved, s.Voyage.CP.ETAChangeHours)
	}
	if left := rep.MasterETA.Sub(rep.Time).Hours() / 24; !(left > 3 && left-1 < 3) {
		t.Errorf("%.2f days to ETA; story needs the 3-day notice to fall due before the next report", left)
	}
	if last.DaysLabel != 5 {
		t.Errorf("last notice was the %d-day notice; story expects the 5-day", last.DaysLabel)
	}
	if !strings.Contains(s.Email.Body, FormatLat(rep.Lat)) {
		t.Error("e-mail lacks the position")
	}
	if rep.Lon > -8.5 || rep.Lon < -12 { // offshore lane, west of the Portuguese coast (~8.9°W at 40.5°N)
		t.Errorf("report position %.2f°E is not off Portugal", rep.Lon)
	}
}

func TestFormatPosition(t *testing.T) {
	for _, c := range []struct {
		v    float64
		lat  bool
		want string
	}{
		{40.5, true, "40-30.0N"}, {-10, false, "010-00.0W"}, {-0.0167, true, "00-01.0S"}, {4.4425, false, "004-26.6E"},
	} {
		got := FormatLon(c.v)
		if c.lat {
			got = FormatLat(c.v)
		}
		if got != c.want {
			t.Errorf("%v: got %s, want %s", c.v, got, c.want)
		}
	}
}
