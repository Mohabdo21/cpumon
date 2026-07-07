package main

import (
	"fmt"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
)

var (
	cpuNumRe  = regexp.MustCompile(`cpu(\d+)`)
	coreNumRe = regexp.MustCompile(`Core\s*(\d+)`)
)

func discoverCPUTopology(fr FileReader) []CPUFreqInfo {
	matches, err := filepath.Glob("/sys/devices/system/cpu/cpu[0-9]*/cpufreq/scaling_cur_freq")
	if err != nil || len(matches) == 0 {
		return nil
	}

	sort.Strings(matches)
	seen := make(map[int]bool)
	var infos []CPUFreqInfo

	for _, freqPath := range matches {
		m := cpuNumRe.FindStringSubmatch(freqPath)
		if m == nil {
			continue
		}

		cpuNum, _ := strconv.Atoi(m[1])
		coreIDPath := fmt.Sprintf("/sys/devices/system/cpu/cpu%d/topology/core_id", cpuNum)
		coreIDStr, err := fr.Read(coreIDPath)
		if err != nil {
			coreIDStr = m[1]
		}

		coreID, _ := strconv.Atoi(strings.TrimSpace(coreIDStr))
		if !seen[coreID] {
			seen[coreID] = true
			infos = append(infos, CPUFreqInfo{Path: freqPath, CoreID: coreID})
		}
	}
	return infos
}

func discoverCPUCoreMap(fr FileReader) map[int]int {
	matches, _ := filepath.Glob("/sys/devices/system/cpu/cpu[0-9]*/topology/core_id")
	if len(matches) == 0 {
		return nil
	}

	m := make(map[int]int, len(matches))
	for _, path := range matches {
		sub := cpuNumRe.FindStringSubmatch(path)
		if sub == nil {
			continue
		}
		cpuNum, _ := strconv.Atoi(sub[1])
		if raw, err := fr.Read(path); err == nil {
			coreID, _ := strconv.Atoi(strings.TrimSpace(raw))
			m[cpuNum] = coreID
		}
	}
	return m
}

func discoverHwmonCPU(fr FileReader) string {
	paths, _ := filepath.Glob("/sys/class/hwmon/hwmon*")
	tiers := [][]string{
		{"coretemp", "k10temp", "zenpower", "cpu_thermal"},
		{"acpitz"},
	}

	for _, tier := range tiers {
		for _, p := range paths {
			name, err := fr.Read(filepath.Join(p, "name"))
			if err != nil {
				continue
			}
			for _, drv := range tier {
				if strings.Contains(strings.ToLower(name), drv) {
					return p
				}
			}
		}
	}
	return ""
}

func readFrequencies(fr FileReader, infos []CPUFreqInfo, coreFreqs map[int]string) string {
	clear(coreFreqs)
	if len(infos) == 0 {
		return "N/A"
	}

	var total, count int64
	for _, info := range infos {
		kHz, ok := readInt(fr, info.Path)
		if !ok || kHz <= 0 {
			continue
		}

		total += kHz
		count++
		coreFreqs[info.CoreID] = formatFreq(kHz)
	}

	if count == 0 {
		return "N/A"
	}

	return formatFreq(total / count)
}

func formatFreq(kHz int64) string {
	if kHz >= 1000000 {
		return fmt.Sprintf("%.1f GHz", float64(kHz)/1000000.0)
	}
	return fmt.Sprintf("%d MHz", kHz/1000)
}

func discoverHwmonTemps(hwmonPath string) []HwmonTemp {
	if hwmonPath == "" {
		return nil
	}
	matches, _ := filepath.Glob(filepath.Join(hwmonPath, "temp*_input"))
	temps := make([]HwmonTemp, 0, len(matches))
	for _, input := range matches {
		temps = append(temps, HwmonTemp{
			Input: input,
			Label: strings.Replace(input, "_input", "_label", 1),
			Crit:  strings.Replace(input, "_input", "_crit", 1),
			Max:   strings.Replace(input, "_input", "_max", 1),
		})
	}
	return temps
}

func readCPUThermal(
	fr FileReader,
	hwmonTemps []HwmonTemp,
	coreFreqs map[int]string,
	coreUsage map[int]float64,
	coreBuf *[]CoreStatus,
) ([]CoreStatus, error) {
	if len(hwmonTemps) > 0 {
		return readThermalFromHwmon(
			fr,
			hwmonTemps,
			coreFreqs,
			coreUsage,
			coreBuf,
		)
	}
	return nil, ErrNoThermalData
}

func readThermalFromHwmon(
	fr FileReader,
	temps []HwmonTemp,
	coreFreqs map[int]string,
	coreUsage map[int]float64,
	coreBuf *[]CoreStatus,
) ([]CoreStatus, error) {
	if len(temps) == 0 {
		return nil, ErrNoThermalData
	}

	cores := (*coreBuf)[:0]
	for _, t := range temps {
		milli, ok := readInt(fr, t.Input)
		if !ok {
			continue
		}
		tempC := float64(milli) / 1000.0

		label := "CPU"
		if l, err := fr.Read(t.Label); err == nil {
			label = l
		}

		temp := fmt.Sprintf("+%.1f°C", tempC)
		limit := ""
		coreLimit := ""
		critM, critOK := readInt(fr, t.Crit)
		if critOK && critM > 0 {
			limit = fmt.Sprintf("(crit = +%.1f°C)", float64(critM)/1000.0)
			coreLimit = fmt.Sprintf("C%.0f", float64(critM)/1000.0)
		}
		if maxM, ok := readInt(fr, t.Max); ok && maxM > 0 {
			if critOK && critM > 0 {
				coreLimit = fmt.Sprintf("H%.0f/C%.0f", float64(maxM)/1000.0, float64(critM)/1000.0)
				limit = fmt.Sprintf(
					"(high = +%.1f°C, crit = +%.1f°C)",
					float64(maxM)/1000.0, float64(critM)/1000.0,
				)
			} else {
				coreLimit = fmt.Sprintf("H%.0f", float64(maxM)/1000.0)
				limit = fmt.Sprintf("(high = +%.1f°C)", float64(maxM)/1000.0)
			}
		}

		isPackage := true
		freq := ""
		usage := -1.0
		if m := coreNumRe.FindStringSubmatch(label); m != nil {
			isPackage = false
			coreNum, _ := strconv.Atoi(m[1])
			if f, ok := coreFreqs[coreNum]; ok {
				freq = f
			}
			if u, ok := coreUsage[coreNum]; ok {
				usage = u
			}
		}

		cores = append(cores, CoreStatus{
			Label:     label,
			Freq:      freq,
			Usage:     usage,
			Temp:      temp,
			Limit:     limit,
			CoreLimit: coreLimit,
			TempC:     tempC,
			IsPackage: isPackage,
		})
	}
	*coreBuf = cores

	if len(cores) == 0 {
		return nil, ErrNoThermalData
	}
	return cores, nil
}
