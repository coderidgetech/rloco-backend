package handlers

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"rloco-backend/internal/models"
	"rloco-backend/internal/services"
)

type TaxHandler struct {
	taxService            services.TaxService
	stripeUS              services.StripeUSTax
	usFallbackTaxFraction float64 // same as ORDER_DEFAULT_TAX_RATE when Stripe/Mongo unavailable (e.g. preview)
}

func NewTaxHandler(taxService services.TaxService, stripeUS services.StripeUSTax, usFallbackTaxFraction float64) *TaxHandler {
	if usFallbackTaxFraction < 0 {
		usFallbackTaxFraction = 0
	}
	return &TaxHandler{taxService: taxService, stripeUS: stripeUS, usFallbackTaxFraction: usFallbackTaxFraction}
}

func (h *TaxHandler) Calculate(c *gin.Context) {
	var req struct {
		Country      string   `json:"country" binding:"required"`
		State        *string  `json:"state,omitempty"`
		City         *string  `json:"city,omitempty"`
		PostalCode   *string  `json:"postal_code,omitempty"`
		Subtotal     float64  `json:"subtotal"` // no "required": Go's validator treats 0 as missing, and 0 is a valid subtotal
		ShippingUSD  *float64 `json:"shipping_usd,omitempty"`
		AddressLine1 *string  `json:"address_line1,omitempty"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	state := ""
	if req.State != nil {
		state = *req.State
	}
	city := ""
	if req.City != nil {
		city = *req.City
	}
	postalCode := ""
	if req.PostalCode != nil {
		postalCode = *req.PostalCode
	}

	if services.CountryLooksUS(req.Country) && h.stripeUS != nil {
		shipUSD := 0.0
		if req.ShippingUSD != nil && *req.ShippingUSD > 0 {
			shipUSD = *req.ShippingUSD
		}
		addr := ""
		if req.AddressLine1 != nil {
			addr = *req.AddressLine1
		}
		taxAmount, err := h.stripeUS.Calculate(c.Request.Context(), req.Subtotal, shipUSD, models.ShippingInfo{
			Address: addr,
			City:    city,
			State:   state,
			ZipCode: postalCode,
			Country: req.Country,
		})
		if err == nil {
			effectivePct := 0.0
			if req.Subtotal > 0 {
				effectivePct = (taxAmount / req.Subtotal) * 100
			}
			c.JSON(http.StatusOK, gin.H{
				"tax_amount": taxAmount,
				"rate": &models.TaxRate{
					Country:  "United States",
					State:    req.State,
					City:     req.City,
					Rate:     effectivePct,
					TaxType:  "sales_tax_stripe",
					IsActive: true,
				},
			})
			return
		}
		// fall through to Mongo / defaults on Stripe errors (e.g. incomplete address)
	}

	taxAmount, rate, err := h.taxService.CalculateTax(c.Request.Context(), req.Country, state, city, postalCode, req.Subtotal)
	if err != nil {
		if services.CountryLooksUS(req.Country) && h.usFallbackTaxFraction >= 0 {
			taxAmt := req.Subtotal * h.usFallbackTaxFraction
			pct := h.usFallbackTaxFraction * 100
			c.JSON(http.StatusOK, gin.H{
				"tax_amount": taxAmt,
				"rate": &models.TaxRate{
					Country:  "United States",
					State:    req.State,
					City:     req.City,
					Rate:     pct,
					TaxType:  "sales_tax_estimate",
					IsActive: true,
				},
			})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"tax_amount": taxAmount,
		"rate":       rate,
	})
}

func (h *TaxHandler) List(c *gin.Context) {
	activeOnly := c.DefaultQuery("active_only", "true") == "true"

	rates, err := h.taxService.GetRates(c.Request.Context(), activeOnly)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, rates)
}

func (h *TaxHandler) Create(c *gin.Context) {
	var rate models.TaxRate
	if err := c.ShouldBindJSON(&rate); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	if err := h.taxService.Create(c.Request.Context(), &rate); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusCreated, rate)
}

func (h *TaxHandler) Update(c *gin.Context) {
	id, err := primitive.ObjectIDFromHex(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid tax rate ID"})
		return
	}

	var rate models.TaxRate
	if err := c.ShouldBindJSON(&rate); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	if err := h.taxService.Update(c.Request.Context(), id, &rate); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Tax rate updated successfully"})
}

func (h *TaxHandler) Delete(c *gin.Context) {
	id, err := primitive.ObjectIDFromHex(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid tax rate ID"})
		return
	}

	if err := h.taxService.Delete(c.Request.Context(), id); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Tax rate deleted successfully"})
}
