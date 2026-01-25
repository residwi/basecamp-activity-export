package basecamp

import (
	"context"
	"time"
)

type Person struct {
	ID           int64
	Name         string
	EmailAddress string
	Admin        bool
	Owner        bool
	CreatedAt    time.Time
	UpdatedAt    time.Time
}

// GetMyProfile returns the current user's profile
func (c *Client) GetMyProfile(ctx context.Context) (Person, error) {
	var person Person
	if err := c.Get(ctx, "/my/profile.json", nil, &person); err != nil {
		return Person{}, err
	}
	return person, nil
}
