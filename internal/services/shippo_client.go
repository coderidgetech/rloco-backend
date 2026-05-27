package services

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"

	"rloco-backend/internal/config"
	"rloco-backend/internal/models"

	"go.mongodb.org/mongo-driver/bson/primitive"
)

type shippoClient struct {
	apiKey     string
	baseURL    string
	httpClient *http.Client
	origin     shippoAddress
	parcel     shippoParcel
}

type shippoAddress struct {
	Name    string `json:"name"`
	Email   string `json:"email,omitempty"`
	Phone   string `json:"phone,omitempty"`
	Street1 string `json:"street1"`
	Street2 string `json:"street2,omitempty"`
	City    string `json:"city"`
	State   string `json:"state"`
	Zip     string `json:"zip"`
	Country string `json:"country"`
}

type shippoParcel struct {
	Length       string `json:"length"`
	Width        string `json:"width"`
	Height       string `json:"height"`
	DistanceUnit string `json:"distance_unit"`
	Weight       string `json:"weight"`
	MassUnit     string `json:"mass_unit"`
}

type shippoShipmentRequest struct {
	AddressFrom shippoAddress  `json:"address_from"`
	AddressTo   shippoAddress  `json:"address_to"`
	Parcels     []shippoParcel `json:"parcels"`
	Async       bool           `json:"async"`
}

type shippoServiceLevel struct {
	Name  string `json:"name"`
	Token string `json:"token"`
}

type shippoRate struct {
	ObjectID      string              `json:"object_id"`
	Amount        string              `json:"amount"`
	Currency      string              `json:"currency"`
	Provider      string              `json:"provider"`
	ServiceLevel  shippoServiceLevel  `json:"servicelevel"`
	EstimatedDays int                 `json:"estimated_days"`
	Messages      []shippoErrorDetail `json:"messages"`
}

type shippoErrorDetail struct {
	Code    string `json:"code"`
	Message string `json:"message"`
	Text    string `json:"text"`
}

type shippoShipmentResponse struct {
	Status   string              `json:"status"`
	Rates    []shippoRate        `json:"rates"`
	Messages []shippoErrorDetail `json:"messages"`
}

type shippoTransactionRequest struct {
	Rate          string `json:"rate"`
	LabelFileType string `json:"label_file_type"`
	Async         bool   `json:"async"`
}

type shippoTransactionResponse struct {
	Status         string `json:"status"`
	TrackingNumber string `json:"tracking_number"`
	LabelURL       string `json:"label_url"`
	Messages       []shippoErrorDetail `json:"messages"`
}

func NewShippoClient(cfg *config.Config) *shippoClient {
	if cfg == nil || strings.TrimSpace(cfg.ShippoAPIKey) == "" {
		return nil
	}

	return &shippoClient{
		apiKey:  strings.TrimSpace(cfg.ShippoAPIKey),
		baseURL: strings.TrimRight(strings.TrimSpace(cfg.ShippoBaseURL), "/"),
		httpClient: &http.Client{
			Timeout: 15 * time.Second,
		},
		origin: shippoAddress{
			Name:    strings.TrimSpace(cfg.ShippoFromName),
			Email:   strings.TrimSpace(cfg.ShippoFromEmail),
			Phone:   strings.TrimSpace(cfg.ShippoFromPhone),
			Street1: strings.TrimSpace(cfg.ShippoFromStreet1),
			Street2: strings.TrimSpace(cfg.ShippoFromStreet2),
			City:    strings.TrimSpace(cfg.ShippoFromCity),
			State:   strings.TrimSpace(cfg.ShippoFromState),
			Zip:     strings.TrimSpace(cfg.ShippoFromZip),
			Country: normalizeCountryCode(cfg.ShippoFromCountry),
		},
		parcel: shippoParcel{
			Length:       strings.TrimSpace(cfg.ShippoParcelLength),
			Width:        strings.TrimSpace(cfg.ShippoParcelWidth),
			Height:       strings.TrimSpace(cfg.ShippoParcelHeight),
			DistanceUnit: strings.TrimSpace(cfg.ShippoDistanceUnit),
			Weight:       strings.TrimSpace(cfg.ShippoParcelWeight),
			MassUnit:     strings.TrimSpace(cfg.ShippoMassUnit),
		},
	}
}

func (c *shippoClient) canQuote() bool {
	if c == nil {
		return false
	}

	return c.apiKey != "" &&
		c.baseURL != "" &&
		c.origin.Name != "" &&
		c.origin.Street1 != "" &&
		c.origin.City != "" &&
		c.origin.State != "" &&
		c.origin.Zip != "" &&
		c.origin.Country != "" &&
		c.parcel.Length != "" &&
		c.parcel.Width != "" &&
		c.parcel.Height != "" &&
		c.parcel.DistanceUnit != "" &&
		c.parcel.Weight != "" &&
		c.parcel.MassUnit != ""
}

func (c *shippoClient) parcelForQuote(weightLb *float64) shippoParcel {
	p := c.parcel
	if weightLb != nil && *weightLb > 0 {
		w := *weightLb
		if w < 0.01 {
			w = 0.01
		}
		p.Weight = strconv.FormatFloat(w, 'f', 2, 64)
	}
	return p
}

func (c *shippoClient) quoteRates(ctx context.Context, destination shippoAddress, weightLb *float64) ([]*models.ShippingMethod, error) {
	if !c.canQuote() {
		return nil, fmt.Errorf("shippo is not fully configured")
	}
	if !isCompleteShippoAddress(destination) {
		return nil, fmt.Errorf("destination address is incomplete for shippo quoting")
	}

	payload := shippoShipmentRequest{
		AddressFrom: c.origin,
		AddressTo:   destination,
		Parcels:     []shippoParcel{c.parcelForQuote(weightLb)},
		Async:       false,
	}

	body, err := json.Marshal(payload)
	if err != nil {
		return nil, err
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+"/shipments/", bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "ShippoToken "+c.apiKey)
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	responseBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	if resp.StatusCode >= http.StatusBadRequest {
		return nil, fmt.Errorf("shippo quote failed: %s", strings.TrimSpace(string(responseBody)))
	}

	var shipment shippoShipmentResponse
	if err := json.Unmarshal(responseBody, &shipment); err != nil {
		return nil, err
	}

	if len(shipment.Rates) == 0 {
		if len(shipment.Messages) > 0 {
			return nil, fmt.Errorf("shippo quote returned no rates: %s", firstShippoMessage(shipment.Messages))
		}
		return nil, fmt.Errorf("shippo quote returned no rates")
	}

	methods := make([]*models.ShippingMethod, 0, len(shipment.Rates))
	for _, rate := range shipment.Rates {
		amount, err := strconv.ParseFloat(rate.Amount, 64)
		if err != nil {
			continue
		}

		methods = append(methods, &models.ShippingMethod{
			ID:            primitive.NewObjectID(),
			Name:          firstNonEmpty(rate.ServiceLevel.Name, rate.Provider, "Shippo Rate"),
			Carrier:       normalizeCarrier(rate.Provider),
			Type:          deriveShippingType(rate.ServiceLevel.Token, rate.ServiceLevel.Name, rate.EstimatedDays),
			BaseCost:      amount,
			Currency:      strings.ToUpper(strings.TrimSpace(rate.Currency)),
			EstimatedDays: fallbackEstimatedDays(rate.EstimatedDays),
			Zones:         []models.ShippingZone{},
			IsActive:      true,
		})
	}

	if len(methods) == 0 {
		return nil, fmt.Errorf("shippo returned rates but none could be parsed")
	}

	sortShippingMethodsByCost(methods)
	return methods, nil
}

func isCompleteShippoAddress(address shippoAddress) bool {
	return strings.TrimSpace(address.Name) != "" &&
		strings.TrimSpace(address.Street1) != "" &&
		strings.TrimSpace(address.City) != "" &&
		strings.TrimSpace(address.State) != "" &&
		strings.TrimSpace(address.Zip) != "" &&
		strings.TrimSpace(address.Country) != ""
}

func normalizeCountryCode(country string) string {
	normalized := strings.ToUpper(strings.TrimSpace(country))
	switch normalized {
	case "INDIA":
		return "IN"
	case "UNITED STATES", "UNITED STATES OF AMERICA", "USA":
		return "US"
	default:
		return normalized
	}
}

func normalizeCarrier(provider string) string {
	normalized := strings.ToLower(strings.TrimSpace(provider))
	if normalized == "" {
		return "custom"
	}
	return normalized
}

func deriveShippingType(token, name string, estimatedDays int) string {
	value := strings.ToLower(strings.TrimSpace(token + " " + name))
	switch {
	case strings.Contains(value, "overnight"), strings.Contains(value, "next_day"), estimatedDays <= 1:
		return "overnight"
	case strings.Contains(value, "express"), strings.Contains(value, "priority"), estimatedDays <= 3:
		return "express"
	default:
		return "standard"
	}
}

func fallbackEstimatedDays(days int) int {
	if days > 0 {
		return days
	}
	return 5
}

func firstShippoMessage(messages []shippoErrorDetail) string {
	for _, message := range messages {
		if text := strings.TrimSpace(firstNonEmpty(message.Text, message.Message, message.Code)); text != "" {
			return text
		}
	}
	return "unknown error"
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if trimmed := strings.TrimSpace(value); trimmed != "" {
			return trimmed
		}
	}
	return ""
}

// purchaseLabel re-quotes rates for the destination and purchases the cheapest one,
// returning the tracking number and label PDF URL from Shippo.
func (c *shippoClient) purchaseLabel(ctx context.Context, destination shippoAddress, weightLb *float64) (trackingNumber, labelURL string, err error) {
	if !c.canQuote() {
		return "", "", fmt.Errorf("shippo is not fully configured")
	}

	payload := shippoShipmentRequest{
		AddressFrom: c.origin,
		AddressTo:   destination,
		Parcels:     []shippoParcel{c.parcelForQuote(weightLb)},
		Async:       false,
	}
	body, err := json.Marshal(payload)
	if err != nil {
		return "", "", err
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+"/shipments/", bytes.NewReader(body))
	if err != nil {
		return "", "", err
	}
	req.Header.Set("Authorization", "ShippoToken "+c.apiKey)
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return "", "", err
	}
	defer resp.Body.Close()

	respBody, _ := io.ReadAll(resp.Body)
	if resp.StatusCode >= http.StatusBadRequest {
		return "", "", fmt.Errorf("shippo shipment failed: %s", strings.TrimSpace(string(respBody)))
	}

	var shipment shippoShipmentResponse
	if err := json.Unmarshal(respBody, &shipment); err != nil {
		return "", "", err
	}
	if len(shipment.Rates) == 0 {
		return "", "", fmt.Errorf("shippo returned no rates for destination")
	}

	// pick cheapest rate
	best := shipment.Rates[0]
	bestCost, _ := strconv.ParseFloat(best.Amount, 64)
	for _, r := range shipment.Rates[1:] {
		if cost, err2 := strconv.ParseFloat(r.Amount, 64); err2 == nil && cost < bestCost {
			best = r
			bestCost = cost
		}
	}

	// purchase the label
	txPayload := shippoTransactionRequest{
		Rate:          best.ObjectID,
		LabelFileType: "PDF",
		Async:         false,
	}
	txBody, _ := json.Marshal(txPayload)

	txReq, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+"/transactions/", bytes.NewReader(txBody))
	if err != nil {
		return "", "", err
	}
	txReq.Header.Set("Authorization", "ShippoToken "+c.apiKey)
	txReq.Header.Set("Content-Type", "application/json")

	txResp, err := c.httpClient.Do(txReq)
	if err != nil {
		return "", "", err
	}
	defer txResp.Body.Close()

	txRespBody, _ := io.ReadAll(txResp.Body)
	if txResp.StatusCode >= http.StatusBadRequest {
		return "", "", fmt.Errorf("shippo transaction failed: %s", strings.TrimSpace(string(txRespBody)))
	}

	var tx shippoTransactionResponse
	if err := json.Unmarshal(txRespBody, &tx); err != nil {
		return "", "", err
	}
	if tx.Status != "SUCCESS" || tx.TrackingNumber == "" {
		msg := firstShippoMessage(tx.Messages)
		return "", "", fmt.Errorf("shippo label purchase failed: %s (status: %s)", msg, tx.Status)
	}

	return tx.TrackingNumber, tx.LabelURL, nil
}

func sortShippingMethodsByCost(methods []*models.ShippingMethod) {
	for i := 0; i < len(methods); i++ {
		for j := i + 1; j < len(methods); j++ {
			if methods[j].BaseCost < methods[i].BaseCost {
				methods[i], methods[j] = methods[j], methods[i]
			}
		}
	}
}
