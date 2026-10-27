package agent

import (
	"fmt"
	"slices"
	"strings"

	"github.com/AmadeusDE/Misa/internal/ugh/config"
	"github.com/AmadeusDE/Misa/internal/ugh/policy"
	"github.com/AmadeusDE/Misa/internal/ugh/provider"
)

// selfPick reports [routing] pick_model = "self": a stopgap that skips the routers and tiers
// and lets the spawning model choose each child's model and thinking.
func selfPick(cfg config.Config) bool { return cfg.Routing.PickModel == "self" }

// selfModels are the models a spawning model may choose: every provider/model ref in the
// routing tiers plus the scoped root models.
func selfModels(cfg config.Config) []string {
	var out []string
	add := func(ref string) {
		if strings.Contains(ref, "/") && ref != "*" && !slices.Contains(out, ref) {
			out = append(out, ref)
		}
	}
	for _, tier := range []string{"low", "high", "oracle"} {
		for _, e := range cfg.Routing.TierEntries(tier) {
			if fam, ok := policy.FamilyByID(e); ok {
				for _, r := range policy.TierRoutes([]string{string(fam)}, "worker", nil) {
					add(r.Ref())
				}
				continue
			}
			add(e)
		}
	}
	refs, _ := provider.ScopedRefs()
	for _, r := range refs {
		add(r)
	}
	return out
}

// planSelf takes the caller's model and thinking as given.
func (r *Runtime) planSelf(t Task, role string) (routeChoice, error) {
	models := selfModels(r.Config)
	ref := strings.TrimSpace(t.Model)
	if ref == "" {
		return routeChoice{}, fmt.Errorf("model selection is yours: resubmit with model, one of: %s", strings.Join(models, ", "))
	}
	if !slices.Contains(models, ref) {
		return routeChoice{}, fmt.Errorf("unknown model %q; choose one of: %s", ref, strings.Join(models, ", "))
	}
	route, ok := policy.RouteByRef(ref)
	if !ok {
		prov, id, _ := strings.Cut(ref, "/")
		route = policy.Route{Provider: prov, ModelID: id}
	}
	thinking := t.Thinking
	if thinking == "" {
		thinking = DefaultThinking
	}
	return routeChoice{Mode: "self", Router: "caller", Role: role, Tier: policy.TierHigh, Thinking: thinking,
		Routes: []policy.Route{route}}, nil
}
