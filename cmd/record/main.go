// Command record runs a scene against a live model and stores the event
// logs the page replays (every run is kept).
//
// Usage: go run ./cmd/record -scene s1 -attempts 5   (needs GOOGLE_API_KEY)
//
//	go run ./cmd/record -scene s2 -attempts 5 -max-calls 60
//	go run ./cmd/record -scene s3 -attempts 5 -max-calls 10
//	go run ./cmd/record -scene s4 -attempts 5 -max-calls 60
//	go run ./cmd/record -scene s5 -attempts 5 -max-calls 30
//	go run ./cmd/record -scene s6 -attempts 5 -max-calls 60
//
// A model ID with a slash (vendor/model) runs on NVIDIA NIM's Chat
// Completions API instead of Gemini and needs NVIDIA_API_KEY:
//
//	go run ./cmd/record -scene s1 -model nvidia/nemotron-3-super-120b-a12b -rpm 30
//
// An ID starting openrouter/ runs the same way on OpenRouter and needs
// OPEN_ROUTER_API_KEY (free models: 20 requests a minute):
//
//	go run ./cmd/record -scene s4 -model openrouter/nvidia/nemotron-3.5-super-vl:free -rpm 10
package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"path/filepath"

	"github.com/captv89/seaRoute/route"
	"github.com/captv89/shorewatch/agent/desk"
	"github.com/captv89/shorewatch/internal/llm"
	"github.com/captv89/shorewatch/sim"
	"github.com/captv89/shorewatch/tools/weather"
)

var rpm int

func main() {
	scene := flag.String("scene", "s1", "scene to record: s1 to s6")
	attempts := flag.Int("attempts", 1, "number of runs to record")
	first := flag.Int("first", 1, "attempt number of the first run")
	totalFlag := flag.Int("total", 0, "attempts in the scene's full set, for the run header (default: first+attempts-1)")
	modelID := flag.String("model", "gemini-3.1-flash-lite", "Gemini model ID, vendor/model for NVIDIA NIM, or openrouter/vendor/model for OpenRouter")
	out := flag.String("out", "runs", "output directory; runs go in <out>/<scene id>")
	network := flag.String("network", "third_party/seaRoute/dataset/marnet_plus_5km.geojson", "MARNET GeoJSON")
	maxCalls := flag.Int("max-calls", 20, "model call budget per run")
	flag.IntVar(&rpm, "rpm", 12, "model requests per minute (free tier limit is 15)")
	flag.Parse()
	if err := run(*scene, *attempts, *first, *totalFlag, *modelID, *out, *network, *maxCalls); err != nil {
		log.Fatal(err)
	}
}

func run(sceneName string, attempts, first, totalAttempts int, modelID, out, network string, maxCalls int) error {
	ctx := context.Background()
	m, err := llm.New(ctx, modelID, rpm)
	if err != nil {
		return err
	}
	g, err := route.Load(network)
	if err != nil {
		return err
	}
	var (
		episodes []sim.Scene
		wx, fc   *weather.Grid
		operator func(desk.Draft) desk.Decision
	)
	switch sceneName {
	case "s1":
		var sc sim.Scene
		sc, err = sim.Scene1(g)
		episodes = []sim.Scene{sc}
	case "s2":
		episodes, err = sim.Scene2(g)
		if err == nil {
			wx, err = weather.Load("data/weather/s2-tormar-spring.json", "data/weather/s2-tormar-spring.json")
		}
		if err == nil {
			fc, err = weather.Load("data/weather/s2-forecast-20250124T00.json", "data/weather/s2-forecast-20250124T00.json")
		}
		// The operator adds one line to the query to the Master (Scene 2).
		operator = desk.AddLine("master@tormarspring.sim-fleet.example", "Please reply before tomorrow's noon report.", "adds one line asking for a reply before the next noon report")
	case "s4":
		// A forecast run, not an e-mail, starts the work (Scene 4).
		var sc sim.Scene
		sc, err = sim.Scene4(g)
		episodes = []sim.Scene{sc}
		if err == nil {
			fc, err = weather.Load("data/weather/s4-forecast-20250122T12.json", "data/weather/s4-forecast-20250122T12.json")
		}
	case "s5":
		// The Rotterdam agent's berth line-up (Scene 5).
		var sc sim.Scene
		sc, err = sim.Scene5(g)
		episodes = []sim.Scene{sc}
	case "s6":
		// The engine-stop noon report (Scene 6). The weather
		// on track serves the Performance Agent; no forecast is offered.
		var sc sim.Scene
		sc, err = sim.Scene6(g)
		episodes = []sim.Scene{sc}
		if err == nil {
			wx, err = weather.Load("data/weather/s2-tormar-spring.json", "data/weather/s2-tormar-spring.json")
		}
	case "s3":
		// The fraud e-mail: only the Security Agent reads it (Scene 3).
		episodes = []sim.Scene{sim.Scene3()}
	default:
		return fmt.Errorf("unknown scene %q", sceneName)
	}
	if err != nil {
		return err
	}
	sc := episodes[0]

	last := first + attempts - 1
	total := last
	if totalAttempts > 0 {
		total = totalAttempts
	}
	for a := first; a <= last; a++ {
		res, err := desk.Record(ctx, desk.Config{
			Episodes: episodes, Graph: g, Weather: wx, Forecast: fc, Operator: operator, Model: m, ModelID: modelID,
			Attempt: a, TotalAttempts: total, OutDir: filepath.Join(out, sc.ID), MaxModelCalls: maxCalls,
			ToolVersions: map[string]string{"adk-go": "v2.6.0", "seaRoute": "shorewatch branch", "network": "Eurostat marnet_plus_5km"},
		})
		if err != nil {
			log.Printf("attempt %d failed: %v (log kept: %s)", a, err, res.LogPath)
			continue
		}
		log.Printf("attempt %d: %s, %d events, %d model calls, %d drafts", a, res.LogPath, res.Events, res.ModelCalls, len(res.Drafts))
	}
	return nil
}
