package database

import (
	"context"
	"time"

	"github.com/flurbudurbur/Shiori/internal/domain"
	"github.com/flurbudurbur/Shiori/internal/logger"
	"github.com/flurbudurbur/Shiori/pkg/errors"
	"github.com/rs/zerolog"
	"gorm.io/gorm"
)

// ProfileUUIDRepo implements the domain.ProfileUUIDRepo interface
type ProfileUUIDRepo struct {
	log zerolog.Logger
	db  *DB
}

// NewProfileUUIDRepo creates a new ProfileUUIDRepo
func NewProfileUUIDRepo(log logger.Logger, db *DB) domain.ProfileUUIDRepo {
	return &ProfileUUIDRepo{
		log: log.With().Str("repo", "profile_uuid").Logger(),
		db:  db,
	}
}

// FindByUserID retrieves a profile UUID by user ID
func (r *ProfileUUIDRepo) FindByUserID(ctx context.Context, userID string) (*domain.ProfileUUID, error) {
	var profileUUID domain.ProfileUUID
	result := r.db.Get().WithContext(ctx).Where("user_id = ?", userID).First(&profileUUID)

	if result.Error != nil {
		if errors.Is(result.Error, gorm.ErrRecordNotFound) {
			// Profile UUID not found is not necessarily an error
			return nil, nil
		}
		r.log.Error().Err(result.Error).Str("user_id", userID).Msg("Failed to find profile UUID by user ID")
		return nil, errors.Wrap(result.Error, "failed to find profile UUID by user ID")
	}

	return &profileUUID, nil
}

// Store saves a profile UUID to the persistent database
func (r *ProfileUUIDRepo) Store(ctx context.Context, profileUUID domain.ProfileUUID) error {
	// Ensure timestamps are set
	if profileUUID.CreatedAt.IsZero() {
		profileUUID.CreatedAt = time.Now()
	}
	if profileUUID.LastActivityAt.IsZero() {
		profileUUID.LastActivityAt = profileUUID.CreatedAt
	}

	result := r.db.Get().WithContext(ctx).Create(&profileUUID)

	if result.Error != nil {
		r.log.Error().Err(result.Error).
			Str("user_id", profileUUID.UserID).
			Str("profile_uuid", profileUUID.ProfileUUID).
			Msg("Failed to store profile UUID")
		return errors.Wrap(result.Error, "failed to store profile UUID")
	}

	r.log.Debug().
		Str("user_id", profileUUID.UserID).
		Str("profile_uuid", profileUUID.ProfileUUID).
		Msg("Successfully stored profile UUID")
	return nil
}

// UpdateLastActivity updates the last activity timestamp for a profile UUID
func (r *ProfileUUIDRepo) UpdateLastActivity(ctx context.Context, userID string, profileUUID string) error {
	now := time.Now()
	result := r.db.Get().WithContext(ctx).
		Model(&domain.ProfileUUID{}).
		Where("user_id = ? AND profile_uuid = ?", userID, profileUUID).
		Update("last_activity_at", now)

	if result.Error != nil {
		r.log.Error().Err(result.Error).
			Str("user_id", userID).
			Str("profile_uuid", profileUUID).
			Msg("Failed to update profile UUID last activity")
		return errors.Wrap(result.Error, "failed to update profile UUID last activity")
	}

	if result.RowsAffected == 0 {
		r.log.Warn().
			Str("user_id", userID).
			Str("profile_uuid", profileUUID).
			Msg("UpdateLastActivity affected 0 rows, profile UUID might not exist")
		return errors.Wrap(gorm.ErrRecordNotFound, "profile UUID %s for user %s not found", profileUUID, userID)
	}

	r.log.Debug().
		Str("user_id", userID).
		Str("profile_uuid", profileUUID).
		Time("last_activity_at", now).
		Msg("Successfully updated profile UUID last activity")
	return nil
}

// FindExpiredProfileUUIDs finds profile UUIDs that haven't been active for a specified duration
func (r *ProfileUUIDRepo) FindExpiredProfileUUIDs(ctx context.Context, olderThan time.Time) ([]domain.ProfileUUID, error) {
	var profileUUIDs []domain.ProfileUUID
	result := r.db.Get().WithContext(ctx).
		Where("last_activity_at < ?", olderThan).
		Find(&profileUUIDs)

	if result.Error != nil {
		r.log.Error().Err(result.Error).
			Time("older_than", olderThan).
			Msg("Failed to find expired profile UUIDs")
		return nil, errors.Wrap(result.Error, "failed to find expired profile UUIDs")
	}

	return profileUUIDs, nil
}

// FindStaleProfileUUIDs finds profile UUIDs that haven't been active for a specified duration
// and are candidates for cleanup
func (r *ProfileUUIDRepo) FindStaleProfileUUIDs(ctx context.Context, olderThan time.Time, limit int) ([]domain.ProfileUUID, error) {
	var profileUUIDs []domain.ProfileUUID
	result := r.db.Get().WithContext(ctx).
		Where("last_activity_at < ?", olderThan).
		Limit(limit).
		Find(&profileUUIDs)

	if result.Error != nil {
		r.log.Error().Err(result.Error).
			Time("older_than", olderThan).
			Int("limit", limit).
			Msg("Failed to find stale profile UUIDs")
		return nil, errors.Wrap(result.Error, "failed to find stale profile UUIDs")
	}

	r.log.Info().
		Time("older_than", olderThan).
		Int("found_count", len(profileUUIDs)).
		Msg("Found stale profile UUIDs")
	return profileUUIDs, nil
}

// FindOrphanedProfileUUIDs finds profile UUIDs whose associated user accounts no longer exist
func (r *ProfileUUIDRepo) FindOrphanedProfileUUIDs(ctx context.Context, limit int) ([]domain.ProfileUUID, error) {
	var profileUUIDs []domain.ProfileUUID

	// This query finds profile UUIDs where the user_id doesn't exist in the users table
	// The exact SQL might need adjustment based on your database schema and GORM setup
	result := r.db.Get().WithContext(ctx).
		Joins("LEFT JOIN users ON profile_uuids.user_id = users.id").
		Where("users.id IS NULL").
		Limit(limit).
		Find(&profileUUIDs)

	if result.Error != nil {
		r.log.Error().Err(result.Error).
			Int("limit", limit).
			Msg("Failed to find orphaned profile UUIDs")
		return nil, errors.Wrap(result.Error, "failed to find orphaned profile UUIDs")
	}

	r.log.Info().
		Int("found_count", len(profileUUIDs)).
		Msg("Found orphaned profile UUIDs")
	return profileUUIDs, nil
}

// DeleteProfileUUIDs deletes multiple profile UUIDs from the persistent database
func (r *ProfileUUIDRepo) DeleteProfileUUIDs(ctx context.Context, profileUUIDs []domain.ProfileUUID) (int, error) {
	if len(profileUUIDs) == 0 {
		return 0, nil
	}

	// Extract IDs for deletion
	var ids []int64
	for _, uuid := range profileUUIDs {
		ids = append(ids, uuid.ID)
	}

	// Delete by IDs
	result := r.db.Get().WithContext(ctx).
		Where("id IN ?", ids).
		Delete(&domain.ProfileUUID{})

	if result.Error != nil {
		r.log.Error().Err(result.Error).
			Int("count", len(profileUUIDs)).
			Msg("Failed to delete multiple profile UUIDs")
		return 0, errors.Wrap(result.Error, "failed to delete multiple profile UUIDs")
	}

	r.log.Info().
		Int64("deleted_count", result.RowsAffected).
		Msg("Successfully deleted multiple profile UUIDs")
	return int(result.RowsAffected), nil
}

// SoftDeleteProfileUUIDs marks multiple profile UUIDs as deleted without removing them
func (r *ProfileUUIDRepo) SoftDeleteProfileUUIDs(ctx context.Context, profileUUIDs []domain.ProfileUUID) (int, error) {
	if len(profileUUIDs) == 0 {
		return 0, nil
	}

	// Extract IDs for soft deletion
	var ids []int64
	for _, uuid := range profileUUIDs {
		ids = append(ids, uuid.ID)
	}

	// Update the deleted_at field (assuming your model has this field for soft deletes)
	// If your model doesn't have this field, you'll need to add it to the domain.ProfileUUID struct
	result := r.db.Get().WithContext(ctx).
		Model(&domain.ProfileUUID{}).
		Where("id IN ?", ids).
		Update("deleted_at", time.Now())

	if result.Error != nil {
		r.log.Error().Err(result.Error).
			Int("count", len(profileUUIDs)).
			Msg("Failed to soft delete multiple profile UUIDs")
		return 0, errors.Wrap(result.Error, "failed to soft delete multiple profile UUIDs")
	}

	r.log.Info().
		Int64("soft_deleted_count", result.RowsAffected).
		Msg("Successfully soft deleted multiple profile UUIDs")
	return int(result.RowsAffected), nil
}

// DeleteProfileUUID deletes a profile UUID from the persistent database
func (r *ProfileUUIDRepo) DeleteProfileUUID(ctx context.Context, userID string, profileUUID string) error {
	result := r.db.Get().WithContext(ctx).
		Where("user_id = ? AND profile_uuid = ?", userID, profileUUID).
		Delete(&domain.ProfileUUID{})

	if result.Error != nil {
		r.log.Error().Err(result.Error).
			Str("user_id", userID).
			Str("profile_uuid", profileUUID).
			Msg("Failed to delete profile UUID")
		return errors.Wrap(result.Error, "failed to delete profile UUID")
	}

	if result.RowsAffected == 0 {
		r.log.Warn().
			Str("user_id", userID).
			Str("profile_uuid", profileUUID).
			Msg("DeleteProfileUUID affected 0 rows, profile UUID might not exist")
		return errors.Wrap(gorm.ErrRecordNotFound, "profile UUID %s for user %s not found for deletion", profileUUID, userID)
	}

	r.log.Info().
		Str("user_id", userID).
		Str("profile_uuid", profileUUID).
		Msg("Successfully deleted profile UUID")
	return nil
}
