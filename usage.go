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

func parseProcStatFields(fields []string) (idle, total uint64) {
	for i := 1; i < len(fields); i++ {
		v, _ := strconv.ParseUint(fields[i], 10, 64)
		total += v
		if i == 4 || i == 5 {
			idle += v
		}
	}
	return
}

func readProcStat(fr FileReader, cores map[int]CPUTimes) (CPUTimes, map[int]CPUTimes) {
	clear(cores)
	data, err := fr.Read(procStatPath)
	if err != nil {
		return CPUTimes{}, nil
	}

	var agg CPUTimes
	if cores == nil {
		cores = make(map[int]CPUTimes)
	}

	for line, rest := "", data; rest != ""; {
		line, rest, _ = strings.Cut(rest, "\n")
		if !strings.HasPrefix(strings.TrimSpace(line), "cpu") {
			continue
		}
		fields := strings.Fields(line)
		if len(fields) < 5 {
			continue
		}

		if fields[0] == "cpu" {
			idle, total := parseProcStatFields(fields)
			agg = CPUTimes{Idle: idle, Total: total, Valid: true}
		} else if strings.HasPrefix(fields[0], "cpu") {
			num, err := strconv.Atoi(fields[0][3:])
			if err != nil {
				continue
			}
			idle, total := parseProcStatFields(fields)
			cores[num] = CPUTimes{Idle: idle, Total: total, Valid: true}
		}
	}

	return agg, cores
}

func calcUsage(prev, cur CPUTimes) float64 {
	if !prev.Valid || !cur.Valid {
		return -1
	}
	// Counters must be monotonic; a decrease (e.g. iowait dropping or CPUs
	// going offline at suspend) would otherwise underflow the unsigned delta
	// and yield a bogus percentage. Treat it as no data.
	if cur.Total < prev.Total || cur.Idle < prev.Idle {
		return -1
	}
	dt := cur.Total - prev.Total
	if dt == 0 {
		return 0
	}
	return float64(dt-(cur.Idle-prev.Idle)) / float64(dt) * 100
}

type cpuDelta struct{ busy, total uint64 }

func calcPerCoreUsage(prev, cur map[int]CPUTimes, cpuToCore map[int]int,
	agg map[int]cpuDelta, usage map[int]float64,
) map[int]float64 {
	clear(agg)
	clear(usage)
	if prev == nil || cur == nil {
		return nil
	}

	// Aggregate per physical core.
	if agg == nil {
		agg = make(map[int]cpuDelta)
	}

	for cpu, c := range cur {
		p, ok := prev[cpu]
		if !ok || !p.Valid || !c.Valid {
			continue
		}

		// Skip non-monotonic deltas (iowait decrease, CPU offline) so the
		// unsigned subtraction below cannot underflow into a bogus usage.
		if c.Total < p.Total || c.Idle < p.Idle {
			continue
		}

		coreID := cpu
		if id, ok := cpuToCore[cpu]; ok {
			coreID = id
		}

		dt := c.Total - p.Total
		di := c.Idle - p.Idle

		d := agg[coreID]
		d.busy += dt - di
		d.total += dt
		agg[coreID] = d
	}

	if usage == nil {
		usage = make(map[int]float64, len(agg))
	}
	for id, d := range agg {
		if d.total > 0 {
			usage[id] = float64(d.busy) / float64(d.total) * 100
		}
	}
	return usage
}
