//go:build !windows

package procmon

import (
	"bufio"
	"os"
	"os/user"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

// procRoot is the procfs root. Tests point it at a fake tree.
var procRoot = "/proc"

func procFile(elem ...string) string {
	return filepath.Join(append([]string{procRoot}, elem...)...)
}

var uidCache = map[uint32]string{}

func LookupUser(uid uint32) string {
	if name, ok := uidCache[uid]; ok {
		return name
	}
	name := strconv.Itoa(int(uid))
	if u, err := user.LookupId(name); err == nil {
		name = u.Username
	}
	uidCache[uid] = name
	return name
}

func ReadUptime() float64 {
	data, err := os.ReadFile(procFile("uptime"))
	if err != nil {
		return 0
	}
	fields := strings.Fields(string(data))
	if len(fields) < 1 {
		return 0
	}
	v, _ := strconv.ParseFloat(fields[0], 64)
	return v
}

func ReadLoadAvg() [3]float64 {
	var la [3]float64
	data, err := os.ReadFile(procFile("loadavg"))
	if err != nil {
		return la
	}
	fields := strings.Fields(string(data))
	if len(fields) < 3 {
		return la
	}
	la[0], _ = strconv.ParseFloat(fields[0], 64)
	la[1], _ = strconv.ParseFloat(fields[1], 64)
	la[2], _ = strconv.ParseFloat(fields[2], 64)
	return la
}

func parseCPULine(fields []string) (CPUTimes, bool) {
	if len(fields) < 5 {
		return CPUTimes{}, false
	}
	var ct CPUTimes
	ct.User, _ = strconv.ParseUint(fields[1], 10, 64)
	ct.Nice, _ = strconv.ParseUint(fields[2], 10, 64)
	ct.System, _ = strconv.ParseUint(fields[3], 10, 64)
	ct.Idle, _ = strconv.ParseUint(fields[4], 10, 64)
	if len(fields) > 5 {
		ct.IOWait, _ = strconv.ParseUint(fields[5], 10, 64)
	}
	if len(fields) > 6 {
		ct.IRQ, _ = strconv.ParseUint(fields[6], 10, 64)
	}
	if len(fields) > 7 {
		ct.SoftIRQ, _ = strconv.ParseUint(fields[7], 10, 64)
	}
	if len(fields) > 8 {
		ct.Steal, _ = strconv.ParseUint(fields[8], 10, 64)
	}
	return ct, true
}

// ReadCPUTimes reports whether /proc/stat could be read. EACCES and a missing
// file are unavailable (ok == false): callers show "—" and do not fail.
func ReadCPUTimes() (CPUTimes, []CPUTimes, bool) {
	f, err := os.Open(procFile("stat"))
	if err != nil {
		return CPUTimes{}, nil, false
	}
	defer f.Close()

	var all CPUTimes
	var cores []CPUTimes
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := scanner.Text()
		fields := strings.Fields(line)
		if len(fields) < 2 {
			continue
		}
		if fields[0] == "cpu" {
			all, _ = parseCPULine(fields)
		} else if strings.HasPrefix(fields[0], "cpu") {
			if ct, ok := parseCPULine(fields); ok {
				cores = append(cores, ct)
			}
		}
	}
	return all, cores, true
}

func ReadMemInfo() MemInfo {
	var mi MemInfo
	f, err := os.Open(procFile("meminfo"))
	if err != nil {
		return mi
	}
	defer f.Close()

	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := scanner.Text()
		parts := strings.SplitN(line, ":", 2)
		if len(parts) != 2 {
			continue
		}
		key := strings.TrimSpace(parts[0])
		valStr := strings.TrimSpace(parts[1])
		valStr = strings.TrimSuffix(valStr, " kB")
		val, _ := strconv.ParseUint(strings.TrimSpace(valStr), 10, 64)
		val *= 1024

		switch key {
		case "MemTotal":
			mi.Total = val
		case "MemFree":
			mi.Free = val
		case "MemAvailable":
			mi.Available = val
		case "Buffers":
			mi.Buffers = val
		case "Cached":
			mi.Cached = val
		case "SwapTotal":
			mi.SwapTotal = val
		case "SwapFree":
			mi.SwapFree = val
		case "Shmem":
			mi.Shmem = val
		case "SReclaimable":
			mi.SReclaimable = val
		}
	}
	return mi
}

func readProcess(pid int) (Process, bool) {
	dir := procFile(strconv.Itoa(pid))

	statData, err := os.ReadFile(filepath.Join(dir, "stat"))
	if err != nil {
		return Process{}, false
	}

	s := string(statData)
	openP := strings.IndexByte(s, '(')
	closeP := strings.LastIndexByte(s, ')')
	if openP < 0 || closeP < 0 || closeP <= openP {
		return Process{}, false
	}

	var p Process
	p.PID = pid
	p.Comm = s[openP+1 : closeP]

	rest := strings.Fields(s[closeP+2:])
	if len(rest) < 22 {
		return Process{}, false
	}

	p.State = rest[0][0]
	p.PPID, _ = strconv.Atoi(rest[1])
	p.TTY, _ = strconv.Atoi(rest[4])
	p.UTime, _ = strconv.ParseUint(rest[11], 10, 64)
	p.STime, _ = strconv.ParseUint(rest[12], 10, 64)
	p.Priority, _ = strconv.ParseInt(rest[15], 10, 64)
	p.Nice, _ = strconv.ParseInt(rest[16], 10, 64)
	p.Threads, _ = strconv.Atoi(rest[17])
	p.StartTime, _ = strconv.ParseUint(rest[19], 10, 64)
	p.Identity = p.StartTime
	p.VSize, _ = strconv.ParseUint(rest[20], 10, 64)
	rss, _ := strconv.ParseInt(rest[21], 10, 64)
	p.RSS = rss

	if statmData, err := os.ReadFile(filepath.Join(dir, "statm")); err == nil {
		fields := strings.Fields(string(statmData))
		if len(fields) >= 3 {
			shr, _ := strconv.ParseInt(fields[2], 10, 64)
			p.SHR = shr
		}
	}

	if statusData, err := os.ReadFile(filepath.Join(dir, "status")); err == nil {
		for _, line := range strings.Split(string(statusData), "\n") {
			if strings.HasPrefix(line, "Uid:") {
				fields := strings.Fields(line)
				if len(fields) >= 2 {
					uid, _ := strconv.ParseUint(fields[1], 10, 32)
					p.UID = uint32(uid)
				}
				break
			}
		}
	}

	if data, err := os.ReadFile(filepath.Join(dir, "cmdline")); err == nil {
		p.Cmdline = strings.ReplaceAll(strings.TrimRight(string(data), "\x00"), "\x00", " ")
	}
	if p.Cmdline == "" {
		p.Cmdline = "[" + p.Comm + "]"
	}

	p.User = LookupUser(p.UID)
	return p, true
}

func ReadAllProcs() []Process {
	entries, err := os.ReadDir(procRoot)
	if err != nil {
		return nil
	}
	var procs []Process
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		pid, err := strconv.Atoi(e.Name())
		if err != nil {
			continue
		}
		if p, ok := readProcess(pid); ok {
			procs = append(procs, p)
		}
	}
	return procs
}

func ReadNetDev() []NetDev {
	f, err := os.Open(procFile("net", "dev"))
	if err != nil {
		return nil
	}
	defer f.Close()

	var devs []NetDev
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := scanner.Text()
		parts := strings.SplitN(line, ":", 2)
		if len(parts) != 2 {
			continue
		}
		name := strings.TrimSpace(parts[0])
		if name == "lo" {
			continue
		}
		fields := strings.Fields(parts[1])
		if len(fields) < 10 {
			continue
		}
		rx, _ := strconv.ParseUint(fields[0], 10, 64)
		rxp, _ := strconv.ParseUint(fields[1], 10, 64)
		tx, _ := strconv.ParseUint(fields[8], 10, 64)
		txp, _ := strconv.ParseUint(fields[9], 10, 64)
		devs = append(devs, NetDev{Name: name, RxBytes: rx, TxBytes: tx, RxPkts: rxp, TxPkts: txp})
	}
	return devs
}

func ReadDiskStats() []DiskStat {
	f, err := os.Open(procFile("diskstats"))
	if err != nil {
		return nil
	}
	defer f.Close()

	var disks []DiskStat
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		fields := strings.Fields(scanner.Text())
		if len(fields) < 14 {
			continue
		}
		name := fields[2]
		if strings.HasPrefix(name, "loop") || strings.HasPrefix(name, "ram") ||
			strings.HasPrefix(name, "dm-") || strings.HasPrefix(name, "zram") {
			continue
		}
		readSectors, _ := strconv.ParseUint(fields[5], 10, 64)
		writeSectors, _ := strconv.ParseUint(fields[9], 10, 64)
		disks = append(disks, DiskStat{
			Name:       name,
			ReadBytes:  readSectors * 512,
			WriteBytes: writeSectors * 512,
		})
	}
	return disks
}

func enrichDiskMounts(ioDisks []DiskStat) []DiskStat {
	f, err := os.Open(procFile("mounts"))
	if err != nil {
		return ioDisks
	}
	defer f.Close()

	ioByName := map[string]*DiskStat{}
	for i := range ioDisks {
		ioByName[ioDisks[i].Name] = &ioDisks[i]
	}

	seen := map[string]bool{}
	var result []DiskStat
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		fields := strings.Fields(scanner.Text())
		if len(fields) < 2 {
			continue
		}
		dev := fields[0]
		mount := fields[1]
		if !strings.HasPrefix(dev, "/dev/") {
			continue
		}
		devName := filepath.Base(dev)
		if seen[devName] {
			continue
		}
		total, free, err := diskSpace(mount)
		if err != nil {
			continue
		}
		if total == 0 {
			continue
		}
		seen[devName] = true
		d := DiskStat{
			Name:  devName,
			Mount: mount,
			Total: total,
			Used:  total - free,
		}
		if io, ok := ioByName[devName]; ok {
			d.ReadBytes = io.ReadBytes
			d.WriteBytes = io.WriteBytes
		}
		result = append(result, d)
	}
	if len(result) == 0 {
		return ioDisks
	}
	return result
}

func TakeSnapshot() Snapshot {
	cpuAll, cpus, cpuOK := ReadCPUTimes()
	return Snapshot{
		SystemAvailable: true,
		Time:            time.Now(),
		Uptime:          ReadUptime(),
		LoadAvg:         ReadLoadAvg(),
		CPUAll:          cpuAll,
		CPUs:            cpus,
		CPUAvailable:    cpuOK,
		Mem:             ReadMemInfo(),
		Procs:           ReadAllProcs(),
		Net:             ReadNetDev(),
		Disks:           enrichDiskMounts(ReadDiskStats()),
	}
}

func TakeFullSnapshot() Snapshot {
	cpuAll, cpus, cpuOK := ReadCPUTimes()
	return Snapshot{
		SystemAvailable: true,
		Time:            time.Now(),
		Uptime:          ReadUptime(),
		LoadAvg:         ReadLoadAvg(),
		CPUAll:          cpuAll,
		CPUs:            cpus,
		CPUAvailable:    cpuOK,
		Mem:             ReadMemInfo(),
		Procs:           ReadAllProcs(),
		Net:             ReadNetDev(),
		Disks:           enrichDiskMounts(ReadDiskStats()),
	}
}
