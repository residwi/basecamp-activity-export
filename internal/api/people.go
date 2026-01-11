package api

import (
	"context"

	"github.com/residwi/bcreview/internal/models"
)

// GetMyProfile returns the current user's profile
func (c *Client) GetMyProfile(ctx context.Context) (*models.PersonProfile, error) {
	var profile models.PersonProfile
	if err := c.get(ctx, "/my/profile.json", nil, &profile); err != nil {
		return nil, err
	}
	return &profile, nil
}
