package main

import (
	"errors"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"time"
)

const (
	// CPU
	cpuGovernorPath   = "/sys/devices/system/cpu/cpu0/cpufreq/scaling_governor"
	cpuEnergyBiasPath = "/sys/devices/system/cpu/cpu0/cpufreq/energy_performance_preference"
	cpuInfoPath       = "/proc/cpuinfo"
	intelNoTurboPath  = "/sys/devices/system/cpu/intel_pstate/no_turbo"
	cpuBoostPath      = "/sys/devices/system/cpu/cpufreq/boost"

	// Thermal throttle
	cpuThrottlePath   = "/sys/devices/system/cpu/cpu0/thermal_throttle/package_throttle_count"
	pkgThrottleTotal  = "/sys/devices/system/cpu/cpu0/thermal_throttle/package_throttle_total_time_ms"
	pkgThrottleMax    = "/sys/devices/system/cpu/cpu0/thermal_throttle/package_throttle_max_time_ms"
	coreThrottleCount = "/sys/devices/system/cpu/cpu0/thermal_throttle/core_throttle_count"
	coreThrottleTotal = "/sys/devices/system/cpu/cpu0/thermal_throttle/core_throttle_total_time_ms"
	coreThrottleMax   = "/sys/devices/system/cpu/cpu0/thermal_throttle/core_throttle_max_time_ms"

	// Proc
	procLoadAvgPath   = "/proc/loadavg"
	procStatPath      = "/proc/stat"
	procUptimePath    = "/proc/uptime"
	kernelReleasePath = "/proc/sys/kernel/osrelease"

	// DMI / device
	dmiProductName    = "/sys/class/dmi/id/product_name"
	dmiProductVersion = "/sys/class/dmi/id/product_version"
	dmiBoardName      = "/sys/class/dmi/id/board_name"
	dmiBoardVendor    = "/sys/class/dmi/id/board_vendor"

	// Fan
	thinkpadFanPath = "/proc/acpi/ibm/fan"
)

var (
	ErrNoThermalData = errors.New("no thermal data")
	ErrNoFanData     = errors.New("no fan interface")
	ErrNoMonitorData = errors.New("no monitoring data available (VM or container environment?)")
)

type SessionStats struct {
	PeakCPU      float64
	PeakTemp     float64
	MinTemp      float64
	Samples      uint64
	StartTime    time.Time
	TotalCPU     float64
	PeakPower    float64
	TotalPower   float64
	PowerSamples uint64
}

type Metrics struct {
	DeviceModel string
	CPUModel    string
	Kernel      string
	Uptime      string
	LoadAvg     string
	Governor    string
	EnergyBias  string
	TurboBoost  string
	AvgFreq     string
	CPUUsage    float64
	Cores       []CoreStatus
	Throttle    ThrottleInfo
	FanStatus   string
	Power       PowerReading
	SensorsHint bool
	Stats       SessionStats
	Topology    CoreTopology
}

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

type CoreStatus struct {
	Label     string
	Freq      string
	Usage     float64
	Temp      string
	Limit     string
	TempC     float64
	IsPackage bool
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

func readOrNA(fr FileReader, path string) string {
	if s, err := fr.Read(path); err == nil {
		return s
	}
	return "N/A"
}

func readInt(fr FileReader, path string) (int64, bool) {
	raw, err := fr.Read(path)
	if err != nil || raw == "" {
		return 0, false
	}
	v, err := strconv.ParseInt(raw, 10, 64)
	return v, err == nil
}
