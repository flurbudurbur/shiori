package domain

import (
	"context"
	"time"
)

type UserRepo interface {
	GetUserCount(ctx context.Context) (int, error)
	FindByHashedUUID(ctx context.Context, hashedUUID string) (*User, error)
	Store(ctx context.Context, user User) error
	// Update method removed from interface as its implementation was removed.
	UpdateDeletionDate(ctx context.Context, hashedUUID string, newDeletionDate time.Time) error // Changed userID to hashedUUID
	FindExpiredUserIDs(ctx context.Context, now time.Time) ([]string, error) // Changed []int to []string for HashedUUIDs
	FindByAPITokenHash(ctx context.Context, tokenHash string) (*User, error)
	DeleteUserAndAssociatedData(ctx context.Context, hashedUUID string) error // Changed userID to hashedUUID
	UpdateTokenAndScopes(ctx context.Context, hashedUUID string, apiTokenHash string, scopes string) error
}

// User represents a user in the system, identified by a hashed UUID.
type User struct {
	HashedUUID   string    `json:"-" gorm:"primaryKey;column:hashed_uuid"` // Primary Key
	APITokenHash string    `json:"-" gorm:"column:api_token_hash;unique"`  // Don't expose hash via JSON, ensure uniqueness
	Scopes       string    `json:"scopes" gorm:"column:scopes;type:jsonb"` // Store as JSONB
	DeletionDate time.Time `json:"deletion_date" gorm:"column:deletion_date"`
}
