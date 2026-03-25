package services

import (
	"errors"
	"strings"
)

// NormalizePhoneKey extracts digits from the client string. The API does not apply a default country code:
// bare 10-digit "national" numbers are rejected so the client must send country + number (e.g. from a country selector).
func NormalizePhoneKey(raw string) (string, error) {
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
	// Ambiguous without country (common for US/IN 10-digit mobiles).
	if len(s) == 10 {
		return "", errors.New("phone must include country code: select your country and enter your number, or send full digits only (e.g. 919876543210)")
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
