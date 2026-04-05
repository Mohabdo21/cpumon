package main

import (
	"fmt"
	"strconv"
	"strings"
	"time"
)

// ANSI escape codes
const (
	ansiReset  = "\033[0m"
	ansiBold   = "\033[1m"
	ansiDim    = "\033[2m"
	ansiRed    = "\033[31m"
	ansiGreen  = "\033[32m"
	ansiYellow = "\033[33m"
	ansiCyan   = "\033[36m"
	ansiWhite  = "\033[97m"
)

func display(m Metrics, interval time.Duration) {
	var b strings.Builder
	b.Grow(2048)

	b.WriteString("\033[H\033[2J")

	writeHeader(&b, "System Information")
	writeField(&b, "Device", m.DeviceModel)
	writeField(&b, "CPU", m.CPUModel)
	writeField(&b, "Kernel", m.Kernel)
	writeField(&b, "Uptime", m.Uptime)
	if m.LoadAvg != "N/A" {
		writeField(&b, "Load Avg", m.LoadAvg)
	}
	b.WriteByte('\n')

	if m.Governor != "N/A" || m.EnergyBias != "N/A" || m.TurboBoost != "N/A" || m.AvgFreq != "N/A" {
		writeHeader(&b, "CPU Performance")
		if m.Governor != "N/A" {
			writeField(&b, "Governor", m.Governor)
		}
		if m.EnergyBias != "N/A" {
			writeField(&b, "Energy Bias", m.EnergyBias)
		}
		if m.TurboBoost != "N/A" {
			writeField(&b, "Turbo Boost", m.TurboBoost)
		}
		if m.AvgFreq != "N/A" {
			writeField(&b, "Avg Freq", m.AvgFreq)
		}
		if m.CPUUsage >= 0 {
			writeUsageBar(&b, m.CPUUsage, 30)
		}
		b.WriteByte('\n')
	}

	if len(m.Cores) > 0 {
		writeHeader(&b, "CPU Status")
		writeCoreGrid(&b, m.Cores)
		b.WriteByte('\n')
	}

	if m.Throttle.Available {
		writeHeader(&b, "Thermal Throttling")
		writeThrottleField(&b, "Pkg Events", m.Throttle.PackageCount)
		writeThrottleField(&b, "Pkg Total Time", m.Throttle.PackageTotalTime)
		writeThrottleField(&b, "Pkg Max Event", m.Throttle.PackageMaxTime)
		writeThrottleField(&b, "Core Events", m.Throttle.CoreCount)
		writeThrottleField(&b, "Core Total Time", m.Throttle.CoreTotalTime)
		writeThrottleField(&b, "Core Max Event", m.Throttle.CoreMaxTime)
		b.WriteByte('\n')
	}

	if m.FanStatus != "" {
		writeHeader(&b, "Fan Status")
		for line := range strings.SplitSeq(m.FanStatus, "\n") {
			if line != "" {
				fmt.Fprintf(&b, "  %s%s%s\n", ansiWhite, line, ansiReset)
			}
		}
		b.WriteByte('\n')
	}

	if m.SensorsHint {
		fmt.Fprintf(&b, "  %s%sHint:%s Install lm-sensors for better thermal data%s\n",
			ansiDim, ansiYellow, ansiReset+ansiDim, ansiReset)
	}

	fmt.Fprintf(&b, "  %sRefreshing every %v... (Ctrl+C to exit)%s\n", ansiDim, interval, ansiReset)

	fmt.Print(b.String())
}

func writeHeader(b *strings.Builder, title string) {
	fmt.Fprintf(b, "  %s%s-- %s --%s\n", ansiBold, ansiCyan, title, ansiReset)
}

func writeField(b *strings.Builder, label, value string) {
	fmt.Fprintf(b, "  %s%-14s%s %s%s%s\n", ansiDim, label+":", ansiReset, ansiWhite, value, ansiReset)
}

func writeUsageBar(b *strings.Builder, pct float64, width int) {
	n := max(min(int(pct/100*float64(width)+0.5), width), 0)

	color := ansiGreen
	if pct >= 80 {
		color = ansiRed
	} else if pct >= 50 {
		color = ansiYellow
	}

	fmt.Fprintf(b, "  %s%-14s%s %s[%s%s%s%s]%s %s%4.1f%%%s\n",
		ansiDim, "CPU Usage:", ansiReset,
		ansiDim,
		color+ansiBold, strings.Repeat("█", n),
		ansiReset+ansiDim, strings.Repeat("░", width-n),
		ansiReset,
		color, pct, ansiReset)
}

func writeCoreGrid(b *strings.Builder, cores []CoreStatus) {
	// Package / global temps first, on their own lines.
	for _, c := range cores {
		if !c.IsPackage {
			continue
		}
		tc := tempColor(c.TempC)
		if c.Limit != "" {
			fmt.Fprintf(b, "  %s%-14s%s %s%s%s  %s%s%s\n",
				ansiDim, c.Label+":", ansiReset,
				tc, c.Temp, ansiReset,
				ansiDim, c.Limit, ansiReset)
		} else {
			fmt.Fprintf(b, "  %s%-14s%s %s%s%s\n",
				ansiDim, c.Label+":", ansiReset,
				tc, c.Temp, ansiReset)
		}
	}

	// Collect per-core entries.
	var perCore []CoreStatus
	for i := range cores {
		if !cores[i].IsPackage {
			perCore = append(perCore, cores[i])
		}
	}
	if len(perCore) == 0 {
		return
	}

	// Two-column grid: left half / right half.
	half := (len(perCore) + 1) / 2
	for i := range half {
		writeCoreEntry(b, perCore[i])
		if i+half < len(perCore) {
			b.WriteString("    ")
			writeCoreEntry(b, perCore[i+half])
		}
		b.WriteByte('\n')
	}
}

func writeCoreEntry(b *strings.Builder, c CoreStatus) {
	tc := tempColor(c.TempC)
	freq := c.Freq
	if freq == "" {
		freq = "---"
	}
	fmt.Fprintf(b, "  %s%-9s%s %s%-10s%s %s%-8s%s",
		ansiDim, c.Label+":", ansiReset,
		ansiWhite, freq, ansiReset,
		tc, c.Temp, ansiReset)
}

func writeThrottleField(b *strings.Builder, label, value string) {
	color := ansiWhite
	if v, err := strconv.ParseInt(value, 10, 64); err == nil && v > 0 {
		color = ansiRed
	}
	fmt.Fprintf(b, "  %s%-18s%s %s%s%s\n", ansiDim, label+":", ansiReset, color, value, ansiReset)
}

func tempColor(c float64) string {
	switch {
	case c < 0:
		return ansiWhite
	case c < 60:
		return ansiGreen
	case c < 80:
		return ansiYellow
	default:
		return ansiRed
	}
}
