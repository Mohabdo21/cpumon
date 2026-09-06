package main

import (
	"errors"
	"os"
	"testing"
)

// mockFileReader serves fake file contents from a map; used to exercise
// parsing and math without touching the real /sys or /proc filesystem.
type mockFileReader struct {
	files map[string]string
	errs  map[string]error
}

func (m mockFileReader) Read(path string) (string, error) {
	if err := m.errs[path]; err != nil {
		return "", err
	}
	if s, ok := m.files[path]; ok {
		return s, nil
	}
	return "", os.ErrNotExist
}

func mkMock(files map[string]string) mockFileReader {
	return mockFileReader{files: files, errs: map[string]error{}}
}

func TestReadProcStat(t *testing.T) {
	tests := []struct {
		name     string
		data     string
		errPath  string
		wantAgg  CPUTimes
		wantCore map[int]CPUTimes
	}{
		{
			name: "aggregate and per-core",
			data: "cpu  100 0 0 120 0 0 0 0 0 0\n" +
				"cpu0 30 0 0 40 0 0 0 0 0 0\n" +
				"cpu1 70 0 0 80 0 0 0 0 0 0\n",
			wantAgg: CPUTimes{Idle: 120, Total: 220, Valid: true},
			wantCore: map[int]CPUTimes{
				0: {Idle: 40, Total: 70, Valid: true},
				1: {Idle: 80, Total: 150, Valid: true},
			},
		},
		{
			name:    "idle sums idle+iowait columns",
			data:    "cpu 100 10 20 30 0 0 0 0 0 0\n",
			wantAgg: CPUTimes{Idle: 30, Total: 160, Valid: true},
		},
		{
			name:    "read error returns invalid",
			errPath: procStatPath,
			wantAgg: CPUTimes{}, wantCore: nil,
		},
		{
			name: "short lines skipped",
			data: "cpu 1\ncpu0\ncpu1 5\n" +
				"cpu 10 10 20 30 0\n" +
				"cpu0 10 10 20 30 0\n",
			wantAgg:  CPUTimes{Idle: 30, Total: 70, Valid: true},
			wantCore: map[int]CPUTimes{0: {Idle: 30, Total: 70, Valid: true}},
		},
		{
			name: "irq/softirq guest lines ignored",
			data: "cpu 10 0 0 5 0 0 0 0 0 0\n" +
				"intr 12345\n" +
				"ctxt 99\n",
			wantAgg:  CPUTimes{Idle: 5, Total: 15, Valid: true},
			wantCore: map[int]CPUTimes{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			fs := mkMock(map[string]string{procStatPath: tt.data})
			if tt.errPath != "" {
				fs.errs[tt.errPath] = errors.New("read failed")
			}
			agg, cores := readProcStat(fs)
			if agg != tt.wantAgg {
				t.Errorf("agg = %+v, want %+v", agg, tt.wantAgg)
			}
			if len(cores) != len(tt.wantCore) {
				t.Fatalf("cores = %v, want %v", cores, tt.wantCore)
			}
			for k, v := range tt.wantCore {
				if cores[k] != v {
					t.Errorf("core[%d] = %+v, want %+v", k, cores[k], v)
				}
			}
		})
	}
}

func TestCalcUsage(t *testing.T) {
	tests := []struct {
		name string
		prev CPUTimes
		cur  CPUTimes
		want float64
	}{
		{
			name: "simple delta",
			prev: CPUTimes{Idle: 50, Total: 100, Valid: true},
			cur:  CPUTimes{Idle: 60, Total: 200, Valid: true},
			want: 90,
		},
		{
			name: "zero busy",
			prev: CPUTimes{Idle: 50, Total: 200, Valid: true},
			cur:  CPUTimes{Idle: 60, Total: 210, Valid: true},
			want: 0,
		},
		{name: "prev invalid", prev: CPUTimes{}, cur: CPUTimes{Valid: true}, want: -1},
		{name: "cur invalid", prev: CPUTimes{Valid: true}, cur: CPUTimes{}, want: -1},
		{
			name: "zero delta",
			prev: CPUTimes{Idle: 50, Total: 100, Valid: true},
			cur:  CPUTimes{Idle: 50, Total: 100, Valid: true},
			want: 0,
		},
		{
			name: "total decrease treated as no data",
			prev: CPUTimes{Idle: 50, Total: 200, Valid: true},
			cur:  CPUTimes{Idle: 60, Total: 100, Valid: true},
			want: -1,
		},
		{
			name: "idle decrease treated as no data",
			prev: CPUTimes{Idle: 50, Total: 100, Valid: true},
			cur:  CPUTimes{Idle: 40, Total: 200, Valid: true},
			want: -1,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := calcUsage(tt.prev, tt.cur)
			if got != tt.want {
				t.Errorf("calcUsage() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestCalcPerCoreUsage(t *testing.T) {
	tests := []struct {
		name      string
		prev      map[int]CPUTimes
		cur       map[int]CPUTimes
		cpuToCore map[int]int
		want      map[int]float64
	}{
		{
			name: "cpu maps to core",
			prev: map[int]CPUTimes{
				0: {Idle: 50, Total: 100, Valid: true},
				1: {Idle: 30, Total: 100, Valid: true},
			},
			cur: map[int]CPUTimes{
				0: {Idle: 60, Total: 200, Valid: true},
				1: {Idle: 40, Total: 200, Valid: true},
			},
			cpuToCore: map[int]int{0: 0, 1: 1},
			want:      map[int]float64{0: 90, 1: 90},
		},
		{
			name: "two cpus aggregate into one core",
			prev: map[int]CPUTimes{
				0: {Idle: 0, Total: 0, Valid: true},
				1: {Idle: 0, Total: 0, Valid: true},
			},
			cur: map[int]CPUTimes{
				0: {Idle: 0, Total: 100, Valid: true},
				1: {Idle: 0, Total: 300, Valid: true},
			},
			cpuToCore: map[int]int{0: 3, 1: 3},
			want:      map[int]float64{3: 100},
		},
		{
			name: "cpu missing in prev skipped",
			prev: map[int]CPUTimes{0: {Idle: 0, Total: 0, Valid: true}},
			cur: map[int]CPUTimes{
				0: {Idle: 0, Total: 100, Valid: true},
				1: {Idle: 0, Total: 100, Valid: true},
			},
			cpuToCore: map[int]int{},
			want:      map[int]float64{0: 100},
		},
		{
			name: "nil maps",
			prev: nil,
			cur:  map[int]CPUTimes{},
			want: nil,
		},
		{
			name: "total decrease skipped",
			prev: map[int]CPUTimes{
				0: {Idle: 50, Total: 200, Valid: true},
				1: {Idle: 30, Total: 100, Valid: true},
			},
			cur: map[int]CPUTimes{
				0: {Idle: 60, Total: 100, Valid: true}, // total dropped -> skip cpu0
				1: {Idle: 40, Total: 200, Valid: true},
			},
			cpuToCore: map[int]int{0: 0, 1: 1},
			want:      map[int]float64{1: 90},
		},
		{
			name: "idle decrease skipped",
			prev: map[int]CPUTimes{
				0: {Idle: 50, Total: 100, Valid: true},
				1: {Idle: 30, Total: 100, Valid: true},
			},
			cur: map[int]CPUTimes{
				0: {Idle: 40, Total: 200, Valid: true}, // idle dropped -> skip cpu0
				1: {Idle: 40, Total: 200, Valid: true},
			},
			cpuToCore: map[int]int{0: 0, 1: 1},
			want:      map[int]float64{1: 90},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := calcPerCoreUsage(tt.prev, tt.cur, tt.cpuToCore)
			if len(got) != len(tt.want) {
				t.Fatalf("got %v, want %v", got, tt.want)
			}
			for k, v := range tt.want {
				if got[k] != v {
					t.Errorf("core[%d] = %v, want %v", k, got[k], v)
				}
			}
		})
	}
}
