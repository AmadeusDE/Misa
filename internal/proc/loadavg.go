package proc

import (
	"errors"
	"strconv"
	"strings"
)

// Loadavg is one /proc/loadavg line: the 1, 5 and 15 minute load averages plus
// the runnable/total task counts. A machine without the count field (some
// containers blank the file) has Running and Total set to -1.
type Loadavg struct {
	One, Five, Fifteen float64
	Running, Total     int
}

// ParseLoadavg parses /proc/loadavg's "0.52 0.58 0.59 2/1234 56789" line. The
// three averages are required; the trailing counts are best effort.
func ParseLoadavg(data []byte) (Loadavg, error) {
	fields := strings.Fields(string(data))
	if len(fields) < 3 {
		return Loadavg{}, errors.New("proc: bad loadavg")
	}
	avg := [3]float64{}
	for i := range avg {
		v, err := strconv.ParseFloat(fields[i], 64)
		if err != nil || v < 0 {
			return Loadavg{}, errors.New("proc: bad loadavg " + strconv.Quote(fields[i]))
		}
		avg[i] = v
	}
	l := Loadavg{One: avg[0], Five: avg[1], Fifteen: avg[2], Running: -1, Total: -1}
	if len(fields) > 3 {
		if running, total, ok := strings.Cut(fields[3], "/"); ok {
			r, errR := strconv.Atoi(running)
			t, errT := strconv.Atoi(total)
			if errR == nil && errT == nil && r >= 0 && t >= 0 {
				l.Running, l.Total = r, t
			}
		}
	}
	return l, nil
}
