package main

import (
	"bufio"
	"fmt"
	"strconv"
	"strings"
)

func readLoadAvg(fr FileReader) string {
	data, err := fr.Read(procLoadAvgPath)
	if err != nil {
		return "N/A"
	}

	fields := strings.Fields(data)
	if len(fields) < 3 {
		return "N/A"
	}

	return strings.Join(fields[:3], " ")
}

func readTurboBoost(fr FileReader) string {
	if raw, err := fr.Read(intelNoTurboPath); err == nil {
		switch strings.TrimSpace(raw) {
		case "0":
			return "Enabled"
		case "1":
			return "Disabled"
		}
	}

	if raw, err := fr.Read(cpuBoostPath); err == nil {
		enabled, err := strconv.ParseInt(strings.TrimSpace(raw), 10, 64)
		if err == nil {
			if enabled != 0 {
				return "Enabled"
			}
			return "Disabled"
		}
	}

	return "N/A"
}

func readCPUModel(fr FileReader) string {
	data, err := fr.Read(cpuInfoPath)
	if err != nil {
		return "Unknown CPU"
	}

	fields := []string{"model name", "Model", "Hardware", "Processor", "cpu model"}
	found := make(map[string]string)

	scanner := bufio.NewScanner(strings.NewReader(data))
	for scanner.Scan() {
		line := scanner.Text()
		for _, field := range fields {
			if strings.HasPrefix(strings.ToLower(line), strings.ToLower(field)) {
				parts := strings.SplitN(line, ":", 2)
				if len(parts) == 2 {
					val := strings.TrimSpace(parts[1])
					if val != "" && found[field] == "" {
						found[field] = val
					}
				}
			}
		}
	}

	for _, field := range fields {
		if v := found[field]; v != "" {
			return v
		}
	}
	return "Unknown CPU"
}

func readDeviceModel(fr FileReader) string {
	if name, err := fr.Read(dmiProductName); err == nil && name != "" {
		ver, _ := fr.Read(dmiProductVersion)
		if ver != "" && ver != "ThinkPad" && !strings.Contains(name, ver) {
			return name + " " + ver
		}
		return name
	}

	boardName, boardErr := fr.Read(dmiBoardName)
	boardVendor, _ := fr.Read(dmiBoardVendor)
	if boardErr == nil && boardName != "" {
		if boardVendor != "" && !strings.Contains(boardName, boardVendor) {
			return boardVendor + " " + boardName
		}
		return boardName
	}

	if vm := detectVM(fr); vm != "" {
		return vm
	}

	if fileExists("/proc/device-tree/model") {
		if model, err := fr.Read("/proc/device-tree/model"); err == nil && model != "" {
			return strings.TrimRight(model, "\x00")
		}
	}

	return "Unknown Device"
}

func detectVM(fr FileReader) string {
	if data, err := fr.Read("/proc/1/cgroup"); err == nil {
		if strings.Contains(data, "docker") {
			return "Docker Container"
		}
		if strings.Contains(data, "lxc") {
			return "LXC Container"
		}
	}

	if hyp, err := fr.Read("/sys/hypervisor/type"); err == nil && hyp != "" {
		return fmt.Sprintf("Virtual Machine (%s)", hyp)
	}

	if vendor, err := fr.Read("/sys/class/dmi/id/sys_vendor"); err == nil {
		v := strings.ToLower(vendor)
		switch {
		case strings.Contains(v, "vmware"):
			return "VMware Virtual Machine"
		case strings.Contains(v, "qemu"), strings.Contains(v, "kvm"):
			return "QEMU/KVM Virtual Machine"
		case strings.Contains(v, "virtualbox"):
			return "VirtualBox Virtual Machine"
		case strings.Contains(v, "microsoft") && strings.Contains(v, "hyper"):
			return "Hyper-V Virtual Machine"
		}
	}

	return ""
}
