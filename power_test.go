package main

import (
	"testing"
	"time"
)

// raplMaxTest is a real Intel RAPL max_energy_range_uj.
const raplMaxTest = 262143328850

func TestEnergyDelta(t *testing.T) {
	tests := []struct {
		name string
		cur  int64
		prev int64
		max  int64
		want int64
	}{
		{"normal increase", 200, 100, raplMaxTest, 100},
		{"wrap at hardware range", 5, raplMaxTest - 1, raplMaxTest, 6},
		{"wrap at 32-bit range", 5, (1 << 32) - 1, 1 << 32, 6},
		{"invalid range falls back to 32-bit", 5, (1 << 32) - 1, 0, 6},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := energyDelta(tt.cur, tt.prev, tt.max); got != tt.want {
				t.Fatalf(
					"energyDelta(%d, %d, %d) = %d, want %d",
					tt.cur, tt.prev, tt.max, got, tt.want,
				)
			}
		})
	}
}

// TestReadWrapDoesNotProduceNegativeWatts guards the wrap delta against negative watts.
func TestReadWrapDoesNotProduceNegativeWatts(t *testing.T) {
	s := raplState{
		zones:    []RAPLZone{{Name: "Package", EnergyPath: "/energy_uj", MaxRangeUJ: raplMaxTest}},
		prevUJ:   []int64{raplMaxTest - 1},
		prevTime: time.Now().Add(-time.Second),
	}
	fr := mkMock(map[string]string{"/energy_uj": "5"})

	r := s.Read(fr)
	if len(r.Zones) != 1 {
		t.Fatalf("got %d zones, want 1", len(r.Zones))
	}
	if r.Zones[0].Watts < 0 {
		t.Fatalf("watts = %v, want >= 0 after wrap at %d", r.Zones[0].Watts, raplMaxTest)
	}
}
