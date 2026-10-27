package bot

import (
	"github.com/AmadeusDE/Misa/internal/ugh/tools"
	"github.com/AmadeusDE/Misa/internal/ughbot/config"
)

var (
	readTools  = []string{"read", "ls", "grep", "find"}
	writeTools = []string{"write", "edit"}
)

// baseTools are the file and shell tools the capabilities grant; task sessions get only these.
func baseTools(c config.Capabilities) *tools.Registry {
	var names []string
	if c.Files == config.FilesRead || c.Files == config.FilesWrite {
		names = append(names, readTools...)
	}
	if c.Files == config.FilesWrite {
		names = append(names, writeTools...)
	}
	if c.Shell {
		names = append(names, "bash")
	}
	reg := tools.NewRegistry()
	for _, t := range tools.All() {
		for _, n := range names {
			if t.Spec().Name == n {
				reg.Add(t)
			}
		}
	}
	return reg
}

// mainTools is everything the main thread may use: the base tools plus the bot's own.
func (b *Bot) mainTools() *tools.Registry {
	c := b.d.Bot.Capabilities
	reg := baseTools(c)
	if c.Tasks > 0 {
		reg.Add(startTask{b})
	}
	if len(c.Message) > 0 || len(c.Manage) > 0 {
		reg.Add(listBots{b})
	}
	if len(c.Message) > 0 {
		reg.Add(messageBot{b})
		reg.Add(askBot{b})
	}
	if len(c.Manage) > 0 {
		reg.Add(manageBot{b})
	}
	if b.d.Scheduler != nil {
		for _, t := range []tools.Tool{scheduleAdd{b}, scheduleList{b}, scheduleCancel{b}} {
			reg.Add(t)
		}
	}
	if c.CreateBots {
		for _, t := range []tools.Tool{createBot{b}, updateBot{b}, deleteBot{b}, showBot{b}} {
			reg.Add(t)
		}
	}
	if len(b.codeDirs()) > 0 {
		for _, t := range []tools.Tool{codeTaskTool{b}, codeTasksTool{b}, codeSteerTool{b}} {
			reg.Add(t)
		}
	}
	if len(b.codeDirs()) > 0 || len(c.Manage) > 0 {
		reg.Add(requestRepo{b})
	}
	reg.Add(replyTool{b})
	return reg
}
