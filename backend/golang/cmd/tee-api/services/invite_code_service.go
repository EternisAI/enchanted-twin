package services

import (
	"errors"
	"time"

	"github.com/eternisai/enchanted-twin/backend/golang/cmd/tee-api/models"
	"gorm.io/gorm"
)

type InviteCodeService struct {
	db *gorm.DB
}

func NewInviteCodeService(db *gorm.DB) *InviteCodeService {
	return &InviteCodeService{db: db}
}

func (s *InviteCodeService) CreateInviteCode(inviteCode *models.InviteCode) error {
	return s.db.Create(inviteCode).Error
}

// GetAllInviteCodes returns all invite codes
func (s *InviteCodeService) GetAllInviteCodes() ([]models.InviteCode, error) {
	var inviteCodes []models.InviteCode
	err := s.db.Find(&inviteCodes).Error
	return inviteCodes, err
}

// GetInviteCodeByCode returns an invite code by its code (using hash lookup)
func (s *InviteCodeService) GetInviteCodeByCode(code string) (*models.InviteCode, error) {
	codeHash := models.HashCode(code)
	var inviteCode models.InviteCode
	err := s.db.Model(&models.InviteCode{}).Where("code_hash = ?", codeHash).First(&inviteCode).Error
	if err != nil {
		return nil, err
	}
	// Set the original code for response (not stored in DB)
	inviteCode.Code = code
	return &inviteCode, nil
}

// GetInviteCodeByID returns an invite code by its ID
func (s *InviteCodeService) GetInviteCodeByID(id uint) (*models.InviteCode, error) {
	var inviteCode models.InviteCode
	err := s.db.Model(&models.InviteCode{}).Where("id = ?", id).First(&inviteCode).Error
	if err != nil {
		return nil, err
	}
	return &inviteCode, nil
}

// UseInviteCode marks an invite code as used by a user email
func (s *InviteCodeService) UseInviteCode(code string, email string) error {
	return s.db.Transaction(func(tx *gorm.DB) error {
		// Check if invite code ends with "-eternis" for special whitelisting
		isEternisCode := len(code) > 8 && code[len(code)-8:] == "-eternis"

		if isEternisCode {
			inviteCode := models.InviteCode{}
			inviteCode.SetCodeAndHash()
			inviteCode.IsUsed = true
			inviteCode.RedeemedBy = &email
			now := time.Now()
			inviteCode.RedeemedAt = &now
			inviteCode.IsActive = true
			inviteCode.CreatedBy = 0
			return tx.Create(&inviteCode).Error

		}

		// For regular codes, follow normal flow
		codeHash := models.HashCode(code)
		var inviteCode models.InviteCode
		if err := tx.Where("code_hash = ?", codeHash).First(&inviteCode).Error; err != nil {
			return err
		}

		// Check if the invite code can be used
		if !inviteCode.CanBeUsed() {
			if inviteCode.IsExpired() {
				return errors.New("invite code has expired")
			}
			if !inviteCode.IsActive {
				return errors.New("invite code is inactive")
			}
			if inviteCode.IsUsed {
				return errors.New("invite code already used")
			}
		}

		// Check if code is bound to a specific email
		if inviteCode.BoundEmail != nil && *inviteCode.BoundEmail != email {
			return errors.New("code bound to a different email")
		}

		// Update the invite code
		now := time.Now()
		inviteCode.IsUsed = true
		inviteCode.RedeemedBy = &email
		inviteCode.RedeemedAt = &now

		return tx.Save(&inviteCode).Error
	})
}

// DeleteInviteCode soft deletes an invite code
func (s *InviteCodeService) DeleteInviteCode(id uint) error {
	return s.db.Model(&models.InviteCode{}).Where("id = ?", id).Delete(&models.InviteCode{}).Error
}

// DeactivateInviteCode deactivates an invite code
func (s *InviteCodeService) DeactivateInviteCode(id uint) error {
	return s.db.Model(&models.InviteCode{}).Where("id = ?", id).Updates(map[string]interface{}{"is_active": false}).Error
}

// IsEmailWhitelisted checks if an email has valid invite codes (is whitelisted)
func (s *InviteCodeService) IsEmailWhitelisted(email string) (bool, error) {
	var count int64
	err := s.db.Model(&models.InviteCode{}).
		Where("redeemed_by = ?", email).
		Count(&count).Error
	if err != nil {
		return false, err
	}

	return count > 0, nil
}

// ResetInviteCode resets an invite code by clearing redeemed_by, redeemed_at and setting is_used to false
func (s *InviteCodeService) ResetInviteCode(code string) error {
	return s.db.Transaction(func(tx *gorm.DB) error {
		var inviteCode models.InviteCode
		if err := tx.Where("code_hash = ?", models.HashCode(code)).First(&inviteCode).Error; err != nil {
			return err
		}

		// Reset the invite code
		inviteCode.IsUsed = false
		inviteCode.RedeemedBy = nil
		inviteCode.RedeemedAt = nil

		return tx.Save(&inviteCode).Error
	})
}
