package main

import (
	"bufio"
	"fmt"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
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

func discoverHwmonCPU(fr FileReader) string {
	paths, _ := filepath.Glob("/sys/class/hwmon/hwmon*")
	drivers := []string{"coretemp", "k10temp", "zenpower", "cpu_thermal", "acpitz"}

	for _, p := range paths {
		name, err := fr.Read(filepath.Join(p, "name"))
		if err != nil {
			continue
		}
		for _, drv := range drivers {
			if strings.Contains(strings.ToLower(name), drv) {
				return p
			}
		}
	}
	return ""
}

func readOrNA(fr FileReader, path string) string {
	if s, err := fr.Read(path); err == nil {
		return s
	}
	return "N/A"
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

func readCPUThermal(fr FileReader, cr CmdRunner, sensorsOK bool, hwmonTemps []HwmonTemp, coreFreqs map[int]string, lineBuf *[]string) (string, error) {
	if sensorsOK {
		if out, err := readThermalFromSensors(cr, coreFreqs, lineBuf); err == nil {
			return out, nil
		}
	}
	if len(hwmonTemps) > 0 {
		if out, err := readThermalFromHwmon(fr, hwmonTemps, coreFreqs, lineBuf); err == nil {
			return out, nil
		}
	}
	return "", ErrNoThermalData
}

func readThermalFromSensors(cr CmdRunner, coreFreqs map[int]string, lineBuf *[]string) (string, error) {
	out, err := cr.Run("sensors")
	if err != nil {
		return "", err
	}

	lines := (*lineBuf)[:0]
	scanner := bufio.NewScanner(strings.NewReader(out))
	for scanner.Scan() {
		line := scanner.Text()

		if m := packageRe.FindStringSubmatch(line); m != nil {
			lines = append(lines, fmt.Sprintf("%s:%s%-10s %s", m[1], m[2], "", m[3]))
		} else if m := amdTctlRe.FindStringSubmatch(line); m != nil {
			lines = append(lines, fmt.Sprintf("%-14s%s%-10s %s", m[1]+":", m[2], "", m[3]))
		} else if m := coreRe.FindStringSubmatch(line); m != nil {
			coreNum, _ := strconv.Atoi(m[2])
			freq := "N/A"
			if f, ok := coreFreqs[coreNum]; ok {
				freq = f
			}
			lines = append(lines, fmt.Sprintf("%s:%s%-10s %s", m[1], m[3], freq, m[4]))
		}
	}
	*lineBuf = lines

	if len(lines) == 0 {
		return "", ErrNoThermalData
	}
	return strings.Join(lines, "\n"), nil
}

func readThermalFromHwmon(fr FileReader, temps []HwmonTemp, coreFreqs map[int]string, lineBuf *[]string) (string, error) {
	if len(temps) == 0 {
		return "", ErrNoThermalData
	}

	lines := (*lineBuf)[:0]
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

		info := fmt.Sprintf("+%.1f°C", tempC)
		if m, ok := readInt(fr, t.Crit); ok && m > 0 {
			info += fmt.Sprintf("  (crit = +%.1f°C)", float64(m)/1000.0)
		} else if m, ok := readInt(fr, t.Max); ok && m > 0 {
			info += fmt.Sprintf("  (high = +%.1f°C)", float64(m)/1000.0)
		}

		if m := coreNumRe.FindStringSubmatch(label); m != nil {
			coreNum, _ := strconv.Atoi(m[1])
			freq := "N/A"
			if f, ok := coreFreqs[coreNum]; ok {
				freq = f
			}
			lines = append(lines, fmt.Sprintf("%-14s %-10s %s", label+":", freq, info))
		} else {
			lines = append(lines, fmt.Sprintf("%-14s %-10s %s", label+":", "", info))
		}
	}
	*lineBuf = lines

	if len(lines) == 0 {
		return "", ErrNoThermalData
	}
	return strings.Join(lines, "\n"), nil
}
