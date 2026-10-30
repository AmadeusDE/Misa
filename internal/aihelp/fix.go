package aihelp

import (
	"errors"
	"fmt"
	"strings"
)

const fixSystem = "You are a command correction assistant for Unix terminals. " +
	"You have tools for fetching man pages, TLDR pages and Arch Wiki content: use them to check the correct syntax and flags when needed. " +
	"Given a failed shell command, its exit status and optionally its error output, diagnose the problem and provide the fix.\n\n" +
	"Your FINAL response (after any tool calls) MUST follow this EXACT format:\n" +
	"Line 1: The corrected command (ONLY the command: no backticks, no markdown, no explanation)\n" +
	"Line 2: Empty line\n" +
	"Line 3+: A brief explanation of what was wrong and what you changed.\n\n" +
	"If the command looks correct and you cannot determine a fix, respond with the original command on line 1 " +
	"and explain that the command appears correct on line 3+."

// maxErrorOutput is how much of a failed command's output is sent: the end, where errors are.
const maxErrorOutput = 4000

func fixPrompt(cmd, output string, exit int) string {
	p := "Failed command: " + cmd
	if exit != 0 {
		p += fmt.Sprintf("\nExit status: %d", exit)
	}
	if output = strings.TrimSpace(output); output != "" {
		if len(output) > maxErrorOutput {
			output = "..." + output[len(output)-maxErrorOutput:]
		}
		p += "\n\nError output:\n" + output
	}
	return p
}

// ParseFix splits a --fix answer into the corrected command (the first line, without markdown) and the
// explanation (what follows the first blank line).
func ParseFix(response string) (cmd, explanation string, err error) {
	lines := strings.Split(strings.TrimSpace(response), "\n")
	i := 0
	for i < len(lines) && (strings.TrimSpace(lines[i]) == "" || strings.HasPrefix(strings.TrimSpace(lines[i]), "```")) {
		i++ // a fence the model added despite the instructions
	}
	if i == len(lines) {
		return "", "", errors.New("empty response from model")
	}
	cmd = strings.TrimSpace(strings.Trim(strings.TrimSpace(lines[i]), "`"))
	if cmd == "" {
		return "", "", errors.New("could not parse a corrected command from the response")
	}
	rest := lines[i+1:]
	for len(rest) > 0 && strings.HasPrefix(strings.TrimSpace(rest[0]), "```") {
		rest = rest[1:] // closing fence
	}
	if len(rest) > 0 && strings.TrimSpace(rest[0]) == "" {
		rest = rest[1:]
	}
	return cmd, strings.TrimSpace(strings.Join(rest, "\n")), nil
}
