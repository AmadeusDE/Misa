package agent

import (
	"strings"

	"github.com/AmadeusDE/Misa/internal/ugh/config"
	"github.com/AmadeusDE/Misa/internal/ugh/policy"
)

// Delegated thinking: each tier has a range ([routing.thinking], e.g. low =
// "minimal..low") and each model family may have a cap ([routing.thinking_cap],
// e.g. Claude ≤ high). The root's thinking is the user's and never passes
// through here.

// thinkingRange parses "min..max" (or a single level) into ThinkingLevels
// indexes; an empty or unparseable range allows everything above off.
func thinkingRange(spec string) (lo, hi int) {
	lo, hi = 1, len(ThinkingLevels)-1
	if spec == "" {
		return lo, hi
	}
	a, b, isRange := strings.Cut(spec, "..")
	if !isRange {
		b = a
	}
	if i := indexOf(ThinkingLevels, strings.TrimSpace(a)); i >= 1 {
		lo = i
	}
	if i := indexOf(ThinkingLevels, strings.TrimSpace(b)); i >= 1 {
		hi = i
	}
	if lo > hi {
		lo = hi
	}
	return lo, hi
}

// thinkingCap is the highest level allowed for a route ref, as a
// ThinkingLevels index (the top when no cap applies).
func thinkingCap(cfg config.Config, ref string) int {
	top := len(ThinkingLevels) - 1
	id := ref
	if r, ok := policy.RouteByRef(ref); ok {
		id = policy.Families[r.Family].CardID
	} else if _, model, ok := strings.Cut(ref, "/"); ok {
		id = model
	}
	for prefix, level := range cfg.Routing.ThinkingCap {
		if i := indexOf(ThinkingLevels, level); i >= 1 && strings.HasPrefix(id, prefix) && i < top {
			top = i
		}
	}
	return top
}

// thinkingChoices are the levels a delegated agent on ref may run at in tier:
// the model's levels inside the tier's range and under its cap, weakest
// first. When none fit, the single closest level to the effective top.
// known=false: the model's levels are unverified (the range is used as is).
func thinkingChoices(cfg config.Config, tier policy.Tier, ref string) (choices []string, known bool) {
	lo, hi := thinkingRange(cfg.Routing.Thinking[string(tier)])
	if c := thinkingCap(cfg, ref); c < hi {
		hi = c
		if lo > hi {
			lo = hi
		}
	}
	levels, known := ModelThinking(ref)
	if !known {
		return []string{ThinkingLevels[hi]}, false
	}
	for _, l := range levels {
		if i := indexOf(ThinkingLevels, l); i >= lo && i <= hi {
			choices = append(choices, l)
		}
	}
	if len(choices) == 0 {
		choices = []string{clampLevel(levels, ThinkingLevels[hi])}
	}
	return choices, true
}

// fitThinking maps a wanted level into choices: above the range → its top,
// below → its bottom, inside → the closest supported level.
func fitThinking(choices []string, want string) string {
	if len(choices) == 0 {
		return want
	}
	w := indexOf(ThinkingLevels, want)
	switch {
	case w < 0 || w > indexOf(ThinkingLevels, choices[len(choices)-1]):
		return choices[len(choices)-1]
	case w < indexOf(ThinkingLevels, choices[0]):
		return choices[0]
	}
	return clampLevel(choices, want)
}
