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

	coreFreqBuf map[int]string
	lineBuf     []string
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
		coreFreqBuf: make(map[int]string, 32),
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

	cpuStatus, _ := readCPUThermal(m.fr, m.cr, m.sensorsOK, m.hwmonTemps, m.coreFreqBuf, &m.lineBuf)
	fanStatus, _ := readFanStatus(m.fr, m.fanFiles, m.thinkpadFan, &m.lineBuf)

	return Metrics{
		DeviceModel: m.deviceModel,
		CPUModel:    m.cpuModel,
		Governor:    readOrNA(m.fr, cpuGovernorPath),
		EnergyBias:  readOrNA(m.fr, cpuEnergyBiasPath),
		AvgFreq:     avgFreq,
		CPUUsage:    usage,
		CPUStatus:   cpuStatus,
		Throttle:    readThrottleInfo(m.fr, m.throttleOK),
		FanStatus:   fanStatus,
		SensorsHint: !m.sensorsOK,
	}
}

func (m *Monitor) Run(ctx context.Context, interval time.Duration) error {
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, os.Interrupt, syscall.SIGTERM)
	defer signal.Stop(sigCh)

	var wg sync.WaitGroup
	wg.Go(func() {
		select {
		case <-sigCh:
			cancel()
		case <-ctx.Done():
		}
	})

	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	display(m.collect(), interval)

	for {
		select {
		case <-ctx.Done():
			fmt.Println("\nShutting down...")
			wg.Wait()
			return nil
		case <-ticker.C:
			display(m.collect(), interval)
		}
	}
}
