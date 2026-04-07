package main

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"os/signal"
	"strings"
	"sync"
	"syscall"
	"time"
)

type Monitor struct {
	fr          FileReader
	cr          CmdRunner
	cpuModel    string
	deviceModel string
	cpuFreqs    []CPUFreqInfo
	hwmonTemps  []HwmonTemp
	fanFiles    []string
	thinkpadFan bool
	sensorsOK   bool
	throttleOK  bool
	prevStat    CPUTimes
	prevCore    map[int]CPUTimes
	cpuCoreMap  map[int]int

	coreFreqBuf map[int]string
	coreBuf     []CoreStatus
	lineBuf     []string

	stats SessionStats
}

func NewMonitor() (*Monitor, error) {
	fr := sysFileReader{}
	cr := sysCmdRunner{}

	hwmonPath := discoverHwmonCPU(fr)
	cpuFreqs := discoverCPUTopology(fr)
	hwmonTemps := discoverHwmonTemps(hwmonPath)
	fanFiles := discoverFanFiles()
	thinkpadFan := fileExists(thinkpadFanPath)
	throttleOK := fileExists(cpuThrottlePath)

	sensorsOK := false
	if _, err := exec.LookPath("sensors"); err == nil {
		if out, err := cr.Run("sensors"); err == nil && len(out) > 0 {
			sensorsOK = strings.Contains(out, "Package id") ||
				strings.Contains(out, "Core ") ||
				strings.Contains(out, "Tctl:") ||
				strings.Contains(out, "Tdie:")
		}
	}

	m := &Monitor{
		fr:          fr,
		cr:          cr,
		cpuModel:    readCPUModel(fr),
		deviceModel: readDeviceModel(fr),
		cpuFreqs:    cpuFreqs,
		hwmonTemps:  hwmonTemps,
		fanFiles:    fanFiles,
		thinkpadFan: thinkpadFan,
		sensorsOK:   sensorsOK,
		throttleOK:  throttleOK,
		prevStat:    readCPUStat(fr),
		prevCore:    readPerCoreStat(fr),
		cpuCoreMap:  discoverCPUCoreMap(fr),
		coreFreqBuf: make(map[int]string, 32),
		coreBuf:     make([]CoreStatus, 0, 32),
		lineBuf:     make([]string, 0, 32),
	}

	hasFreq := len(cpuFreqs) > 0
	hasThermal := sensorsOK || len(hwmonTemps) > 0
	hasFan := thinkpadFan || len(fanFiles) > 0

	if !hasFreq && !hasThermal && !hasFan && !throttleOK {
		return nil, ErrNoMonitorData
	}

	return m, nil
}

func (m *Monitor) collect() Metrics {
	avgFreq := readFrequencies(m.fr, m.cpuFreqs, m.coreFreqBuf)

	cur := readCPUStat(m.fr)
	usage := calcUsage(m.prevStat, cur)
	m.prevStat = cur

	curCore := readPerCoreStat(m.fr)
	coreUsage := calcPerCoreUsage(m.prevCore, curCore, m.cpuCoreMap)
	m.prevCore = curCore

	cores, _ := readCPUThermal(m.fr, m.cr, m.sensorsOK, m.hwmonTemps, m.coreFreqBuf, coreUsage, &m.coreBuf)
	fanStatus, _ := readFanStatus(m.fr, m.fanFiles, m.thinkpadFan, &m.lineBuf)

	m.updateStats(usage, cores)

	return Metrics{
		DeviceModel: m.deviceModel,
		CPUModel:    m.cpuModel,
		Kernel:      readOrNA(m.fr, kernelReleasePath),
		Uptime:      readUptime(m.fr),
		LoadAvg:     readLoadAvg(m.fr),
		Governor:    readOrNA(m.fr, cpuGovernorPath),
		EnergyBias:  readOrNA(m.fr, cpuEnergyBiasPath),
		TurboBoost:  readTurboBoost(m.fr),
		AvgFreq:     avgFreq,
		CPUUsage:    usage,
		Cores:       cores,
		Throttle:    readThrottleInfo(m.fr, m.throttleOK),
		FanStatus:   fanStatus,
		SensorsHint: !m.sensorsOK,
		Stats:       m.stats,
	}
}

func (m *Monitor) updateStats(usage float64, cores []CoreStatus) {
	if usage >= 0 && usage > m.stats.PeakCPU {
		m.stats.PeakCPU = usage
	}

	for _, c := range cores {
		if !c.IsPackage || c.TempC < 0 {
			continue
		}
		if m.stats.Samples == 0 || c.TempC > m.stats.PeakTemp {
			m.stats.PeakTemp = c.TempC
		}
		if m.stats.Samples == 0 || c.TempC < m.stats.MinTemp {
			m.stats.MinTemp = c.TempC
		}
		break
	}
	m.stats.Samples++
}

func (m *Monitor) Run(ctx context.Context, interval time.Duration) error {
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, os.Interrupt, syscall.SIGTERM)
	defer signal.Stop(sigCh)

	winCh := make(chan os.Signal, 1)
	signal.Notify(winCh, syscall.SIGWINCH)
	defer signal.Stop(winCh)

	orig, rawErr := enableRawMode()
	if rawErr == nil {
		defer restoreTermMode(orig)
	}

	keyCh := make(chan byte, 1)
	var wg sync.WaitGroup
	wg.Go(func() {
		select {
		case <-sigCh:
			cancel()
		case <-ctx.Done():
		}
	})
	go func() {
		var buf [1]byte
		for {
			n, err := os.Stdin.Read(buf[:])
			if n == 1 && (buf[0] == 'q' || buf[0] == 'Q' || buf[0] == 0x03) {
				keyCh <- buf[0]
				return
			}
			if err != nil {
				return
			}
		}
	}()

	fmt.Print("\033[?1049h")
	defer fmt.Print("\033[?1049l")

	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	last := m.collect()
	display(last, interval)

	for {
		select {
		case <-ctx.Done():
			wg.Wait()
			return nil
		case <-keyCh:
			cancel()
			wg.Wait()
			return nil
		case <-ticker.C:
			last = m.collect()
			display(last, interval)
		case <-winCh:
			display(last, interval)
		}
	}
}
