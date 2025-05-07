package domain

import (
	"context"
	"time"
)

type SyncRepo interface {
	// Get etag of sync data.
	// For avoid memory usage, only the etag will be returned.
	GetSyncDataETag(ctx context.Context, userHashedUUID string) (*string, error) // Changed apiKey to userHashedUUID
	// Get sync data and etag
	GetSyncDataAndETag(ctx context.Context, userHashedUUID string) ([]byte, *string, error) // Changed apiKey to userHashedUUID
	// Create or replace sync data, returns the new etag.
	SetSyncData(ctx context.Context, userHashedUUID string, data []byte) (*string, error) // Changed apiKey to userHashedUUID
	// Replace sync data only if the etag matches,
	// returns the new etag if updated, or nil if not.
	SetSyncDataIfMatch(ctx context.Context, userHashedUUID string, etag string, data []byte) (*string, error) // Changed apiKey to userHashedUUID
}

// SyncData represents the synchronization data for a user.
type SyncData struct {
	UserHashedUUID string    `json:"user_hashed_uuid" gorm:"primaryKey;column:user_hashed_uuid"`
	Data           []byte    `json:"data" gorm:"column:data"`
	ETag           string    `json:"etag" gorm:"column:etag"`
	CreatedAt      time.Time `json:"created_at" gorm:"column:created_at;autoCreateTime"`
	UpdatedAt      time.Time `json:"updated_at" gorm:"column:updated_at;autoUpdateTime"`
	User           User      `json:"-" gorm:"foreignKey:UserHashedUUID;references:HashedUUID"` // Foreign key to User
}
