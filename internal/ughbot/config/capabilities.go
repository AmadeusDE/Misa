package config

import (
	"fmt"
	"slices"
	"strings"
)

// Capabilities are everything a bot may do. A bot is given the tools of the
// capabilities it holds and nothing else; memory and prompts can never add one.
type Capabilities struct {
	// Files is "read" (read, ls, grep, find) or "write" (those plus write and edit); empty grants none.
	Files string `toml:"files" json:"files,omitempty"`
	// Shell grants the bash tool.
	Shell bool `toml:"shell" json:"shell,omitempty"`
	// Tasks is how many task sessions (start_task) may run at once; 0 grants none.
	Tasks int `toml:"tasks" json:"tasks,omitempty"`
	// Message lists the bots this one may mail and ask ("*" for every bot).
	Message []string `toml:"message" json:"message,omitempty"`
	// Manage lists the bots whose status this one may read and which it may steer,
	// interrupt and give tasks ("*" for every bot).
	Manage []string `toml:"manage" json:"manage,omitempty"`
	// CreateBots lets the bot ask to create, change and delete bots; every request waits for approval.
	CreateBots bool `toml:"create_bots" json:"create_bots,omitempty"`
	// Schedule lets the bot add and cancel routines for itself and the bots it may message without
	// approval (unless the permission classifiers flag the call). Bots without it ask for approval.
	Schedule bool `toml:"schedule" json:"schedule,omitempty"`
	// Code lists host directories the bot may start code tasks in: a full ugh session whose tools
	// run in a throwaway VM that sees only that directory.
	Code []string `toml:"code" json:"code,omitempty"`
}

// String lists the capabilities held, e.g. "files=read shell tasks=1 message=* create_bots", or "none".
func (c Capabilities) String() string {
	var parts []string
	if c.Files != "" {
		parts = append(parts, "files="+c.Files)
	}
	if c.Shell {
		parts = append(parts, "shell")
	}
	if c.Tasks > 0 {
		parts = append(parts, fmt.Sprintf("tasks=%d", c.Tasks))
	}
	if len(c.Message) > 0 {
		parts = append(parts, "message="+strings.Join(c.Message, ","))
	}
	if len(c.Manage) > 0 {
		parts = append(parts, "manage="+strings.Join(c.Manage, ","))
	}
	if c.CreateBots {
		parts = append(parts, "create_bots")
	}
	if c.Schedule {
		parts = append(parts, "schedule")
	}
	if len(c.Code) > 0 {
		parts = append(parts, "code="+strings.Join(c.Code, ","))
	}
	if len(parts) == 0 {
		return "none"
	}
	return strings.Join(parts, " ")
}

const (
	FilesRead  = "read"
	FilesWrite = "write"
)

// MayMessage reports whether the bot may mail the target.
func (c Capabilities) MayMessage(to string) bool { return granted(c.Message, to) }

// MayManage reports whether the bot may manage the target.
func (c Capabilities) MayManage(to string) bool { return granted(c.Manage, to) }

func granted(list []string, name string) bool {
	return slices.Contains(list, "*") || slices.Contains(list, name)
}
