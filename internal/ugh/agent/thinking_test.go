package agent

import (
	"strings"
	"testing"

	"github.com/AmadeusDE/Misa/internal/ugh/config"
	"github.com/AmadeusDE/Misa/internal/ugh/policy"
	"github.com/AmadeusDE/Misa/internal/ugh/provider"
)

func TestThinkingChoicesFollowTierRangeAndCap(t *testing.T) {
	provider.SetModelsForTest(t, "openrouter", []provider.Model{
		{Provider: "openrouter", ID: "anthropic/claude-haiku-5.5", Thinking: []string{"low", "medium", "high", "xhigh", "max"}},
	})
	cfg := config.Default()
	haiku, sol := "openrouter/anthropic/claude-haiku-5.5", "codex/gpt-6.1-sol"
	for _, c := range []struct {
		tier policy.Tier
		ref  string
		want string
	}{
		{policy.TierLow, haiku, "low"},          // minimal..low; Haiku's lowest is low
		{policy.TierHigh, haiku, "medium,high"}, // medium..high, Claude cap high
		{policy.TierHigh, sol, "medium,high"},
		{policy.TierOracle, haiku, "high"}, // high..max under the Claude cap
		{policy.TierOracle, "codex/gpt-6-astra", "high,xhigh,max"},
	} {
		got, known := thinkingChoices(cfg, c.tier, c.ref)
		if !known || strings.Join(got, ",") != c.want {
			t.Errorf("%s %s = %v (known=%v), want %s", c.tier, c.ref, got, known, c.want)
		}
	}
	// Unverified levels: the range top, under the cap.
	if got, known := thinkingChoices(cfg, policy.TierHigh, "openrouter/unknown/model"); known || strings.Join(got, ",") != "high" {
		t.Errorf("unknown model = %v %v", got, known)
	}
	if got := fitThinking([]string{"medium", "high"}, "max"); got != "high" {
		t.Errorf("fit max = %s", got)
	}
	if got := fitThinking([]string{"medium", "high"}, "minimal"); got != "medium" {
		t.Errorf("fit minimal = %s", got)
	}
	if c := thinkingCap(cfg, haiku); ThinkingLevels[c] != "high" {
		t.Errorf("haiku cap = %s", ThinkingLevels[c])
	}
	if lo, hi := thinkingRange("low"); lo != hi || ThinkingLevels[lo] != "low" {
		t.Errorf("single level range = %d..%d", lo, hi)
	}
}
