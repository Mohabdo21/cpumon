package main

import (
	"fmt"
	"strconv"
	"strings"
)

func readUptime(fr FileReader) string {
	data, err := fr.Read(procUptimePath)
	if err != nil {
		return "N/A"
	}

	// /proc/uptime: "uptime_seconds idle_seconds"
	raw, _, _ := strings.Cut(data, " ")
	secs, err := strconv.ParseFloat(raw, 64)
	if err != nil || secs < 0 {
		return "N/A"
	}

	return formatUptime(int64(secs))
}

func formatUptime(sec int64) string {
	days := sec / 86400
	hours := (sec % 86400) / 3600
	mins := (sec % 3600) / 60

	switch {
	case days > 0:
		return fmt.Sprintf("%d days, %d hours, %d min", days, hours, mins)
	case hours > 0:
		return fmt.Sprintf("%d hours, %d min", hours, mins)
	default:
		return fmt.Sprintf("%d min", mins)
	}
}
