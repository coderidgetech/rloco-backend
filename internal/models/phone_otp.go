package models

import (
	"time"

	"go.mongodb.org/mongo-driver/bson/primitive"
)

// PhoneOTPChallenge stores registration OTP state: Twilio Verify Verification SID (VE...).
type PhoneOTPChallenge struct {
	ID         primitive.ObjectID `bson:"_id,omitempty" json:"id"`
	PhoneKey   string             `bson:"phone_key" json:"phone_key"`
	Purpose    string             `bson:"purpose" json:"purpose"` // "registration"
	VerifyID  string `bson:"verify_id,omitempty" json:"-"` // Twilio Verification SID (VE...)
	ExpiresAt  time.Time          `bson:"expires_at" json:"expires_at"`
	Attempts   int                `bson:"attempts" json:"attempts"`
	LastSentAt time.Time          `bson:"last_sent_at" json:"last_sent_at"`
	UpdatedAt  time.Time          `bson:"updated_at" json:"updated_at"`
}
