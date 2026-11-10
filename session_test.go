package main

import (
	"io"
	"os"
	"strings"
	"testing"
	"time"
)

func TestUpdateStatsFreqAndFan(t *testing.T) {
	m := &Monitor{}

	m.updateStats(50, nil, PowerReading{}, 2400000, 1500)
	m.updateStats(80, nil, PowerReading{}, 4200000, 1800)

	if m.stats.FreqSamples != 2 || m.stats.TotalFreq != 6600000 || m.stats.PeakFreq != 4200000 {
		t.Errorf("freq stats = %d samples, %d total, %d peak; want 2, 6600000, 4200000",
			m.stats.FreqSamples, m.stats.TotalFreq, m.stats.PeakFreq)
	}
	if m.stats.FanSamples != 2 || m.stats.TotalFan != 3300 || m.stats.PeakFan != 1800 {
		t.Errorf("fan stats = %d samples, %d total, %d peak; want 2, 3300, 1800",
			m.stats.FanSamples, m.stats.TotalFan, m.stats.PeakFan)
	}

	// Zero frequency and negative fan speed mean "no data this sample".
	m.updateStats(10, nil, PowerReading{}, 0, -1)
	if m.stats.FreqSamples != 2 || m.stats.FanSamples != 2 {
		t.Errorf("zero values must be skipped, got freq=%d fan=%d",
			m.stats.FreqSamples, m.stats.FanSamples)
	}
	if m.stats.Samples != 3 {
		t.Errorf("Samples = %d, want 3 (all samples count)", m.stats.Samples)
	}
}

func TestCollectFanSummaryRejectsInvalidReadings(t *testing.T) {
	for _, source := range []string{"thinkpad", "hwmon"} {
		t.Run(source, func(t *testing.T) {
			fr := mkMock(map[string]string{"hwmon0/name": "thinkpad"})
			m := Monitor{
				fr: fr, thinkpadFan: source == "thinkpad",
				coreFreqBuf: make(map[int]string), curCoreBuf: make(map[int]CPUTimes),
				coreDeltaBuf: make(map[int]cpuDelta), coreUsageBuf: make(map[int]float64),
				fanLabels: make(map[string]string),
			}
			if source == "hwmon" {
				m.fanFiles = []string{"hwmon0/fan1_input"}
			}
			for _, reading := range []string{"3000", "65535", "0", "junk", "-1", "6000"} {
				fr.files[thinkpadFanPath] = "status: enabled\nspeed: " + reading + "\nlevel: auto\n"
				fr.files["hwmon0/fan1_input"] = reading
				metrics := m.collect()
				if strings.Contains(metrics.FanStatus, "65535") {
					t.Errorf("invalid reading displayed: %q", metrics.FanStatus)
				}
			}
			delete(fr.files, thinkpadFanPath)
			delete(fr.files, "hwmon0/fan1_input")
			metrics := m.collect()
			if metrics.FanStatus != "N/A" {
				t.Errorf("failed reading status = %q, want N/A", metrics.FanStatus)
			}
			if m.stats.FanSamples != 3 || m.stats.TotalFan != 9000 || m.stats.PeakFan != 6000 {
				t.Fatalf("fan stats = %d samples, %d total, %d peak; want 3, 9000, 6000",
					m.stats.FanSamples, m.stats.TotalFan, m.stats.PeakFan)
			}
			out := captureStdout(t, func() { printSessionSummary(m.stats) })
			if !strings.Contains(out, "avg 3000 RPM  peak 6000 RPM") {
				t.Errorf("incorrect fan summary: %s", out)
			}
		})
	}
}

func TestPrintSessionSummaryIncludesFreqAndFan(t *testing.T) {
	// 2 min session, 120 samples at 1s.
	s := SessionStats{
		StartTime:   time.Now().Add(-2 * time.Minute),
		Samples:     120,
		TotalCPU:    6000,
		PeakCPU:     90,
		TotalFreq:   3000000 * 120,
		FreqSamples: 120,
		PeakFreq:    4300000,
		TotalFan:    2500 * 120,
		FanSamples:  120,
		PeakFan:     3400,
	}

	out := captureStdout(t, func() { printSessionSummary(s) })

	for _, want := range []string{
		"-- Session Summary --",
		"2m0s (120 samples)",
		"avg 50.0%  peak 90.0%",
		"avg 3.0 GHz  peak 4.3 GHz",
		"avg 2500 RPM  peak 3400 RPM",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("printSessionSummary() missing %q in:\n%s", want, out)
		}
	}
}

func TestPrintSessionSummaryOmitsFreqAndFanWithoutSamples(t *testing.T) {
	s := SessionStats{
		StartTime: time.Now(),
		Samples:   1,
		TotalCPU:  10,
	}

	out := captureStdout(t, func() { printSessionSummary(s) })

	for _, absent := range []string{"CPU Freq:", "Fan Speed:"} {
		if strings.Contains(out, absent) {
			t.Errorf("printSessionSummary() should not contain %q in:\n%s", absent, out)
		}
	}
}

func captureStdout(t *testing.T, fn func()) string {
	t.Helper()
	orig := os.Stdout
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("pipe: %v", err)
	}
	os.Stdout = w
	defer func() { os.Stdout = orig }()

	fn()
	if err := w.Close(); err != nil {
		t.Fatalf("close pipe: %v", err)
	}
	data, err := io.ReadAll(r)
	if err != nil {
		t.Fatalf("read pipe: %v", err)
	}
	return string(data)
}
