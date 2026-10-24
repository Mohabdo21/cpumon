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
	m.collect()
	b.ReportAllocs()
	b.ResetTimer()
	for b.Loop() {
		m.collect()
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
