package main

import (
	"strings"
	"testing"
	"time"
)

type countingFileReader struct {
	mockFileReader
	reads map[string]int
}

func (fr countingFileReader) Read(path string) (string, error) {
	fr.reads[path]++
	return fr.mockFileReader.Read(path)
}

func TestThermalLimitsCachedBetweenSamples(t *testing.T) {
	files := map[string]string{"input": "50000", "crit": "100000", "max": "90000"}
	fr := countingFileReader{mockFileReader: mkMock(files), reads: make(map[string]int)}
	temps := []HwmonTemp{{Input: "input", LabelName: "Core 2", Crit: "crit", Max: "max"}}
	var buf []CoreStatus
	if _, err := readThermalFromHwmon(fr, temps, nil, nil, &buf); err != nil {
		t.Fatal(err)
	}
	files["input"] = "60000"
	files["crit"] = "110000"
	cores, err := readThermalFromHwmon(fr, temps, nil, nil, &buf)
	if err != nil {
		t.Fatal(err)
	}
	if cores[0].TempC != 60 || cores[0].CoreLimit != "H90/C100" {
		t.Fatalf("live temperature/cached limits = %+v", cores[0])
	}
	if fr.reads["input"] != 2 || fr.reads["crit"] != 1 || fr.reads["max"] != 1 {
		t.Fatalf("sample reads = %v", fr.reads)
	}
	temps[0].limitsRefreshAt = time.Now().Add(-time.Second)
	cores, err = readThermalFromHwmon(fr, temps, nil, nil, &buf)
	if err != nil || cores[0].CoreLimit != "H90/C110" {
		t.Fatalf("expired limits = %v, %v", cores, err)
	}
	delete(files, "crit")
	delete(files, "max")
	temps[0].limitsRefreshAt = time.Time{}
	cores, err = readThermalFromHwmon(fr, temps, nil, nil, &buf)
	if err != nil || cores[0].CoreLimit != "" || cores[0].Limit != "" {
		t.Fatalf("removed limits = %v, %v", cores, err)
	}
}

func TestPreparedCoreGridMatchesOutput(t *testing.T) {
	for _, hybrid := range []bool{false, true} {
		topo := CoreTopology{
			Hybrid: hybrid,
			Classes: map[int]CoreClass{
				2:  CoreClassPerformance,
				10: CoreClassPerformance,
				4:  CoreClassEfficiency,
			},
		}
		files := map[string]string{"pkg": "50000", "c2": "40000", "c10": "45000", "c4": "42000"}
		temps := []HwmonTemp{
			{Input: "c10", LabelName: "Core 10"},
			{Input: "pkg", LabelName: "Package id 0"},
			{Input: "c4", LabelName: "Core 4"},
			{Input: "c2", LabelName: "Core 2"},
		}
		fr := mkMock(files)
		var buf []CoreStatus
		ordered := append([]HwmonTemp(nil), temps...)
		preparedTopo := topo
		prepareThermalOrder(ordered, &preparedTopo)
		for _, missing := range []string{"none", "c10", "pkg", "c4", "c2"} {
			delete(files, missing)
			for _, width := range []int{40, 120, 200} {
				original, originalErr := readThermalFromHwmon(fr, temps, nil, nil, &buf)
				var want strings.Builder
				writeCoreGrid(&want, original, SessionStats{}, width, topo)
				prepared, preparedErr := readThermalFromHwmon(fr, ordered, nil, nil, &buf)
				var got strings.Builder
				writeCoreGrid(&got, prepared, SessionStats{}, width, preparedTopo)
				if originalErr != preparedErr || got.String() != want.String() {
					t.Fatalf(
						"hybrid=%v missing=%s width=%d: output changed\ngot %q (%v)\nwant %q (%v)",
						hybrid,
						missing,
						width,
						got.String(),
						preparedErr,
						want.String(),
						originalErr,
					)
				}
			}
		}
	}
}
