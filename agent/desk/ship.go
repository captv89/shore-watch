package desk

import (
	"encoding/json"
	"strings"

	"github.com/captv89/shorewatch/agent/eventlog"
	"github.com/captv89/shorewatch/sim"
)

// baseShip is the ship field of a run's events unless an event names its
// own: the scene's one ship, or "fleet" for a run over several (schema 1.6).
func baseShip(sc sim.Scene) string {
	if len(sc.Fleet) > 1 {
		return eventlog.Fleet
	}
	return sc.Voyage.Ship.ID
}

// eventShip is the ship an event in a fleet run is about: the ship_id a
// tool used, the ship whose people a draft goes to, the ship of an earlier
// event on the same call (its first source), or the one ship its words
// name. Anything about more than one ship, or none, is "fleet". Only the
// event's own call counts: the other sources a report cites may be any ship's.
func (r *run) eventShip(e eventlog.Event) string {
	b, _ := json.Marshal(map[string]any{"summary": e.Summary, "detail": e.Detail})
	var doc map[string]any
	_ = json.Unmarshal(b, &doc)
	detail, _ := doc["detail"].(map[string]any)

	ship := ""
	if id := findShipID(detail); id != "" {
		if v, ok := r.voyage(id); ok {
			ship = v.Ship.ID
		}
	}
	if ship == "" {
		ship = r.recipientShip(detail)
	}
	if ship == "" {
		if len(e.SourceRefs) > 0 {
			r.mu.Lock()
			ship = r.callShip[callID(e.SourceRefs[0])]
			r.mu.Unlock()
		}
	}
	if ship == "" {
		ship = r.namedShip(string(b))
	}
	if ship == "" {
		return eventlog.Fleet
	}
	if len(e.SourceRefs) > 0 {
		if id := callID(e.SourceRefs[0]); id != "" {
			r.mu.Lock()
			r.callShip[id] = ship
			r.mu.Unlock()
		}
	}
	return ship
}

// callID is the call behind a ref like "tool:draft_email#call-1" or "approval:call-1".
func callID(ref string) string {
	if _, id, ok := strings.Cut(ref, "#"); ok {
		return id
	}
	for _, p := range []string{"approval:", "question:"} {
		if strings.HasPrefix(ref, p) {
			return strings.TrimPrefix(ref, p)
		}
	}
	return ""
}

// findShipID looks for a ship_id where tools put it: in the detail, the
// call's arguments, or the result.
func findShipID(d map[string]any) string {
	for _, m := range []any{d, d["args"], d["result"], dig(d, "result", "output")} {
		if mm, ok := m.(map[string]any); ok {
			if id, ok := mm["ship_id"].(string); ok && id != "" {
				return id
			}
		}
	}
	return ""
}

func dig(m map[string]any, keys ...string) any {
	var v any = m
	for _, k := range keys {
		mm, ok := v.(map[string]any)
		if !ok {
			return nil
		}
		v = mm[k]
	}
	return v
}

// recipientShip is the ship a draft's recipients all belong to, if one.
func (r *run) recipientShip(d map[string]any) string {
	to, _ := dig(d, "draft", "to").([]any)
	if to == nil {
		to, _ = dig(d, "recipients", "to").([]any)
	}
	ship := ""
	for _, a := range to {
		addr, _ := a.(string)
		s := ""
		for _, v := range r.fleet() {
			if strings.EqualFold(addr, v.Ship.EmailLocal) {
				s = v.Ship.ID
			}
			for _, c := range v.Contacts {
				if strings.EqualFold(addr, c.Email) && c.Role == "ship" {
					s = v.Ship.ID
				}
			}
		}
		if s == "" || (ship != "" && s != ship) {
			return ""
		}
		ship = s
	}
	return ship
}

// namedShip is the one ship of the fleet the text names by ID or name.
func (r *run) namedShip(text string) string {
	low := strings.ToLower(text)
	found := ""
	for _, v := range r.fleet() {
		if strings.Contains(low, strings.ToLower(v.Ship.ID)) || strings.Contains(low, strings.TrimPrefix(strings.ToLower(v.Ship.Name), "mv ")) {
			if found != "" {
				return ""
			}
			found = v.Ship.ID
		}
	}
	return found
}
