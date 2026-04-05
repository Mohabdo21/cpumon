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

func readFanStatus(fr FileReader, fanFiles []string, thinkpadFan bool, lineBuf *[]string) (string, error) {
	if thinkpadFan {
		if out, err := readThinkPadFan(fr, lineBuf); err == nil {
			return out, nil
		}
	}

	if len(fanFiles) > 0 {
		if out, err := readHwmonFan(fr, fanFiles, lineBuf); err == nil {
			return out, nil
		}
	}

	return "", ErrNoFanData
}

func readThinkPadFan(fr FileReader, lineBuf *[]string) (string, error) {
	data, err := fr.Read(thinkpadFanPath)
	if err != nil {
		return "", err
	}

	lines := (*lineBuf)[:0]
	scanner := bufio.NewScanner(strings.NewReader(data))
	for scanner.Scan() {
		line := scanner.Text()
		if fanFilterRe.MatchString(line) {
			lines = append(lines, line)
		}
	}
	*lineBuf = lines

	if len(lines) == 0 {
		return "", ErrNoFanData
	}
	return "[ThinkPad]\n" + strings.Join(lines, "\n"), nil
}

func readHwmonFan(fr FileReader, fanFiles []string, lineBuf *[]string) (string, error) {
	lines := (*lineBuf)[:0]

	for _, f := range fanFiles {
		rpmVal, ok := readInt(fr, f)
		if !ok {
			continue
		}

		label := filepath.Base(filepath.Dir(f))
		if l, err := fr.Read(strings.Replace(f, "_input", "_label", 1)); err == nil {
			label = l
		}

		lines = append(lines, fmt.Sprintf("%s: %d RPM", label, rpmVal))
	}
	*lineBuf = lines

	if len(lines) == 0 {
		return "", ErrNoFanData
	}
	return "[hwmon]\n" + strings.Join(lines, "\n"), nil
}
