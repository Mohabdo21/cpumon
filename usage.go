package main

import (
	"strconv"
	"strings"
)

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

func readPerCoreStat(fr FileReader) map[int]CPUTimes {
	data, err := fr.Read(procStatPath)
	if err != nil {
		return nil
	}

	cores := make(map[int]CPUTimes)
	for line, rest := "", data; rest != ""; {
		line, rest, _ = strings.Cut(rest, "\n")
		fields := strings.Fields(line)
		if len(fields) < 5 || !strings.HasPrefix(fields[0], "cpu") || fields[0] == "cpu" {
			continue
		}

		num, err := strconv.Atoi(fields[0][3:])
		if err != nil {
			continue
		}

		var total, idle uint64
		for i := 1; i < len(fields); i++ {
			v, _ := strconv.ParseUint(fields[i], 10, 64)
			total += v
			if i == 4 || i == 5 {
				idle += v
			}
		}
		cores[num] = CPUTimes{Idle: idle, Total: total, Valid: true}
	}
	return cores
}

func calcPerCoreUsage(prev, cur map[int]CPUTimes, cpuToCore map[int]int) map[int]float64 {
	if prev == nil || cur == nil {
		return nil
	}

	// Aggregate per physical core.
	type delta struct{ busy, total uint64 }
	agg := make(map[int]*delta)

	for cpu, c := range cur {
		p, ok := prev[cpu]
		if !ok || !p.Valid || !c.Valid {
			continue
		}

		coreID := cpu
		if id, ok := cpuToCore[cpu]; ok {
			coreID = id
		}

		dt := c.Total - p.Total
		di := c.Idle - p.Idle

		d, ok := agg[coreID]
		if !ok {
			d = &delta{}
			agg[coreID] = d
		}
		d.busy += dt - di
		d.total += dt
	}

	usage := make(map[int]float64, len(agg))
	for id, d := range agg {
		if d.total > 0 {
			usage[id] = float64(d.busy) / float64(d.total) * 100
		}
	}
	return usage
}
