package main

import (
	"path/filepath"
	"strings"
	"time"
)

type RAPLZone struct {
	Name       string
	EnergyPath string
	MaxRangeUJ int64
}

type PowerReading struct {
	Available bool
	Zones     []PowerZone
}

type PowerZone struct {
	Name  string
	Watts float64
}

type raplState struct {
	zones    []RAPLZone
	prevUJ   []int64
	prevTime time.Time
}

func discoverRAPL(fr FileReader) []RAPLZone {
	matches, _ := filepath.Glob("/sys/class/powercap/intel-rapl:*/energy_uj")
	if len(matches) == 0 {
		matches, _ = filepath.Glob("/sys/class/powercap/intel-rapl/intel-rapl:*/energy_uj")
	}
	if len(matches) == 0 {
		return nil
	}

	zones := make([]RAPLZone, 0, len(matches))
	for _, ePath := range matches {
		dir := filepath.Dir(ePath)
		name, err := fr.Read(filepath.Join(dir, "name"))
		if err != nil || name == "" {
			continue
		}
		if _, ok := readInt(fr, ePath); !ok {
			continue
		}
		zones = append(zones, RAPLZone{
			Name:       formatRAPLName(name),
			EnergyPath: ePath,
			MaxRangeUJ: readMaxEnergyRange(fr, dir),
		})
	}
	return zones
}

// readMaxEnergyRange returns the counter wrap width in uJ, defaulting to 32-bit.
func readMaxEnergyRange(fr FileReader, dir string) int64 {
	if v, ok := readInt(fr, filepath.Join(dir, "max_energy_range_uj")); ok && v > 0 {
		return v
	}
	return 1 << 32
}

// energyDelta returns the forward energy delta, correcting for counter wraparound.
func energyDelta(cur, prev, maxRange int64) int64 {
	delta := cur - prev
	if delta < 0 {
		if maxRange <= 0 {
			maxRange = 1 << 32
		}
		delta = ((delta % maxRange) + maxRange) % maxRange
	}
	return delta
}

func formatRAPLName(name string) string {
	switch strings.ToLower(name) {
	case "package-0", "package-1":
		return "Package"
	case "core":
		return "Cores"
	case "uncore":
		return "Uncore"
	case "dram":
		return "DRAM"
	case "psys":
		return "Platform"
	default:
		return name
	}
}

func newRAPLState(fr FileReader, zones []RAPLZone) raplState {
	s := raplState{
		zones:    zones,
		prevUJ:   make([]int64, len(zones)),
		prevTime: time.Now(),
	}
	for i, z := range zones {
		if v, ok := readInt(fr, z.EnergyPath); ok {
			s.prevUJ[i] = v
		}
	}
	return s
}

func (s *raplState) Read(fr FileReader) PowerReading {
	if len(s.zones) == 0 {
		return PowerReading{}
	}

	now := time.Now()
	dt := now.Sub(s.prevTime).Seconds()
	if dt <= 0 {
		return PowerReading{Available: true}
	}

	result := PowerReading{
		Available: true,
		Zones:     make([]PowerZone, 0, len(s.zones)),
	}

	for i, z := range s.zones {
		cur, ok := readInt(fr, z.EnergyPath)
		if !ok {
			continue
		}

		delta := energyDelta(cur, s.prevUJ[i], z.MaxRangeUJ)

		watts := float64(delta) / 1e6 / dt
		s.prevUJ[i] = cur

		result.Zones = append(result.Zones, PowerZone{
			Name:  z.Name,
			Watts: watts,
		})
	}

	s.prevTime = now
	return result
}
