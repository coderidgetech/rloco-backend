package middleware

import (
	"github.com/gin-gonic/gin"
	"go.mongodb.org/mongo-driver/bson/primitive"
)

// GetUserIDFromContext extracts user ID from context
func GetUserIDFromContext(c *gin.Context) (primitive.ObjectID, error) {
	userID, exists := c.Get("user_id")
	if !exists {
		return primitive.NilObjectID, gin.Error{Err: nil, Type: gin.ErrorTypePublic, Meta: "User ID not found in context"}
	}

	userIDStr, ok := userID.(string)
	if !ok {
		return primitive.NilObjectID, gin.Error{Err: nil, Type: gin.ErrorTypePublic, Meta: "Invalid user ID type"}
	}

	id, err := primitive.ObjectIDFromHex(userIDStr)
	if err != nil {
		return primitive.NilObjectID, gin.Error{Err: err, Type: gin.ErrorTypePublic, Meta: "Invalid user ID format"}
	}

	return id, nil
}

// GetVendorIDFromContext extracts vendor ID from context
func GetVendorIDFromContext(c *gin.Context) *primitive.ObjectID {
	vendorID, exists := c.Get("vendor_id")
	if !exists {
		return nil // Not all users have vendor_id
	}

	vendorIDObj, ok := vendorID.(*primitive.ObjectID)
	if ok {
		return vendorIDObj
	}

	// Try string conversion
	vendorIDStr, ok := vendorID.(string)
	if ok {
		id, err := primitive.ObjectIDFromHex(vendorIDStr)
		if err == nil {
			return &id
		}
	}

	return nil
}
