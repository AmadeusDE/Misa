package builtins

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"sort"
	"strings"
)

const tsortSpec = "V"

var tsortSpecParsed *Spec

func init() {
	var err error
	tsortSpecParsed, err = ParseSpec(tsortSpec)
	if err != nil {
		panic("builtins: bad tsort spec: " + err.Error())
	}
	Register(Command{Name: "tsort", Run: runTsort})
}

type tsortNodeInfo struct {
	name  string
	succs []string
	indeg int
}

func runTsort(ctx *Context) int {
	flags, err := tsortSpecParsed.ParseArgs(ctx.Args)
	if err != nil {
		fmt.Fprintln(ctx.Stderr, "tsort:", err)
		return 1
	}
	if HandleHelpVersion(ctx, flags, tsortSpecParsed) {
		return 0
	}
	if flags.Bool("V") {
		fmt.Fprintf(ctx.Stdout, "%s (Misa) 0.1.0\n", ctx.CmdName)
		return 0
	}

	args := flags.Operands
	if len(args) > 1 {
		fmt.Fprintf(ctx.Stderr, "tsort: extra operand '%s'\nTry 'tsort --help' for more information.\n", args[1])
		return 1
	}

	sourceName := "-"
	var r io.Reader
	if len(args) == 1 && args[0] != "-" {
		sourceName = args[0]
		path := resolvePath(ctx, args[0])
		f, err := os.Open(path)
		if err != nil {
			fmt.Fprintf(ctx.Stderr, "tsort: %s: %s\n", args[0], fmtOsErr(err))
			return 1
		}
		defer f.Close()
		fi, serr := f.Stat()
		if serr == nil && fi.IsDir() {
			fmt.Fprintf(ctx.Stderr, "tsort: %s: read error: Is a directory\n", args[0])
			return 1
		}
		r = f
	} else {
		r = ctx.Stdin
	}

	var tokens []string
	sc := bufio.NewScanner(r)
	sc.Split(bufio.ScanWords)
	for sc.Scan() {
		tokens = append(tokens, sc.Text())
	}
	if len(tokens)%2 != 0 {
		fmt.Fprintf(ctx.Stderr, "tsort: %s: input contains an odd number of tokens\n", sourceName)
		return 1
	}

	nodeMap := map[string]*tsortNodeInfo{}

	getNode := func(name string) *tsortNodeInfo {
		n, ok := nodeMap[name]
		if !ok {
			n = &tsortNodeInfo{name: name}
			nodeMap[name] = n
		}
		return n
	}

	for i := 0; i < len(tokens); i += 2 {
		a, b := tokens[i], tokens[i+1]
		getNode(a)
		getNode(b)
		if a == b {
			continue
		}
		na := nodeMap[a]
		na.succs = append(na.succs, b)
		nodeMap[b].indeg++
	}

	done := map[string]bool{}
	var result []string

	var roots []string
	for name := range nodeMap {
		if nodeMap[name].indeg == 0 {
			roots = append(roots, name)
		}
	}
	sort.Strings(roots)
	queue := make([]string, len(roots))
	copy(queue, roots)

	rc := 0
	for {
		for len(queue) > 0 {
			n := queue[0]
			queue = queue[1:]
			if done[n] {
				continue
			}
			done[n] = true
			result = append(result, n)

			succs := nodeMap[n].succs
			for i := len(succs) - 1; i >= 0; i-- {
				s := succs[i]
				nodeMap[s].indeg--
				if nodeMap[s].indeg == 0 && !done[s] {
					queue = append(queue, s)
				}
			}
		}

		if len(result) >= len(nodeMap) {
			break
		}

		stuck := tsortFindSmallestCycleNode(nodeMap, done)
		if stuck == "" {
			break
		}

		tsortReportCycles(ctx, stuck, nodeMap, done, sourceName)
		rc = 1

		done[stuck] = true
		result = append(result, stuck)
		succs := nodeMap[stuck].succs
		for i := len(succs) - 1; i >= 0; i-- {
			s := succs[i]
			nodeMap[s].indeg--
			if nodeMap[s].indeg == 0 && !done[s] {
				queue = append(queue, s)
			}
		}
	}

	fmt.Fprint(ctx.Stdout, strings.Join(result, "\n"))
	if len(result) > 0 {
		fmt.Fprintln(ctx.Stdout)
	}
	return rc
}

func tsortFindSmallestCycleNode(nodeMap map[string]*tsortNodeInfo, done map[string]bool) string {
	var candidates []string
	for name := range nodeMap {
		if !done[name] {
			candidates = append(candidates, name)
		}
	}
	sort.Strings(candidates)
	for _, c := range candidates {
		if tsortInCycle(c, nodeMap, done) {
			return c
		}
	}
	if len(candidates) > 0 {
		return candidates[0]
	}
	return ""
}

func tsortInCycle(start string, nodeMap map[string]*tsortNodeInfo, done map[string]bool) bool {
	visited := map[string]bool{}
	var walk func(string) bool
	walk = func(n string) bool {
		if n == start && visited[n] {
			return true
		}
		if visited[n] {
			return false
		}
		visited[n] = true
		for _, s := range nodeMap[n].succs {
			if done[s] {
				continue
			}
			if walk(s) {
				return true
			}
		}
		return false
	}
	visited[start] = true
	for _, s := range nodeMap[start].succs {
		if done[s] {
			continue
		}
		if walk(s) {
			return true
		}
	}
	return false
}

func tsortReportCycles(ctx *Context, start string, nodeMap map[string]*tsortNodeInfo, done map[string]bool, sourceName string) {
	path := []string{start}
	inPath := map[string]bool{start: true}

	var dfs func(node string)
	dfs = func(node string) {
		for _, s := range nodeMap[node].succs {
			if done[s] {
				continue
			}
			if inPath[s] {
				var cycle []string
				for i := len(path) - 1; i >= 0; i-- {
					cycle = append([]string{path[i]}, cycle...)
					if path[i] == s {
						break
					}
				}
				sort.Strings(cycle)
				fmt.Fprintf(ctx.Stderr, "tsort: %s: input contains a loop:\n", sourceName)
				for _, c := range cycle {
					fmt.Fprintf(ctx.Stderr, "tsort: %s\n", c)
				}
				continue
			}
			inPath[s] = true
			path = append(path, s)
			dfs(s)
			path = path[:len(path)-1]
			delete(inPath, s)
		}
	}
	dfs(start)
}
