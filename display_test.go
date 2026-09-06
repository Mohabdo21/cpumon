package main

import (
	"strings"
	"testing"
	"time"
)

func TestRenderFullMetrics(t *testing.T) {
	m := Metrics{
		DeviceModel: "ThinkPad X1",
		CPUModel:    "Intel Core i7",
		Kernel:      "6.12.0",
		Uptime:      "2 days, 3 hours, 4 min",
		LoadAvg:     "0.45 0.50 0.39",
		Governor:    "powersave",
		EnergyBias:  "default",
		TurboBoost:  "Enabled",
		AvgFreq:     "3.4GHz",
		CPUUsage:    42.5,
		Throttle: ThrottleInfo{
			Available:        true,
			PackageCount:     "3",
			PackageTotalTime: "2 sec",
			PackageMaxTime:   "500 ms",
			CoreCount:        "7",
			CoreTotalTime:    "1 min 2 sec",
			CoreMaxTime:      "1000 ms",
		},
		Cores: []CoreStatus{
			{
				Label:     "Package",
				Temp:      "+65.0°C",
				Limit:     "(crit = +100.0°C)",
				TempC:     65,
				IsPackage: true,
			},
			{
				Label:     "Core 0",
				Freq:      "3.4GHz",
				Usage:     55.5,
				Temp:      "+55.0°C",
				CoreLimit: "H85/C100",
				TempC:     55,
			},
		},
		Power: PowerReading{
			Available: true,
			Zones: []PowerZone{
				{Name: "Package", Watts: 30.5},
				{Name: "Cores", Watts: 12.0},
			},
		},
		FanStatus: "fan1: 4200 RPM\nfan2: 3100 RPM",
		Stats: SessionStats{
			PeakCPU:   88.8,
			PeakTemp:  90,
			MinTemp:   40,
			Samples:   10,
			StartTime: time.Now(),
		},
		Topology: CoreTopology{
			Hybrid: true, Vendor: "Intel",
			Classes: map[int]CoreClass{0: CoreClassPerformance},
		},
	}

	out := render(m, time.Second)

	for _, want := range []string{
		"-- System Information --",
		"ThinkPad X1",
		"Intel Core i7",
		"6.12.0",
		"2 days, 3 hours, 4 min",
		"0.45 0.50 0.39",
		"-- CPU Performance --",
		"powersave",
		"default",
		"Enabled",
		"3.4GHz",
		"42.5%",
		"peak: 88.8%",
		"-- Power Consumption --",
		"Package:",
		"30.5 W",
		"Cores:",
		"-- CPU Status [1P + 0E (hybrid)] --",
		"+65.0°C",
		"(crit = +100.0°C)",
		"56%",
		"H85/C100",
		"-- Thermal Throttling --",
		"Pkg Events",
		"3",
		"Core Total Time",
		"1 min 2 sec",
		"-- Fan Status --",
		"fan1: 4200 RPM",
		"fan2: 3100 RPM",
		"Refreshing every",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("render() missing %q in:\n%s", want, out)
		}
	}
}

func TestRenderMinimalMetrics(t *testing.T) {
	m := Metrics{
		DeviceModel: "VM",
		CPUModel:    "Virtual CPU",
		Kernel:      "6.12.0",
		Uptime:      "5 min",
		Governor:    "N/A",
		EnergyBias:  "N/A",
		TurboBoost:  "N/A",
		AvgFreq:     "N/A",
		LoadAvg:     "N/A",
	}

	out := render(m, time.Second)

	for _, want := range []string{
		"-- System Information --",
		"VM",
		"Refreshing every",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("render() missing %q in:\n%s", want, out)
		}
	}

	for _, absent := range []string{
		"Load Avg",
		"-- CPU Performance --",
		"-- Power Consumption --",
		"-- CPU Status --",
		"-- Thermal Throttling --",
		"-- Fan Status --",
	} {
		if strings.Contains(out, absent) {
			t.Errorf("render() should not contain %q in:\n%s", absent, out)
		}
	}
}

func TestRenderPowerFallbackTotal(t *testing.T) {
	// No Package zone => total stays 0, so the fallback loop sums all zones.
	m := Metrics{
		Power: PowerReading{
			Available: true,
			Zones:     []PowerZone{{Name: "DRAM", Watts: 4.2}},
		},
	}

	out := render(m, time.Second)

	if !strings.Contains(out, "DRAM:") || !strings.Contains(out, "4.2 W") {
		t.Errorf("render() power section wrong:\n%s", out)
	}
}
