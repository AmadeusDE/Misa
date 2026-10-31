//go:build linux

package builtins

import (
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

// sysfsCacheDir describes cpu0's caches. The kernel documents the attributes
// read below (level, type, size, ways_of_associativity, coherency_line_size);
// reading them is the only way to answer the LEVEL*_CACHE_* names without a C
// library. The linux build tag covers Android too.
const sysfsCacheDir = "/sys/devices/system/cpu/cpu0/cache"

// platformCacheInfo reads cpu0's cache geometry, keyed by getconf name. A cache
// the machine does not have gets no entry, so those names report "undefined";
// sysfs may be missing entirely (some containers, old Android kernels), which
// leaves the result nil.
func platformCacheInfo() map[string]string {
	dirs, err := filepath.Glob(filepath.Join(sysfsCacheDir, "index*"))
	if err != nil || len(dirs) == 0 {
		return nil
	}
	out := map[string]string{}
	for _, dir := range dirs {
		level, ok := sysfsInt(filepath.Join(dir, "level"))
		if !ok {
			continue
		}
		kind, ok := sysfsLine(filepath.Join(dir, "type"))
		if !ok {
			continue
		}
		var prefix string
		switch {
		case level == 1 && kind == "Data":
			prefix = "LEVEL1_DCACHE_"
		case level == 1 && kind == "Instruction":
			prefix = "LEVEL1_ICACHE_"
		case level >= 2:
			prefix = "LEVEL" + strconv.FormatInt(level, 10) + "_CACHE_"
		default:
			continue
		}
		if size, ok := sysfsLine(filepath.Join(dir, "size")); ok {
			if n, ok := parseSysfsSize(size); ok {
				out[prefix+"SIZE"] = strconv.FormatInt(n, 10)
			}
		}
		if n, ok := sysfsInt(filepath.Join(dir, "ways_of_associativity")); ok && n > 0 {
			out[prefix+"ASSOC"] = strconv.FormatInt(n, 10)
		}
		if n, ok := sysfsInt(filepath.Join(dir, "coherency_line_size")); ok && n > 0 {
			out[prefix+"LINESIZE"] = strconv.FormatInt(n, 10)
		}
	}
	return out
}

// parseSysfsSize parses a cache size attribute: "48K", "2048K", "1M" or plain
// bytes.
func parseSysfsSize(s string) (int64, bool) {
	mult := int64(1)
	switch {
	case strings.HasSuffix(s, "K"):
		mult, s = 1024, strings.TrimSuffix(s, "K")
	case strings.HasSuffix(s, "M"):
		mult, s = 1024*1024, strings.TrimSuffix(s, "M")
	case strings.HasSuffix(s, "G"):
		mult, s = 1024*1024*1024, strings.TrimSuffix(s, "G")
	}
	n, err := strconv.ParseInt(strings.TrimSpace(s), 10, 64)
	if err != nil || n <= 0 {
		return 0, false
	}
	return n * mult, true
}

func sysfsLine(path string) (string, bool) {
	b, err := os.ReadFile(path)
	if err != nil {
		return "", false
	}
	return strings.TrimSpace(string(b)), true
}

func sysfsInt(path string) (int64, bool) {
	s, ok := sysfsLine(path)
	if !ok {
		return 0, false
	}
	n, err := strconv.ParseInt(s, 10, 64)
	if err != nil {
		return 0, false
	}
	return n, true
}
