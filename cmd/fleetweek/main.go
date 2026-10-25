// Command fleetweek builds the fleet-week: 30 ships over
// 20–26 Jan 2025.
//
//	go run ./cmd/fleetweek -jobs      (boxes for data/ecmwf/prepare.py --batch)
//	go run ./cmd/fleetweek -summary   (eval/results/fleet-week.json, from cmd/eval's results)
package main

import (
	"flag"
	"fmt"
	"log"

	"github.com/captv89/seaRoute/route"
	"github.com/captv89/shorewatch/eval"
	"github.com/captv89/shorewatch/sim"
)

func main() {
	jobs := flag.Bool("jobs", false, "write the forecast boxes for prepare.py")
	summary := flag.Bool("summary", false, "write eval/results/fleet-week.json")
	network := flag.String("network", "third_party/seaRoute/dataset/marnet_plus_5km.geojson", "MARNET GeoJSON")
	flag.Parse()
	if *summary {
		if err := summarise(); err != nil {
			log.Fatal(err)
		}
		return
	}
	g, err := route.Load(*network)
	if err != nil {
		log.Fatal(err)
	}
	fleet, err := sim.BuildFleet(g)
	if err != nil {
		log.Fatal(err)
	}
	if *jobs {
		files, err := eval.WriteGridJobs(fleet, ".")
		if err != nil {
			log.Fatal(err)
		}
		for _, f := range files {
			fmt.Println(f)
		}
	}
}
