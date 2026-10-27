package agent

import (
	"context"
	"github.com/AmadeusDE/Misa/internal/ugh/roots"

	"github.com/AmadeusDE/Misa/internal/ugh/config"
	"github.com/AmadeusDE/Misa/internal/ugh/perm/model"
	"github.com/AmadeusDE/Misa/internal/ugh/provider"
)

// ContextConfig is everything that shapes what a turn puts in front of the
// model: the visibility ladder, AGENTS.md instructions, big-output digests and
// compaction. The zero value is "all off", so tests and one-shot agents get no
// context machinery by default. Spawn, resume, the TUI and the CLI copy it
// explicitly; nothing reads it from a global.
type ContextConfig struct {
	Ladder          string // "jev" | "off"
	KeepRecent      int
	MinChunkBytes   int
	MinSavings      int
	Instructions    bool
	Digest          string // routing tier that writes digests; "" or "off" disables
	DigestTools     []string
	DigestMinBytes  int // below: never digested; from here to DigestAutoBytes: Jev decides
	DigestAutoBytes int // at or above: always digested

	CompactModel      string // summarizer ref; "" = the conversation model
	CompactThreshold  int    // absolute prompt tokens; 0 = no absolute limit
	CompactThresholds map[string]int
	CompactRoles      map[string]int // absolute prompt tokens by agent role; wins over CompactThresholds

	// AskUser puts an ask_user question in front of the user and waits for
	// the answer (ctx cancellation abandons it). nil means headless: the
	// tool is not offered. Every agent of a tree shares the one UI.
	// Jev answers the ladder's and digest's visibility questions; nil uses
	// the configured decisions endpoint (routing.routers.jev). Tests set a fake.
	Jev model.CostAsker

	AskUser func(ctx context.Context, req AskRequest) (AskAnswer, error)
}

// ContextConfigFrom builds the agent-facing view of the user's configuration.
// A summarizer saved with /compact-model overrides [compaction] model.
func ContextConfigFrom(c config.Config) ContextConfig {
	return ContextConfigIn(roots.Roots{}, c)
}

// ContextConfigIn is ContextConfigFrom for the instance owning r.
func ContextConfigIn(r roots.Roots, c config.Config) ContextConfig {
	cc := ContextConfig{
		Ladder: c.Context.Ladder, KeepRecent: c.Context.KeepRecent, MinChunkBytes: c.Context.MinChunkBytes,
		MinSavings: c.Context.MinSavings, Instructions: c.Context.Instructions,
		Digest: c.Context.Digest, DigestTools: append([]string(nil), c.Context.DigestTools...),
		DigestMinBytes: c.Context.DigestMinBytes, DigestAutoBytes: c.Context.DigestAutoBytes,
		CompactModel: c.Compaction.Model, CompactThreshold: c.Compaction.Threshold,
	}
	if len(c.Compaction.Thresholds) > 0 {
		cc.CompactThresholds = make(map[string]int, len(c.Compaction.Thresholds))
		for k, v := range c.Compaction.Thresholds {
			cc.CompactThresholds[k] = v
		}
	}
	if len(c.Compaction.Roles) > 0 {
		cc.CompactRoles = make(map[string]int, len(c.Compaction.Roles))
		for k, v := range c.Compaction.Roles {
			cc.CompactRoles[k] = v
		}
	}
	if m := provider.CompactModelIn(r); m != "" {
		cc.CompactModel = m
	}
	return cc
}

func (c ContextConfig) ladderOn() bool { return c.Ladder == "jev" }

// DefaultSystem is the system prompt of the interactive and headless CLI when
// none is given. One definition, shared by the TUI and `ugh -p`.
const DefaultSystem = "You are ugh, a terse coding agent working in one directory. " +
	"Use the provided tools to inspect files, make edits and run commands; prefer " +
	"tools over guessing. When the task is done, reply with a short summary."
