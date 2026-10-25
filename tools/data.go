// Package tools holds the stand-in connectors the agent reaches over MCP.
// In a real company these would be the voyage management system, the CP
// library and the address book; here they read the simulated world.
package tools

import (
	"context"
	"fmt"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/captv89/seaRoute/mcpserver"
	"github.com/captv89/seaRoute/route"
	"github.com/captv89/shorewatch/sim"
	"github.com/captv89/shorewatch/tools/weather"
)

type shipIn struct {
	ShipID string `json:"ship_id" jsonschema:"ship ID, e.g. SIM-0001"`
}

type historyOut struct {
	ShipID      string           `json:"ship_id"`
	Ship        string           `json:"ship"`
	From        string           `json:"from"`
	To          string           `json:"to"`
	Cargo       string           `json:"cargo"`
	Avoid       []string         `json:"avoid_passages"`
	NoonReports []sim.NoonReport `json:"previous_noon_reports"`
	Notices     []sim.Notice     `json:"eta_notices_sent"`
	VoyageNotes []string         `json:"voyage_notes,omitempty"`
}

type cpOut struct {
	sim.CPTerms
	Note string `json:"note"`
}

type vmsOut struct {
	ShipID string `json:"ship_id"`
	Ship   string `json:"ship"`
	sim.VMS
	Note string `json:"note"`
}

type aisOut struct {
	ShipID string    `json:"ship_id"`
	Ship   string    `json:"ship"`
	Time   time.Time `json:"time"`
	Lat    float64   `json:"lat"`
	Lon    float64   `json:"lon"`
	Note   string    `json:"note"`
}

type speedIn struct {
	ShipID   string `json:"ship_id" jsonschema:"ship ID, e.g. SIM-0001"`
	BerthUTC string `json:"berth_utc" jsonschema:"the expected berthing time at the next port, RFC 3339, as given by the port agent, e.g. 2025-01-27T06:00:00Z"`
}

type offHireIn struct {
	ShipID  string  `json:"ship_id" jsonschema:"ship ID, e.g. SIM-0002"`
	FromUTC string  `json:"from_utc" jsonschema:"when the ship went off hire, RFC 3339, as the Master reported it"`
	ToUTC   string  `json:"to_utc" jsonschema:"when she came back on hire, RFC 3339, as the Master reported it"`
	VLSFOt  float64 `json:"vlsfo_t,omitempty" jsonschema:"VLSFO used while off hire, tonnes, as the Master reported it"`
	MGOt    float64 `json:"mgo_t,omitempty" jsonschema:"MGO used while off hire, tonnes, as the Master reported it"`
	// CranesOut is for a crane breakdown under a pro rata crane clause.
	CranesOut int `json:"cranes_out,omitempty" jsonschema:"for a cargo crane breakdown only: how many of the ship's cranes were out of use, as the Master reported it; leave out for a whole stop"`
}

type contactsOut struct {
	Contacts []sim.Contact `json:"contacts"`
}

// Server returns an MCP server with seaRoute's tools (distance_to_go,
// route, list_passages) and the voyage stand-ins (voyage_history,
// cp_terms, contacts) for the given voyages. With a weather grid it also
// serves weather_on_track.
func Server(g *route.Graph, voyages []sim.Voyage, wx *weather.Grid) *mcp.Server {
	ports := map[string]route.Position{}
	byShip := map[string]sim.Voyage{}
	for _, v := range voyages {
		byShip[v.Ship.ID] = v
		for _, p := range []sim.Port{v.From, v.To} {
			ports[p.Name] = route.Position{p.Lon, p.Lat}
		}
	}
	s := mcpserver.New(g, func(name string) (route.Position, bool) { p, ok := ports[name]; return p, ok })
	readOnly := &mcp.ToolAnnotations{ReadOnlyHint: true, IdempotentHint: true}
	voyage := func(id string) (sim.Voyage, error) {
		v, ok := byShip[id]
		if !ok {
			return sim.Voyage{}, errUnknownShip(id)
		}
		return v, nil
	}

	mcp.AddTool(s, &mcp.Tool{
		Name:        "voyage_history",
		Description: "Previous noon reports and ETA notices already sent on the ship's current voyage (stand-in for the voyage database).",
		Annotations: readOnly,
	}, func(_ context.Context, _ *mcp.CallToolRequest, in shipIn) (*mcp.CallToolResult, historyOut, error) {
		v, err := voyage(in.ShipID)
		if err != nil {
			return nil, historyOut{}, err
		}
		return nil, historyOut{
			ShipID: v.Ship.ID, Ship: v.Ship.Name, From: v.From.Name, To: v.To.Name, Cargo: v.Cargo,
			Avoid: nonNil(v.Avoid), NoonReports: v.History, Notices: v.Notices, VoyageNotes: v.Notes,
		}, nil
	})

	mcp.AddTool(s, &mcp.Tool{
		Name:        "cp_terms",
		Description: "Charter party terms for the ship's current voyage: ETA notice rules and the good-weather definition (stand-in for the CP library).",
		Annotations: readOnly,
	}, func(_ context.Context, _ *mcp.CallToolRequest, in shipIn) (*mcp.CallToolResult, cpOut, error) {
		v, err := voyage(in.ShipID)
		if err != nil {
			return nil, cpOut{}, err
		}
		return nil, cpOut{CPTerms: v.CP, Note: "Simplified, fictional terms for this demo."}, nil
	})

	mcp.AddTool(s, &mcp.Tool{
		Name:        "contacts",
		Description: "Verified stakeholders for the ship's voyage, with their e-mail addresses and domains (stand-in for the address book).",
		Annotations: readOnly,
	}, func(_ context.Context, _ *mcp.CallToolRequest, in shipIn) (*mcp.CallToolResult, contactsOut, error) {
		v, err := voyage(in.ShipID)
		if err != nil {
			return nil, contactsOut{}, err
		}
		return nil, contactsOut{Contacts: v.Contacts}, nil
	})

	mcp.AddTool(s, &mcp.Tool{
		Name:        "voyage_system",
		Description: "The voyage as entered in the company's voyage management system: the port calls with their ETAs, and the time charter hire (owner, rate, off-hire periods already entered). Read-only: changes are proposed with propose_entry and wait for the operator.",
		Annotations: readOnly,
	}, func(_ context.Context, _ *mcp.CallToolRequest, in shipIn) (*mcp.CallToolResult, vmsOut, error) {
		v, err := voyage(in.ShipID)
		if err != nil {
			return nil, vmsOut{}, err
		}
		if v.VMS == nil {
			return nil, vmsOut{}, fmt.Errorf("no voyage management system record for %s in this demo", in.ShipID)
		}
		return nil, vmsOut{ShipID: v.Ship.ID, Ship: v.Ship.Name, VMS: *v.VMS, Note: "Stand-in for the voyage management system. Fictional figures."}, nil
	})

	mcp.AddTool(s, &mcp.Tool{
		Name:        "ais_position",
		Description: "The ship's latest AIS position, with its time (stand-in for the AIS feed).",
		Annotations: readOnly,
	}, func(_ context.Context, _ *mcp.CallToolRequest, in shipIn) (*mcp.CallToolResult, aisOut, error) {
		v, err := voyage(in.ShipID)
		if err != nil {
			return nil, aisOut{}, err
		}
		if len(v.Track) == 0 {
			return nil, aisOut{}, fmt.Errorf("no AIS track for %s in this demo", in.ShipID)
		}
		f := v.Track[len(v.Track)-1]
		return nil, aisOut{ShipID: v.Ship.ID, Ship: v.Ship.Name, Time: f.Time, Lat: f.Lat, Lon: f.Lon, Note: "Simulated AIS track."}, nil
	})

	mcp.AddTool(s, &mcp.Tool{
		Name:        "speed_advice",
		Description: "Voyage optimisation service (stand-in; figures made up for this simulation): the steady speed from the ship's AIS position to arrive at the pilot station in time for a given berthing time, with the hours at anchor and the fuel it would save. Advice only: the Master decides the speed and the operator decides whether to order it.",
		Annotations: readOnly,
	}, func(_ context.Context, _ *mcp.CallToolRequest, in speedIn) (*mcp.CallToolResult, SpeedAdvice, error) {
		v, err := voyage(in.ShipID)
		if err != nil {
			return nil, SpeedAdvice{}, err
		}
		berth, err := time.Parse(time.RFC3339, in.BerthUTC)
		if err != nil {
			return nil, SpeedAdvice{}, fmt.Errorf("give berth_utc as an RFC 3339 time, e.g. 2025-01-27T06:00:00Z")
		}
		if len(v.Track) == 0 {
			return nil, SpeedAdvice{}, fmt.Errorf("no AIS track for %s in this demo", in.ShipID)
		}
		f := v.Track[len(v.Track)-1]
		r, err := g.Route([]route.Position{{f.Lon, f.Lat}, {v.To.Lon, v.To.Lat}}, route.Options{Avoid: v.Avoid})
		if err != nil {
			return nil, SpeedAdvice{}, err
		}
		out, err := AdviseSpeed(v, r.Path, berth.UTC())
		return nil, out, err
	})

	mcp.AddTool(s, &mcp.Tool{
		Name:        "off_hire_statement",
		Description: "Voyage management system (stand-in; figures made up for this simulation): previews an off-hire period on the hire statement: the hire that stops at the daily rate (pro rata to the cranes out of use, for a crane breakdown), and the bunkers used while off hire, charged to the owner. Enters nothing.",
		Annotations: readOnly,
	}, func(_ context.Context, _ *mcp.CallToolRequest, in offHireIn) (*mcp.CallToolResult, OffHireStatement, error) {
		v, err := voyage(in.ShipID)
		if err != nil {
			return nil, OffHireStatement{}, err
		}
		from, err1 := time.Parse(time.RFC3339, in.FromUTC)
		to, err2 := time.Parse(time.RFC3339, in.ToUTC)
		if err1 != nil || err2 != nil {
			return nil, OffHireStatement{}, fmt.Errorf("give from_utc and to_utc as RFC 3339 times, e.g. 2025-01-26T06:10:00Z")
		}
		out, err := PreviewOffHire(v, from.UTC(), to.UTC(), map[string]float64{"VLSFO": in.VLSFOt, "MGO": in.MGOt}, in.CranesOut)
		return nil, out, err
	})

	if wx != nil {
		mcp.AddTool(s, &mcp.Tool{
			Name:        "weather_on_track",
			Description: "Weather where the ship was during a past period, from its AIS track and ECMWF forecasts as issued at the time: wind force (Beaufort), sea state (Douglas) and where they came from relative to the ship's heading.",
			Annotations: readOnly,
		}, func(_ context.Context, _ *mcp.CallToolRequest, in weatherIn) (*mcp.CallToolResult, WeatherOut, error) {
			v, err := voyage(in.ShipID)
			if err != nil {
				return nil, WeatherOut{}, err
			}
			from, err1 := time.Parse(time.RFC3339, in.FromUTC)
			to, err2 := time.Parse(time.RFC3339, in.ToUTC)
			if err1 != nil || err2 != nil {
				return nil, WeatherOut{}, fmt.Errorf("give from_utc and to_utc as RFC 3339 times, e.g. 2025-01-21T12:00:00Z")
			}
			out, err := WeatherOnTrack(wx, v, from.UTC(), to.UTC())
			return nil, out, err
		})
	}
	return s
}

type errUnknownShip string

func (e errUnknownShip) Error() string { return "unknown ship ID " + string(e) }

func nonNil(s []string) []string {
	if s == nil {
		return []string{}
	}
	return s
}
