package database

import (
	"context"
	"fmt" // Import fmt for error messages
	"time"

	"github.com/flurbudurbur/Shiori/internal/domain"
	"github.com/flurbudurbur/Shiori/internal/logger"
	"github.com/flurbudurbur/Shiori/pkg/errors"
	"github.com/rs/zerolog"
	"gorm.io/gorm"
)

type UserRepo struct {
	log zerolog.Logger
	db  *DB
}

func NewUserRepo(log logger.Logger, db *DB) domain.UserRepo {
	return &UserRepo{
		log: log.With().Str("repo", "user").Logger(),
		db:  db,
	}
}

func (r *UserRepo) GetUserCount(ctx context.Context) (int, error) {
	var count int64
	result := r.db.Get().WithContext(ctx).Model(&domain.User{}).Count(&count)

	if result.Error != nil {
		r.log.Error().Err(result.Error).Msg("Failed to get user count")
		return 0, errors.Wrap(result.Error, "failed to get user count")
	}

	return int(count), nil
}

func (r *UserRepo) FindByHashedUUID(ctx context.Context, hashedUUID string) (*domain.User, error) {
	var user domain.User
	result := r.db.Get().WithContext(ctx).Where("hashed_uuid = ?", hashedUUID).First(&user)

	if result.Error != nil {
		if errors.Is(result.Error, gorm.ErrRecordNotFound) {
			// User not found is not necessarily an application error, return nil, nil
			return nil, nil
		}
		// Log prefix for debugging without exposing full hash
		uuidPrefix := hashedUUID
		if len(uuidPrefix) > 8 {
			uuidPrefix = uuidPrefix[:8]
		}
		r.log.Error().Err(result.Error).Str("hashed_uuid_prefix", uuidPrefix).Msg("Failed to find user by hashed UUID")
		return nil, errors.Wrap(result.Error, "failed to find user by hashed UUID")
	}

	return &user, nil
}

func (r *UserRepo) Store(ctx context.Context, user domain.User) error {
	// GORM's Create will attempt to insert the user.
	// It uses the HashedUUID as the primary key.
	result := r.db.Get().WithContext(ctx).Create(&user)

	if result.Error != nil {
		// Consider checking for unique constraint errors (e.g., duplicate HashedUUID or APITokenHash)
		r.log.Error().Err(result.Error).Str("hashed_uuid", user.HashedUUID).Msg("Failed to store user")
		return errors.Wrap(result.Error, "failed to store user")
	}

	r.log.Debug().Str("hashed_uuid", user.HashedUUID).Msg("Successfully stored user")
	return nil
}

// FindByAPITokenHash finds a user by their API token hash.
func (r *UserRepo) FindByAPITokenHash(ctx context.Context, tokenHash string) (*domain.User, error) {
	var user domain.User
	result := r.db.Get().WithContext(ctx).Where("api_token_hash = ?", tokenHash).First(&user)

	if result.Error != nil {
		if errors.Is(result.Error, gorm.ErrRecordNotFound) {
			// Token hash not found is not necessarily an application error
			return nil, nil
		}
		// Log prefix for debugging without exposing full hash
		hashPrefix := tokenHash
		if len(hashPrefix) > 8 {
			hashPrefix = hashPrefix[:8]
		}
		r.log.Error().Err(result.Error).Str("token_hash_prefix", hashPrefix).Msg("Failed to find user by API token hash")
		return nil, errors.Wrap(result.Error, "failed to find user by API token hash")
	}

	return &user, nil
}

// UpdateDeletionDate updates the deletion date for a user identified by their hashed UUID.
func (r *UserRepo) UpdateDeletionDate(ctx context.Context, hashedUUID string, newDeletionDate time.Time) error {
	result := r.db.Get().WithContext(ctx).
		Model(&domain.User{}).                   // Specify the model
		Where("hashed_uuid = ?", hashedUUID).    // Condition using hashed UUID
		Update("deletion_date", newDeletionDate) // Update specific column

	if result.Error != nil {
		r.log.Error().Err(result.Error).Str("hashed_uuid", hashedUUID).Msg("Failed to update user deletion date")
		return errors.Wrap(result.Error, "failed to update user deletion date")
	}

	if result.RowsAffected == 0 {
		r.log.Warn().Str("hashed_uuid", hashedUUID).Msg("UpdateDeletionDate affected 0 rows, user HashedUUID might not exist")
		// Return an error indicating user not found
		return errors.Wrap(gorm.ErrRecordNotFound, fmt.Sprintf("user with hashed_uuid %s not found for deletion date update", hashedUUID))
	}

	r.log.Debug().Str("hashed_uuid", hashedUUID).Time("newDeletionDate", newDeletionDate).Msg("Successfully updated user deletion date")
	return nil
}

// FindExpiredUserIDs finds the HashedUUIDs of users whose deletion date has passed.
func (r *UserRepo) FindExpiredUserIDs(ctx context.Context, now time.Time) ([]string, error) {
	var userHashedUUIDs []string // Changed type to []string
	// Select only the 'hashed_uuid' column for efficiency
	result := r.db.Get().WithContext(ctx).
		Model(&domain.User{}).           // Specify the model
		Select("hashed_uuid").           // Select only the HashedUUID
		Where("deletion_date < ?", now). // Condition for expiration
		Find(&userHashedUUIDs)           // Find matching HashedUUIDs

	if result.Error != nil {
		r.log.Error().Err(result.Error).Msg("Failed to find expired user HashedUUIDs")
		return nil, errors.Wrap(result.Error, "failed to find expired user HashedUUIDs")
	}

	return userHashedUUIDs, nil
}

// DeleteUserAndAssociatedData deletes a user identified by their hashed UUID.
// Assumes ON DELETE CASCADE constraints handle associated data in other tables.
func (r *UserRepo) DeleteUserAndAssociatedData(ctx context.Context, hashedUUID string) error {
	db := r.db.Get().WithContext(ctx)

	// GORM's Delete needs a Where clause when not using the default 'ID' primary key.
	result := db.Where("hashed_uuid = ?", hashedUUID).Delete(&domain.User{})

	if result.Error != nil {
		r.log.Error().Err(result.Error).Str("hashed_uuid", hashedUUID).Msg("Failed to delete user")
		return errors.Wrap(result.Error, "failed to delete user")
	}

	if result.RowsAffected == 0 {
		r.log.Warn().Str("hashed_uuid", hashedUUID).Msg("DeleteUser query affected 0 rows, user HashedUUID might not exist")
		// Return error indicating user not found
		return errors.Wrap(gorm.ErrRecordNotFound, fmt.Sprintf("user with hashed_uuid %s not found for deletion", hashedUUID))
	}

	r.log.Info().Str("hashed_uuid", hashedUUID).Msg("Successfully deleted user (assuming CASCADE for related data)")
	return nil
}

// UpdateTokenAndScopes updates the API token hash and scopes for a user.
func (r *UserRepo) UpdateTokenAndScopes(ctx context.Context, hashedUUID string, apiTokenHash string, scopes string) error {
	result := r.db.Get().WithContext(ctx).
		Model(&domain.User{}).
		Where("hashed_uuid = ?", hashedUUID).
		Updates(map[string]interface{}{
			"api_token_hash": apiTokenHash,
			"scopes":         scopes,
		})

	if result.Error != nil {
		r.log.Error().Err(result.Error).Str("hashed_uuid", hashedUUID).Msg("Failed to update user token and scopes")
		return errors.Wrap(result.Error, "failed to update user token and scopes")
	}

	if result.RowsAffected == 0 {
		r.log.Warn().Str("hashed_uuid", hashedUUID).Msg("UpdateTokenAndScopes affected 0 rows, user HashedUUID might not exist")
		return errors.Wrap(gorm.ErrRecordNotFound, fmt.Sprintf("user with hashed_uuid %s not found for token and scopes update", hashedUUID))
	}

	r.log.Debug().Str("hashed_uuid", hashedUUID).Msg("Successfully updated user token and scopes")
	return nil
}
