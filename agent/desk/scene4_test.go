package desk

import (
	"context"
	"fmt"
	"os"
	"slices"
	"strings"
	"testing"

	"github.com/captv89/shorewatch/agent/eventlog"
	"github.com/captv89/shorewatch/sim"
	"github.com/captv89/shorewatch/tools/weather"
)

// TestScene4Pipeline: a forecast run, not an e-mail, starts the work. The
// Weather Agent scans both ships; each Master gets a heads-up, each voyage
// file a note, and nothing goes to anyone else.
var masters4 = []string{"master@valdacrest.sim-fleet.example", "master@tormarspring.sim-fleet.example"}

func TestScene4Pipeline(t *testing.T) {
	scene1(t) // loads the graph
	sc, err := sim.Scene4(graph)
	if err != nil {
		t.Fatal(err)
	}
	fc, err := weather.Load("../../data/weather/s4-forecast-20250122T12.json", "data/weather/s4-forecast-20250122T12.json")
	if err != nil {
		t.Fatal(err)
	}
	m := &staff{}
	res, err := Record(context.Background(), Config{
		Episodes: []sim.Scene{sc}, Graph: graph, Forecast: fc, Model: m, ModelID: "scripted", Attempt: 1, TotalAttempts: 1, OutDir: t.TempDir(), MaxModelCalls: 40,
	})
	if err != nil {
		if b, rerr := os.ReadFile(res.LogPath); rerr == nil {
			t.Log(string(b))
		}
		t.Fatal(err)
	}
	f, err := os.Open(res.LogPath)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	events, err := eventlog.Read(f)
	if err != nil {
		t.Fatal(err)
	}
	for _, e := range events {
		t.Logf("%2d %-10s %-18s %s", e.Seq, e.Actor, e.Type, e.Summary)
	}

	if !strings.Contains(res.RunID, "s4-weather-fleet-") {
		t.Errorf("run ID %q, want the fleet", res.RunID)
	}
	if e := events[0]; e.Type != eventlog.TypeForecastAvailable || e.Actor != eventlog.ActorSim || e.SchemaVersion != "1.6" || e.Ship != eventlog.Fleet {
		t.Errorf("first event %s by %s (schema %s, ship %s), want forecast_available by sim, 1.6, fleet", e.Type, e.Actor, e.SchemaVersion, e.Ship)
	}
	// Schema 1.6: each event names the ship it is about.
	for _, e := range events {
		id, _ := e.Detail["ship_id"].(string)
		if e.Type == eventlog.TypeCheck && e.Detail["tool"] == "weather_ahead" {
			id = fmt.Sprint(e.Detail["result"].(map[string]any)["ship_id"])
		}
		if e.Type == eventlog.TypeDraft {
			id = map[string]string{masters4[0]: sc.Fleet[0].Ship.ID, masters4[1]: sc.Fleet[1].Ship.ID}[fmt.Sprint(e.Detail["draft"].(map[string]any)["to"].([]any)[0])]
		}
		// A report or summary that covers both ships is about the fleet.
		if (e.Type == eventlog.TypeToolResult && e.Actor == eventlog.ActorSubagent && e.Agent == weatherName) || e.Type == eventlog.TypeForecastAvailable {
			id = eventlog.Fleet
		}
		if id != "" && e.Ship != id {
			t.Errorf("seq %d %s: ship %s, want %s", e.Seq, e.Type, e.Ship, id)
		}
	}
	var scans, notes int
	var noted []string
	for _, e := range events {
		switch {
		case e.Type == eventlog.TypeMessageReceived:
			t.Errorf("seq %d: an e-mail in a forecast scene", e.Seq)
		case e.Type == eventlog.TypeCheck && e.Detail["tool"] == "weather_ahead":
			scans++
			if e.Detail["samples"] == nil || e.Detail["path"] == nil || e.Detail["grid_ref"] == nil {
				t.Errorf("seq %d: weather_ahead check without samples, path or grid_ref", e.Seq)
			}
		case e.Type == eventlog.TypeToolResult && e.Detail["tool"] == "add_voyage_note":
			notes++
			noted = append(noted, fmt.Sprint(e.Detail["ship_id"]))
		case e.Type == eventlog.TypeError:
			t.Errorf("seq %d: unexpected error: %s", e.Seq, e.Summary)
		case e.Type == eventlog.TypeAlert:
			t.Errorf("seq %d: unexpected alert: %s", e.Seq, e.Summary)
		}
	}
	if scans != 2 || notes != 2 {
		t.Errorf("%d weather scans and %d notes, want 2 and 2", scans, notes)
	}
	slices.Sort(noted)
	if !slices.Equal(noted, []string{sc.Fleet[0].Ship.ID, sc.Fleet[1].Ship.ID}) {
		t.Errorf("notes filed for %v, want one per ship", noted)
	}
	if m.sawLogOnly {
		t.Error("the model was shown weather_ahead's samples, path or grid file")
	}

	masters := []string{"master@valdacrest.sim-fleet.example", "master@tormarspring.sim-fleet.example"}
	if len(res.Drafts) != 2 {
		t.Fatalf("%d drafts, want one heads-up to each Master", len(res.Drafts))
	}
	for i, d := range res.Drafts {
		if len(d.To) != 1 || d.To[0] != masters[i] {
			t.Errorf("draft %d to %v, want only %s", i+1, d.To, masters[i])
		}
		if !slices.Equal(d.About, []string{"heavy_weather"}) || d.Status != "approved, not sent (demo)" {
			t.Errorf("draft %d: about %v, status %q", i+1, d.About, d.Status)
		}
	}
}

// TestVoyageNoteNamesShip: in a fleet, a voyage-file note must say which
// ship it is for; with one ship in the work, the ship may be left out.
func TestVoyageNoteNamesShip(t *testing.T) {
	scene1(t)
	sc, err := sim.Scene4(graph)
	if err != nil {
		t.Fatal(err)
	}
	fleet := &run{ep: sc}
	if _, err := fleet.noteShip(""); err == nil {
		t.Error("fleet note without ship_id accepted")
	}
	if _, err := fleet.noteShip("SIM-9999"); err == nil {
		t.Error("note for a ship outside the work accepted")
	}
	if v, err := fleet.noteShip(sc.Fleet[1].Ship.ID); err != nil || v.Ship.ID != sc.Fleet[1].Ship.ID {
		t.Errorf("note for %s: got %s, %v", sc.Fleet[1].Ship.ID, v.Ship.ID, err)
	}
	one := &run{ep: sim.Scene{Voyage: sc.Fleet[0]}}
	if v, err := one.noteShip(""); err != nil || v.Ship.ID != sc.Fleet[0].Ship.ID {
		t.Errorf("one-ship note without ship_id: got %s, %v", v.Ship.ID, err)
	}
}
