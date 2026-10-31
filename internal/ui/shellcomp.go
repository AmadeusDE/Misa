package ui

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/AmadeusDE/Misa/internal/builtins"
	"github.com/AmadeusDE/Misa/internal/hostpath"
	"github.com/AmadeusDE/Misa/internal/psreadline"
)

const maxHistoryLines = 5000

type histEntry struct {
	ts  int64
	cmd string
}

func builtinNames() []string {
	names := []string{"cd", "clear", "exit", "edit", "misa", "alias", "unalias"}
	names = append(names, builtins.Names()...)
	return names
}

func historyPath() string { return hostpath.HistoryFile() }

const onWindows = runtime.GOOS == "windows"

func loadHistory() []string {
	entries := loadHistoryEntries()
	out := make([]string, len(entries))
	for i, e := range entries {
		out[i] = e.cmd
	}
	return out
}

func loadHistoryEntries() []histEntry {
	path := historyPath()
	if path == "" {
		return nil
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return nil
	}
	return parseZshHistory(string(data))
}

func parseZshHistory(data string) []histEntry {
	lines := strings.Split(strings.TrimRight(data, "\n"), "\n")
	var entries []histEntry
	for _, l := range lines {
		if l == "" {
			continue
		}
		if strings.HasPrefix(l, ": ") {
			parts := strings.SplitN(l, ";", 2)
			if len(parts) == 2 {
				cmd := parts[1]
				tsStr := strings.TrimPrefix(parts[0], ": ")
				tsStr = strings.SplitN(tsStr, ":", 2)[0]
				ts, _ := strconv.ParseInt(tsStr, 10, 64)
				if cmd != "" {
					entries = append(entries, histEntry{ts: ts, cmd: cmd})
				}
				continue
			}
		}
		entries = append(entries, histEntry{ts: 0, cmd: l})
	}
	return entries
}

func saveHistory(history []string) {
	entries := make([]histEntry, len(history))
	existing := loadHistoryEntries()
	eMap := make(map[string]int64, len(existing))
	for _, e := range existing {
		eMap[e.cmd] = e.ts
	}
	now := time.Now().Unix()
	for i, cmd := range history {
		if ts, ok := eMap[cmd]; ok && ts > 0 {
			entries[i] = histEntry{ts: ts, cmd: cmd}
		} else {
			entries[i] = histEntry{ts: now, cmd: cmd}
		}
	}
	saveHistoryEntries(entries)
}

func saveHistoryEntries(entries []histEntry) {
	path := historyPath()
	if path == "" {
		return
	}
	os.MkdirAll(filepath.Dir(path), 0o755)
	start := 0
	if len(entries) > maxHistoryLines {
		start = len(entries) - maxHistoryLines
	}
	var b strings.Builder
	for _, e := range entries[start:] {
		fmt.Fprintf(&b, ": %d:0;%s\n", e.ts, e.cmd)
	}
	os.WriteFile(path, []byte(b.String()), 0o644)
}

func parseBashHistory(data string) []histEntry {
	lines := strings.Split(strings.TrimRight(data, "\n"), "\n")
	var entries []histEntry
	for _, l := range lines {
		l = strings.TrimSpace(l)
		if l == "" || strings.HasPrefix(l, "#") {
			continue
		}
		entries = append(entries, histEntry{ts: 0, cmd: l})
	}
	return entries
}

func ImportExternalHistory() (int, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return 0, err
	}

	existing := loadHistoryEntries()
	seen := make(map[string]bool, len(existing))
	for _, e := range existing {
		seen[e.cmd] = true
	}

	var bashEntries, zshEntries []histEntry

	bashPath := filepath.Join(home, ".bash_history")
	if data, err := os.ReadFile(bashPath); err == nil {
		bashEntries = parseBashHistory(string(data))
	}

	zshPath := filepath.Join(home, ".zsh_history")
	if data, err := os.ReadFile(zshPath); err == nil {
		zshEntries = parseZshHistory(string(data))
	}

	var psEntries []histEntry
	if onWindows {
		psEntries = psReadLineEntries(os.Getenv("APPDATA"))
	}

	var imported []histEntry
	for _, e := range bashEntries {
		if !seen[e.cmd] {
			seen[e.cmd] = true
			imported = append(imported, e)
		}
	}
	for _, e := range zshEntries {
		if !seen[e.cmd] {
			seen[e.cmd] = true
			imported = append(imported, e)
		}
	}
	for _, e := range psEntries {
		if !seen[e.cmd] {
			seen[e.cmd] = true
			imported = append(imported, e)
		}
	}

	if len(imported) == 0 {
		return 0, nil
	}

	noTS := make([]histEntry, 0)
	withTS := make([]histEntry, 0)
	for _, e := range existing {
		if e.ts == 0 {
			noTS = append(noTS, e)
		} else {
			withTS = append(withTS, e)
		}
	}
	for _, e := range imported {
		if e.ts == 0 {
			noTS = append(noTS, e)
		} else {
			withTS = append(withTS, e)
		}
	}

	sort.SliceStable(withTS, func(i, j int) bool {
		return withTS[i].ts < withTS[j].ts
	})

	merged := append(noTS, withTS...)
	saveHistoryEntries(merged)
	return len(imported), nil
}

// psReadLineEntries reads PSReadLine history under appData. Misa history holds one command per line, so a
// multiline entry is flattened to one line (a backtick continuation is whitespace to PowerShell).
func psReadLineEntries(appData string) []histEntry {
	path := psreadline.Path(appData)
	if path == "" {
		return nil
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return nil
	}
	var entries []histEntry
	for _, cmd := range psreadline.Parse(data) {
		cmd = strings.Join(strings.Fields(cmd), " ")
		if cmd != "" {
			entries = append(entries, histEntry{cmd: cmd})
		}
	}
	return entries
}

type shellState struct {
	history       []string
	historyIdx    int
	suggestion    string
	savedInput    string
	searchPrefix  string
	searchHistory bool

	completions []string
	compIdx     int
	compInput   string

	lastCmd    string
	lastOutput string
}

func (s *shellState) loadFromDisk() {
	s.history = loadHistory()
	s.historyIdx = -1
}

func (s *shellState) addHistory(cmd string) {
	cmd = strings.TrimSpace(cmd)
	if cmd == "" {
		return
	}
	if len(s.history) > 0 && s.history[len(s.history)-1] == cmd {
		return
	}
	s.history = append(s.history, cmd)
	s.historyIdx = -1
	appendHistoryEntry(histEntry{ts: time.Now().Unix(), cmd: cmd})
}

func appendHistoryEntry(e histEntry) {
	path := historyPath()
	if path == "" {
		return
	}
	os.MkdirAll(filepath.Dir(path), 0o755)
	f, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
	if err != nil {
		return
	}
	defer f.Close()
	fmt.Fprintf(f, ": %d:0;%s\n", e.ts, e.cmd)
}

func (s *shellState) historyUp(currentInput string) string {
	if len(s.history) == 0 {
		return currentInput
	}
	if s.historyIdx == -1 {
		s.savedInput = currentInput
		if s.searchHistory {
			s.searchPrefix = strings.TrimSpace(currentInput)
		} else {
			s.searchPrefix = ""
		}
		s.historyIdx = len(s.history)
	}
	current := ""
	if s.historyIdx >= 0 && s.historyIdx < len(s.history) {
		current = s.history[s.historyIdx]
	}
	for i := s.historyIdx - 1; i >= 0; i-- {
		if s.history[i] == current {
			continue
		}
		if s.searchPrefix == "" || strings.HasPrefix(s.history[i], s.searchPrefix) {
			s.historyIdx = i
			return s.history[i]
		}
	}
	if s.historyIdx >= len(s.history) {
		s.historyIdx = -1
	}
	return currentInput
}

func (s *shellState) historyDown() string {
	if s.historyIdx == -1 {
		return ""
	}
	if s.historyIdx >= len(s.history) {
		s.historyIdx = -1
		return s.savedInput
	}
	current := s.history[s.historyIdx]
	for i := s.historyIdx + 1; i < len(s.history); i++ {
		if s.history[i] == current {
			continue
		}
		if s.searchPrefix == "" || strings.HasPrefix(s.history[i], s.searchPrefix) {
			s.historyIdx = i
			return s.history[i]
		}
	}
	s.historyIdx = -1
	return s.savedInput
}

func (s *shellState) resetHistory() {
	s.historyIdx = -1
	s.savedInput = ""
	s.searchPrefix = ""
}

func (s *shellState) complete(input, dir string) {
	trimmed := strings.TrimSpace(input)
	if trimmed == "" {
		s.suggestion = ""
		s.completions = nil
		s.compInput = ""
		return
	}

	for i := len(s.history) - 1; i >= 0; i-- {
		if strings.HasPrefix(s.history[i], trimmed) && s.history[i] != trimmed {
			s.suggestion = s.history[i]
			return
		}
	}

	fields := strings.Fields(trimmed)

	if len(fields) <= 1 && !strings.HasSuffix(trimmed, " ") {
		prefix := trimmed
		if match := firstFromPath(prefix); match != "" {
			s.suggestion = match
			return
		}
		for _, b := range builtinNames() {
			if strings.HasPrefix(b, prefix) && b != prefix {
				s.suggestion = b
				return
			}
		}
	}

	if len(fields) >= 1 {
		partial := ""
		if !strings.HasSuffix(trimmed, " ") {
			partial = fields[len(fields)-1]
		}
		if match := firstFromFS(partial, dir); match != "" {
			prefix := trimmed[:len(trimmed)-len(partial)]
			s.suggestion = prefix + match
			return
		}
	}

	s.suggestion = ""
}

func (s *shellState) tabComplete(input, dir string) string {
	trimmed := strings.TrimSpace(input)
	if trimmed == "" {
		return input
	}

	if len(s.completions) > 0 && s.compInput != "" {
		for i, c := range s.completions {
			if c == trimmed {
				s.compIdx = (i + 1) % len(s.completions)
				result := s.completions[s.compIdx]
				s.suggestion = result
				return result
			}
		}
		if trimmed == s.compInput {
			s.compIdx = 0
			result := s.completions[0]
			s.suggestion = result
			return result
		}
	}

	s.completions = nil
	s.compIdx = 0
	s.compInput = trimmed

	fields := strings.Fields(trimmed)

	if len(fields) <= 1 && !strings.HasSuffix(trimmed, " ") {
		prefix := trimmed
		s.completions = allFromPath(prefix)
		for _, b := range builtinNames() {
			if strings.HasPrefix(b, prefix) && b != prefix {
				s.completions = append(s.completions, b)
			}
		}
		s.completions = append(s.completions, allFromFS(prefix, dir)...)
	} else if len(fields) >= 1 {
		partial := ""
		if !strings.HasSuffix(trimmed, " ") {
			partial = fields[len(fields)-1]
		}
		prefix := trimmed[:len(trimmed)-len(partial)]
		for _, m := range allFromFS(partial, dir) {
			s.completions = append(s.completions, prefix+m)
		}
	}

	if len(s.completions) == 0 {
		return input
	}

	s.compIdx = 0
	result := s.completions[0]
	s.suggestion = result
	return result
}

func (s *shellState) acceptSuggestion(input string) string {
	if s.suggestion != "" && strings.HasPrefix(s.suggestion, strings.TrimSpace(input)) {
		result := s.suggestion
		s.suggestion = ""
		s.completions = nil
		s.compInput = ""
		return result
	}
	return input
}

func firstFromPath(prefix string) string {
	pathEnv := os.Getenv("PATH")
	if pathEnv == "" {
		return ""
	}
	var best string
	for _, dir := range filepath.SplitList(pathEnv) {
		entries, err := os.ReadDir(dir)
		if err != nil {
			continue
		}
		for _, e := range entries {
			if e.IsDir() {
				continue
			}
			name := e.Name()
			if strings.HasPrefix(name, prefix) && name != prefix {
				if best == "" || name < best {
					best = name
				}
			}
		}
	}
	return best
}

func allFromPath(prefix string) []string {
	pathEnv := os.Getenv("PATH")
	if pathEnv == "" {
		return nil
	}
	seen := map[string]bool{}
	var matches []string
	for _, dir := range filepath.SplitList(pathEnv) {
		entries, err := os.ReadDir(dir)
		if err != nil {
			continue
		}
		for _, e := range entries {
			if e.IsDir() {
				continue
			}
			name := e.Name()
			if strings.HasPrefix(name, prefix) && name != prefix && !seen[name] {
				seen[name] = true
				matches = append(matches, name)
			}
		}
	}
	sort.Strings(matches)
	return matches
}

func firstFromFS(partial, dir string) string {
	matches := allFromFS(partial, dir)
	if len(matches) > 0 {
		return matches[0]
	}
	return ""
}

func allFromFS(partial, dir string) []string { return completeFS(partial, dir, onWindows) }

// completeFS lists the entries of the directory named by partial's leading part that start with its last
// element. windows adds the backslash separator, drive roots and UNC paths; it is a parameter so both
// behaviours can be tested on any host.
func completeFS(partial, dir string, windows bool) []string {
	searchDir := dir
	pathPrefix, base := hostpath.SplitPath(partial, windows)

	if pathPrefix != "" {
		lookup := pathPrefix
		if windows {
			lookup = strings.ReplaceAll(lookup, `\`, "/")
		}
		if hostpath.IsAbs(pathPrefix, windows) {
			searchDir = filepath.Clean(lookup)
		} else {
			searchDir = filepath.Join(dir, lookup)
		}
	}

	entries, err := os.ReadDir(searchDir)
	if err != nil {
		return nil
	}

	var matches []string
	for _, e := range entries {
		name := e.Name()
		if strings.HasPrefix(name, ".") && !strings.HasPrefix(base, ".") {
			continue
		}
		if base != "" && !strings.HasPrefix(name, base) {
			continue
		}
		if name == base && !e.IsDir() {
			continue
		}
		result := pathPrefix + name
		if e.IsDir() {
			result += hostpath.Separator(partial, windows)
		}
		matches = append(matches, result)
	}
	sort.Strings(matches)
	return matches
}
