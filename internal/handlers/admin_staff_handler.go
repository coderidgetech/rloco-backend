package handlers

import (
	"crypto/rand"
	"math/big"
	"net/http"
	"strings"

	"rloco-backend/internal/models"

	"github.com/gin-gonic/gin"
	"golang.org/x/crypto/bcrypt"
)

const staffPasswordCharset = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"

func generateStaffPassword() (string, error) {
	const n = 16
	b := make([]byte, n)
	for i := range b {
		idx, err := rand.Int(rand.Reader, big.NewInt(int64(len(staffPasswordCharset))))
		if err != nil {
			return "", err
		}
		b[i] = staffPasswordCharset[idx.Int64()]
	}
	return string(b), nil
}

// CreateStaff creates an internal staff (first-party operations) account. Staff
// manage the house catalog + house orders only; the role's scope is enforced in
// the product/order handlers. The issued password must be reset on first login.
func (h *AdminHandler) CreateStaff(c *gin.Context) {
	var req struct {
		Name     string `json:"name" binding:"required"`
		Email    string `json:"email" binding:"required,email"`
		Password string `json:"password"` // optional; generated if blank
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	email := strings.ToLower(strings.TrimSpace(req.Email))
	if existing, _ := h.userRepo.GetByEmail(c.Request.Context(), email); existing != nil {
		c.JSON(http.StatusConflict, gin.H{"error": "a user with this email already exists"})
		return
	}

	password := strings.TrimSpace(req.Password)
	generated := false
	if password == "" {
		p, err := generateStaffPassword()
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to generate password"})
			return
		}
		password = p
		generated = true
	} else if len(password) < 8 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "password must be at least 8 characters"})
		return
	}

	hashed, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to hash password"})
		return
	}

	user := &models.User{
		Email:             email,
		PasswordHash:      string(hashed),
		Name:              strings.TrimSpace(req.Name),
		Role:              "staff",
		Active:            true,
		EmailVerified:     true, // admin-vetted address
		MustResetPassword: true, // force reset of the issued password on first login
	}
	if err := h.userRepo.Create(c.Request.Context(), user); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	resp := gin.H{"user": user}
	if generated {
		resp["temporary_password"] = password // surfaced so admin can share it (no email needed)
	}
	c.JSON(http.StatusCreated, resp)
}

// ListStaff returns internal staff accounts.
func (h *AdminHandler) ListStaff(c *gin.Context) {
	users, err := h.userRepo.List(c.Request.Context(), 1000, 0)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	staff := make([]*models.User, 0)
	for _, u := range users {
		if u.Role == "staff" {
			staff = append(staff, u)
		}
	}
	c.JSON(http.StatusOK, gin.H{"staff": staff, "total": len(staff)})
}
