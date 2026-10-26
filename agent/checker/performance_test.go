package checker

import (
	"testing"

	"github.com/captv89/shorewatch/sim"
)

func TestCheckConsumption(t *testing.T) {
	// TORMAR SPRING: about 12.5 kn on about 24 t, so at least 12.0 kn on at
	// most 25.2 t.
	cases := []struct {
		name         string
		reported, kn float64
		excess, over float64
		fuel, speed  bool
	}{
		{"D1, +15%", 27.6, 12.5, 15.0, 2.4, true, false},
		{"D3, +18%", 28.3, 9.5, 17.9, 3.1, true, true},
		{"on warranty", 24.0, 12.5, 0, 0, false, false},
		{"at the about margin", 25.2, 12.0, 5.0, 0, false, false},
		{"just over the margin", 25.3, 12.5, 5.4, 0.1, true, false},
		{"slow, fuel in", 23.0, 11.9, -4.2, 0, false, true},
		{"no speed given", 22.8, 0, -5.0, 0, false, false},
	}
	for _, c := range cases {
		r, err := CheckConsumption(c.reported, 24.0, c.kn, 12.5)
		if err != nil {
			t.Fatal(err)
		}
		if r.ExcessPct != c.excess || r.OverAllowedT != c.over || r.FuelFlagged != c.fuel || r.SpeedFlagged != c.speed || r.Flagged != (c.fuel || c.speed) {
			t.Errorf("%s: excess %.1f%% over %.1f t fuel %v speed %v, want %.1f%% %.1f t %v %v", c.name, r.ExcessPct, r.OverAllowedT, r.FuelFlagged, r.SpeedFlagged, c.excess, c.over, c.fuel, c.speed)
		}
	}
	d1, _ := CheckConsumption(27.6, 24.0, 12.5, 12.5)
	if d1.AllowedT != 25.2 || d1.OverAllowedPct != 9.5 || d1.MinSpeedKn != 12.0 {
		t.Errorf("D1: allowed %.1f t, %.1f%% over it, min speed %.1f kn; want 25.2, 9.5, 12.0", d1.AllowedT, d1.OverAllowedPct, d1.MinSpeedKn)
	}
	if _, err := CheckConsumption(25, 0, 12, 12.5); err == nil {
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
