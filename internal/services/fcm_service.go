package services

import (
	"bytes"
	"context"
	"crypto/rsa"
	"crypto/x509"
	"encoding/json"
	"encoding/pem"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

const (
	fcmV1Endpoint  = "https://fcm.googleapis.com/v1/projects/%s/messages:send"
	googleTokenURL = "https://oauth2.googleapis.com/token"
	fcmScope       = "https://www.googleapis.com/auth/firebase.messaging"
)

// FCMService sends push notifications via Firebase Cloud Messaging HTTP v1 API.
// Authenticates using a service account JSON (FCM_SERVICE_ACCOUNT_JSON env var).
type FCMService struct {
	projectID   string
	clientEmail string
	privateKey  *rsa.PrivateKey
	httpClient  *http.Client

	mu          sync.Mutex
	cachedToken string
	tokenExpiry time.Time
}

func NewFCMService(serviceAccountJSON string) (*FCMService, error) {
	if strings.TrimSpace(serviceAccountJSON) == "" {
		return &FCMService{httpClient: &http.Client{}}, nil // disabled
	}

	var sa struct {
		ProjectID   string `json:"project_id"`
		ClientEmail string `json:"client_email"`
		PrivateKey  string `json:"private_key"`
	}
	if err := json.Unmarshal([]byte(serviceAccountJSON), &sa); err != nil {
		return nil, fmt.Errorf("FCM: invalid service account JSON: %w", err)
	}
	if sa.ProjectID == "" || sa.ClientEmail == "" || sa.PrivateKey == "" {
		return nil, fmt.Errorf("FCM: service account JSON missing project_id, client_email, or private_key")
	}

	block, _ := pem.Decode([]byte(sa.PrivateKey))
	if block == nil {
		return nil, fmt.Errorf("FCM: failed to decode private key PEM block")
	}
	key, err := x509.ParsePKCS8PrivateKey(block.Bytes)
	if err != nil {
		return nil, fmt.Errorf("FCM: failed to parse private key: %w", err)
	}
	rsaKey, ok := key.(*rsa.PrivateKey)
	if !ok {
		return nil, fmt.Errorf("FCM: private key is not RSA")
	}

	return &FCMService{
		projectID:   sa.ProjectID,
		clientEmail: sa.ClientEmail,
		privateKey:  rsaKey,
		httpClient:  &http.Client{},
	}, nil
}

func (s *FCMService) Enabled() bool {
	return s.privateKey != nil
}

// accessToken returns a cached OAuth 2.0 access token, refreshing if needed.
func (s *FCMService) accessToken(ctx context.Context) (string, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.cachedToken != "" && time.Now().Before(s.tokenExpiry) {
		return s.cachedToken, nil
	}

	now := time.Now()
	claims := jwt.MapClaims{
		"iss":   s.clientEmail,
		"sub":   s.clientEmail,
		"scope": fcmScope,
		"aud":   googleTokenURL,
		"iat":   now.Unix(),
		"exp":   now.Add(time.Hour).Unix(),
	}
	tok := jwt.NewWithClaims(jwt.SigningMethodRS256, claims)
	signed, err := tok.SignedString(s.privateKey)
	if err != nil {
		return "", fmt.Errorf("FCM: JWT sign error: %w", err)
	}

	resp, err := s.httpClient.PostForm(googleTokenURL, url.Values{
		"grant_type": {"urn:ietf:params:oauth:grant-type:jwt-bearer"},
		"assertion":  {signed},
	})
	if err != nil {
		return "", fmt.Errorf("FCM: token exchange request error: %w", err)
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	var result struct {
		AccessToken string `json:"access_token"`
		ExpiresIn   int    `json:"expires_in"`
	}
	if err := json.Unmarshal(body, &result); err != nil || result.AccessToken == "" {
		return "", fmt.Errorf("FCM: token exchange failed: %s", string(body))
	}

	s.cachedToken = result.AccessToken
	// Expire 60s early to avoid edge-case expiry during a request
	s.tokenExpiry = now.Add(time.Duration(result.ExpiresIn-60) * time.Second)
	return s.cachedToken, nil
}

type fcmV1Request struct {
	Message fcmV1Message `json:"message"`
}

type fcmV1Message struct {
	Token        string             `json:"token,omitempty"`
	Notification *fcmV1Notification `json:"notification,omitempty"`
	Data         map[string]string  `json:"data,omitempty"`
}

type fcmV1Notification struct {
	Title string `json:"title"`
	Body  string `json:"body"`
}

func (s *FCMService) sendOne(ctx context.Context, msg fcmV1Message) error {
	if !s.Enabled() {
		return nil
	}
	accessTok, err := s.accessToken(ctx)
	if err != nil {
		return err
	}
	payload, err := json.Marshal(fcmV1Request{Message: msg})
	if err != nil {
		return err
	}
	endpoint := fmt.Sprintf(fcmV1Endpoint, s.projectID)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(payload))
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "Bearer "+accessTok)
	req.Header.Set("Content-Type", "application/json")

	resp, err := s.httpClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 400 {
		b, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("FCM v1 error %d: %s", resp.StatusCode, string(b))
	}
	return nil
}

func (s *FCMService) SendToToken(ctx context.Context, token, title, body string, data map[string]string) error {
	return s.sendOne(ctx, fcmV1Message{
		Token:        token,
		Notification: &fcmV1Notification{Title: title, Body: body},
		Data:         data,
	})
}

// SendToTokens sends to multiple tokens sequentially (FCM v1 has no multicast endpoint).
func (s *FCMService) SendToTokens(ctx context.Context, tokens []string, title, body string, data map[string]string) error {
	var lastErr error
	for _, t := range tokens {
		if err := s.SendToToken(ctx, t, title, body, data); err != nil {
			lastErr = err
		}
	}
	return lastErr
}
