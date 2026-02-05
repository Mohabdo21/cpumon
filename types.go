package main

import (
	"errors"
	"os"
	"os/exec"
	"regexp"
	"strconv"
	"strings"
)

const (
	cpuGovernorPath   = "/sys/devices/system/cpu/cpu0/cpufreq/scaling_governor"
	cpuEnergyBiasPath = "/sys/devices/system/cpu/cpu0/cpufreq/energy_performance_preference"
	cpuThrottlePath   = "/sys/devices/system/cpu/cpu0/thermal_throttle/package_throttle_count"
	pkgThrottleTotal  = "/sys/devices/system/cpu/cpu0/thermal_throttle/package_throttle_total_time_ms"
	pkgThrottleMax    = "/sys/devices/system/cpu/cpu0/thermal_throttle/package_throttle_max_time_ms"
	coreThrottleCount = "/sys/devices/system/cpu/cpu0/thermal_throttle/core_throttle_count"
	coreThrottleTotal = "/sys/devices/system/cpu/cpu0/thermal_throttle/core_throttle_total_time_ms"
	coreThrottleMax   = "/sys/devices/system/cpu/cpu0/thermal_throttle/core_throttle_max_time_ms"
	cpuInfoPath       = "/proc/cpuinfo"
	dmiProductName    = "/sys/class/dmi/id/product_name"
	dmiProductVersion = "/sys/class/dmi/id/product_version"
	dmiBoardName      = "/sys/class/dmi/id/board_name"
	dmiBoardVendor    = "/sys/class/dmi/id/board_vendor"
	thinkpadFanPath   = "/proc/acpi/ibm/fan"
)

var (
	ErrNoThermalData = errors.New("no thermal data")
	ErrNoFanData     = errors.New("no fan interface")
)

var (
	cpuNumRe    = regexp.MustCompile(`cpu(\d+)`)
	packageRe   = regexp.MustCompile(`^(Package id \d+):(\s+)(\S.*)`)
	coreRe      = regexp.MustCompile(`^(Core\s+(\d+)):(\s+)(\S.*)`)
	amdTctlRe   = regexp.MustCompile(`^(Tctl|Tdie|Tccd\d*):(\s+)(\S.*)`)
	coreNumRe   = regexp.MustCompile(`Core\s*(\d+)`)
	fanFilterRe = regexp.MustCompile(`(level:|speed:|status:)`)
)

type CPUFreqInfo struct {
	Path   string
	CoreID int
}

type HwmonTemp struct {
	Input string
	Label string
	Crit  string
	Max   string
}

type FileReader interface {
	Read(path string) (string, error)
}

type CmdRunner interface {
	Run(name string, args ...string) (string, error)
}

type sysFileReader struct{}

func (sysFileReader) Read(path string) (string, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(string(data)), nil
}

type sysCmdRunner struct{}

func (sysCmdRunner) Run(name string, args ...string) (string, error) {
	out, err := exec.Command(name, args...).CombinedOutput()
	if err != nil {
		return "", err
	}
	return string(out), nil
}

func fileExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}

func readInt(fr FileReader, path string) (int64, bool) {
	raw, err := fr.Read(path)
	if err != nil || raw == "" {
		return 0, false
	}
	v, err := strconv.ParseInt(raw, 10, 64)
	return v, err == nil
}
