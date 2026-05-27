package services

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"sync"
	"time"

	"rloco-backend/internal/config"
	"rloco-backend/internal/models"
)

// shiprocketClient handles India domestic shipments via Shiprocket API.
// Auth tokens expire every 24 h; the client re-authenticates automatically.
type shiprocketClient struct {
	email          string
	password       string
	baseURL        string
	pickupLocation string
	parcel         shiprocketParcel
	httpClient     *http.Client

	mu       sync.Mutex
	token    string
	tokenExp time.Time
}

type shiprocketParcel struct {
	Length string
	Width  string
	Height string
	Weight string // kg
}

// -- auth ----------------------------------------------------------------

type shiprocketAuthRequest struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

type shiprocketAuthResponse struct {
	Token string `json:"token"`
}

// -- create order --------------------------------------------------------

type shiprocketOrderItem struct {
	Name         string  `json:"name"`
	SKU          string  `json:"sku"`
	Units        int     `json:"units"`
	SellingPrice float64 `json:"selling_price"`
}

type shiprocketCreateOrderRequest struct {
	OrderID             string                `json:"order_id"`
	OrderDate           string                `json:"order_date"`
	PickupLocation      string                `json:"pickup_location"`
	BillingCustomerName string                `json:"billing_customer_name"`
	BillingLastName     string                `json:"billing_last_name"`
	BillingAddress      string                `json:"billing_address"`
	BillingCity         string                `json:"billing_city"`
	BillingPincode      string                `json:"billing_pincode"`
	BillingState        string                `json:"billing_state"`
	BillingCountry      string                `json:"billing_country"`
	BillingEmail        string                `json:"billing_email"`
	BillingPhone        string                `json:"billing_phone"`
	ShippingIsBilling   bool                  `json:"shipping_is_billing"`
	OrderItems          []shiprocketOrderItem `json:"order_items"`
	PaymentMethod       string                `json:"payment_method"`
	SubTotal            float64               `json:"sub_total"`
	Length              string                `json:"length"`
	Breadth             string                `json:"breadth"`
	Height              string                `json:"height"`
	Weight              string                `json:"weight"`
}

type shiprocketCreateOrderResponse struct {
	OrderID    int    `json:"order_id"`
	ShipmentID int    `json:"shipment_id"`
	Status     string `json:"status"`
	AWBCode    string `json:"awb_code"`
}

// -- generate AWB --------------------------------------------------------

type shiprocketAWBRequest struct {
	ShipmentID string `json:"shipment_id"`
	CourierID  string `json:"courier_id"` // empty = auto-assign recommended carrier
}

type shiprocketAWBResponseData struct {
	AWBCode     string `json:"awb_code"`
	CourierName string `json:"courier_name"`
}

type shiprocketAWBResponseInner struct {
	Data shiprocketAWBResponseData `json:"data"`
}

type shiprocketAWBResponse struct {
	AWBAssignStatus int                        `json:"awb_assign_status"`
	Response        shiprocketAWBResponseInner `json:"response"`
}

// -----------------------------------------------------------------------

func NewShiprocketClient(cfg *config.Config) *shiprocketClient {
	if cfg == nil || strings.TrimSpace(cfg.ShiprocketEmail) == "" || strings.TrimSpace(cfg.ShiprocketPassword) == "" {
		return nil
	}
	return &shiprocketClient{
		email:          strings.TrimSpace(cfg.ShiprocketEmail),
		password:       strings.TrimSpace(cfg.ShiprocketPassword),
		baseURL:        strings.TrimRight(strings.TrimSpace(cfg.ShiprocketBaseURL), "/"),
		pickupLocation: firstNonEmpty(strings.TrimSpace(cfg.ShiprocketPickupLocation), "Primary"),
		parcel: shiprocketParcel{
			Length: firstNonEmpty(strings.TrimSpace(cfg.ShiprocketParcelLength), "10"),
			Width:  firstNonEmpty(strings.TrimSpace(cfg.ShiprocketParcelWidth), "8"),
			Height: firstNonEmpty(strings.TrimSpace(cfg.ShiprocketParcelHeight), "4"),
			Weight: firstNonEmpty(strings.TrimSpace(cfg.ShiprocketParcelWeight), "0.5"),
		},
		httpClient: &http.Client{Timeout: 20 * time.Second},
	}
}

func (c *shiprocketClient) isConfigured() bool {
	return c != nil && c.email != "" && c.password != ""
}

// authenticate fetches (or reuses) a valid JWT token from Shiprocket.
func (c *shiprocketClient) authenticate(ctx context.Context) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.token != "" && time.Now().Before(c.tokenExp) {
		return nil
	}

	body, _ := json.Marshal(shiprocketAuthRequest{Email: c.email, Password: c.password})
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+"/auth/login", bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("shiprocket auth: %w", err)
	}
	defer resp.Body.Close()

	respBody, _ := io.ReadAll(resp.Body)
	if resp.StatusCode >= http.StatusBadRequest {
		return fmt.Errorf("shiprocket auth failed (%d): %s", resp.StatusCode, strings.TrimSpace(string(respBody)))
	}

	var authResp shiprocketAuthResponse
	if err := json.Unmarshal(respBody, &authResp); err != nil || authResp.Token == "" {
		return fmt.Errorf("shiprocket auth: unexpected response")
	}

	c.token = authResp.Token
	c.tokenExp = time.Now().Add(23 * time.Hour) // tokens live 24 h; refresh 1 h early
	return nil
}

func (c *shiprocketClient) authHeader(ctx context.Context) (string, error) {
	if err := c.authenticate(ctx); err != nil {
		return "", err
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	return "Bearer " + c.token, nil
}

// CreateShipment creates a Shiprocket order and auto-assigns an AWB (tracking number).
// It returns the AWB code which serves as the shipment tracking number.
func (c *shiprocketClient) CreateShipment(ctx context.Context, order *models.Order) (awbCode string, err error) {
	if !c.isConfigured() {
		return "", fmt.Errorf("shiprocket is not configured")
	}

	authHeader, err := c.authHeader(ctx)
	if err != nil {
		return "", err
	}

	items := make([]shiprocketOrderItem, 0, len(order.Items))
	for i, item := range order.Items {
		sku := fmt.Sprintf("%s-%s", item.ProductID.Hex(), item.Size)
		items = append(items, shiprocketOrderItem{
			Name:         item.ProductName,
			SKU:          sku,
			Units:        item.Quantity,
			SellingPrice: item.Price,
		})
		_ = i
	}

	si := order.ShippingInfo
	createReq := shiprocketCreateOrderRequest{
		OrderID:             order.OrderNumber,
		OrderDate:           order.CreatedAt.Format("2006-01-02 15:04"),
		PickupLocation:      c.pickupLocation,
		BillingCustomerName: si.FirstName,
		BillingLastName:     si.LastName,
		BillingAddress:      si.Address,
		BillingCity:         si.City,
		BillingPincode:      si.ZipCode,
		BillingState:        si.State,
		BillingCountry:      si.Country,
		BillingEmail:        si.Email,
		BillingPhone:        si.Phone,
		ShippingIsBilling:   true,
		OrderItems:          items,
		PaymentMethod:       "Prepaid",
		SubTotal:            order.Subtotal,
		Length:              c.parcel.Length,
		Breadth:             c.parcel.Width,
		Height:              c.parcel.Height,
		Weight:              c.parcel.Weight,
	}

	body, err := json.Marshal(createReq)
	if err != nil {
		return "", err
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+"/orders/create/adhoc", bytes.NewReader(body))
	if err != nil {
		return "", err
	}
	req.Header.Set("Authorization", authHeader)
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("shiprocket create order: %w", err)
	}
	defer resp.Body.Close()

	respBody, _ := io.ReadAll(resp.Body)
	if resp.StatusCode >= http.StatusBadRequest {
		return "", fmt.Errorf("shiprocket create order failed (%d): %s", resp.StatusCode, strings.TrimSpace(string(respBody)))
	}

	var createResp shiprocketCreateOrderResponse
	if err := json.Unmarshal(respBody, &createResp); err != nil {
		return "", fmt.Errorf("shiprocket create order: parse error: %w", err)
	}
	if createResp.ShipmentID == 0 {
		return "", fmt.Errorf("shiprocket create order: no shipment_id in response")
	}

	// If AWB was already assigned during creation, return it directly
	if createResp.AWBCode != "" {
		return createResp.AWBCode, nil
	}

	// Otherwise generate AWB (auto-assigns recommended carrier)
	return c.generateAWB(ctx, authHeader, fmt.Sprintf("%d", createResp.ShipmentID))
}

func (c *shiprocketClient) generateAWB(ctx context.Context, authHeader, shipmentID string) (string, error) {
	awbReq := shiprocketAWBRequest{ShipmentID: shipmentID, CourierID: ""}
	body, _ := json.Marshal(awbReq)

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+"/courier/generate/awb", bytes.NewReader(body))
	if err != nil {
		return "", err
	}
	req.Header.Set("Authorization", authHeader)
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("shiprocket generate AWB: %w", err)
	}
	defer resp.Body.Close()

	respBody, _ := io.ReadAll(resp.Body)
	if resp.StatusCode >= http.StatusBadRequest {
		return "", fmt.Errorf("shiprocket generate AWB failed (%d): %s", resp.StatusCode, strings.TrimSpace(string(respBody)))
	}

	var awbResp shiprocketAWBResponse
	if err := json.Unmarshal(respBody, &awbResp); err != nil {
		return "", fmt.Errorf("shiprocket generate AWB: parse error: %w", err)
	}
	if awbResp.AWBAssignStatus != 1 || awbResp.Response.Data.AWBCode == "" {
		return "", fmt.Errorf("shiprocket AWB assignment failed (status %d)", awbResp.AWBAssignStatus)
	}

	return awbResp.Response.Data.AWBCode, nil
}
