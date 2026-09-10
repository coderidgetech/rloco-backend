package handlers

import (
	"context"

	"rloco-backend/internal/services"
)

// companyInfo holds the business details printed on generated documents
// (invoices, packing slips). It is sourced from the live SiteConfig's
// "general" section so an admin can update the registered address/GSTIN
// without a redeploy, falling back to the same defaults the admin settings
// screen seeds a new config with.
type companyInfo struct {
	Name         string
	Tagline      string
	Address      string
	Email        string
	Phone        string
	SupportEmail string
	// GSTIN is optional: omitted from documents entirely when unset, since
	// not every deployment is a GST-registered entity.
	GSTIN string
}

// companyInfoFromConfig reads company details from the live SiteConfig,
// falling back to getDefaultConfig's "general" defaults for any field left
// unset. Mirrors the fail-open pattern used by regionStatusFromConfig.
func companyInfoFromConfig(ctx context.Context, cs services.ConfigService) companyInfo {
	general, _ := getDefaultConfig()["general"].(map[string]interface{})

	info := companyInfo{
		Name:         stringOr(general, "siteName", "Rloko"),
		Tagline:      stringOr(general, "tagline", ""),
		Address:      stringOr(general, "address", ""),
		Email:        stringOr(general, "email", ""),
		Phone:        stringOr(general, "phone", ""),
		SupportEmail: stringOr(general, "supportEmail", ""),
	}

	if cs == nil {
		return info
	}
	stored, err := cs.Get(ctx)
	if err != nil || stored == nil || len(stored.Config) == 0 {
		return info
	}
	live, ok := stored.Config["general"].(map[string]interface{})
	if !ok {
		return info
	}

	if v := stringOr(live, "siteName", ""); v != "" {
		info.Name = v
	}
	if v := stringOr(live, "tagline", ""); v != "" {
		info.Tagline = v
	}
	if v := stringOr(live, "address", ""); v != "" {
		info.Address = v
	}
	if v := stringOr(live, "email", ""); v != "" {
		info.Email = v
	}
	if v := stringOr(live, "phone", ""); v != "" {
		info.Phone = v
	}
	if v := stringOr(live, "supportEmail", ""); v != "" {
		info.SupportEmail = v
	}
	info.GSTIN = stringOr(live, "gstin", "")

	return info
}

func stringOr(m map[string]interface{}, key, fallback string) string {
	if m == nil {
		return fallback
	}
	if v, ok := m[key].(string); ok && v != "" {
		return v
	}
	return fallback
}
