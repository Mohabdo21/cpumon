package main

import (
	"fmt"
	"strconv"
)

type ThrottleInfo struct {
	Available        bool
	PackageCount     string
	PackageTotalTime string
	PackageMaxTime   string
	CoreCount        string
	CoreTotalTime    string
	CoreMaxTime      string
}

func readThrottleInfo(fr FileReader, available bool) ThrottleInfo {
	if !available {
		return ThrottleInfo{Available: false}
	}

	info := ThrottleInfo{Available: true}

	info.PackageCount = readCount(fr, cpuThrottlePath)
	info.PackageTotalTime = readDuration(fr, pkgThrottleTotal)
	info.PackageMaxTime = readMs(fr, pkgThrottleMax)
	info.CoreCount = readCount(fr, coreThrottleCount)
	info.CoreTotalTime = readDuration(fr, coreThrottleTotal)
	info.CoreMaxTime = readMs(fr, coreThrottleMax)

	return info
}

func readCount(fr FileReader, path string) string {
	if v, ok := readInt(fr, path); ok {
		return strconv.FormatInt(v, 10)
	}
	return "N/A"
}

func readMs(fr FileReader, path string) string {
	if v, ok := readInt(fr, path); ok {
		return fmt.Sprintf("%d ms", v)
	}
	return "N/A"
}

func readDuration(fr FileReader, path string) string {
	if v, ok := readInt(fr, path); ok {
		return formatDuration(v)
	}
	return "N/A"
}

func formatDuration(ms int64) string {
	if ms < 1000 {
		return fmt.Sprintf("%d ms", ms)
	}
	sec := ms / 1000
	if sec < 60 {
		return fmt.Sprintf("%d seconds", sec)
	}
	min := sec / 60
	remSec := sec % 60
	if min < 60 {
		return fmt.Sprintf("%d min %d sec", min, remSec)
	}
	hr := min / 60
	remMin := min % 60
	return fmt.Sprintf("%d hr %d min", hr, remMin)
}
