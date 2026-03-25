package services

import "testing"

func TestNormalizePhoneKey_International(t *testing.T) {
	cases := []struct {
		raw, want string
	}{
		{"919866873530", "919866873530"},
		{"+91 98668 73530", "919866873530"},
		{"12025550123", "12025550123"},
		{"+1 202 555 0123", "12025550123"},
	}
	for _, tc := range cases {
		got, err := NormalizePhoneKey(tc.raw)
		if err != nil {
			t.Fatalf("NormalizePhoneKey(%q): %v", tc.raw, err)
		}
		if got != tc.want {
			t.Errorf("NormalizePhoneKey(%q) = %q, want %q", tc.raw, got, tc.want)
		}
	}
}

func TestNormalizePhoneKey_RejectsBareTenDigits(t *testing.T) {
	_, err := NormalizePhoneKey("9866873530")
	if err == nil {
		t.Fatal("expected error for 10-digit local without country")
	}
}
