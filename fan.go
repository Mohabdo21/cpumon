package main

import (
	"bufio"
	"fmt"
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

func readFanStatus(
	fr FileReader,
	fanFiles []string,
	thinkpadFan bool,
	lineBuf *[]string,
) (string, int64, error) {
	if thinkpadFan {
		if out, rpm, err := readThinkPadFan(fr, lineBuf); err == nil {
			return out, rpm, nil
		}
	}

	if len(fanFiles) > 0 {
		if out, rpm, err := readHwmonFan(fr, fanFiles, lineBuf); err == nil {
			return out, rpm, nil
		}
	}

	return "", 0, ErrNoFanData
}

func readThinkPadFan(fr FileReader, lineBuf *[]string) (string, int64, error) {
	data, err := fr.Read(thinkpadFanPath)
	if err != nil {
		return "", 0, err
	}

	lines := (*lineBuf)[:0]
	var rpm int64
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

	if len(lines) == 0 {
		return "", 0, ErrNoFanData
	}
	return "[ThinkPad]\n" + strings.Join(lines, "\n"), rpm, nil
}

// fanSpeedRPM extracts the RPM value from a ThinkPad "speed:" line.
func fanSpeedRPM(line string) (int64, bool) {
	rest, ok := strings.CutPrefix(line, "speed:")
	if !ok {
		return 0, false
	}
	return parseInt64(strings.TrimSpace(rest))
}

func readHwmonFan(fr FileReader, fanFiles []string, lineBuf *[]string) (string, int64, error) {
	lines := (*lineBuf)[:0]
	var maxRPM int64

	for _, f := range fanFiles {
		rpmVal, ok := readInt(fr, f)
		if !ok || rpmVal <= 0 {
			continue
		}
		if rpmVal > maxRPM {
			maxRPM = rpmVal
		}

		label := filepath.Base(filepath.Dir(f))
		if l, err := fr.Read(strings.Replace(f, "_input", "_label", 1)); err == nil {
			label = l
		}

		lines = append(lines, fmt.Sprintf("%s: %d RPM", label, rpmVal))
	}
	*lineBuf = lines

	if len(lines) == 0 {
		return "", 0, ErrNoFanData
	}
	return "[hwmon]\n" + strings.Join(lines, "\n"), maxRPM, nil
}
