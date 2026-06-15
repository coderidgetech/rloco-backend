package handlers

import (
	"context"

	"rloco-backend/internal/services"
)

// regionStatusFromConfig reports whether a market is enabled from the live
// SiteConfig, returning its coming-soon message when disabled. It fails open
// (enabled, no message) when the regions config is absent, so an unconfigured
// deployment behaves as before rather than gating everything.
//
// Shared by the order flow (gating checkout) and the region resolver endpoint so
// the config key path and fail-open policy live in exactly one place.
func regionStatusFromConfig(ctx context.Context, cs services.ConfigService, market string) (enabled bool, message string) {
	cfg := getDefaultConfig()
	if cs != nil {
		if stored, err := cs.Get(ctx); err == nil && stored != nil && len(stored.Config) > 0 {
			// ensureRegionDefaults backfills regions for configs predating gating.
			cfg = ensureRegionDefaults(stored.Config)
		}
	}

	general, ok := cfg["general"].(map[string]interface{})
	if !ok {
		return true, ""
	}
	regions, ok := general["regions"].(map[string]interface{})
	if !ok {
		return true, ""
	}
	region, ok := regions[market].(map[string]interface{})
	if !ok {
		return true, ""
	}
	en, ok := region["enabled"].(bool)
	if !ok {
		return true, ""
	}
	msg, _ := region["comingSoonMessage"].(string)
	return en, msg
}
