package main

import (
	"bufio"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

var fanFilterRe = regexp.MustCompile(`(level:|speed:|status:)`)

func discoverFanFiles() []string {
	patterns := []string{
		"/sys/class/hwmon/hwmon*/fan*_input",
		"/sys/devices/platform/*/hwmon/hwmon*/fan*_input",
	}

	var files []string
	for _, p := range patterns {
		matches, _ := filepath.Glob(p)
		files = append(files, matches...)
	}
	return files
}

// readFanStatus returns a negative RPM when no valid reading is available; zero is a stopped fan.
func readFanStatus(
	fr FileReader,
	fanFiles []string,
	labels map[string]string,
	thinkpadFan bool,
	lineBuf *[]string,
) (string, int64, error) {
	if thinkpadFan {
		if out, rpm, err := readThinkPadFan(fr, lineBuf); err == nil {
			return out, rpm, nil
		}
	}

	if len(fanFiles) > 0 {
		if out, rpm, err := readHwmonFan(fr, fanFiles, labels, lineBuf); err == nil {
			return out, rpm, nil
		}
	}

	if thinkpadFan || len(fanFiles) > 0 {
		return "N/A", -1, ErrNoFanData
	}
	return "", -1, ErrNoFanData
}

func readThinkPadFan(fr FileReader, lineBuf *[]string) (string, int64, error) {
	data, err := fr.Read(thinkpadFanPath)
	if err != nil {
		return "", -1, err
	}

	lines := (*lineBuf)[:0]
	rpm := int64(-1)
	scanner := bufio.NewScanner(strings.NewReader(data))
	for scanner.Scan() {
		line := scanner.Text()
		if fanFilterRe.MatchString(line) {
			lines = append(lines, line)
			if v, ok := fanSpeedRPM(line); ok && v > rpm {
				rpm = v
			}
		}
	}
	*lineBuf = lines

	if err := scanner.Err(); err != nil {
		return "", -1, err
	}
	if rpm < 0 {
		return "", -1, ErrNoFanData
	}
	return "[ThinkPad]\n" + strings.Join(lines, "\n"), rpm, nil
}

// fanSpeedRPM extracts the RPM value from a ThinkPad "speed:" line.
func fanSpeedRPM(line string) (int64, bool) {
	rest, ok := strings.CutPrefix(line, "speed:")
	if !ok {
		return 0, false
	}
	rpm, valid := parseInt64(strings.TrimSpace(rest))
	// Defensive ThinkPad quirk: an all-ones EC reading is not a usable tachometer sample.
	// This is not a universal hwmon sentinel: https://docs.kernel.org/admin-guide/laptops/thinkpad-acpi.html
	return rpm, valid && rpm >= 0 && rpm != 65535
}

func readHwmonFan(
	fr FileReader,
	fanFiles []string,
	labels map[string]string,
	lineBuf *[]string,
) (string, int64, error) {
	lines := (*lineBuf)[:0]
	maxRPM := int64(-1)

	for _, f := range fanFiles {
		rpmVal := readHwmonFanRPM(fr, f)
		if rpmVal < 0 {
			continue
		}
		if rpmVal > maxRPM {
			maxRPM = rpmVal
		}

		label, cached := labels[f]
		if !cached {
			label = filepath.Base(filepath.Dir(f))
			if l, err := fr.Read(strings.Replace(f, "_input", "_label", 1)); err == nil {
				label = l
			}
			labels[f] = label
		}

		lines = append(lines, fmt.Sprintf("%s: %d RPM", label, rpmVal))
	}
	*lineBuf = lines

	if len(lines) == 0 {
		return "", -1, ErrNoFanData
	}
	return "[hwmon]\n" + strings.Join(lines, "\n"), maxRPM, nil
}

func readHwmonFanRPM(fr FileReader, path string) int64 {
	fault, err := fr.Read(strings.Replace(path, "_input", "_fault", 1))
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		return -1
	}
	if err == nil && fault != "0" {
		return -1
	}
	rpm, ok := readInt(fr, path)
	if !ok || rpm < 0 {
		return -1
	}
	if rpm == 65535 && readOrNA(fr, filepath.Join(filepath.Dir(path), "name")) == "thinkpad" {
		// Apply the same ThinkPad-only quirk as the procfs parser.
		return -1
	}
	return rpm
}
