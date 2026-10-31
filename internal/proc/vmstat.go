package proc

import (
	"errors"
	"strconv"
	"strings"
)

// Vmstat is the subset of /proc/vmstat the vmstat builtin needs. The pg*
// counters are pages, the pswp* counters are pages too; nothing is converted to
// bytes here, the caller applies the page size.
type Vmstat struct {
	PGPGIn, PGPGOut uint64 // pages read from / written to block devices
	PSWPIn, PSWPOut uint64 // pages swapped in / out

	// Missing names the counters the kernel did not report, in file order of
	// the fields above, so callers can print them as unavailable instead of
	// inventing zeroes.
	Missing []string
}

// ParseVmstat parses a /proc/vmstat file. A file with none of the four counters
// reports an error; individually absent counters are recorded in Missing.
func ParseVmstat(data []byte) (Vmstat, error) {
	var (
		v        Vmstat
		pgpgIn   bool
		pgpgOut  bool
		pswpIn   bool
		pswpOut  bool
		foundAny bool
	)
	for _, line := range strings.Split(string(data), "\n") {
		fields := strings.Fields(line)
		if len(fields) != 2 {
			continue
		}
		n, err := strconv.ParseUint(fields[1], 10, 64)
		if err != nil {
			continue
		}
		switch fields[0] {
		case "pgpgin":
			v.PGPGIn, pgpgIn, foundAny = n, true, true
		case "pgpgout":
			v.PGPGOut, pgpgOut, foundAny = n, true, true
		case "pswpin":
			v.PSWPIn, pswpIn, foundAny = n, true, true
		case "pswpout":
			v.PSWPOut, pswpOut, foundAny = n, true, true
		}
	}
	if !foundAny {
		return Vmstat{}, errors.New("proc: no virtual memory statistics")
	}
	for _, miss := range []struct {
		name string
		seen bool
	}{
		{"pgpgin", pgpgIn}, {"pgpgout", pgpgOut}, {"pswpin", pswpIn}, {"pswpout", pswpOut},
	} {
		if !miss.seen {
			v.Missing = append(v.Missing, miss.name)
		}
	}
	return v, nil
}
