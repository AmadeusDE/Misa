package bot

import (
	"fmt"
	"strings"

	"github.com/AmadeusDE/Misa/internal/ugh/perm"
)

// codePrompter puts a code task's flagged calls to the bot that started it.
type codePrompter struct{ t *codeTask }

func (p codePrompter) Ask(c perm.Call, vs []perm.Verdict) perm.Answer {
	var b strings.Builder
	fmt.Fprintf(&b, "Code task %s's permission check flagged a call and needs your decision.\ntool: %s\n", p.t.id, c.Tool)
	if c.Command != "" {
		fmt.Fprintf(&b, "command: %s\n", c.Command)
	} else {
		fmt.Fprintf(&b, "arguments: %s\n", clip(string(c.Args)))
	}
	for _, v := range vs {
		if v.Flagged {
			fmt.Fprintf(&b, "flagged by %s: %s\n", v.Classifier, v.Reason)
		}
	}
	b.WriteString("Reply \"allow\" to let it run once; any other reply denies it.")
	text, err := p.t.question(b.String())
	if err != nil {
		return perm.AnsStop
	}
	if w := strings.Fields(strings.ToLower(text)); len(w) > 0 && (strings.HasPrefix(w[0], "allow") || strings.HasPrefix(w[0], "yes")) {
		return perm.AnsAllowOnce
	}
	return perm.AnsDeny
}

func (codePrompter) Allowed(perm.Call) bool { return false }
func (codePrompter) Interactive() bool      { return true }
