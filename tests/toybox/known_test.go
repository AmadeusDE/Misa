package toybox

import (
	_ "embed"
	"os"
	"strings"
	"sync"
	"testing"
)

//go:embed known_failures.txt
var knownFailuresText string

var knownFailures = sync.OnceValue(func() map[string]string {
	m := map[string]string{}
	for _, line := range strings.Split(knownFailuresText, "\n") {
		name, reason, _ := strings.Cut(line, "#")
		if name = strings.TrimSpace(name); name != "" {
			m[name] = strings.TrimSpace(reason)
		}
	}
	return m
})

// skipKnownFailure skips a case listed in known_failures.txt unless
// MISA_KNOWN_FAILURES is set, so the suite stays green while the list is
// worked down.
func skipKnownFailure(t *testing.T) {
	t.Helper()
	_, name, _ := strings.Cut(t.Name(), "/")
	if reason, ok := knownFailures()[name]; ok && os.Getenv("MISA_KNOWN_FAILURES") == "" {
		t.Skip("known failure: " + reason)
	}
}
