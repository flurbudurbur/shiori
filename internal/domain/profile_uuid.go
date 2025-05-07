package domain

import (
	"context"
	"time"
)

// ProfileUUIDRepo defines the interface for interacting with persistent profile UUIDs
type ProfileUUIDRepo interface {
	// FindByUserID retrieves a profile UUID by user ID
	FindByUserID(ctx context.Context, userID string) (*ProfileUUID, error)
	
	// Store saves a profile UUID to the persistent database
	Store(ctx context.Context, profileUUID ProfileUUID) error
	
	// UpdateLastActivity updates the last activity timestamp for a profile UUID
	UpdateLastActivity(ctx context.Context, userID string, profileUUID string) error
	
	// FindExpiredProfileUUIDs finds profile UUIDs that haven't been active for a specified duration
	FindExpiredProfileUUIDs(ctx context.Context, olderThan time.Time) ([]ProfileUUID, error)
	
	// FindStaleProfileUUIDs finds profile UUIDs that haven't been active for a specified duration
	// and are candidates for cleanup
	FindStaleProfileUUIDs(ctx context.Context, olderThan time.Time, limit int) ([]ProfileUUID, error)
	
	// FindOrphanedProfileUUIDs finds profile UUIDs whose associated user accounts no longer exist
	FindOrphanedProfileUUIDs(ctx context.Context, limit int) ([]ProfileUUID, error)
	
	// DeleteProfileUUIDs deletes multiple profile UUIDs from the persistent database
	DeleteProfileUUIDs(ctx context.Context, profileUUIDs []ProfileUUID) (int, error)
	
	// SoftDeleteProfileUUIDs marks multiple profile UUIDs as deleted without removing them
	SoftDeleteProfileUUIDs(ctx context.Context, profileUUIDs []ProfileUUID) (int, error)
	
	// DeleteProfileUUID deletes a profile UUID from the persistent database
	DeleteProfileUUID(ctx context.Context, userID string, profileUUID string) error
}

// ProfileUUID represents a persistent profile UUID in the system
type ProfileUUID struct {
	ID             int64      `json:"-" gorm:"primaryKey;autoIncrement"`
	UserID         string     `json:"user_id" gorm:"column:user_id;index"`
	ProfileUUID    string     `json:"profile_uuid" gorm:"column:profile_uuid;uniqueIndex"`
	CreatedAt      time.Time  `json:"created_at" gorm:"column:created_at"`
	LastActivityAt time.Time  `json:"last_activity_at" gorm:"column:last_activity_at"`
	DeletedAt      *time.Time `json:"deleted_at,omitempty" gorm:"column:deleted_at;index"`
}

// TableName specifies the database table name for the ProfileUUID model
func (ProfileUUID) TableName() string {
	return "profile_uuids"
}