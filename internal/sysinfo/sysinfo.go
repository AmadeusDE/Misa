package sysinfo

import (
	"bufio"
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"
)

type Info struct {
	Time     string
	CWD      string
	RAM      string
	Swap     string
	Uptime   string
	Kernel   string
	Terminal string
	Distro   string
}

func Fetch(cwd string) Info {
	return Info{
		Time:     time.Now().Format("15:04:05"),
		CWD:      cwd,
		RAM:      getMemory(),
		Swap:     getSwap(),
		Uptime:   getUptime(),
		Kernel:   getKernel(),
		Terminal: getTerminal(),
		Distro:   getDistro(),
	}
}

func getDistro() string {
	f, err := os.Open("/etc/os-release")
	if err != nil {
		return "linux"
	}
	defer f.Close()

	var id, prettyName string
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := scanner.Text()
		if strings.HasPrefix(line, "ID=") {
			id = strings.Trim(line[3:], `"'`)
		} else if strings.HasPrefix(line, "PRETTY_NAME=") {
			prettyName = strings.Trim(line[len("PRETTY_NAME="):], `"'`)
		}
	}
	if id != "" {
		return id
	}
	if prettyName != "" {
		return prettyName
	}
	return "linux"
}

func getKernel() string {
	data, err := os.ReadFile("/proc/sys/kernel/osrelease")
	if err != nil {
		return "?"
	}
	return strings.TrimSpace(string(data))
}

func getUptime() string {
	data, err := os.ReadFile("/proc/uptime")
	if err != nil {
		return "?"
	}
	fields := strings.Fields(string(data))
	if len(fields) < 1 {
		return "?"
	}
	secs, err := strconv.ParseFloat(fields[0], 64)
	if err != nil {
		return "?"
	}

	s := int(secs)
	d := s / 86400
	h := (s / 3600) % 24
	m := (s / 60) % 60

	var parts []string
	if d > 0 {
		parts = append(parts, fmt.Sprintf("%dd", d))
	}
	if h > 0 {
		parts = append(parts, fmt.Sprintf("%dh", h))
	}
	parts = append(parts, fmt.Sprintf("%dm", m))
	return strings.Join(parts, " ")
}

func getMemory() string {
	total, available, free, buffers, cached, sreclaimable, shmem := parseMeminfo()
	if total == 0 {
		return "?"
	}

	var used uint64
	if available > 0 {
		used = total - available
	} else {
		used = total + shmem - free - buffers - cached - sreclaimable
	}
	pct := (used * 100) / total
	return fmt.Sprintf("%dMB / %dMB (%d%%)", used, total, pct)
}

func getSwap() string {
	f, err := os.Open("/proc/meminfo")
	if err != nil {
		return ""
	}
	defer f.Close()

	var total, free uint64
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		fields := strings.Fields(scanner.Text())
		if len(fields) < 2 {
			continue
		}
		val, err := strconv.ParseUint(fields[1], 10, 64)
		if err != nil {
			continue
		}
		switch strings.TrimSuffix(fields[0], ":") {
		case "SwapTotal":
			total = val / 1024
		case "SwapFree":
			free = val / 1024
		}
	}

	if total == 0 {
		return ""
	}
	used := total - free
	pct := (used * 100) / total
	return fmt.Sprintf("%dMB / %dMB (%d%%)", used, total, pct)
}

func parseMeminfo() (total, available, free, buffers, cached, sreclaimable, shmem uint64) {
	f, err := os.Open("/proc/meminfo")
	if err != nil {
		return
	}
	defer f.Close()

	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		fields := strings.Fields(scanner.Text())
		if len(fields) < 2 {
			continue
		}
		val, err := strconv.ParseUint(fields[1], 10, 64)
		if err != nil {
			continue
		}
		switch strings.TrimSuffix(fields[0], ":") {
		case "MemTotal":
			total = val / 1024
		case "MemAvailable":
			available = val / 1024
		case "MemFree":
			free = val / 1024
		case "Buffers":
			buffers = val / 1024
		case "Cached":
			cached = val / 1024
		case "SReclaimable":
			sreclaimable = val / 1024
		case "Shmem":
			shmem = val / 1024
		}
	}
	return
}
