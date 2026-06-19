package middleware

import "testing"

func TestVendorPermissionAllows(t *testing.T) {
	perms := map[string]interface{}{
		"products": map[string]interface{}{
			"create": true,
			"delete": false,
			// "edit" intentionally absent
			"publish": "yes", // non-bool
		},
	}

	cases := []struct {
		name     string
		category string
		action   string
		want     bool
	}{
		{"explicit true", "products", "create", true},
		{"explicit false denies", "products", "delete", false},
		{"missing action defaults allow", "products", "edit", true},
		{"non-bool value defaults allow", "products", "publish", true},
		{"missing category defaults allow", "orders", "viewOwn", true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := vendorPermissionAllows(perms, tc.category, tc.action); got != tc.want {
				t.Errorf("vendorPermissionAllows(%s,%s) = %v, want %v", tc.category, tc.action, got, tc.want)
			}
		})
	}
}
