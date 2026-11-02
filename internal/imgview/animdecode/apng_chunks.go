package animdecode

import (
	"bytes"
	"encoding/binary"
)

func apngAncillary(typ string, d []byte, ihdr [13]byte, palette []byte, plte, idat bool, ancillary *uint16) bool {
	var bit uint16
	switch typ {
	case "cHRM":
		bit = 1
	case "gAMA":
		bit = 2
	case "iCCP":
		bit = 4
	case "sBIT":
		bit = 8
	case "sRGB":
		bit = 16
	case "bKGD":
		bit = 32
	case "hIST":
		bit = 64
	case "pHYs":
		bit = 128
	case "tIME":
		bit = 256
	}
	if *ancillary&bit != 0 {
		return false
	}
	*ancillary |= bit
	if typ != "tIME" && idat {
		return false
	}
	if bit&31 != 0 && plte {
		return false
	}
	switch typ {
	case "cHRM":
		if len(d) != 32 {
			return false
		}
	case "gAMA":
		if len(d) != 4 || binary.BigEndian.Uint32(d) == 0 {
			return false
		}
	case "iCCP":
		z := bytes.IndexByte(d, 0)
		if z < 1 || z > 79 || z+2 > len(d) || d[z+1] != 0 {
			return false
		}
	case "sRGB":
		if len(d) != 1 || d[0] > 3 {
			return false
		}
	case "sBIT":
		n := 0
		switch ihdr[9] {
		case 0:
			n = 1
		case 2, 3:
			n = 3
		case 4:
			n = 2
		case 6:
			n = 4
		}
		if len(d) != n {
			return false
		}
		max := ihdr[8]
		if ihdr[9] == 3 {
			max = 8
		}
		for _, v := range d {
			if v == 0 || v > max {
				return false
			}
		}
	case "bKGD":
		switch ihdr[9] {
		case 0, 4:
			if len(d) != 2 {
				return false
			}
		case 2, 6:
			if len(d) != 6 {
				return false
			}
		case 3:
			if !plte || len(d) != 1 || int(d[0]) >= (len(palette)-12)/3 {
				return false
			}
		}
	case "hIST":
		if !plte || len(d) != 2*((len(palette)-12)/3) {
			return false
		}
	case "pHYs":
		if len(d) != 9 || d[8] > 1 {
			return false
		}
	case "tIME":
		if len(d) != 7 {
			return false
		}
	}
	return true
}
