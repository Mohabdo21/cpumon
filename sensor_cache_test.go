package main

import (
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
