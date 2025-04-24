package db

import (
	"context"

	"github.com/EternisAI/enchanted-twin/graph/model"
)

// GetUserProfile retrieves the user profile
func (s *Store) GetUserProfile(ctx context.Context) (*model.UserProfile, error) {
	var profile struct {
		Name *string `db:"name"`
		Bio  *string `db:"bio"`
	}
	err := s.db.GetContext(ctx, &profile, `SELECT name, bio FROM user_profiles WHERE id = 'default'`)
	if err != nil {
		return nil, err
	}
	return &model.UserProfile{
		Name: profile.Name,
		Bio:  profile.Bio,
	}, nil
}

// UpdateUserProfile updates the user profile
func (s *Store) UpdateUserProfile(ctx context.Context, input model.UpdateProfileInput) (bool, error) {
	result, err := s.db.ExecContext(ctx, `
		UPDATE user_profiles SET name = ?, bio = ? WHERE id = 'default'
	`, input.Name, input.Bio)
	if err != nil {
		return false, err
	}

	rows, err := result.RowsAffected()
	if err != nil {
		return false, err
	}

	return rows > 0, nil
}
