package main

import (
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"
)

// ANSI escape codes — zeroed when NO_COLOR is set
var (
	ansiReset   = "\033[0m"
	ansiBold    = "\033[1m"
	ansiDim     = "\033[2m"
	ansiRed     = "\033[31m"
	ansiGreen   = "\033[32m"
	ansiYellow  = "\033[33m"
	ansiDefault = "\033[39m"
)

func init() {
	if os.Getenv("NO_COLOR") != "" {
		ansiReset = ""
		ansiBold = ""
		ansiDim = ""
		ansiRed = ""
		ansiGreen = ""
		ansiYellow = ""
		ansiDefault = ""
	}
}

func display(m Metrics, interval time.Duration) {
	width := termWidth()

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
			barWidth := min(max((width-30)/2, 10), 40)
			writeUsageBar(&b, m.CPUUsage, barWidth, m.Stats.PeakCPU)
		}
		b.WriteByte('\n')
	}

	if m.Power.Available && len(m.Power.Zones) > 0 {
		writeHeader(&b, "Power Consumption")
		var total float64
		for _, z := range m.Power.Zones {
			if z.Name == "Package" {
				total += z.Watts
			}
		}
		for _, z := range m.Power.Zones {
			color := ansiDefault
			if z.Name == "Package" && z.Watts > 28 {
				color = ansiRed
			} else if z.Name == "Package" && z.Watts > 15 {
				color = ansiYellow
			}
			fmt.Fprintf(&b, "  %s%-14s%s %s%5.1f W%s\n",
				ansiDim, z.Name+":", ansiReset, color, z.Watts, ansiReset)
		}
		if total == 0 {
			for _, z := range m.Power.Zones {
				total += z.Watts
			}
		}
		b.WriteByte('\n')
	}

	if len(m.Cores) > 0 {
		title := "CPU Status"
		if label := coreClassLabel(m.Topology); label != "" {
			title = fmt.Sprintf("CPU Status [%s]", label)
		}
		writeHeader(&b, title)
		writeCoreGrid(&b, m.Cores, m.Stats, width, m.Topology)
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
				fmt.Fprintf(&b, "  %s%s%s\n", ansiDefault, line, ansiReset)
			}
		}
		b.WriteByte('\n')
	}

	fmt.Fprintf(
		&b,
		"  %sRefreshing every %v... (q or Ctrl+C to exit)%s\n",
		ansiDim,
		interval,
		ansiReset,
	)

	fmt.Print(b.String())
}

func writeHeader(b *strings.Builder, title string) {
	fmt.Fprintf(b, "  %s-- %s --%s\n", ansiBold, title, ansiReset)
}

func writeField(b *strings.Builder, label, value string) {
	fmt.Fprintf(
		b,
		"  %s%-14s%s %s%s%s\n",
		ansiDim,
		label+":",
		ansiReset,
		ansiDefault,
		value,
		ansiReset,
	)
}

func writeUsageBar(b *strings.Builder, pct float64, width int, peak float64) {
	n := max(min(int(pct/100*float64(width)+0.5), width), 0)

	color := ansiGreen
	if pct >= 80 {
		color = ansiRed
	} else if pct >= 50 {
		color = ansiYellow
	}

	fmt.Fprintf(b, "  %s%-14s%s %s[%s%s%s%s]%s %s%4.1f%%%s  %speak: %.1f%%%s\n",
		ansiDim, "CPU Usage:", ansiReset,
		ansiDim,
		color+ansiBold, strings.Repeat("█", n),
		ansiReset+ansiDim, strings.Repeat("░", width-n),
		ansiReset,
		color, pct, ansiReset,
		ansiDim, peak, ansiReset)
}

func writeCoreGrid(
	b *strings.Builder,
	cores []CoreStatus,
	stats SessionStats,
	width int,
	topo CoreTopology,
) {
	for _, c := range cores {
		if !c.IsPackage {
			continue
		}
		tc := tempColor(c.TempC)
		peakStr := ""
		if stats.Samples > 0 && c.TempC >= 0 {
			peakStr = fmt.Sprintf("  %s[%.0f°C / %.0f°C]%s",
				ansiDim, stats.MinTemp, stats.PeakTemp, ansiReset)
		}
		if c.Limit != "" {
			fmt.Fprintf(b, "  %s%-14s%s %s%s%s  %s%s%s%s\n",
				ansiDim, c.Label+":", ansiReset,
				tc, c.Temp, ansiReset,
				ansiDim, c.Limit, ansiReset,
				peakStr)
		} else {
			fmt.Fprintf(b, "  %s%-14s%s %s%s%s%s\n",
				ansiDim, c.Label+":", ansiReset,
				tc, c.Temp, ansiReset,
				peakStr)
		}
	}

	if topo.Hybrid {
		perf, eff := classifyCores(cores, topo)
		writeCoreGroup(b, "P-Cores", perf, width)
		writeCoreGroup(b, "E-Cores", eff, width)
		return
	}

	var perCore []CoreStatus
	for i := range cores {
		if !cores[i].IsPackage {
			perCore = append(perCore, cores[i])
		}
	}
	if len(perCore) == 0 {
		return
	}

	writeCoreRows(b, perCore, width)
}

func writeCoreGroup(b *strings.Builder, title string, cores []CoreStatus, width int) {
	if len(cores) == 0 {
		return
	}
	fmt.Fprintf(b, "  %s%s%s\n", ansiDim, title, ansiReset)
	writeCoreRows(b, cores, width)
}

func coreEntryWidth(c CoreStatus) int {
	freq := c.Freq
	if freq == "" {
		freq = "---"
	}
	usage := "  ---"
	if c.Usage >= 0 {
		usage = fmt.Sprintf("%4.0f%%", c.Usage)
	}
	w := 2 + 9 + 1 + len(freq) + 1 + len(usage) + 1 + 8
	if c.CoreLimit != "" {
		w += 1 + len(c.CoreLimit)
	}
	return w
}

func writeCoreRows(b *strings.Builder, cores []CoreStatus, width int) {
	maxW := 0
	for _, c := range cores {
		if w := coreEntryWidth(c); w > maxW {
			maxW = w
		}
	}
	gap := 4
	cols := min(max(1, (width-2)/(maxW+gap)), len(cores))
	rows := (len(cores) + cols - 1) / cols
	for r := range rows {
		for c := range cols {
			idx := r + c*rows
			if idx >= len(cores) {
				break
			}
			if c > 0 {
				b.WriteString("    ")
			}
			writeCoreEntry(b, cores[idx])
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
	usage := "  ---"
	if c.Usage >= 0 {
		uc := ansiGreen
		if c.Usage >= 80 {
			uc = ansiRed
		} else if c.Usage >= 50 {
			uc = ansiYellow
		}
		usage = fmt.Sprintf("%s%4.0f%%%s", uc, c.Usage, ansiReset)
	}
	fmt.Fprintf(b, "  %s%-9s%s %s%-10s%s %s %s%-8s%s",
		ansiDim, c.Label+":", ansiReset,
		ansiDefault, freq, ansiReset,
		usage,
		tc, c.Temp, ansiReset)
	if c.CoreLimit != "" {
		fmt.Fprintf(b, " %s%s%s", ansiDim, c.CoreLimit, ansiReset)
	}
}

func writeThrottleField(b *strings.Builder, label, value string) {
	color := ansiDefault
	if v, err := strconv.ParseInt(value, 10, 64); err == nil && v > 0 {
		color = ansiRed
	}
	fmt.Fprintf(b, "  %s%-18s%s %s%s%s\n", ansiDim, label+":", ansiReset, color, value, ansiReset)
}

func tempColor(c float64) string {
	switch {
	case c < 0:
		return ansiDefault
	case c < 60:
		return ansiGreen
	case c < 80:
		return ansiYellow
	default:
		return ansiRed
	}
}

func printSessionSummary(s SessionStats) {
	duration := time.Since(s.StartTime)
	mins := int(duration.Minutes())
	secs := int(duration.Seconds()) % 60

	fmt.Printf("\n%s-- Session Summary --%s\n", ansiBold, ansiReset)
	fmt.Printf(
		"  %s%-14s%s %dm%ds (%d samples)\n",
		ansiDim,
		"Duration:",
		ansiReset,
		mins,
		secs,
		s.Samples,
	)

	if s.Samples > 0 {
		avgCPU := s.TotalCPU / float64(s.Samples)
		fmt.Printf(
			"  %s%-14s%s avg %.1f%%  peak %.1f%%\n",
			ansiDim,
			"CPU Usage:",
			ansiReset,
			avgCPU,
			s.PeakCPU,
		)
	}

	if s.Samples > 0 && s.PeakTemp > 0 {
		fmt.Printf(
			"  %s%-14s%s min %.0f°C  peak %.0f°C\n",
			ansiDim,
			"Temperature:",
			ansiReset,
			s.MinTemp,
			s.PeakTemp,
		)
	}

	if s.PowerSamples > 0 {
		avgPower := s.TotalPower / float64(s.PowerSamples)
		fmt.Printf(
			"  %s%-14s%s avg %.1f W  peak %.1f W\n",
			ansiDim,
			"Power:",
			ansiReset,
			avgPower,
			s.PeakPower,
		)
	}

	fmt.Println()
}
