package builtins

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

const topCommandsSpec = "^n#(count)a(all)b(bash)z(zsh)m(misa)"

var topCommandsSpecParsed *Spec

func init() {
	var err error
	topCommandsSpecParsed, err = ParseSpec(topCommandsSpec)
	if err != nil {
		panic("builtins: bad top-commands spec: " + err.Error())
	}
	Register(Command{Name: "top-commands", Run: runTopCommands})
}

func runTopCommands(ctx *Context) int {
	flags, err := topCommandsSpecParsed.ParseArgs(ctx.Args)
	if err != nil {
		fmt.Fprintln(ctx.Stderr, "top-commands:", err)
		return 1
	}

	topN := int64(30)
	if n, ok := flags.Num("n"); ok && n > 0 {
		topN = n
	}

	onlyBash := flags.Bool("b")
	onlyZsh := flags.Bool("z")
	onlyMisa := flags.Bool("m")
	all := flags.Bool("a") || (!onlyBash && !onlyZsh && !onlyMisa)

	home, err := os.UserHomeDir()
	if err != nil {
		fmt.Fprintln(ctx.Stderr, "top-commands: cannot determine home directory")
		return 1
	}

	var commands []string
	total := 0

	if all || onlyBash {
		cmds := readBashHistory(filepath.Join(home, ".bash_history"))
		commands = append(commands, cmds...)
		total += len(cmds)
	}

	if all || onlyZsh {
		cmds := readZshHistory(filepath.Join(home, ".zsh_history"))
		commands = append(commands, cmds...)
		total += len(cmds)
	}

	if all || onlyMisa {
		cmds := readMisaHistory(filepath.Join(home, ".local", "share", "misa", "history"))
		commands = append(commands, cmds...)
		total += len(cmds)
	}

	if len(commands) == 0 {
		fmt.Fprintln(ctx.Stderr, "No history found.")
		return 1
	}

	extracted := make([]string, 0, len(commands))
	for _, cmd := range commands {
		if name := extractCommandName(cmd); name != "" {
			extracted = append(extracted, name)
		}
	}

	counts := map[string]int{}
	for _, name := range extracted {
		counts[name]++
	}

	type ranked struct {
		name  string
		count int
	}
	var ranked_list []ranked
	for name, count := range counts {
		ranked_list = append(ranked_list, ranked{name, count})
	}
	sort.Slice(ranked_list, func(i, j int) bool {
		if ranked_list[i].count != ranked_list[j].count {
			return ranked_list[i].count > ranked_list[j].count
		}
		return ranked_list[i].name < ranked_list[j].name
	})

	if int(topN) > len(ranked_list) {
		topN = int64(len(ranked_list))
	}

	fmt.Fprintf(ctx.Stdout, "\n  %-4s  %-20s  %7s  %s\n", "Rank", "Command", "Count", "Share")
	fmt.Fprintf(ctx.Stdout, "  %-4s  %-20s  %7s  %s\n", "----", "-------", "-----", "-----")
	for i := int64(0); i < topN; i++ {
		r := ranked_list[i]
		pct := float64(r.count) / float64(total) * 100
		fmt.Fprintf(ctx.Stdout, "  %-4d  %-20s  %7d  %5.1f%%\n", i+1, r.name, r.count, pct)
	}
	fmt.Fprintf(ctx.Stdout, "\n  Total history lines: %d\n\n", total)

	return 0
}

func readBashHistory(path string) []string {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil
	}
	lines := strings.Split(strings.TrimRight(string(data), "\n"), "\n")
	var out []string
	for _, l := range lines {
		l = strings.TrimSpace(l)
		if l != "" && !strings.HasPrefix(l, "#") {
			out = append(out, l)
		}
	}
	return out
}

func readZshHistory(path string) []string {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil
	}
	lines := strings.Split(strings.TrimRight(string(data), "\n"), "\n")
	var out []string
	for _, l := range lines {
		if strings.HasPrefix(l, ": ") {
			parts := strings.SplitN(l, ";", 2)
			if len(parts) == 2 && parts[1] != "" {
				out = append(out, parts[1])
			}
		}
	}
	return out
}

func readMisaHistory(path string) []string {
	return readZshHistory(path)
}

var transparentPrefixes = map[string]bool{
	"command": true, "exec": true, "noglob": true, "nocorrect": true,
	"nice": true, "ionice": true, "strace": true, "ltrace": true,
	"time": true, "taskset": true, "chrt": true, "env": true,
	"torify": true, "proxychains": true, "tsocks": true,
	"firejail": true, "flatpak": true, "snap": true,
	"toolbox": true, "distrobox": true, "podman": true, "docker": true,
}

func extractCommandName(line string) string {
	line = strings.TrimSpace(line)
	if line == "" {
		return ""
	}

	fields := strings.Fields(line)
	i := 0

	for i < len(fields) && isEnvAssignment(fields[i]) {
		i++
	}
	if i >= len(fields) {
		return ""
	}

	cmd := fields[i]
	for transparentPrefixes[cmd] {
		i++
		for i < len(fields) && strings.HasPrefix(fields[i], "-") {
			i++
		}
		if i >= len(fields) {
			return ""
		}
		cmd = fields[i]
	}

	cmd = strings.TrimPrefix(cmd, "\\")
	cmd = strings.TrimSuffix(cmd, ";")

	if isEnvAssignment(cmd) || cmd == "" {
		return ""
	}
	if strings.HasPrefix(cmd, "./") {
		return ""
	}

	parts := strings.Split(cmd, "/")
	cmd = parts[len(parts)-1]

	if !isCommandName(cmd) {
		return ""
	}
	return cmd
}

func isEnvAssignment(s string) bool {
	eqIdx := strings.Index(s, "=")
	if eqIdx <= 0 {
		return false
	}
	name := s[:eqIdx]
	for i, c := range name {
		if i == 0 {
			if !((c >= 'A' && c <= 'Z') || (c >= 'a' && c <= 'z') || c == '_') {
				return false
			}
		} else {
			if !((c >= 'A' && c <= 'Z') || (c >= 'a' && c <= 'z') || (c >= '0' && c <= '9') || c == '_') {
				return false
			}
		}
	}
	return true
}

func isCommandName(s string) bool {
	if s == "" {
		return false
	}
	c := s[0]
	if !((c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z') || (c >= '0' && c <= '9')) {
		return false
	}
	for _, r := range s[1:] {
		if !((r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') || r == '_' || r == '-' || r == '+' || r == '.') {
			return false
		}
	}
	return true
}
