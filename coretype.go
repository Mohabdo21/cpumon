package main

import (
	"fmt"
	"path/filepath"
	"sort"
	"strconv"
)

type CoreClass int

const (
	CoreClassUnknown CoreClass = iota
	CoreClassPerformance
	CoreClassEfficiency
)

type CoreTopology struct {
	Classes map[int]CoreClass
	Hybrid  bool
	Vendor  string
}

func discoverCoreClasses(fr FileReader, coreMap map[int]int) CoreTopology {
	topo := CoreTopology{Classes: make(map[int]CoreClass)}

	if detectIntelHybrid(fr, coreMap, &topo) {
		return topo
	}
	if detectAMDPreferred(fr, coreMap, &topo) {
		return topo
	}
	return topo
}

func classifyByThreshold(
	values map[int]int64,
	minSpread int64,
	topo *CoreTopology,
	vendor string,
) bool {
	if len(values) < 2 {
		return false
	}

	var lo, hi int64
	for _, v := range values {
		if lo == 0 || v < lo {
			lo = v
		}
		if v > hi {
			hi = v
		}
	}

	if hi-lo < minSpread {
		return false
	}

	mid := (lo + hi) / 2
	for id, v := range values {
		if v >= mid {
			topo.Classes[id] = CoreClassPerformance
		} else {
			topo.Classes[id] = CoreClassEfficiency
		}
	}
	topo.Hybrid = true
	topo.Vendor = vendor
	return true
}

func detectIntelHybrid(fr FileReader, coreMap map[int]int, topo *CoreTopology) bool {
	baseFreqs := make(map[int]int64)
	seen := make(map[int]bool)

	for cpu, coreID := range coreMap {
		if seen[coreID] {
			continue
		}
		seen[coreID] = true

		path := fmt.Sprintf("/sys/devices/system/cpu/cpu%d/cpufreq/base_frequency", cpu)
		if freq, ok := readInt(fr, path); ok && freq > 0 {
			baseFreqs[coreID] = freq
		}
	}

	return classifyByThreshold(baseFreqs, 200000, topo, "Intel")
}

func detectAMDPreferred(fr FileReader, coreMap map[int]int, topo *CoreTopology) bool {
	matches, _ := filepath.Glob(
		"/sys/devices/system/cpu/cpu[0-9]*/cpufreq/amd_pstate_prefcore_ranking",
	)
	if len(matches) == 0 {
		return false
	}

	rankings := make(map[int]int64)
	seen := make(map[int]bool)

	for cpu, coreID := range coreMap {
		if seen[coreID] {
			continue
		}
		seen[coreID] = true

		path := fmt.Sprintf(
			"/sys/devices/system/cpu/cpu%d/cpufreq/amd_pstate_prefcore_ranking",
			cpu,
		)
		if rank, ok := readInt(fr, path); ok {
			rankings[coreID] = rank
		}
	}

	return classifyByThreshold(rankings, 10, topo, "AMD")
}

func classifyCores(cores []CoreStatus, topo CoreTopology) (perf, eff []CoreStatus) {
	for i := range cores {
		if cores[i].IsPackage {
			continue
		}

		id := extractCoreID(cores[i].Label)
		switch topo.Classes[id] {
		case CoreClassEfficiency:
			eff = append(eff, cores[i])
		default:
			perf = append(perf, cores[i])
		}
	}
	sortCoresByID(perf)
	sortCoresByID(eff)
	return
}

func extractCoreID(label string) int {
	m := coreNumRe.FindStringSubmatch(label)
	if m == nil {
		return -1
	}
	id, _ := strconv.Atoi(m[1])
	return id
}

func sortCoresByID(cores []CoreStatus) {
	sort.Slice(cores, func(i, j int) bool {
		return extractCoreID(cores[i].Label) < extractCoreID(cores[j].Label)
	})
}

func coreClassLabel(topo CoreTopology) string {
	if !topo.Hybrid {
		return ""
	}

	var p, e int
	for _, c := range topo.Classes {
		switch c {
		case CoreClassPerformance:
			p++
		case CoreClassEfficiency:
			e++
		}
	}

	switch topo.Vendor {
	case "AMD":
		return fmt.Sprintf("%dP + %dE (preferred-core)", p, e)
	default:
		return fmt.Sprintf("%dP + %dE (hybrid)", p, e)
	}
}
