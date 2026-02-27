package main

import (
	"strconv"
	"strings"
)

const procStatPath = "/proc/stat"

type CPUTimes struct {
	Idle  uint64
	Total uint64
	Valid bool
}

func readCPUStat(fr FileReader) CPUTimes {
	data, err := fr.Read(procStatPath)
	if err != nil {
		return CPUTimes{}
	}

	line, _, _ := strings.Cut(data, "\n")
	fields := strings.Fields(line)
	if len(fields) < 5 || fields[0] != "cpu" {
		return CPUTimes{}
	}

	var total, idle uint64
	for i := 1; i < len(fields); i++ {
		v, _ := strconv.ParseUint(fields[i], 10, 64)
		total += v
		if i == 4 || i == 5 { // idle + iowait
			idle += v
		}
	}
	return CPUTimes{Idle: idle, Total: total, Valid: true}
}

func calcUsage(prev, cur CPUTimes) float64 {
	if !prev.Valid || !cur.Valid {
		return -1
	}
	dt := cur.Total - prev.Total
	if dt == 0 {
		return 0
	}
	return float64(dt-cur.Idle+prev.Idle) / float64(dt) * 100
}
