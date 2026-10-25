package checker

import (
	"testing"

	"github.com/captv89/shorewatch/sim"
)

func TestCheckConsumption(t *testing.T) {
	cases := []struct {
		name     string
		reported float64
		excess   float64
		flagged  bool
	}{
		{"D1, +15%", 27.6, 15.0, true},
		{"D3, +18%", 28.3, 17.9, true},
		{"on warranty", 24.0, 0, false},
		{"at tolerance", 26.4, 10.0, false},
		{"under", 22.8, -5.0, false},
	}
	for _, c := range cases {
		r, err := CheckConsumption(c.reported, 24.0)
		if err != nil {
			t.Fatal(err)
		}
		if r.ExcessPct != c.excess || r.Flagged != c.flagged {
			t.Errorf("%s: excess %.1f%% flagged %v, want %.1f%% %v", c.name, r.ExcessPct, r.Flagged, c.excess, c.flagged)
		}
	}
	if _, err := CheckConsumption(25, 0); err == nil {
		t.Error("want error without a warranted figure")
	}
}

func TestClassifyWeatherDay(t *testing.T) {
	cp := sim.DefaultCP()
	cases := []struct {
		name               string
		in                 WeatherDay
		good, classifiable bool
	}{
		{"calm Channel day", WeatherDay{Hours: 24, Samples: 8, SamplesWithSea: 8, MaxBeaufort: 4, MaxDouglas: 3}, true, true},
		{"gale", WeatherDay{Hours: 24, Samples: 8, SamplesWithSea: 8, MaxBeaufort: 10, MaxDouglas: 8}, false, true},
		{"wind fine, swell high", WeatherDay{Hours: 24, Samples: 8, SamplesWithSea: 8, MaxBeaufort: 4, MaxDouglas: 5}, false, true},
		{"part day", WeatherDay{Hours: 12, Samples: 4, SamplesWithSea: 4, MaxBeaufort: 3, MaxDouglas: 2}, false, false},
		{"sea missing", WeatherDay{Hours: 24, Samples: 8, SamplesWithSea: 6, MaxBeaufort: 3, MaxDouglas: 2}, false, false},
	}
	for _, c := range cases {
		r := ClassifyWeatherDay(c.in, cp)
		if r.GoodWeatherDay != c.good || r.Classified != c.classifiable {
			t.Errorf("%s: good %v classified %v, want %v %v (%s)", c.name, r.GoodWeatherDay, r.Classified, c.good, c.classifiable, r.Interpretation)
		}
	}
}
