package main

import (
	"errors"
	"regexp"
	"slices"
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
	prepareThermalOrder(temps, &CoreTopology{})
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

func TestCoreGridGroupsSurviveMissingSensors(t *testing.T) {
	gridItem := regexp.MustCompile(`Package id 0|P-Cores|E-Cores|Core \d+`)
	removal := []string{"", "c10", "pkg", "c4", "c2"}
	for _, tt := range []struct {
		hybrid bool
		want   [][]string // Grid after removing removal[:i+1].
	}{
		{hybrid: false, want: [][]string{
			{"Package id 0", "Core 10", "Core 4", "Core 2"},
			{"Package id 0", "Core 4", "Core 2"},
			{"Core 4", "Core 2"},
			{"Core 2"},
			nil,
		}},
		{hybrid: true, want: [][]string{
			{"Package id 0", "P-Cores", "Core 2", "Core 10", "E-Cores", "Core 4"},
			{"Package id 0", "P-Cores", "Core 2", "E-Cores", "Core 4"},
			{"P-Cores", "Core 2", "E-Cores", "Core 4"},
			{"P-Cores", "Core 2"},
			nil,
		}},
	} {
		topo := CoreTopology{
			Hybrid: tt.hybrid,
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
		prepareThermalOrder(temps, &topo)
		fr := mkMock(files)
		var buf []CoreStatus
		for i, missing := range removal {
			delete(files, missing)
			var wantErr error
			if tt.want[i] == nil {
				wantErr = ErrNoThermalData
			}
			for _, width := range []int{40, 120, 200} {
				cores, err := readThermalFromHwmon(fr, temps, nil, nil, &buf)
				var out strings.Builder
				writeCoreGrid(&out, cores, SessionStats{}, width, topo)
				got := gridItem.FindAllString(out.String(), -1)
				if !errors.Is(err, wantErr) || !slices.Equal(got, tt.want[i]) {
					t.Fatalf("hybrid=%v missing=%q width=%d: grid = %q (%v), want %q (%v)",
						tt.hybrid, removal[:i+1], width, got, err, tt.want[i], wantErr)
				}
			}
		}
	}
}
