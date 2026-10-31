//go:build unix

package builtins

import "golang.org/x/sys/unix"

func machineName() (string, error) {
	var u unix.Utsname
	if err := unix.Uname(&u); err != nil {
		return "", err
	}
	machine := make([]byte, 0, len(u.Machine))
	for _, b := range u.Machine {
		if b == 0 {
			break
		}
		machine = append(machine, byte(b))
	}
	return string(machine), nil
}
