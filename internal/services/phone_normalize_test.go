package services

import "testing"

func TestNormalizePhoneKey_India(t *testing.T) {
	const cc = "91"
	cases := []struct {
		raw, want string
	}{
		{"9866873530", "919866873530"},
		{"09866873530", "919866873530"},
		{"919866873530", "919866873530"},
		{"+91 98668 73530", "919866873530"},
	}
	for _, tc := range cases {
		got, err := NormalizePhoneKey(tc.raw, cc)
		if err != nil {
			t.Fatalf("NormalizePhoneKey(%q): %v", tc.raw, err)
		}
		if got != tc.want {
			t.Errorf("NormalizePhoneKey(%q) = %q, want %q", tc.raw, got, tc.want)
		}
	}
}
