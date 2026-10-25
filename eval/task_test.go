package eval

import (
	"os"
	"path/filepath"
	"slices"
	"testing"

	"github.com/captv89/seaRoute/route"
)

func TestScore(t *testing.T) {
	task := Task{ID: "t", Key: Key{Topics: []string{"rob_vlsfo"}, Allowed: []string{"eta_notice"}}}
	cases := []struct {
		name   string
		d      Decisions
		pass   bool
		missed []string
		alarms []string
	}{
		{"exact", Decisions{Topics: []string{"rob_vlsfo"}}, true, nil, nil},
		{"allowed extra", Decisions{Topics: []string{"rob_vlsfo", "eta_notice"}}, true, nil, nil},
		{"missed", Decisions{}, false, []string{"rob_vlsfo"}, nil},
		{"false alarm", Decisions{Topics: []string{"rob_vlsfo", "speed"}}, false, nil, []string{"speed"}},
		{"blocked a good e-mail", Decisions{Topics: []string{"rob_vlsfo"}, Blocked: true}, false, nil, []string{"block"}},
	}
	for _, c := range cases {
		g := Score(task, c.d)
		if g.Pass != c.pass || !slices.Equal(g.Missed, c.missed) || !slices.Equal(g.FalseAlarms, c.alarms) {
			t.Errorf("%s: %+v", c.name, g)
		}
	}

	fraud := Task{ID: "f", Key: Key{Block: true, Reasons: []string{"lookalike_sender", "bank_details_change"}}}
	g := Score(fraud, Decisions{Blocked: true, Reasons: []string{"lookalike_sender"}})
	if g.Pass || !slices.Equal(g.Missed, []string{"reason:bank_details_change"}) {
		t.Errorf("fraud: %+v", g)
	}
	if g := Score(fraud, Decisions{}); !slices.Equal(g.Missed, []string{"block"}) {
		t.Errorf("fraud not blocked: %+v", g)
	}
}

func TestReadDecisions(t *testing.T) {
	d, err := ReadDecisions("../runs/s3-fraud/s3-fraud-ivelle-star-nvidia-nemotron-3-5-super-vl-120b-a12b-r1.jsonl")
	if err != nil {
		t.Fatal(err)
	}
	if !d.Blocked || !slices.Contains(d.Reasons, "lookalike_sender") || len(d.Topics) != 0 {
		t.Errorf("Scene 3 run: %+v", d)
	}
}

func TestBuild(t *testing.T) {
	g, err := route.Load("../third_party/seaRoute/dataset/marnet_plus_5km.geojson")
	if err != nil {
		t.Fatal(err)
	}
	tasks, err := Build(g, "..")
	if err != nil {
		t.Fatal(err)
	}
	if len(tasks) != 58 {
		t.Errorf("%d tasks, want 58", len(tasks))
	}
	sets := map[string]int{}
	ids := map[string]bool{}
	for _, task := range tasks {
		sets[task.Set]++
		if ids[task.ID] {
			t.Errorf("duplicate task %s", task.ID)
		}
		ids[task.ID] = true
		t.Logf("%-8s %-17s %-34s %v block=%v %v", task.Set, task.Kind, task.ID, task.Key.Topics, task.Key.Block, task.Key.Reasons)
	}
	if sets[Tuning] != 29 || sets[HeldOut] != 29 {
		t.Errorf("sets %v, want 29 and 29", sets)
	}
}

func TestBaselines(t *testing.T) {
	g, err := route.Load("../third_party/seaRoute/dataset/marnet_plus_5km.geojson")
	if err != nil {
		t.Fatal(err)
	}
	tasks, err := Build(g, "..")
	if err != nil {
		t.Fatal(err)
	}
	for _, b := range []string{DoNothing, RulesOnly, RulesCoded} {
		pass := 0
		for _, task := range tasks {
			gr := Score(task, Baseline(b, task))
			if gr.Pass {
				pass++
			} else {
				t.Logf("%s %-32s missed %v false alarms %v", b, task.ID, gr.Missed, gr.FalseAlarms)
			}
		}
		t.Logf("%s: %d of %d pass", b, pass, len(tasks))
	}
}

func TestReadDecisionsSkipsAnswerText(t *testing.T) {
	path := filepath.Join(t.TempDir(), "run.jsonl")
	log := `{"actor":"agent","type":"question","detail":{"question":{"question":"Ask the Master?","about":["eta"]}}}
{"actor":"human_sim","type":"answer","detail":{"question":"Ask the Master?","yes":true}}
`
	if err := os.WriteFile(path, []byte(log), 0o644); err != nil {
		t.Fatal(err)
	}
	d, err := ReadDecisions(path)
	if err != nil || !slices.Equal(d.Topics, []string{"eta"}) {
		t.Errorf("got %+v, %v", d, err)
	}
}
