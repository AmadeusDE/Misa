package kittygfx

import "strconv"

const maxReplyMessage = 200

// Reply builds the terminal's answer to cmd: i=<image id>, I=<image number>
// and p=<placement id> when set, then the code and an optional sanitized message. A quiet level of 1 suppresses success
// replies and 2 suppresses every reply, in which case Reply returns nil.
func Reply(cmd Command, code, message string) []byte {
	if cmd.Quiet >= 2 {
		return nil
	}
	if code == "" {
		code = "OK"
	}
	if cmd.Quiet == 1 && code == "OK" {
		return nil
	}
	out := make([]byte, 0, len(apcStart)+len(st)+len(code)+len(message)+24)
	out = append(out, apcStart...)
	if cmd.ID != 0 {
		out = append(out, 'i', '=')
		out = strconv.AppendUint(out, uint64(cmd.ID), 10)
	}
	if cmd.Number != 0 {
		if cmd.ID != 0 {
			out = append(out, ',')
		}
		out = append(out, 'I', '=')
		out = strconv.AppendUint(out, uint64(cmd.Number), 10)
	}
	if cmd.PlacementID != 0 {
		if cmd.ID != 0 || cmd.Number != 0 {
			out = append(out, ',')
		}
		out = append(out, 'p', '=')
		out = strconv.AppendUint(out, uint64(cmd.PlacementID), 10)
	}
	out = append(out, ';')
	out = append(out, code...)
	if message = printable(message); message != "" {
		out = append(out, ':')
		out = append(out, message...)
	}
	return append(out, st...)
}

func printable(message string) string {
	if len(message) > maxReplyMessage {
		message = message[:maxReplyMessage]
	}
	clean := make([]byte, 0, len(message))
	for i := 0; i < len(message); i++ {
		if c := message[i]; c >= 0x20 && c < 0x7f && c != ';' {
			clean = append(clean, c)
		}
	}
	return string(clean)
}
