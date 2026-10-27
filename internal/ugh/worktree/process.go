package worktree

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"syscall"
)

var procReadDir = os.ReadDir

// procRoot is where /proc is mounted (tests point it at a missing directory
// to act like a system without /proc).
var procRoot = "/proc"

// isPIDAlive checks if a process with the given PID is currently alive on the system.
func isPIDAlive(pid int) bool {
	if pid <= 0 {
		return false
	}
	err := syscall.Kill(pid, 0)
	if err == nil {
		return true
	}
	// EPERM means the process exists and is alive, but belongs to another user
	if errors.Is(err, syscall.EPERM) {
		return true
	}
	return false
}

// isGroupAlive reports whether any process of process group pgid exists.
func isGroupAlive(pgid int) bool {
	if pgid <= 1 {
		return false
	}
	err := syscall.Kill(-pgid, 0)
	return err == nil || errors.Is(err, syscall.EPERM)
}

// LiveGroups returns the groups of groups that still have a process.
func LiveGroups(groups []int) []int {
	var out []int
	for _, g := range groups {
		if isGroupAlive(g) {
			out = append(out, g)
		}
	}
	return out
}

// detectActive is DetectActiveProcesses for a worktree's metadata, plus its
// tracked process groups (reported by pgid). The groups are the check that
// works without /proc.
func detectActive(worktreeDir string, m Metadata) ([]int, error) {
	active, err := DetectActiveProcesses(worktreeDir, m.ActivePIDs, m.PIDStartTimes)
	if err != nil {
		return nil, err
	}
	seen := make(map[int]bool, len(active))
	for _, p := range active {
		seen[p] = true
	}
	for _, g := range LiveGroups(m.ActiveGroups) {
		if !seen[g] {
			active = append(active, g)
		}
	}
	sort.Ints(active)
	return active, nil
}

// getProcessStartTime returns the process start time (in clock ticks after boot)
// by inspecting /proc/<pid>/stat on Linux systems.
func getProcessStartTime(pid int) (uint64, error) {
	if pid <= 0 {
		return 0, errors.New("invalid pid")
	}
	data, err := os.ReadFile(filepath.Join(procRoot, strconv.Itoa(pid), "stat"))
	if err != nil {
		return 0, err
	}
	s := string(data)
	lastParen := strings.LastIndex(s, ")")
	if lastParen == -1 || lastParen+2 >= len(s) {
		return 0, errors.New("malformed /proc/stat")
	}
	fields := strings.Fields(s[lastParen+2:])
	// In /proc/[pid]/stat, index 19 after comm is starttime (field 22)
	if len(fields) <= 19 {
		return 0, errors.New("insufficient fields in /proc/stat")
	}
	startTime, err := strconv.ParseUint(fields[19], 10, 64)
	if err != nil {
		return 0, fmt.Errorf("parse starttime: %w", err)
	}
	return startTime, nil
}

// isPIDAliveWithIdentity checks if the PID is alive and matches the expected start time,
// preventing confusion caused by OS PID recycling.
func isPIDAliveWithIdentity(pid int, expectedStartTime uint64) bool {
	if !isPIDAlive(pid) {
		return false
	}
	if expectedStartTime == 0 {
		return true
	}
	st, err := getProcessStartTime(pid)
	if err != nil {
		return false
	}
	return st == expectedStartTime
}

// DetectActiveProcesses scans for active processes related to the worktree directory:
//  1. Validates tracked PIDs against live system processes using start time identity.
//  2. Scans /proc (Linux) to find any running process whose current working directory
//     is inside worktreeDir (excluding the current harness process itself). Without
//     /proc this step is skipped: only tracked processes and groups are seen there.
//
// Fails closed if the surviving process scan encounters an inspection error (e.g. /proc unreadable).
// Returns a sorted, deduplicated slice of live PIDs and an error if inspection failed.
func DetectActiveProcesses(worktreeDir string, trackedPIDs []int, startTimes ...map[int]uint64) ([]int, error) {
	if worktreeDir == "" {
		return nil, nil
	}
	cleanTarget := filepath.Clean(worktreeDir)
	activeMap := make(map[int]bool)

	var stMap map[int]uint64
	if len(startTimes) > 0 {
		stMap = startTimes[0]
	}

	// 1. Check explicitly tracked PIDs with start time identity
	for _, pid := range trackedPIDs {
		if pid > 0 && pid != os.Getpid() {
			var expected uint64
			if stMap != nil {
				expected = stMap[pid]
			}
			if isPIDAliveWithIdentity(pid, expected) {
				activeMap[pid] = true
			}
		}
	}

	// 2. Scan /proc on Linux systems; fail closed if /proc exists but cannot be read
	if fi, statErr := os.Stat(procRoot); statErr == nil && fi.IsDir() {
		procEntries, err := procReadDir(procRoot)
		if err != nil {
			return nil, fmt.Errorf("scan /proc: %w", err)
		}
		selfPid := os.Getpid()
		for _, entry := range procEntries {
			if !entry.IsDir() {
				continue
			}
			pid, parseErr := strconv.Atoi(entry.Name())
			if parseErr != nil || pid <= 0 || pid == selfPid {
				continue
			}
			cwdLink, readErr := os.Readlink(filepath.Join(procRoot, entry.Name(), "cwd"))
			if readErr != nil {
				continue
			}
			cleanCwd := filepath.Clean(cwdLink)
			if cleanCwd == cleanTarget || strings.HasPrefix(cleanCwd, cleanTarget+"/") {
				if isPIDAlive(pid) {
					activeMap[pid] = true
				}
			}
		}
	}

	out := make([]int, 0, len(activeMap))
	for pid := range activeMap {
		out = append(out, pid)
	}
	sort.Ints(out)
	return out, nil
}
