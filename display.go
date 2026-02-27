package main

import (
	"fmt"
	"strings"
	"time"
)

type Metrics struct {
	DeviceModel string
	CPUModel    string
	Governor    string
	EnergyBias  string
	AvgFreq     string
	CPUUsage    float64
	CPUStatus   string
	Throttle    ThrottleInfo
	FanStatus   string
	SensorsHint bool
}

func display(m Metrics, interval time.Duration) {
	var b strings.Builder
	b.Grow(1024)

	b.WriteString("\033[H\033[2J\033[3J")

	b.WriteString("=== System Information ===\n")
	fmt.Fprintf(&b, "Device: %s\n", m.DeviceModel)
	fmt.Fprintf(&b, "CPU: %s\n\n", m.CPUModel)

	if m.Governor != "N/A" || m.AvgFreq != "N/A" {
		b.WriteString("=== CPU Performance ===\n")
		if m.Governor != "N/A" {
			fmt.Fprintf(&b, "Governor: %s\n", m.Governor)
		}
		if m.EnergyBias != "N/A" {
			fmt.Fprintf(&b, "Energy Bias: %s\n", m.EnergyBias)
		}
		if m.AvgFreq != "N/A" {
			fmt.Fprintf(&b, "Current Freq (AVG): %s\n", m.AvgFreq)
		}
		if m.CPUUsage >= 0 {
			fmt.Fprintf(&b, "CPU Usage: %s\n", usageBar(m.CPUUsage, 20))
		}
		b.WriteByte('\n')
	}

	if m.CPUStatus != "" {
		b.WriteString("=== CPU Status ===\n")
		b.WriteString(m.CPUStatus)
		b.WriteString("\n\n")
	}

	if m.Throttle.Available {
		b.WriteString("=== Thermal Throttling Details ===\n")
		fmt.Fprintf(&b, "Package Throttle Events: %s\n", m.Throttle.PackageCount)
		fmt.Fprintf(&b, "Package Total Throttle Time: %s\n", m.Throttle.PackageTotalTime)
		fmt.Fprintf(&b, "Package Max Throttle Event: %s\n", m.Throttle.PackageMaxTime)
		fmt.Fprintf(&b, "Core Throttle Events: %s\n", m.Throttle.CoreCount)
		fmt.Fprintf(&b, "Core Total Throttle Time: %s\n", m.Throttle.CoreTotalTime)
		fmt.Fprintf(&b, "Core Max Throttle Event: %s\n\n", m.Throttle.CoreMaxTime)
	}

	if m.FanStatus != "" {
		b.WriteString("=== Fan Status ===\n")
		b.WriteString(m.FanStatus)
		b.WriteString("\n\n")
	}

	if m.SensorsHint {
		b.WriteString("Hint: Install lm-sensors for better thermal data: sudo dnf install lm_sensors\n")
	}

	fmt.Fprintf(&b, "Refreshing every %v... (Press Ctrl+C to exit)\n", interval)

	fmt.Print(b.String())
}

func usageBar(pct float64, width int) string {
	n := max(min(int(pct/100*float64(width)+0.5), width), 0)
	return fmt.Sprintf("[%s%s] %4.1f%%",
		strings.Repeat("█", n),
		strings.Repeat("░", width-n),
		pct)
}
