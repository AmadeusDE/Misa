package sysinfo

import (
	"fmt"
	"strconv"
	"strings"
)

func windowsVersion(product, display, build string, ubr uint64, hasUBR bool, server bool) (string, string) {
	product = strings.TrimSpace(product)
	build = strings.TrimSpace(build)
	n, _ := strconv.ParseUint(build, 10, 64)
	distro := "windows"
	if !server && (strings.Contains(product, "Windows 10") || strings.Contains(product, "Windows 11")) {
		if n >= 22000 {
			product = strings.Replace(product, "Windows 10", "Windows 11", 1)
			distro = "windows_11"
		} else if n > 0 {
			distro = "windows_10"
		}
	}
	if product == "" {
		product = "Windows"
	}
	if display = strings.TrimSpace(display); display != "" {
		product += " " + display
	}
	if build != "" {
		product += " (build " + build
		if hasUBR {
			product += fmt.Sprintf(".%d", ubr)
		}
		product += ")"
	}
	return product, distro
}

func windowsMemory(total, available uint64) string {
	if total == 0 || available > total {
		return "?"
	}
	return fmt.Sprintf("%dMB / %dMB (%d%%)", (total-available)/(1<<20), total/(1<<20), uint64(float64(total-available)*100/float64(total)))
}

func windowsUptime(ms uint64) string {
	if ms == 0 {
		return "?"
	}
	s := ms / 1000
	var parts []string
	if d := s / 86400; d > 0 {
		parts = append(parts, fmt.Sprintf("%dd", d))
	}
	if h := s / 3600 % 24; h > 0 {
		parts = append(parts, fmt.Sprintf("%dh", h))
	}
	parts = append(parts, fmt.Sprintf("%dm", s/60%60))
	return strings.Join(parts, " ")
}

func windowsVersionFallback(product, edition, display string, major, minor, build uint32, ubr uint64, hasUBR, server bool) (string, string) {
	if strings.TrimSpace(product) == "" {
		product = "Windows"
		if !server && major == 10 && build != 0 {
			product = "Windows 10"
		}
		if edition = strings.TrimSpace(edition); edition != "" {
			product += " " + edition
		}
	}
	buildStr := ""
	if build != 0 {
		buildStr = strconv.FormatUint(uint64(build), 10)
	}
	return windowsVersion(product, display, buildStr, ubr, hasUBR, server)
}

func windowsKernel(major, minor, build uint32, ubr uint64, hasUBR bool) string {
	if major == 0 || build == 0 {
		return "?"
	}
	version := fmt.Sprintf("%d.%d.%d", major, minor, build)
	if hasUBR {
		version += fmt.Sprintf(".%d", ubr)
	}
	return version
}

func windowsStorageLine(root string, total, free uint64) string {
	if total == 0 || free > total {
		return ""
	}
	used := total - free
	return fmt.Sprintf("%s: %s / %s (%.0f%%)", root, formatBytes(used), formatBytes(total), float64(used)*100/float64(total))
}
