package main

import (
	"errors"
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
			name: "hwmon all fans off errors",
			files: map[string]string{
				"/sys/class/hwmon/hwmon0/fan1_input": "0",
				"/sys/class/hwmon/hwmon0/fan2_input": "junk",
			},
			fanFiles: []string{
				"/sys/class/hwmon/hwmon0/fan1_input",
				"/sys/class/hwmon/hwmon0/fan2_input",
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			fs := mkMock(tt.files)
			status, rpm, err := readFanStatus(fs, tt.fanFiles, tt.thinkpad, &[]string{})
			if tt.wantErr {
				if !errors.Is(err, ErrNoFanData) {
					t.Fatalf("err = %v, want ErrNoFanData", err)
				}
				if rpm != 0 {
					t.Errorf("rpm = %d, want 0 on no data", rpm)
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
