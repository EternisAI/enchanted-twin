package models

import (
	"crypto/rand"
	"crypto/sha256"
	"fmt"
	"time"

	"gorm.io/gorm"
)

// InviteCode represents an invite code in the system
type InviteCode struct {
	ID         uint           `json:"id" gorm:"primaryKey"`
	Code       string         `json:"code,omitempty"`                // The actual code (not stored in DB)
	CodeHash   string         `json:"-" gorm:"uniqueIndex;not null"` // SHA256 hash of the code
	BoundEmail *string        `json:"bound_email,omitempty"`         // Optional email binding
	CreatedBy  uint           `json:"created_by"`
	IsUsed     bool           `json:"is_used" gorm:"default:false"`
	RedeemedBy *string        `json:"redeemed_by,omitempty"` // Email of redeemer
	RedeemedAt *time.Time     `json:"redeemed_at,omitempty"`
	ExpiresAt  *time.Time     `json:"expires_at,omitempty"`
	IsActive   bool           `json:"is_active" gorm:"default:true"`
	CreatedAt  time.Time      `json:"created_at"`
	UpdatedAt  time.Time      `json:"updated_at"`
	DeletedAt  gorm.DeletedAt `json:"-" gorm:"index"`
}

// GenerateNanoID creates a new nanoid with custom alphabet (no confusing characters)
func GenerateNanoID() (string, error) {
	// Custom alphabet excluding 0/O/1/I for clarity
	alphabet := "ABCDEFGHJKLMNPQRSTUVWXYZ23456789"
	length := 10

	bytes := make([]byte, length)
	if _, err := rand.Read(bytes); err != nil {
		return "", err
	}

	for i, b := range bytes {
		bytes[i] = alphabet[b%byte(len(alphabet))]
	}

	return string(bytes), nil
}

// HashCode creates SHA256 hash of the invite code
func HashCode(code string) string {
	hash := sha256.Sum256([]byte(code))
	return fmt.Sprintf("%x", hash)
}

// SetCodeAndHash generates a new code and sets both code and hash
func (ic *InviteCode) SetCodeAndHash() error {
	code, err := GenerateNanoID()
	if err != nil {
		return err
	}
	ic.Code = code
	ic.CodeHash = HashCode(code)
	return nil
}

// IsExpired checks if the invite code has expired
func (ic *InviteCode) IsExpired() bool {
	if ic.ExpiresAt == nil {
		return false
	}
	return time.Now().After(*ic.ExpiresAt)
}

// CanBeUsed checks if the invite code can still be used
func (ic *InviteCode) CanBeUsed() bool {
	return ic.IsActive && !ic.IsExpired() && !ic.IsUsed
}
