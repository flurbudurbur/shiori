package sync

import (
	"context"
	"fmt"

	"github.com/flurbudurbur/Shiori/internal/notification"

	"github.com/flurbudurbur/Shiori/internal/domain"
	"github.com/flurbudurbur/Shiori/internal/logger"
	"github.com/rs/zerolog"
)

type Service interface {
	// Get etag of sync data.
	// For avoid memory usage, only the etag will be returnedj
	GetSyncDataETag(ctx context.Context, userHashedUUID string) (*string, error)
	// Get sync data and etag
	GetSyncDataAndETag(ctx context.Context, userHashedUUID string) ([]byte, *string, error)
	// Create or replace sync data, returns the new etag.
	SetSyncData(ctx context.Context, userHashedUUID string, data []byte) (*string, error)
	// Replace sync data only if the etag matches,
	// returns the new etag if updated, or nil if not.
	SetSyncDataIfMatch(ctx context.Context, userHashedUUID string, etag string, data []byte) (*string, error)
}

func NewService(log logger.Logger, repo domain.SyncRepo, notificationSvc notification.Service) Service {
	return &service{
		log:                 log.With().Str("module", "sync").Logger(),
		repo:                repo,
		notificationService: notificationSvc,
		// apiRepo removed
	}
}

type service struct {
	log                 zerolog.Logger
	repo                domain.SyncRepo
	notificationService notification.Service
	// apiRepo removed
}

// Get etag of sync data.
// For avoid memory usage, only the etag will be returned.
func (s service) GetSyncDataETag(ctx context.Context, userHashedUUID string) (*string, error) {
	return s.repo.GetSyncDataETag(ctx, userHashedUUID)
}

// Get sync data and etag
func (s service) GetSyncDataAndETag(ctx context.Context, userHashedUUID string) ([]byte, *string, error) {
	return s.repo.GetSyncDataAndETag(ctx, userHashedUUID)
}

// Create or replace sync data, returns the new etag.
func (s service) SetSyncData(ctx context.Context, userHashedUUID string, data []byte) (*string, error) {
	return s.repo.SetSyncData(ctx, userHashedUUID, data)
}

// Replace sync data only if the etag matches,
// returns the new etag if updated, or nil if not.
func (s service) SetSyncDataIfMatch(ctx context.Context, userHashedUUID string, etag string, data []byte) (*string, error) {
	return s.repo.SetSyncDataIfMatch(ctx, userHashedUUID, etag, data)
}

func (s service) notifySyncStarted(username string) {
	s.notificationService.Send(domain.NotificationEventSyncStarted, domain.NotificationPayload{
		Subject: "Data Transmission Initiated",
		Message: fmt.Sprintf("A data transmission between your Tachiyomi library and user **%s** has been initiated. "+
			"Please wait for the process to complete.", username),
	})
}

func (s service) notifySyncSuccess(username string) {
	s.notificationService.Send(domain.NotificationEventSyncSuccess, domain.NotificationPayload{
		Subject: "Data Send Successful",
		Message: fmt.Sprintf("Your Tachiyomi library data has been successfully sent to user **%s**.", username),
	})
}

func (s service) notifySyncFailed(username string, errMsg string) {
	s.notificationService.Send(domain.NotificationEventSyncFailed, domain.NotificationPayload{
		Subject: "Sync Operation Failed",
		Message: fmt.Sprintf("The synchronization with Tachiyomi failed for user **%s**. Error: %s", username, errMsg),
	})
}

func (s service) notifySyncError(username string, errMsg string) {
	s.notificationService.Send(domain.NotificationEventSyncError, domain.NotificationPayload{
		Subject: "Error During Sync",
		Message: fmt.Sprintf("An error occurred during synchronization with Tachiyomi for user **%s**. Error: %s", username, errMsg),
	})
}
