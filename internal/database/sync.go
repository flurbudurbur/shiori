package database

import (
	"context"
	"time"

	// sq "github.com/Masterminds/squirrel" // No longer needed
	// "database/sql" // No longer needed
	"github.com/flurbudurbur/Shiori/internal/domain"
	"github.com/flurbudurbur/Shiori/internal/logger"
	"github.com/flurbudurbur/Shiori/pkg/errors"
	"github.com/google/uuid"
	"github.com/rs/zerolog"
	"gorm.io/gorm"
)

// SyncData represents the structure of the 'sync_data' table.
// Defined internally as it's an implementation detail of this repo.
type SyncData struct {
	UserAPIKey string    `gorm:"primaryKey;column:user_api_key"`
	Data       []byte    `gorm:"column:data"`
	DataETag   string    `gorm:"column:data_etag"`
	UpdatedAt  time.Time `gorm:"column:updated_at;autoUpdateTime"` // GORM handles updates
	// CreatedAt time.Time `gorm:"column:created_at;autoCreateTime"` // Add if table has created_at
}

// TableName specifies the table name for GORM.
func (SyncData) TableName() string {
	return "sync_data"
}

func NewSyncRepo(log logger.Logger, db *DB) domain.SyncRepo {
	return &SyncRepo{
		log: log.With().Str("repo", "sync").Logger(), // Changed module name for clarity
		db:  db,
	}
}

type SyncRepo struct {
	log zerolog.Logger
	db  *DB
}

// GetSyncDataETag retrieves only the ETag for a given API key.
func (r *SyncRepo) GetSyncDataETag(ctx context.Context, apiKey string) (*string, error) {
	var syncData SyncData
	result := r.db.Get().WithContext(ctx).
		Model(&SyncData{}).  // Specify the model
		Select("data_etag"). // Select only the ETag field
		Where("user_api_key = ?", apiKey).
		First(&syncData) // Fetch the record

	if result.Error != nil {
		if errors.Is(result.Error, gorm.ErrRecordNotFound) {
			// Not found is not an error in this context, return nil ETag
			return nil, nil
		}
		r.log.Error().Err(result.Error).Str("apiKey", "REDACTED").Msg("Failed to get sync data ETag")
		return nil, errors.Wrap(result.Error, "failed to get sync data ETag")
	}

	return &syncData.DataETag, nil
}

// GetSyncDataAndETag retrieves both the data and ETag.
func (r *SyncRepo) GetSyncDataAndETag(ctx context.Context, apiKey string) ([]byte, *string, error) {
	var syncData SyncData
	result := r.db.Get().WithContext(ctx).
		Where("user_api_key = ?", apiKey).
		First(&syncData) // Fetch the full record

	if result.Error != nil {
		if errors.Is(result.Error, gorm.ErrRecordNotFound) {
			// Not found is not an error, return nil data and ETag
			return nil, nil, nil
		}
		r.log.Error().Err(result.Error).Str("apiKey", "REDACTED").Msg("Failed to get sync data and ETag")
		return nil, nil, errors.Wrap(result.Error, "failed to get sync data and ETag")
	}

	return syncData.Data, &syncData.DataETag, nil
}

// SetSyncData creates or replaces sync data (UPSERT logic).
func (r *SyncRepo) SetSyncData(ctx context.Context, apiKey string, data []byte) (*string, error) {
	newEtag := "uuid=" + uuid.NewString()
	now := time.Now() // Needed for manual CreatedAt if not using autoCreateTime

	syncData := SyncData{
		UserAPIKey: apiKey,
		Data:       data,
		DataETag:   newEtag,
		UpdatedAt:  now, // Explicitly set for clarity, though autoUpdateTime handles it
		// CreatedAt: now, // Set if using manual CreatedAt
	}

	// Use GORM's Save for UPSERT behavior (updates if primary key exists, inserts otherwise)
	// Note: This assumes UserAPIKey is the primary key. If not, adjust logic.
	// GORM's Save updates all fields. Use Updates for specific fields.
	// Let's stick to the original logic: Try Update, then Insert if needed.

	db := r.db.Get().WithContext(ctx)

	// Try to update first
	updateResult := db.Model(&SyncData{}).
		Where("user_api_key = ?", apiKey).
		Updates(map[string]interface{}{
			"data":       data,
			"data_etag":  newEtag,
			"updated_at": now, // Explicitly set update time
		})

	if updateResult.Error != nil {
		r.log.Error().Err(updateResult.Error).Str("apiKey", "REDACTED").Msg("Error updating sync data")
		return nil, errors.Wrap(updateResult.Error, "error updating sync data")
	}

	// If no rows were affected by the update, insert a new record
	if updateResult.RowsAffected == 0 {
		createResult := db.Create(&syncData) // GORM handles CreatedAt/UpdatedAt via tags if present
		if createResult.Error != nil {
			r.log.Error().Err(createResult.Error).Str("apiKey", "REDACTED").Msg("Error inserting sync data after failed update")
			// Consider checking for race conditions (e.g., unique constraint violation)
			return nil, errors.Wrap(createResult.Error, "error inserting sync data")
		}
		if createResult.RowsAffected == 0 {
			// Should not happen with Create unless there's a severe issue or race condition not caught by error
			r.log.Error().Str("apiKey", "REDACTED").Msg("Insert operation affected 0 rows unexpectedly")
			return nil, errors.New("failed to insert sync data, 0 rows affected")
		}
		r.log.Debug().Str("apiKey", "REDACTED").Msg("Sync data inserted")
	} else {
		r.log.Debug().Str("apiKey", "REDACTED").Msg("Sync data updated")
	}

	return &newEtag, nil
}

// SetSyncDataIfMatch replaces sync data only if the provided ETag matches.
func (r *SyncRepo) SetSyncDataIfMatch(ctx context.Context, apiKey string, etag string, data []byte) (*string, error) {
	newEtag := "uuid=" + uuid.NewString()
	now := time.Now()

	// Perform a conditional update
	result := r.db.Get().WithContext(ctx).
		Model(&SyncData{}).                                        // Specify the model
		Where("user_api_key = ? AND data_etag = ?", apiKey, etag). // Match key and ETag
		Updates(map[string]interface{}{                            // Update specific fields
			"data":       data,
			"data_etag":  newEtag,
			"updated_at": now,
		})

	if result.Error != nil {
		r.log.Error().Err(result.Error).Str("apiKey", "REDACTED").Str("etag", etag).Msg("Error conditionally updating sync data")
		return nil, errors.Wrap(result.Error, "error conditionally updating sync data")
	}

	if result.RowsAffected == 0 {
		// ETag mismatch or record not found
		// Check if the record exists at all to differentiate ETag mismatch from non-existence
		var count int64
		countResult := r.db.Get().WithContext(ctx).Model(&SyncData{}).Where("user_api_key = ?", apiKey).Count(&count)
		if countResult.Error == nil && count > 0 {
			// Record exists, so it must be an ETag mismatch
			r.log.Warn().Str("apiKey", "REDACTED").Str("expectedETag", etag).Msg("ETag mismatch detected during conditional update. Remote data likely changed.")
			return nil, nil // Return nil, nil for ETag mismatch as per original logic
		}
		// If count error or count is 0, the record might not exist
		r.log.Warn().Str("apiKey", "REDACTED").Str("expectedETag", etag).Msg("Conditional update failed: 0 rows affected (record not found or ETag mismatch)")
		return nil, nil // Return nil, nil as per original logic (covers not found too)

	}

	r.log.Debug().Str("apiKey", "REDACTED").Str("oldEtag", etag).Str("newEtag", newEtag).Msg("Sync data replaced conditionally")
	return &newEtag, nil
}
