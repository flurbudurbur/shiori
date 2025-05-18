package database

import (
	"context"

	// "database/sql" // No longer needed
	// sq "github.com/Masterminds/squirrel" // No longer needed
	"github.com/flurbudurbur/Shiori/internal/domain"
	"github.com/flurbudurbur/Shiori/internal/logger"
	"github.com/flurbudurbur/Shiori/pkg/errors"

	// "github.com/lib/pq" // No longer needed
	"github.com/rs/zerolog"
	"gorm.io/gorm"
)

type NotificationRepo struct {
	log zerolog.Logger
	db  *DB
}

func NewNotificationRepo(log logger.Logger, db *DB) domain.NotificationRepo {
	return &NotificationRepo{
		log: log.With().Str("repo", "notification").Logger(),
		db:  db,
	}
}

// Find retrieves notifications based on query parameters.
// Note: Original implementation used COUNT(*) OVER(). This uses separate Find and Count queries.
// Pagination/Filtering based on NotificationQueryParams needs further implementation if required.
func (r *NotificationRepo) Find(ctx context.Context, params domain.NotificationQueryParams) ([]domain.Notification, int, error) {
	var notifications []domain.Notification
	var totalCount int64

	db := r.db.Get().WithContext(ctx).Model(&domain.Notification{})

	// --- Apply Filtering (Example - adapt based on actual params usage) ---
	// if params.Search != "" {
	//    db = db.Where("name LIKE ?", "%"+params.Search+"%") // Example search
	// }
	// Add other filters based on params.Filters

	// Count total records matching filters (before pagination)
	if err := db.Count(&totalCount).Error; err != nil {
		r.log.Error().Err(err).Msg("Failed to count notifications")
		return nil, 0, errors.Wrap(err, "failed to count notifications")
	}

	// --- Apply Sorting ---
	// Default sort or apply from params.Sort
	db = db.Order("name asc") // Default sort by name

	// --- Apply Pagination ---
	if params.Limit > 0 {
		db = db.Limit(int(params.Limit))
	}
	if params.Offset > 0 {
		db = db.Offset(int(params.Offset))
	}

	// Find the records
	if err := db.Find(&notifications).Error; err != nil {
		r.log.Error().Err(err).Msg("Failed to find notifications")
		return nil, 0, errors.Wrap(err, "failed to find notifications")
	}

	return notifications, int(totalCount), nil
}

// List retrieves all notifications, ordered by name.
func (r *NotificationRepo) List(ctx context.Context) ([]domain.Notification, error) {
	var notifications []domain.Notification
	result := r.db.Get().WithContext(ctx).Order("name asc").Find(&notifications)

	if result.Error != nil {
		r.log.Error().Err(result.Error).Msg("Failed to list notifications")
		return nil, errors.Wrap(result.Error, "failed to list notifications")
	}

	return notifications, nil
}

// FindByID retrieves a specific notification by its ID.
func (r *NotificationRepo) FindByID(ctx context.Context, id int) (*domain.Notification, error) {
	var notification domain.Notification
	result := r.db.Get().WithContext(ctx).First(&notification, id) // Find by primary key

	if result.Error != nil {
		if errors.Is(result.Error, gorm.ErrRecordNotFound) {
			// Use Wrap with Sprintf as Wrapf is not available
			return nil, errors.Wrap(gorm.ErrRecordNotFound, "notification with id %d not found", id)
		}
		r.log.Error().Err(result.Error).Int("id", id).Msg("Failed to find notification by ID")
		return nil, errors.Wrap(result.Error, "failed to find notification by ID")
	}

	return &notification, nil
}

// Store creates a new notification record.
func (r *NotificationRepo) Store(ctx context.Context, notification domain.Notification) (*domain.Notification, error) {
	// GORM automatically handles ID, CreatedAt, UpdatedAt based on tags
	result := r.db.Get().WithContext(ctx).Create(&notification)

	if result.Error != nil {
		r.log.Error().Err(result.Error).Msg("Failed to store notification")
		return nil, errors.Wrap(result.Error, "failed to store notification")
	}

	r.log.Debug().Int("id", notification.ID).Msg("Successfully stored notification")
	// The notification object passed by reference is updated with ID, CreatedAt, UpdatedAt
	return &notification, nil
}

// Update modifies an existing notification record.
// Uses GORM's Save which updates all fields based on the primary key.
func (r *NotificationRepo) Update(ctx context.Context, notification domain.Notification) (*domain.Notification, error) {
	if notification.ID == 0 {
		return nil, errors.New("cannot update notification with zero ID")
	}
	// GORM's Save updates the record matching the primary key (ID)
	// It also automatically updates the UpdatedAt field.
	result := r.db.Get().WithContext(ctx).Save(&notification)

	if result.Error != nil {
		r.log.Error().Err(result.Error).Int("id", notification.ID).Msg("Failed to update notification")
		return nil, errors.Wrap(result.Error, "failed to update notification")
	}

	if result.RowsAffected == 0 {
		// This case might indicate the record didn't exist, though Save usually UPSERTS.
		// Depending on exact GORM behavior/config, you might check this.
		// For now, assume Save error handles non-existence if applicable.
		r.log.Warn().Int("id", notification.ID).Msg("Update operation affected 0 rows, notification might not exist")
		// Optionally return gorm.ErrRecordNotFound or a custom error
		// return nil, errors.Wrap(gorm.ErrRecordNotFound, "notification with id %d not found for update", notification.ID)
	}

	r.log.Debug().Int("id", notification.ID).Msg("Successfully updated notification")
	// The notification object is updated in place by Save if necessary (e.g., UpdatedAt)
	return &notification, nil
}

// Delete removes a notification record by its ID.
func (r *NotificationRepo) Delete(ctx context.Context, notificationID int) error {
	// GORM's Delete requires a pointer to the struct type and the primary key
	result := r.db.Get().WithContext(ctx).Delete(&domain.Notification{}, notificationID)

	if result.Error != nil {
		r.log.Error().Err(result.Error).Int("id", notificationID).Msg("Failed to delete notification")
		return errors.Wrap(result.Error, "failed to delete notification")
	}

	if result.RowsAffected == 0 {
		r.log.Warn().Int("id", notificationID).Msg("Attempted to delete non-existent notification")
		// Return an error indicating the record was not found
		// Use Wrap with Sprintf as Wrapf is not available
		return errors.Wrap(gorm.ErrRecordNotFound, "notification with id %d not found for deletion", notificationID)
	}

	r.log.Info().Int("id", notificationID).Msg("Successfully deleted notification")
	return nil
}
