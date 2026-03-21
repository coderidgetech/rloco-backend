package services

import (
	"errors"
	"strings"
)

// NormalizePhoneKey extracts digits and optionally prefixes default country code (e.g. "91") when local number is 10 digits.
func NormalizePhoneKey(raw string, defaultCountryDigits string) (string, error) {
	var b strings.Builder
	for _, r := range raw {
		if r >= '0' && r <= '9' {
			b.WriteRune(r)
		}
	}
	s := b.String()
	if s == "" {
		return "", errors.New("invalid phone number")
	}
	// India-style leading 0 on 10-digit mobile (09866873530 → same as 9866873530)
	if defaultCountryDigits == "91" && len(s) == 11 && s[0] == '0' {
		s = s[1:]
	}
	if len(s) == 10 && defaultCountryDigits != "" {
		s = defaultCountryDigits + s
	}
	if len(s) < 10 || len(s) > 15 {
		return "", errors.New("invalid phone number length")
	}
	return s, nil
}

// PhoneKeyToE164Plus returns "+{digits}" for storage/display.
func PhoneKeyToE164Plus(key string) string {
	if key == "" {
		return ""
	}
	return "+" + key
}
