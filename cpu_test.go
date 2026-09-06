package main

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
)

func TestDiscoverHwmonTemps(t *testing.T) {
	dir := t.TempDir()
	must := func(name string) {
		t.Helper()
		p := filepath.Join(dir, name)
		if err := os.WriteFile(p, []byte("0"), 0o600); err != nil {
			t.Fatalf("write %s: %v", name, err)
		}
	}
	must("temp1_input")
	must("temp2_input")

	tests := []struct {
		name  string
		files map[string]string
		errs  map[string]error
		want  map[string]string // input path -> expected LabelName
	}{
		{
			name:  "caches label at discovery",
			files: map[string]string{filepath.Join(dir, "temp1_label"): "Core 0"},
			want: map[string]string{
				filepath.Join(dir, "temp1_input"): "Core 0",
				filepath.Join(dir, "temp2_input"): "CPU",
			},
		},
		{
			name: "unreadable label defaults to CPU",
			errs: map[string]error{
				filepath.Join(dir, "temp1_label"): errors.New("denied"),
				filepath.Join(dir, "temp2_label"): errors.New("denied"),
			},
			want: map[string]string{
				filepath.Join(dir, "temp1_input"): "CPU",
				filepath.Join(dir, "temp2_input"): "CPU",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := discoverHwmonTemps(mockFileReader{files: tt.files, errs: tt.errs}, dir)
			if len(got) != len(tt.want) {
				t.Fatalf("got %d temps, want %d: %v", len(got), len(tt.want), got)
			}
			for _, temp := range got {
				if want := tt.want[temp.Input]; temp.LabelName != want {
					t.Errorf(
						"%s LabelName = %q, want %q",
						filepath.Base(temp.Input), temp.LabelName, want,
					)
				}
				if temp.Crit == "" || temp.Max == "" {
					t.Errorf("%s: missing crit/max path", filepath.Base(temp.Input))
				}
			}
		})
	}
}

func TestReadThermalFromHwmon(t *testing.T) {
	// Base fixture: one package temp with full crit+max limit data.
	//
	//   input 50000 milli  -> +50.0°C
	//   crit  90000 milli  -> +90.0°C -> C90
	//   max   85000 milli  -> +85.0°C -> H85/C90
	base := map[string]string{
		"t/input": "50000",
		"t/label": "Package id 0",
		"t/crit":  "90000",
		"t/max":   "85000",
	}

	tests := []struct {
		name      string
		files     map[string]string
		errs      map[string]error
		temps     []HwmonTemp
		coreFreqs map[int]string
		coreUsage map[int]float64
		wantCores int
		check     func(*testing.T, []CoreStatus)
		wantErr   bool
	}{
		{
			name:    "no temps errors",
			temps:   nil,
			wantErr: true,
		},
		{
			name:  "package temp with crit and max",
			files: base,
			temps: []HwmonTemp{
				{Input: "t/input", LabelName: "Package id 0", Crit: "t/crit", Max: "t/max"},
			},
			wantCores: 1,
			check: func(t *testing.T, cores []CoreStatus) {
				c := cores[0]
				if c.Temp != "+50.0°C" {
					t.Errorf("Temp = %q, want +50.0°C", c.Temp)
				}
				if c.Limit != "(high = +85.0°C, crit = +90.0°C)" {
					t.Errorf("Limit = %q", c.Limit)
				}
				if c.CoreLimit != "H85/C90" {
					t.Errorf("CoreLimit = %q, want H85/C90", c.CoreLimit)
				}
				if !c.IsPackage {
					t.Errorf("expected package core, got IsPackage=false")
				}
				if c.Usage != -1 {
					t.Errorf("Usage = %v, want -1 for package", c.Usage)
				}
			},
		},
		{
			name: "core temp maps freq and usage",
			files: map[string]string{
				"c/input": "42000",
				"c/label": "Core 2",
			},
			temps:     []HwmonTemp{{Input: "c/input", LabelName: "Core 2"}},
			coreFreqs: map[int]string{2: "3.4GHz"},
			coreUsage: map[int]float64{2: 55.5},
			wantCores: 1,
			check: func(t *testing.T, cores []CoreStatus) {
				c := cores[0]
				if c.IsPackage {
					t.Errorf("expected core, got IsPackage=true")
				}
				if c.Freq != "3.4GHz" {
					t.Errorf("Freq = %q, want 3.4GHz", c.Freq)
				}
				if c.Usage != 55.5 {
					t.Errorf("Usage = %v, want 55.5", c.Usage)
				}
			},
		},
		{
			name: "limit only when crit present",
			files: map[string]string{
				"t/input": "50000",
				"t/label": "Tctl",
				"t/crit":  "105000",
			},
			temps:     []HwmonTemp{{Input: "t/input", LabelName: "Tctl", Crit: "t/crit"}},
			wantCores: 1,
			check: func(t *testing.T, cores []CoreStatus) {
				c := cores[0]
				if c.Limit != "(crit = +105.0°C)" {
					t.Errorf("Limit = %q, want (crit = +105.0°C)", c.Limit)
				}
				if c.CoreLimit != "C105" {
					t.Errorf("CoreLimit = %q, want C105", c.CoreLimit)
				}
			},
		},
		{
			name: "limit when only max present",
			files: map[string]string{
				"t/input": "50000",
				"t/label": "Tctl",
				"t/max":   "85000",
			},
			temps:     []HwmonTemp{{Input: "t/input", LabelName: "Tctl", Max: "t/max"}},
			wantCores: 1,
			check: func(t *testing.T, cores []CoreStatus) {
				c := cores[0]
				if c.Limit != "(high = +85.0°C)" {
					t.Errorf("Limit = %q, want (high = +85.0°C)", c.Limit)
				}
				if c.CoreLimit != "H85" {
					t.Errorf("CoreLimit = %q, want H85", c.CoreLimit)
				}
			},
		},
		{
			name: "no limit when neither crit nor max present",
			files: map[string]string{
				"t/input": "50000",
				"t/label": "Tctl",
			},
			temps:     []HwmonTemp{{Input: "t/input", LabelName: "Tctl"}},
			wantCores: 1,
			check: func(t *testing.T, cores []CoreStatus) {
				c := cores[0]
				if c.Limit != "" {
					t.Errorf("Limit = %q, want empty", c.Limit)
				}
				if c.CoreLimit != "" {
					t.Errorf("CoreLimit = %q, want empty", c.CoreLimit)
				}
			},
		},
		{
			name:  "unreadable input skipped",
			files: base,
			errs:  map[string]error{"t/input": errors.New("denied")},
			temps: []HwmonTemp{
				{Input: "t/input", LabelName: "Package id 0", Crit: "t/crit", Max: "t/max"},
			},
			wantErr: true,
		},
		{
			name: "normally an empty result set would error but temps exist and one skips",
			files: map[string]string{
				"b/input": "bad",
				"b/label": "Core 0",
			},
			temps:   []HwmonTemp{{Input: "b/input", LabelName: "Core 0"}},
			wantErr: true, // readInt fails on non-numeric input => coerced empty
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			fs := mockFileReader{files: tt.files, errs: tt.errs}
			buf := make([]CoreStatus, 0, 4)
			cores, err := readThermalFromHwmon(fs, tt.temps, tt.coreFreqs, tt.coreUsage, &buf)
			if tt.wantErr {
				if err == nil {
					t.Fatalf("expected error, got cores %v", cores)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if len(cores) != tt.wantCores {
				t.Fatalf("got %d cores, want %d: %v", len(cores), tt.wantCores, cores)
			}
			if tt.check != nil {
				tt.check(t, cores)
			}
		})
	}
}
