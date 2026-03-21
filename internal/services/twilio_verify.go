package services

import (
	"context"
	"errors"
	"fmt"
	"log"
	"strings"

	"github.com/twilio/twilio-go"
	openapi "github.com/twilio/twilio-go/rest/verify/v2"
)

// TwilioVerifyClient sends and validates registration OTPs via Twilio Verify (SMS).
// https://www.twilio.com/docs/verify/api
type TwilioVerifyClient struct {
	client     *twilio.RestClient
	serviceSid string
}

// NewTwilioVerifyClient builds a client for Twilio Verify v2. accountSid and authToken are API credentials.
func NewTwilioVerifyClient(accountSid, authToken, verifyServiceSid string) *TwilioVerifyClient {
	accountSid = strings.TrimSpace(accountSid)
	authToken = strings.TrimSpace(authToken)
	verifyServiceSid = strings.TrimSpace(verifyServiceSid)
	if accountSid == "" || authToken == "" || verifyServiceSid == "" {
		return &TwilioVerifyClient{}
	}
	c := twilio.NewRestClientWithParams(twilio.ClientParams{
		Username: accountSid,
		Password: authToken,
	})
	return &TwilioVerifyClient{
		client:     c,
		serviceSid: verifyServiceSid,
	}
}

// Enabled is true when the client can call Twilio Verify.
func (c *TwilioVerifyClient) Enabled() bool {
	return c != nil && c.client != nil && c.serviceSid != ""
}

// StartVerification sends an SMS OTP via Twilio Verify. toE164 must include leading + (e.g. +919876543210).
// Returns the Verification SID (VE...) to store for the check step.
func (c *TwilioVerifyClient) StartVerification(ctx context.Context, toE164 string) (verificationSid string, err error) {
	_ = ctx // Twilio SDK does not accept context on these calls
	if !c.Enabled() {
		return "", errors.New("twilio verify is not configured")
	}
	toE164 = strings.TrimSpace(toE164)
	if toE164 == "" {
		return "", errors.New("phone number is required")
	}
	params := &openapi.CreateVerificationParams{}
	params.SetTo(toE164)
	params.SetChannel("sms")

	resp, err := c.client.VerifyV2.CreateVerification(c.serviceSid, params)
	if err != nil {
		return "", err
	}
	if resp == nil || resp.Sid == nil || *resp.Sid == "" {
		return "", fmt.Errorf("twilio verify: empty verification sid in response")
	}
	if resp.Status != nil {
		st := strings.ToLower(strings.TrimSpace(*resp.Status))
		if st != "" {
			// pending = SMS/voice in progress; approved is rare on create
			switch st {
			case "pending", "approved":
				// ok
			default:
				return "", fmt.Errorf("twilio verify: verification could not be sent (status=%q sid=%s)", *resp.Status, *resp.Sid)
			}
		}
	}
	stLog := ""
	if resp.Status != nil {
		stLog = *resp.Status
	}
	masked := maskE164ForLog(toE164)
	log.Printf("twilio verify: verification started to=%s status=%s sid=%s", masked, stLog, *resp.Sid)
	return *resp.Sid, nil
}

// MapStartVerificationError turns Twilio SDK errors into clearer API messages for clients.
func MapStartVerificationError(err error) error {
	if err == nil {
		return nil
	}
	msg := err.Error()
	// https://www.twilio.com/docs/api/errors
	switch {
	case strings.Contains(msg, "21608"):
		return errors.New("cannot send SMS to this number on a Twilio trial: verify the number in Twilio Console (Phone Numbers → Verified Caller IDs) or upgrade the account — https://www.twilio.com/docs/errors/21608")
	default:
		return fmt.Errorf("failed to send verification SMS: %w", err)
	}
}

func maskE164ForLog(e164 string) string {
	e164 = strings.TrimSpace(e164)
	if len(e164) <= 4 {
		return "****"
	}
	return "…" + e164[len(e164)-4:]
}

// CheckVerification validates the OTP using To + Code (same as Twilio REST VerificationCheck with To and Code).
func (c *TwilioVerifyClient) CheckVerification(ctx context.Context, toE164, code string) error {
	_ = ctx
	if !c.Enabled() {
		return errors.New("twilio verify is not configured")
	}
	toE164 = strings.TrimSpace(toE164)
	code = strings.TrimSpace(code)
	if toE164 == "" || code == "" {
		return errors.New("phone and code are required")
	}
	params := &openapi.CreateVerificationCheckParams{}
	params.SetTo(toE164)
	params.SetCode(code)

	check, err := c.client.VerifyV2.CreateVerificationCheck(c.serviceSid, params)
	if err != nil {
		return err
	}
	if check == nil {
		return errors.New("twilio verify: empty check response")
	}
	if check.Status != nil && strings.EqualFold(*check.Status, "approved") {
		return nil
	}
	if check.Valid != nil && *check.Valid {
		return nil
	}
	return errors.New("invalid verification code")
}
