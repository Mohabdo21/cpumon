package main

import (
	"fmt"
	"strings"
	"testing"
)

func BenchmarkCollect(b *testing.B) {
	files := map[string]string{}
	var stat strings.Builder
	stat.WriteString("cpu 100 0 0 100 0 0 0 0 0 0\n")
	m := Monitor{
		coreFreqBuf:  make(map[int]string),
		cpuCoreMap:   make(map[int]int),
		curCoreBuf:   make(map[int]CPUTimes),
		coreDeltaBuf: make(map[int]cpuDelta),
		coreUsageBuf: make(map[int]float64),
	}
	for i := range 16 {
		fmt.Fprintf(&stat, "cpu%d 100 0 0 100 0 0 0 0 0 0\n", i)
		input := fmt.Sprintf("temp%d_input", i)
		files[input] = "50000"
		files[input+"crit"] = "100000"
		files[input+"max"] = "90000"
		m.hwmonTemps = append(m.hwmonTemps, HwmonTemp{
			Input: input, LabelName: fmt.Sprintf("Core %d", i),
			Crit: input + "crit", Max: input + "max",
		})
		m.cpuCoreMap[i] = i
	}
	stat.WriteString("intr " + strings.Repeat("0 ", 2200) + "\nctxt 123\n")
	files[procStatPath] = stat.String()
	m.fr = mkMock(files)
	prepareThermalOrder(m.hwmonTemps, &m.topology)
	m.collect()
	b.ReportAllocs()
	b.ResetTimer()
	for b.Loop() {
		m.collect()
	}
}

func BenchmarkCoreGrid(b *testing.B) {
	topo := CoreTopology{Hybrid: true, Classes: map[int]CoreClass{}}
	cores := []CoreStatus{{Label: "Package id 0", IsPackage: true, Temp: "+50.0°C", TempC: 50}}
	for i := range 16 {
		topo.Classes[i] = CoreClassPerformance
		if i >= 8 {
			topo.Classes[i] = CoreClassEfficiency
		}
		cores = append(
			cores,
			CoreStatus{
				coreID: i,
				Label:  fmt.Sprintf("Core %d", i),
				Temp:   "+50.0°C",
				TempC:  50,
				Usage:  50,
			},
		)
	}
	prepareThermalOrder(nil, &topo)
	var out strings.Builder
	b.ReportAllocs()
	b.ResetTimer()
	for b.Loop() {
		out.Reset()
		writeCoreGrid(&out, cores, SessionStats{}, 120, topo)
	}
}

func TestProcStatIgnoresLargeNonCPURows(t *testing.T) {
	fr := mkMock(
		map[string]string{
			procStatPath: "cpu 10 0 0 10 0\n" + strings.Repeat(
				"intr "+strings.Repeat("0 ", 2200)+"\n",
				20,
			),
		},
	)
	allocs := testing.AllocsPerRun(10, func() { readProcStat(fr, nil) })
	if allocs > 5 {
		t.Fatalf("unrelated rows allocated: %.0f allocations, want <= 5", allocs)
	}
}
