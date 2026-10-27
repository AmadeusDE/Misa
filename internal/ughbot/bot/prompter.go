package bot

import (
	"slices"
	"strconv"
	"strings"

	"github.com/AmadeusDE/Misa/internal/ugh/perm"
)

// headlessPrompter answers every prompt with the configured unattended policy; no human is
// attached to a bot's thread. A flagged routine call is deferred to the approval queue instead.
type headlessPrompter struct {
	answer perm.Answer
	bot    *Bot
}

var deferredTools = []string{scheduleAddTool, scheduleCancelTool}

func (p *headlessPrompter) Ask(c perm.Call, _ []perm.Verdict) perm.Answer {
	if p.bot != nil && p.bot.d.Proposer != nil && slices.Contains(deferredTools, c.Tool) {
		return perm.AnswerDefer
	}
	return p.answer
}

func (p *headlessPrompter) Allowed(perm.Call) bool { return false }
func (p *headlessPrompter) Interactive() bool      { return false }

// Defer queues the exact call for approval and returns its ticket.
func (p *headlessPrompter) Defer(c perm.Call, vs []perm.Verdict) string {
	var why []string
	for _, v := range vs {
		if v.Flagged {
			why = append(why, v.Classifier+": "+v.Reason)
		}
	}
	id, err := p.bot.d.Proposer.ProposeToolCall(p.bot.name, c.Tool, c.Args, strings.Join(why, "; "))
	if err != nil {
		return "none (" + err.Error() + ")"
	}
	return strconv.FormatInt(id, 10)
}

func (p *headlessPrompter) SetHeadless(policy string) {
	switch policy {
	case "allow":
		p.answer = perm.AnsAllowOnce
	case "stop":
		p.answer = perm.AnsStop
	default:
		p.answer = perm.AnsDeny
	}
}
