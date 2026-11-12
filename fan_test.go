package main

import (
	"errors"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
)

func TestReadFanStatus(t *testing.T) {
	tests := []struct {
		name     string
		files    map[string]string
		fanFiles []string
		thinkpad bool
		wantRPM  int64
		wantErr  bool
	}{
		{
			name: "thinkpad",
			files: map[string]string{
				thinkpadFanPath: "status:\t\tenabled\nspeed:\t\t3480\nlevel:\t\tauto\n",
			},
			thinkpad: true,
			wantRPM:  3480,
		},
		{
			name: "hwmon returns max across fans",
			files: map[string]string{
				"/sys/class/hwmon/hwmon0/fan1_input": "4200",
				"/sys/class/hwmon/hwmon0/fan1_label": "fan1",
				"/sys/class/hwmon/hwmon0/fan2_input": "3100",
				"/sys/class/hwmon/hwmon0/fan2_label": "fan2",
			},
			fanFiles: []string{
				"/sys/class/hwmon/hwmon0/fan1_input",
				"/sys/class/hwmon/hwmon0/fan2_input",
			},
			wantRPM: 4200,
		},
		{
			name: "hwmon stopped fan is valid",
			files: map[string]string{
				"/sys/class/hwmon/hwmon0/fan1_input": "0",
				"/sys/class/hwmon/hwmon0/fan2_input": "junk",
			},
			fanFiles: []string{
				"/sys/class/hwmon/hwmon0/fan1_input",
				"/sys/class/hwmon/hwmon0/fan2_input",
			},
			wantRPM: 0,
		},
		{
			name: "thinkpad stopped fan is valid",
			files: map[string]string{
				thinkpadFanPath: "status: enabled\nspeed: 0\nlevel: auto\n",
			},
			thinkpad: true,
		},
		{
			name: "thinkpad rejects all-ones reading",
			files: map[string]string{
				thinkpadFanPath: "status: enabled\nspeed: 65535\nlevel: auto\n",
			},
			thinkpad: true,
			wantErr:  true,
		},
		{
			name:     "thinkpad missing speed is unavailable",
			files:    map[string]string{thinkpadFanPath: "status: enabled\nlevel: auto\n"},
			thinkpad: true,
			wantErr:  true,
		},
		{
			name: "thinkpad invalid speed falls back to hwmon",
			files: map[string]string{
				thinkpadFanPath:     "status: enabled\nspeed: 65535\nlevel: auto\n",
				"hwmon0/fan1_input": "3200",
			},
			thinkpad: true,
			fanFiles: []string{"hwmon0/fan1_input"},
			wantRPM:  3200,
		},
		{
			name:     "thinkpad hwmon rejects all-ones reading",
			files:    map[string]string{"hwmon0/name": "thinkpad", "hwmon0/fan1_input": "65535"},
			fanFiles: []string{"hwmon0/fan1_input"},
			wantErr:  true,
		},
		{
			name:     "other hwmon accepts same reading",
			files:    map[string]string{"hwmon0/name": "other", "hwmon0/fan1_input": "65535"},
			fanFiles: []string{"hwmon0/fan1_input"},
			wantRPM:  65535,
		},
		{
			name:     "hwmon rejects negative speed",
			files:    map[string]string{"hwmon0/fan1_input": "-1"},
			fanFiles: []string{"hwmon0/fan1_input"},
			wantErr:  true,
		},
		{
			name:     "missing fan is unavailable",
			fanFiles: []string{"hwmon0/fan1_input"},
			wantErr:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			fs := mkMock(tt.files)
			status, rpm, err := readFanStatus(
				fs,
				tt.fanFiles,
				make(map[string]string),
				tt.thinkpad,
				&[]string{},
			)
			if tt.wantErr {
				if !errors.Is(err, ErrNoFanData) {
					t.Fatalf("err = %v, want ErrNoFanData", err)
				}
				if rpm != -1 {
					t.Errorf("rpm = %d, want -1 on no data", rpm)
				}
				if status != "N/A" {
					t.Errorf("unavailable fan status = %q, want N/A", status)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if rpm != tt.wantRPM {
				t.Errorf("rpm = %d, want %d", rpm, tt.wantRPM)
			}
			if status == "" {
				t.Errorf("status should not be empty")
			}
		})
	}
}

func TestFanSpeedRPM(t *testing.T) {
	tests := []struct {
		line string
		want int64
		ok   bool
	}{
		{line: "speed:\t\t3480", want: 3480, ok: true},
		{line: "speed: 0", want: 0, ok: true},
		{line: "speed: 65535", want: 65535, ok: false},
		{line: "speed: -1", want: -1, ok: false},
		{line: "speed: 65534", want: 65534, ok: true},
		{line: "speed: 65536", want: 65536, ok: true},
		{line: "level:\t\tauto", want: 0, ok: false},
		{line: "speed:\t\tnotanumber", want: 0, ok: false},
	}

	for _, tt := range tests {
		got, ok := fanSpeedRPM(tt.line)
		if got != tt.want || ok != tt.ok {
			t.Errorf("fanSpeedRPM(%q) = %d, %v; want %d, %v", tt.line, got, ok, tt.want, tt.ok)
		}
	}
}

func TestHwmonFanFault(t *testing.T) {
	for _, tt := range []struct {
		name    string
		fault   string
		err     error
		wantRPM int64
	}{
		{name: "faulted", fault: "1", wantRPM: 2100},
		{name: "healthy", fault: "0", wantRPM: 4200},
		{name: "unsupported", err: os.ErrNotExist, wantRPM: 4200},
		{name: "unreadable", err: os.ErrPermission, wantRPM: 2100},
		{name: "malformed", fault: "junk", wantRPM: 2100},
	} {
		t.Run(tt.name, func(t *testing.T) {
			fr := mkMock(map[string]string{
				"hwmon0/fan1_input": "4200", "hwmon0/fan1_label": "CPU",
				"hwmon0/fan1_fault": tt.fault,
				"hwmon0/fan2_input": "2100", "hwmon0/fan2_label": "Case",
			})
			fr.errs["hwmon0/fan1_fault"] = tt.err
			status, rpm, err := readFanStatus(fr,
				[]string{"hwmon0/fan1_input", "hwmon0/fan2_input"},
				make(map[string]string), false, &[]string{})
			if err != nil || rpm != tt.wantRPM {
				t.Fatalf("fan = %q, %d, %v; want %d RPM", status, rpm, err, tt.wantRPM)
			}
			if tt.wantRPM == 2100 && strings.Contains(status, "CPU:") {
				t.Errorf("untrusted reading shown: %q", status)
			}
		})
	}
}

func TestFanLabelCached(t *testing.T) {
	files := map[string]string{"hwmon0/fan1_input": "4200", "hwmon0/fan1_label": "CPU"}
	fr := countingFileReader{mockFileReader: mkMock(files), reads: make(map[string]int)}
	var buf []string
	paths := []string{"hwmon0/fan1_input"}
	labels := make(map[string]string)
	if _, _, err := readFanStatus(fr, paths, labels, false, &buf); err != nil {
		t.Fatal(err)
	}
	files["hwmon0/fan1_label"] = "changed"
	status, rpm, err := readFanStatus(fr, paths, labels, false, &buf)
	if err != nil || rpm != 4200 || status != "[hwmon]\nCPU: 4200 RPM" {
		t.Fatalf("cached fan label = %q, %d, %v", status, rpm, err)
	}
	if fr.reads["hwmon0/fan1_input"] != 2 || fr.reads["hwmon0/fan1_label"] != 1 {
		t.Fatalf("sample reads = %v", fr.reads)
	}
}

func TestDiscoverFanFilesListsEachFanOnce(t *testing.T) {
	root := t.TempDir()
	device := filepath.Join(root, "devices/platform/fanchip/hwmon/hwmon0")
	class := filepath.Join(root, "class/hwmon")
	for _, dir := range []string{device, class} {
		if err := os.MkdirAll(dir, 0o750); err != nil {
			t.Fatal(err)
		}
	}
	input := filepath.Join(device, "fan1_input")
	if err := os.WriteFile(input, []byte("1200\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(device, filepath.Join(class, "hwmon0")); err != nil {
		t.Fatal(err)
	}

	got := discoverFanFiles(root)
	want := []string{filepath.Join(class, "hwmon0/fan1_input")}
	if !slices.Equal(got, want) {
		t.Fatalf("discoverFanFiles = %q, want %q", got, want)
	}
}
