package help

import "strings"

// validPageName accepts a command name as a man or tldr page name: no path
// separators, dot names or glob characters, so a name cannot reach a file
// outside the page directories.
func validPageName(name string) bool {
	return name != "" && name != "." && name != ".." && !strings.ContainsAny(name, "/\\\x00*?[")
}
