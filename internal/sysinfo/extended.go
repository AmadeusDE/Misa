package sysinfo

import (
	"bufio"
	"bytes"
	"fmt"
	"math"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"syscall"
)

type ExtendedInfo struct {
	Title        string
	OS           string
	Host         string
	CPU          string
	GPU          string
	Shell        string
	Editor       string
	WM           string
	DE           string
	Packages     string
	Storage      string
	Monitors     string
	Audio        string
	LocalIP      string
	Battery      string
	PowerAdapter string
	Bluetooth    string
	Terminal     string
	LoginManager string
	Gamepad      string
}

func FetchExtended() ExtendedInfo {
	var info ExtendedInfo
	info.Title = getTitle()
	info.OS = getOS()
	info.Host = getHost()
	info.CPU = getCPU()
	info.GPU = getGPU()
	info.Shell = getShell()
	info.Editor = getEditor()
	info.WM = getWM()
	info.DE = getDE()
	info.Packages = getPackages()
	info.Storage = getStorage()
	info.Monitors = getMonitors()
	info.Audio = getAudio()
	info.LocalIP = getLocalIP()
	info.Battery = getBattery()
	info.PowerAdapter = getPowerAdapter()
	info.Bluetooth = getBluetooth()
	info.Terminal = getTerminal()
	info.LoginManager = getLoginManager()
	info.Gamepad = getGamepad()
	return info
}

func getTitle() string {
	user := os.Getenv("USER")
	if user == "" {
		user = os.Getenv("LOGNAME")
	}
	if user == "" {
		user = "user"
	}
	hostname := ""
	data, err := os.ReadFile("/etc/hostname")
	if err == nil {
		hostname = strings.TrimSpace(string(data))
	}
	if hostname == "" {
		h, err := os.Hostname()
		if err == nil {
			hostname = h
		} else {
			hostname = "localhost"
		}
	}
	return fmt.Sprintf("%s@%s", user, hostname)
}

func getOS() string {
	if name := detectPlatform().osName(); name != "" {
		return name
	}
	f, err := os.Open("/etc/os-release")
	if err != nil {
		return "Linux"
	}
	defer f.Close()
	var prettyName, name string
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := scanner.Text()
		if strings.HasPrefix(line, "PRETTY_NAME=") {
			prettyName = strings.Trim(line[len("PRETTY_NAME="):], `"'`)
		} else if strings.HasPrefix(line, "NAME=") {
			name = strings.Trim(line[len("NAME="):], `"'`)
		}
	}
	if prettyName != "" {
		return prettyName
	}
	if name != "" {
		return name
	}
	return "Linux"
}

func getHost() string {
	if host := detectPlatform().hostName(); host != "" {
		return host
	}
	productName := ""
	data, err := os.ReadFile("/sys/devices/virtual/dmi/id/product_name")
	if err == nil {
		productName = strings.TrimSpace(string(data))
	}
	productVersion := ""
	data, err = os.ReadFile("/sys/devices/virtual/dmi/id/product_version")
	if err == nil {
		productVersion = strings.TrimSpace(string(data))
	}
	model := ""
	data, err = os.ReadFile("/sys/firmware/devicetree/base/model")
	if err == nil {
		model = strings.TrimRight(strings.TrimSpace(string(data)), "\x00")
	}

	var parts []string
	if productName != "" {
		parts = append(parts, productName)
	}
	if productVersion != "" {
		parts = append(parts, productVersion)
	}
	if model != "" {
		parts = append(parts, model)
	}

	hostStr := strings.Join(parts, " ")
	blacklist := map[string]bool{
		"To": true, "be": true, "filled": true, "by": true,
		"O.E.M.": true, "OEM": true, "Not": true, "Applicable": true,
		"Specified": true, "System": true, "Product": true, "Name": true,
		"Version": true, "Undefined": true, "Default": true, "string": true,
		"INVALID": true,
	}
	var cleaned []string
	for _, w := range strings.Fields(hostStr) {
		if !blacklist[w] {
			cleaned = append(cleaned, w)
		}
	}
	if len(cleaned) > 0 {
		return strings.Join(cleaned, " ")
	}
	archData, err := exec.Command("uname", "-m").Output()
	if err == nil {
		return strings.TrimSpace(string(archData))
	}
	return ""
}

func getCPU() string {
	f, err := os.Open("/proc/cpuinfo")
	if err != nil {
		return ""
	}
	defer f.Close()
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := scanner.Text()
		if strings.HasPrefix(line, "model name") {
			parts := strings.SplitN(line, ":", 2)
			if len(parts) >= 2 {
				name := strings.TrimSpace(parts[1])
				for _, r := range []string{"(R)", "(TM)", "Processor", "CPU",
					"Dual-Core", "Quad-Core", "Six-Core", "Eight-Core",
					"12-Core", "16-Core", "24-Core", "32-Core"} {
					name = strings.ReplaceAll(name, r, "")
				}
				name = strings.Join(strings.Fields(name), " ")
				return fmt.Sprintf("%s (%d)", name, runtime.NumCPU())
			}
		}
	}
	return ""
}

func resolvePCI(vendorHex, deviceHex string) (string, string) {
	vID := strings.ToLower(strings.TrimPrefix(vendorHex, "0x"))
	dID := strings.ToLower(strings.TrimPrefix(deviceHex, "0x"))
	vID = fmt.Sprintf("%04s", vID)
	dID = fmt.Sprintf("%04s", dID)

	paths := []string{"/usr/share/hwdata/pci.ids", "/usr/share/misc/pci.ids"}
	var file *os.File
	for _, p := range paths {
		f, err := os.Open(p)
		if err == nil {
			file = f
			break
		}
	}
	if file == nil {
		return "", ""
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	vendorName := ""
	deviceName := ""
	inVendor := false

	for scanner.Scan() {
		line := scanner.Text()
		if len(line) == 0 || line[0] == '#' {
			continue
		}
		if !strings.HasPrefix(line, "\t") {
			inVendor = false
			parts := strings.SplitN(line, "  ", 2)
			if len(parts) >= 2 {
				if strings.ToLower(strings.TrimSpace(parts[0])) == vID {
					vendorName = strings.TrimSpace(parts[1])
					inVendor = true
				}
			}
		} else if inVendor {
			trimmed := line[1:]
			if strings.HasPrefix(trimmed, "\t") {
				continue
			}
			parts := strings.SplitN(strings.TrimSpace(trimmed), "  ", 2)
			if len(parts) >= 2 {
				if strings.ToLower(strings.TrimSpace(parts[0])) == dID {
					deviceName = strings.TrimSpace(parts[1])
					break
				}
			}
		}
	}
	return vendorName, deviceName
}

func cleanGPUName(vendor, device string) string {
	vendor = strings.TrimSpace(vendor)
	device = strings.TrimSpace(device)

	vClean := vendor
	vLower := strings.ToLower(vendor)
	if strings.Contains(vLower, "intel") {
		vClean = "Intel"
	} else if strings.Contains(vLower, "advanced micro devices") || strings.Contains(vLower, "amd") {
		vClean = "AMD"
	} else if strings.Contains(vLower, "nvidia") {
		vClean = "NVIDIA"
	}

	dClean := device
	if idx := strings.LastIndex(device, "["); idx != -1 {
		if end := strings.Index(device[idx:], "]"); end != -1 {
			dClean = device[idx+1 : idx+end]
		}
	}

	if vClean != "" && dClean != "" {
		if strings.Contains(strings.ToLower(dClean), strings.ToLower(vClean)) {
			return dClean
		}
		return vClean + " " + dClean
	} else if dClean != "" {
		return dClean
	} else if vClean != "" {
		return vClean
	}
	return ""
}

func getGPU() string {
	devices, err := filepath.Glob("/sys/bus/pci/devices/*")
	if err != nil {
		return ""
	}
	var gpus []string
	seen := map[string]bool{}
	for _, dev := range devices {
		classData, err := os.ReadFile(filepath.Join(dev, "class"))
		if err != nil {
			continue
		}
		if !strings.HasPrefix(strings.TrimSpace(string(classData)), "0x03") {
			continue
		}
		vendorData, _ := os.ReadFile(filepath.Join(dev, "vendor"))
		deviceData, _ := os.ReadFile(filepath.Join(dev, "device"))
		vendorHex := strings.TrimSpace(string(vendorData))
		deviceHex := strings.TrimSpace(string(deviceData))

		vendorName, deviceName := resolvePCI(vendorHex, deviceHex)
		gpuName := cleanGPUName(vendorName, deviceName)
		if gpuName == "" {
			gpuName = fmt.Sprintf("GPU (%s:%s)", vendorHex, deviceHex)
		}
		if !seen[gpuName] {
			seen[gpuName] = true
			gpus = append(gpus, gpuName)
		}
	}
	if len(gpus) == 0 {
		return ""
	}
	return strings.Join(gpus, ", ")
}

func getShell() string {
	s := os.Getenv("SHELL")
	if s == "" {
		return ""
	}
	return filepath.Base(s)
}

func getEditor() string {
	e := os.Getenv("VISUAL")
	if e == "" {
		e = os.Getenv("EDITOR")
	}
	if e == "" {
		return ""
	}
	return filepath.Base(e)
}

func getWM() string {
	if os.Getenv("DISPLAY") == "" && os.Getenv("WAYLAND_DISPLAY") == "" {
		return ""
	}
	procDirs, err := os.ReadDir("/proc")
	if err != nil {
		return ""
	}
	knownWMs := map[string]string{
		"sway": "sway", "hyprland": "Hyprland", "i3": "i3",
		"bspwm": "bspwm", "dwm": "dwm", "awesome": "awesome",
		"openbox": "openbox", "fluxbox": "fluxbox", "xmonad": "xmonad",
		"wayfire": "wayfire", "river": "river",
		"kwin_wayland": "kwin", "kwin_x11": "kwin",
		"mutter": "mutter", "gnome-shell": "mutter",
		"xfwm4": "xfwm4", "marco": "marco", "compiz": "compiz", "sowm": "sowm",
	}
	for _, d := range procDirs {
		if !d.IsDir() {
			continue
		}
		if _, err := strconv.Atoi(d.Name()); err != nil {
			continue
		}
		commData, err := os.ReadFile(filepath.Join("/proc", d.Name(), "comm"))
		if err == nil {
			comm := strings.TrimSpace(string(commData))
			if wmName, ok := knownWMs[comm]; ok {
				return wmName
			}
		}
	}
	if os.Getenv("DISPLAY") != "" {
		if _, err := exec.LookPath("xprop"); err == nil {
			idOut, err := exec.Command("xprop", "-root", "-notype", "_NET_SUPPORTING_WM_CHECK").Output()
			if err == nil {
				fields := strings.Fields(string(idOut))
				if len(fields) > 0 {
					id := fields[len(fields)-1]
					nameOut, err := exec.Command("xprop", "-id", id, "-notype", "-len", "25", "-f", "_NET_WM_NAME", "8t").Output()
					if err == nil {
						if parts := strings.SplitN(string(nameOut), "_NET_WM_NAME = \"", 2); len(parts) > 1 {
							return strings.SplitN(parts[1], "\"", 2)[0]
						}
					}
				}
			}
		}
	}
	return ""
}

func getDE() string {
	de := os.Getenv("XDG_CURRENT_DESKTOP")
	if de == "" {
		de = os.Getenv("DESKTOP_SESSION")
	}
	if de != "" {
		return de
	}
	procDirs, err := os.ReadDir("/proc")
	if err != nil {
		return ""
	}
	knownDEs := map[string]string{
		"plasmashell": "KDE", "gnome-shell": "GNOME", "xfce4-session": "XFCE",
		"mate-session": "MATE", "lxsession": "LXDE", "cinnamon": "Cinnamon",
	}
	for _, d := range procDirs {
		if !d.IsDir() {
			continue
		}
		if _, err := strconv.Atoi(d.Name()); err != nil {
			continue
		}
		commData, err := os.ReadFile(filepath.Join("/proc", d.Name(), "comm"))
		if err == nil {
			if deName, ok := knownDEs[strings.TrimSpace(string(commData))]; ok {
				return deName
			}
		}
	}
	return ""
}

func getPackages() string {
	var parts []string

	if dirs, err := os.ReadDir("/var/lib/pacman/local"); err == nil {
		count := 0
		for _, d := range dirs {
			if d.IsDir() && d.Name() != "ALPM_DB_VERSION" {
				count++
			}
		}
		if count > 0 {
			parts = append(parts, fmt.Sprintf("%d (pacman)", count))
		}
	}

	if f, err := os.Open("/var/lib/dpkg/status"); err == nil {
		count := 0
		scanner := bufio.NewScanner(f)
		for scanner.Scan() {
			if strings.HasPrefix(scanner.Text(), "Package: ") {
				count++
			}
		}
		f.Close()
		if count > 0 {
			parts = append(parts, fmt.Sprintf("%d (dpkg)", count))
		}
	}

	if f, err := os.Open("/lib/apk/db/installed"); err == nil {
		count := 0
		scanner := bufio.NewScanner(f)
		for scanner.Scan() {
			if strings.HasPrefix(scanner.Text(), "P:") {
				count++
			}
		}
		f.Close()
		if count > 0 {
			parts = append(parts, fmt.Sprintf("%d (apk)", count))
		}
	}

	flatpak := 0
	if dirs, err := os.ReadDir("/var/lib/flatpak/app"); err == nil {
		flatpak += len(dirs)
	}
	if home, err := os.UserHomeDir(); err == nil {
		if dirs, err := os.ReadDir(filepath.Join(home, ".local/share/flatpak/app")); err == nil {
			flatpak += len(dirs)
		}
	}
	if flatpak > 0 {
		parts = append(parts, fmt.Sprintf("%d (flatpak)", flatpak))
	}

	if dirs, err := os.ReadDir("/var/lib/snapd/snaps"); err == nil {
		count := 0
		for _, d := range dirs {
			if strings.HasSuffix(d.Name(), ".snap") {
				count++
			}
		}
		if count > 0 {
			parts = append(parts, fmt.Sprintf("%d (snap)", count))
		}
	}

	if _, err := exec.LookPath("rpm"); err == nil {
		out, err := exec.Command("rpm", "-qa").Output()
		if err == nil {
			lines := bytes.Split(bytes.TrimSpace(out), []byte("\n"))
			if len(lines) > 0 && len(lines[0]) > 0 {
				parts = append(parts, fmt.Sprintf("%d (rpm)", len(lines)))
			}
		}
	}

	if swDirs, err := os.ReadDir("/run/current-system/sw/bin"); err == nil {
		if len(swDirs) > 0 {
			parts = append(parts, fmt.Sprintf("%d (nix)", len(swDirs)))
		}
	} else if dirs, err := os.ReadDir("/nix/store"); err == nil && len(dirs) > 0 {
		parts = append(parts, fmt.Sprintf("%d (nix)", len(dirs)))
	}

	if _, err := exec.LookPath("xbps-query"); err == nil {
		out, err := exec.Command("xbps-query", "-l").Output()
		if err == nil {
			lines := bytes.Split(bytes.TrimSpace(out), []byte("\n"))
			if len(lines) > 0 && len(lines[0]) > 0 {
				parts = append(parts, fmt.Sprintf("%d (xbps)", len(lines)))
			}
		}
	}

	home, err := os.UserHomeDir()
	if err == nil {
		steamCount := 0
		vdfPath := filepath.Join(home, ".local/share/Steam/steamapps/libraryfolders.vdf")
		var steamPaths []string
		if f, err := os.Open(vdfPath); err == nil {
			scanner := bufio.NewScanner(f)
			for scanner.Scan() {
				line := scanner.Text()
				fields := strings.Split(line, "\"")
				if len(fields) >= 5 && fields[1] == "path" {
					steamPaths = append(steamPaths, fields[3])
				}
			}
			f.Close()
		}
		if len(steamPaths) > 0 {
			for _, p := range steamPaths {
				if dirs, err := os.ReadDir(filepath.Join(p, "steamapps")); err == nil {
					for _, d := range dirs {
						if !d.IsDir() && strings.HasSuffix(d.Name(), ".acf") {
							steamCount++
						}
					}
				}
			}
		} else {
			if dirs, err := os.ReadDir(filepath.Join(home, ".local/share/Steam/steamapps")); err == nil {
				for _, d := range dirs {
					if !d.IsDir() && strings.HasSuffix(d.Name(), ".acf") {
						steamCount++
					}
				}
			}
		}
		if steamCount > 0 {
			parts = append(parts, fmt.Sprintf("%d (steam)", steamCount))
		}
	}

	if len(parts) == 0 {
		return ""
	}
	return strings.Join(parts, ", ")
}

func formatBytes(b uint64) string {
	const unit = 1024
	if b < unit {
		return fmt.Sprintf("%d B", b)
	}
	div, exp := uint64(unit), 0
	for n := b / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}
	val := float64(b) / float64(div)
	units := []string{"KB", "MB", "GB", "TB", "PB"}
	if exp >= len(units) {
		return fmt.Sprintf("%.1f B", val)
	}
	if val == float64(int(val)) {
		return fmt.Sprintf("%d %s", int(val), units[exp])
	}
	return fmt.Sprintf("%.1f %s", val, units[exp])
}

func getMountsMap() map[string]string {
	f, err := os.Open("/proc/mounts")
	if err != nil {
		return nil
	}
	defer f.Close()
	mounts := make(map[string]string)
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		fields := strings.Fields(scanner.Text())
		if len(fields) < 3 || !strings.HasPrefix(fields[0], "/dev/") {
			continue
		}
		resolved, err := filepath.EvalSymlinks(fields[0])
		if err != nil {
			resolved = fields[0]
		}
		devName := filepath.Base(resolved)
		target := fields[1]
		if existing, ok := mounts[devName]; ok {
			if target == "/" || (existing != "/" && len(target) < len(existing)) {
				mounts[devName] = target
			}
		} else {
			mounts[devName] = target
		}
	}
	return mounts
}

func decodeUdevValue(val string) string {
	var sb strings.Builder
	for i := 0; i < len(val); {
		if i+3 < len(val) && val[i] == '\\' && val[i+1] == 'x' {
			if n, err := strconv.ParseUint(val[i+2:i+4], 16, 8); err == nil {
				sb.WriteByte(byte(n))
				i += 4
				continue
			}
		}
		sb.WriteByte(val[i])
		i++
	}
	return sb.String()
}

func parseUdevData(major, minor int) (fsType, fsLabel, partName, fsUsage string) {
	path := fmt.Sprintf("/run/udev/data/b%d:%d", major, minor)
	data, err := os.ReadFile(path)
	if err != nil {
		return
	}
	scanner := bufio.NewScanner(bytes.NewReader(data))
	for scanner.Scan() {
		line := scanner.Text()
		if !strings.HasPrefix(line, "E:") {
			continue
		}
		kv := strings.SplitN(line[2:], "=", 2)
		if len(kv) != 2 {
			continue
		}
		switch kv[0] {
		case "ID_FS_TYPE":
			fsType = kv[1]
		case "ID_FS_LABEL":
			fsLabel = decodeUdevValue(kv[1])
		case "ID_PART_ENTRY_NAME":
			partName = decodeUdevValue(kv[1])
		case "ID_FS_USAGE":
			fsUsage = kv[1]
		}
	}
	return
}

func getStorage() string {
	mounts := getMountsMap()
	f, err := os.Open("/proc/partitions")
	if err != nil {
		return ""
	}
	defer f.Close()

	var lines []string
	scanner := bufio.NewScanner(f)
	scanner.Scan()
	scanner.Scan()

	for scanner.Scan() {
		fields := strings.Fields(scanner.Text())
		if len(fields) < 4 {
			continue
		}
		major, _ := strconv.Atoi(fields[0])
		minor, _ := strconv.Atoi(fields[1])
		blocks, _ := strconv.ParseUint(fields[2], 10, 64)
		name := fields[3]

		if strings.HasPrefix(name, "loop") || strings.HasPrefix(name, "ram") {
			continue
		}

		fsType, fsLabel, partName, fsUsage := parseUdevData(major, minor)
		if fsUsage != "filesystem" && fsType == "" {
			continue
		}
		if fsType == "swap" {
			continue
		}

		totalSize := blocks * 1024
		if mountPoint, ok := mounts[name]; ok {
			var stat syscall.Statfs_t
			if err := syscall.Statfs(mountPoint, &stat); err == nil {
				total, free := getMountSpace(&stat)
				used := total - free
				pct := 0.0
				if total > 0 {
					pct = float64(used) * 100.0 / float64(total)
				}
				lines = append(lines, fmt.Sprintf("%s (%s): %s / %s (%.0f%%)", mountPoint, fsType, formatBytes(used), formatBytes(total), pct))
			} else {
				lines = append(lines, fmt.Sprintf("%s (%s): %s (mounted)", mountPoint, fsType, formatBytes(totalSize)))
			}
		} else {
			label := fsLabel
			if label == "" {
				label = partName
			}
			if label == "" {
				label = name
			}
			lines = append(lines, fmt.Sprintf("unmounted %s (%s): %s", label, fsType, formatBytes(totalSize)))
		}
	}
	if len(lines) == 0 {
		return ""
	}
	return strings.Join(lines, "\n")
}

func parseEDID(data []byte) (name string, hRes, vRes, refresh, inches int) {
	if len(data) < 128 {
		return
	}
	for _, offset := range []int{54, 72, 90, 108} {
		if offset+18 > len(data) {
			continue
		}
		desc := data[offset : offset+18]
		if desc[0] == 0 && desc[1] == 0 && desc[2] == 0 && desc[3] == 0xfc {
			nameBytes := desc[5:18]
			name = strings.TrimSpace(strings.TrimRight(string(nameBytes), "\n\r\x00"))
		}
	}
	wCm := int(data[21])
	hCm := int(data[22])
	if wCm > 0 || hCm > 0 {
		diag := math.Sqrt(float64(wCm*wCm+hCm*hCm)) / 2.54
		inches = int(math.Round(diag))
	}
	pref := data[54:72]
	pixelClock := int(pref[0]) | (int(pref[1]) << 8)
	activeH := int(pref[2]) | ((int(pref[4]) & 0xf0) << 4)
	blankH := int(pref[3]) | ((int(pref[4]) & 0x0f) << 8)
	activeV := int(pref[5]) | ((int(pref[7]) & 0xf0) << 4)
	blankV := int(pref[6]) | ((int(pref[7]) & 0x0f) << 8)
	hTotal := activeH + blankH
	vTotal := activeV + blankV
	if hTotal > 0 && vTotal > 0 && pixelClock > 0 {
		hRes = activeH
		vRes = activeV
		refresh = int(math.Round(float64(pixelClock) * 10000.0 / float64(hTotal*vTotal)))
	}
	return
}

func getMonitors() string {
	dirs, err := filepath.Glob("/sys/class/drm/card*-*")
	if err != nil {
		return ""
	}
	var lines []string
	for _, d := range dirs {
		statusData, err := os.ReadFile(filepath.Join(d, "status"))
		if err != nil || strings.TrimSpace(string(statusData)) != "connected" {
			continue
		}
		edidData, err := os.ReadFile(filepath.Join(d, "edid"))
		if err != nil || len(edidData) < 128 {
			continue
		}
		name, hRes, vRes, refresh, inches := parseEDID(edidData)
		if hRes == 0 || vRes == 0 {
			continue
		}
		var parts []string
		if name != "" {
			parts = append(parts, name+":")
		}
		parts = append(parts, fmt.Sprintf("%dx%d", hRes, vRes))
		if refresh > 0 {
			parts = append(parts, fmt.Sprintf("@ %dHz", refresh))
		}
		if inches > 0 {
			parts = append(parts, fmt.Sprintf("(%d\")", inches))
		}
		lines = append(lines, strings.Join(parts, " "))
	}
	if len(lines) == 0 {
		return ""
	}
	return strings.Join(lines, "\n")
}

func getAudio() string {
	audioServer := "ALSA"
	procDirs, err := os.ReadDir("/proc")
	if err == nil {
		for _, d := range procDirs {
			if !d.IsDir() {
				continue
			}
			if _, err := strconv.Atoi(d.Name()); err != nil {
				continue
			}
			commData, err := os.ReadFile(filepath.Join("/proc", d.Name(), "comm"))
			if err == nil {
				comm := strings.TrimSpace(string(commData))
				if comm == "pipewire" {
					audioServer = "PipeWire"
					break
				} else if comm == "pulseaudio" {
					audioServer = "PulseAudio"
				}
			}
		}
	}

	f, err := os.Open("/proc/asound/cards")
	var cards []string
	if err == nil {
		scanner := bufio.NewScanner(f)
		for scanner.Scan() {
			line := scanner.Text()
			trimmed := strings.TrimSpace(line)
			if trimmed == "" {
				continue
			}
			if line[0] == ' ' || (line[0] >= '0' && line[0] <= '9') {
				parts := strings.SplitN(trimmed, ":", 2)
				if len(parts) >= 2 {
					descPart := strings.TrimSpace(parts[1])
					subParts := strings.SplitN(descPart, " - ", 2)
					cardName := ""
					if len(subParts) >= 2 {
						cardName = strings.TrimSpace(subParts[1])
					} else {
						cardName = descPart
					}
					if cardName != "" {
						cards = append(cards, cardName)
					}
				}
			}
		}
		f.Close()
	}

	if len(cards) > 0 {
		return fmt.Sprintf("%s (%s)", audioServer, strings.Join(cards, ", "))
	}
	return audioServer
}

func getLocalIP() string {
	ifaces, err := net.Interfaces()
	if err != nil {
		return ""
	}
	for _, iface := range ifaces {
		if (iface.Flags&net.FlagLoopback) != 0 || (iface.Flags&net.FlagUp) == 0 {
			continue
		}
		addrs, err := iface.Addrs()
		if err != nil {
			continue
		}
		for _, addr := range addrs {
			if ipnet, ok := addr.(*net.IPNet); ok {
				if ip4 := ipnet.IP.To4(); ip4 != nil {
					return ip4.String()
				}
			}
		}
	}
	return ""
}

func getBattery() string {
	files, err := os.ReadDir("/sys/class/power_supply")
	if err != nil {
		return ""
	}
	var infos []string
	for _, f := range files {
		typeBytes, err := os.ReadFile(filepath.Join("/sys/class/power_supply", f.Name(), "type"))
		if err != nil || strings.TrimSpace(string(typeBytes)) != "Battery" {
			continue
		}
		capBytes, err := os.ReadFile(filepath.Join("/sys/class/power_supply", f.Name(), "capacity"))
		if err != nil {
			continue
		}
		capacity := strings.TrimSpace(string(capBytes))
		status := "Unknown"
		if statusBytes, err := os.ReadFile(filepath.Join("/sys/class/power_supply", f.Name(), "status")); err == nil {
			status = strings.TrimSpace(string(statusBytes))
		}
		infos = append(infos, fmt.Sprintf("%s%% [%s]", capacity, status))
	}
	if len(infos) == 0 {
		return ""
	}
	return strings.Join(infos, ", ")
}

func getPowerAdapter() string {
	files, err := os.ReadDir("/sys/class/power_supply")
	if err != nil {
		return ""
	}
	var infos []string
	for _, f := range files {
		typeBytes, err := os.ReadFile(filepath.Join("/sys/class/power_supply", f.Name(), "type"))
		if err != nil || strings.TrimSpace(string(typeBytes)) != "Mains" {
			continue
		}
		onlineBytes, err := os.ReadFile(filepath.Join("/sys/class/power_supply", f.Name(), "online"))
		if err != nil {
			continue
		}
		status := "offline"
		if strings.TrimSpace(string(onlineBytes)) == "1" {
			status = "online"
		}
		infos = append(infos, fmt.Sprintf("%s [%s]", f.Name(), status))
	}
	if len(infos) == 0 {
		return ""
	}
	return strings.Join(infos, ", ")
}

func getBluetooth() string {
	btRunning := false
	procDirs, err := os.ReadDir("/proc")
	if err == nil {
		for _, d := range procDirs {
			if !d.IsDir() {
				continue
			}
			if _, err := strconv.Atoi(d.Name()); err != nil {
				continue
			}
			commData, err := os.ReadFile(filepath.Join("/proc", d.Name(), "comm"))
			if err == nil && strings.TrimSpace(string(commData)) == "bluetoothd" {
				btRunning = true
				break
			}
		}
	}
	if !btRunning {
		return ""
	}

	adapters := 0
	hciDirs, err := os.ReadDir("/sys/class/bluetooth")
	if err == nil {
		for _, d := range hciDirs {
			if strings.HasPrefix(d.Name(), "hci") {
				adapters++
			}
		}
	}
	if adapters == 0 {
		return "enabled (no adapter)"
	}

	connected := 0
	for i := 0; i < adapters; i++ {
		hciDir := fmt.Sprintf("/sys/class/bluetooth/hci%d", i)
		subDirs, err := os.ReadDir(hciDir)
		if err == nil {
			for _, sd := range subDirs {
				if sd.IsDir() && strings.Contains(sd.Name(), ":") {
					connected++
				}
			}
		}
	}
	if connected > 0 {
		return fmt.Sprintf("enabled (%d connected)", connected)
	}
	return "enabled"
}

func getProcessInfo(pid int) (string, int) {
	data, err := os.ReadFile(fmt.Sprintf("/proc/%d/status", pid))
	if err != nil {
		return "", 0
	}
	name := ""
	ppid := 0
	scanner := bufio.NewScanner(bytes.NewReader(data))
	for scanner.Scan() {
		line := scanner.Text()
		if strings.HasPrefix(line, "Name:") {
			name = strings.TrimSpace(line[5:])
		} else if strings.HasPrefix(line, "PPid:") {
			fields := strings.Fields(line)
			if len(fields) >= 2 {
				ppid, _ = strconv.Atoi(fields[1])
			}
		}
	}
	return name, ppid
}

func getTerminal() string {
	if termProg := os.Getenv("TERM_PROGRAM"); termProg != "" {
		switch strings.ToLower(termProg) {
		case "vscode":
			return "VS Code"
		case "apple_terminal":
			return "Apple Terminal"
		case "iterm.app":
			return "iTerm2"
		default:
			return termProg
		}
	}
	if os.Getenv("KITTY_PID") != "" {
		return "kitty"
	}
	if os.Getenv("ALACRITTY_LOG") != "" {
		return "alacritty"
	}

	pid := os.Getppid()
	seen := map[int]bool{}
	skip := map[string]bool{
		"bash": true, "zsh": true, "fish": true, "sh": true, "dash": true,
		"ksh": true, "tcsh": true, "sudo": true, "su": true, "go": true,
		"language_server": true,
	}
	for pid > 1 && !seen[pid] {
		seen[pid] = true
		name, ppid := getProcessInfo(pid)
		if name == "" {
			break
		}
		nameLower := strings.ToLower(name)
		if !skip[nameLower] {
			switch nameLower {
			case "gnome-terminal-", "gnome-terminal-server":
				return "gnome-terminal"
			case "wezterm-gui", "wezterm-muxer":
				return "wezterm"
			case "antigravity-ide":
				return "Antigravity IDE"
			default:
				return name
			}
		}
		pid = ppid
	}
	if term := os.Getenv("TERM"); term != "" {
		return term
	}
	return ""
}

func getLoginManager() string {
	knownDMs := map[string]string{
		"sddm": "sddm", "gdm": "gdm", "gdm3": "gdm",
		"lightdm": "lightdm", "ly": "ly", "lxdm": "lxdm", "greetd": "greetd",
	}
	dmName := ""
	procDirs, err := os.ReadDir("/proc")
	if err == nil {
		for _, d := range procDirs {
			if !d.IsDir() {
				continue
			}
			if _, err := strconv.Atoi(d.Name()); err != nil {
				continue
			}
			commData, err := os.ReadFile(filepath.Join("/proc", d.Name(), "comm"))
			if err == nil {
				if name, ok := knownDMs[strings.TrimSpace(string(commData))]; ok {
					dmName = name
					break
				}
			}
		}
	}
	if dmName == "" {
		return ""
	}

	sessionType := os.Getenv("XDG_SESSION_TYPE")
	if sessionType == "" {
		if os.Getenv("WAYLAND_DISPLAY") != "" {
			sessionType = "Wayland"
		} else if os.Getenv("DISPLAY") != "" {
			sessionType = "X11"
		}
	} else {
		switch strings.ToLower(sessionType) {
		case "wayland":
			sessionType = "Wayland"
		case "x11":
			sessionType = "X11"
		}
	}

	if sessionType != "" {
		return fmt.Sprintf("%s (%s)", dmName, sessionType)
	}
	return dmName
}

func isUSBKeyboardOrMouse(deviceDir string) bool {
	files, err := os.ReadDir(deviceDir)
	if err != nil {
		return false
	}
	for _, f := range files {
		if !f.IsDir() || !strings.Contains(f.Name(), ":") {
			continue
		}
		intfDir := filepath.Join(deviceDir, f.Name())
		cls, _ := os.ReadFile(filepath.Join(intfDir, "bInterfaceClass"))
		sub, _ := os.ReadFile(filepath.Join(intfDir, "bInterfaceSubClass"))
		proto, _ := os.ReadFile(filepath.Join(intfDir, "bInterfaceProtocol"))
		if strings.TrimSpace(string(cls)) == "03" &&
			strings.TrimSpace(string(sub)) == "01" {
			p := strings.TrimSpace(string(proto))
			if p == "01" || p == "02" {
				return true
			}
		}
	}
	return false
}

func getGamepad() string {
	gamepadVendors := map[string]bool{
		"054c": true, "045e": true, "28de": true, "057e": true,
		"0f0d": true, "0e6f": true, "1532": true, "1b1c": true,
		"24c6": true, "11c0": true,
	}

	seen := map[string]bool{}
	var gamepads []string

	if usbDirs, err := os.ReadDir("/sys/bus/usb/devices"); err == nil {
		for _, d := range usbDirs {
			deviceDir := filepath.Join("/sys/bus/usb/devices", d.Name())
			vendorBytes, err := os.ReadFile(filepath.Join(deviceDir, "idVendor"))
			if err != nil {
				continue
			}
			productBytes, err := os.ReadFile(filepath.Join(deviceDir, "idProduct"))
			if err != nil {
				continue
			}
			vendor := strings.ToLower(strings.TrimSpace(string(vendorBytes)))
			product := strings.ToLower(strings.TrimSpace(string(productBytes)))
			if !gamepadVendors[vendor] || isUSBKeyboardOrMouse(deviceDir) {
				continue
			}
			prodBytes, _ := os.ReadFile(filepath.Join(deviceDir, "product"))
			fullName := strings.TrimSpace(string(prodBytes))
			if fullName == "" {
				fullName = fmt.Sprintf("Gamepad (%s:%s)", vendor, product)
			}
			if vendor == "054c" && (product == "09cc" || product == "05c4") {
				fullName = "Sony DualShock 4"
			} else if vendor == "054c" && product == "0ce6" {
				fullName = "Sony DualSense"
			} else if vendor == "28de" && (product == "1304" || product == "1142") {
				fullName = "Steam Controller (wireless)"
			} else if vendor == "28de" && (product == "1102" || product == "1302") {
				fullName = "Steam Controller (wired)"
			}
			if !seen[fullName] {
				seen[fullName] = true
				gamepads = append(gamepads, fullName)
			}
		}
	}

	if hidDirs, err := os.ReadDir("/sys/class/hidraw"); err == nil {
		for _, d := range hidDirs {
			ueventBytes, err := os.ReadFile(filepath.Join("/sys/class/hidraw", d.Name(), "device/uevent"))
			if err != nil {
				continue
			}
			var hidID, hidName string
			for _, line := range strings.Split(string(ueventBytes), "\n") {
				if strings.HasPrefix(line, "HID_ID=") {
					hidID = strings.TrimPrefix(line, "HID_ID=")
				} else if strings.HasPrefix(line, "HID_NAME=") {
					hidName = strings.TrimPrefix(line, "HID_NAME=")
				}
			}
			if hidID == "" {
				continue
			}
			parts := strings.Split(hidID, ":")
			if len(parts) < 3 {
				continue
			}
			vendor := strings.ToLower(parts[1])
			product := strings.ToLower(parts[2])
			if len(vendor) >= 4 {
				vendor = vendor[len(vendor)-4:]
			}
			if len(product) >= 4 {
				product = product[len(product)-4:]
			}
			if !gamepadVendors[vendor] {
				continue
			}
			nameLower := strings.ToLower(hidName)
			if strings.Contains(nameLower, "keyboard") || strings.Contains(nameLower, "mouse") ||
				strings.Contains(nameLower, "trackpad") || strings.Contains(nameLower, "touchpad") {
				continue
			}
			fullName := hidName
			if fullName == "" {
				fullName = fmt.Sprintf("Gamepad (%s:%s)", vendor, product)
			}
			if vendor == "054c" && (product == "09cc" || product == "05c4") {
				fullName = "Sony DualShock 4"
			} else if vendor == "054c" && product == "0ce6" {
				fullName = "Sony DualSense"
			}
			if !seen[fullName] {
				seen[fullName] = true
				gamepads = append(gamepads, fullName)
			}
		}
	}

	if len(gamepads) == 0 {
		return ""
	}
	return strings.Join(gamepads, ", ")
}
