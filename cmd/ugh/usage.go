package main

import (
	"context"
	"fmt"
	"sort"
	"time"

	"github.com/AmadeusDE/Misa/internal/ugh/config"
	"github.com/AmadeusDE/Misa/internal/ugh/provider"
	"github.com/AmadeusDE/Misa/internal/ugh/provstate"
	usagerpt "github.com/AmadeusDE/Misa/internal/ugh/usage"
)

// usageCmd prints subscription and credit usage per provider.
func usageCmd([]string) error {
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	secs, errs := usagerpt.Fetch(ctx, provider.UsageSources())
	fmt.Println(usagerpt.Render(secs, errs, time.Now(), nil))
	if p := provstate.Summary(); p != "" {
		fmt.Println("\nRouting restrictions: " + p)
	}
	if t := routeTable(); t != "" {
		fmt.Println("\nRoutes, last 7 days (input includes cached):\n" + t)
	}
	if len(secs) == 0 {
		return fmt.Errorf("no usage available")
	}
	return nil
}

// routeTable is the last week's per-route usage, with the tiers that list
// each route, so tier choices can be checked against cost and cache rate.
func routeTable() string {
	tiers := map[string][]string{}
	if cfg, err := config.Load(); err == nil {
		names := make([]string, 0, len(cfg.Routing.Tiers))
		for name := range cfg.Routing.Tiers {
			names = append(names, name)
		}
		sort.Strings(names)
		for _, name := range names {
			for _, ref := range cfg.Routing.TierEntries(name) {
				tiers[ref] = append(tiers[ref], name)
			}
		}
	}
	return usagerpt.RenderRoutes(usagerpt.RouteStats(time.Now().AddDate(0, 0, -7)), tiers)
}
