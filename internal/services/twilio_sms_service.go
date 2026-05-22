package services

import (
	"context"
	"encoding/base64"
	"fmt"
	"net/http"
	"net/url"
	"strings"
)

// TwilioSMSService sends transactional SMS via Twilio Programmable Messaging.
// It is distinct from TwilioVerifyClient which is used only for OTP.
type TwilioSMSService struct {
	accountSid  string
	authToken   string
	fromNumber  string
	httpClient  *http.Client
}

func NewTwilioSMSService(accountSid, authToken, fromNumber string) *TwilioSMSService {
	return &TwilioSMSService{
		accountSid: accountSid,
		authToken:  authToken,
		fromNumber: fromNumber,
		httpClient: &http.Client{},
	}
}

func (s *TwilioSMSService) Enabled() bool {
	return strings.TrimSpace(s.accountSid) != "" &&
		strings.TrimSpace(s.authToken) != "" &&
		strings.TrimSpace(s.fromNumber) != ""
}

func (s *TwilioSMSService) send(ctx context.Context, to, body string) error {
	if !s.Enabled() {
		return nil
	}
	apiURL := fmt.Sprintf("https://api.twilio.com/2010-04-01/Accounts/%s/Messages.json", s.accountSid)
	data := url.Values{}
	data.Set("To", to)
	data.Set("From", s.fromNumber)
	data.Set("Body", body)

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, apiURL, strings.NewReader(data.Encode()))
	if err != nil {
		return err
	}
	creds := base64.StdEncoding.EncodeToString([]byte(s.accountSid + ":" + s.authToken))
	req.Header.Set("Authorization", "Basic "+creds)
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	resp, err := s.httpClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 400 {
		return fmt.Errorf("twilio SMS error: HTTP %d", resp.StatusCode)
	}
	return nil
}

func (s *TwilioSMSService) SendOrderConfirmation(ctx context.Context, toPhone, orderNumber string) error {
	msg := fmt.Sprintf("Your Rloko order %s has been confirmed! We'll notify you when it ships.", orderNumber)
	return s.send(ctx, toPhone, msg)
}

func (s *TwilioSMSService) SendShippingNotification(ctx context.Context, toPhone, orderNumber, trackingNumber string) error {
	msg := fmt.Sprintf("Your Rloko order %s has shipped! Tracking: %s", orderNumber, trackingNumber)
	return s.send(ctx, toPhone, msg)
}

func (s *TwilioSMSService) SendOrderDelivered(ctx context.Context, toPhone, orderNumber string) error {
	msg := fmt.Sprintf("Your Rloko order %s has been delivered. Enjoy! Need help? Visit rloko.com/support", orderNumber)
	return s.send(ctx, toPhone, msg)
}

func (s *TwilioSMSService) SendReturnApproved(ctx context.Context, toPhone, returnID string) error {
	msg := fmt.Sprintf("Your Rloko return request %s has been approved. Your refund will be processed shortly.", returnID)
	return s.send(ctx, toPhone, msg)
}
