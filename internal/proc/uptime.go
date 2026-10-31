package proc

import (
	"errors"
	"strconv"
	"strings"
	"time"
)

// ParseUptime parses /proc/uptime's "<seconds since boot> <idle>" line.
func ParseUptime(data []byte) (time.Duration, error) {
	fields := strings.Fields(string(data))
	if len(fields) == 0 {
		return 0, errors.New("proc: empty uptime")
	}
	secs, err := strconv.ParseFloat(fields[0], 64)
	if err != nil || secs < 0 {
		return 0, errors.New("proc: bad uptime " + strconv.Quote(fields[0]))
	}
	return time.Duration(secs * float64(time.Second)), nil
}
