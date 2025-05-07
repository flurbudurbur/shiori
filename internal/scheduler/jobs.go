package scheduler

import (
	"context"
	"time"

	"github.com/flurbudurbur/Shiori/internal/domain"
	"github.com/flurbudurbur/Shiori/internal/notification"
	"github.com/flurbudurbur/Shiori/internal/update"
	"github.com/rs/zerolog"
)

// BatchSize defines the maximum number of UUIDs to process in a single batch
const BatchSize = 1000

type CheckUpdatesJob struct {
	Name          string
	Log           zerolog.Logger
	Version       string
	NotifSvc      notification.Service
	updateService *update.Service

	lastCheckVersion string
}

func (j *CheckUpdatesJob) Run() {
	newRelease, err := j.updateService.CheckUpdateAvailable(context.TODO())
	if err != nil {
		j.Log.Error().Err(err).Msg("could not check for new release")
		return
	}

	if newRelease != nil {
		// this is not persisted so this can trigger more than once
		// lets check if we have different versions between runs
		if newRelease.TagName != j.lastCheckVersion {
			j.Log.Info().Msgf("a new release has been found: %v Consider updating.", newRelease.TagName)

			j.NotifSvc.Send(domain.NotificationEventAppUpdateAvailable, domain.NotificationPayload{
				Subject:   "New update available!",
				Message:   newRelease.TagName,
				Event:     domain.NotificationEventAppUpdateAvailable,
				Timestamp: time.Now(),
			})
		}

		j.lastCheckVersion = newRelease.TagName
	}
}

type PruneExpiredUsersJob struct {
	Name string
	Log  zerolog.Logger
	Repo domain.UserRepo // Inject UserRepo
}

func (j *PruneExpiredUsersJob) Run() {
	j.Log.Info().Msg("Starting expired user pruning job")
	ctx := context.Background() // Use background context for scheduled job
	now := time.Now()

	// 1. Find expired user IDs
	expiredIDs, err := j.Repo.FindExpiredUserIDs(ctx, now)
	if err != nil {
		j.Log.Error().Err(err).Msg("Failed to find expired user IDs")
		return // Exit job run on error
	}

	if len(expiredIDs) == 0 {
		j.Log.Info().Msg("No expired users found to prune.")
		return
	}

	j.Log.Info().Msgf("Found %d expired user(s) to prune.", len(expiredIDs))

	// 2. Delete each expired user and their associated data
	successCount := 0
	failCount := 0
	for _, userID := range expiredIDs {
		err := j.Repo.DeleteUserAndAssociatedData(ctx, userID)
		if err != nil {
			j.Log.Error().Err(err).Str("userHashedUUID", userID).Msg("Failed to delete expired user and associated data")
			failCount++
		} else {
			j.Log.Info().Str("userHashedUUID", userID).Msg("Successfully pruned expired user")
			successCount++
		}
		// Consider adding a small delay between deletions if needed to reduce DB load
		// time.Sleep(100 * time.Millisecond)
	}

	j.Log.Info().Msgf("Expired user pruning job finished. Success: %d, Failed: %d", successCount, failCount)
}

// ProfileUUIDCleanupJob implements a scheduled job to clean up stale and orphaned profile UUIDs
type ProfileUUIDCleanupJob struct {
	Name   string
	Log    zerolog.Logger
	Repo   domain.ProfileUUIDRepo
	Config *domain.UUIDCleanupConfig
}

// Run executes the profile UUID cleanup job
func (j *ProfileUUIDCleanupJob) Run() {
	if !j.Config.Enabled {
		j.Log.Info().Msg("Profile UUID cleanup job is disabled in configuration")
		return
	}

	j.Log.Info().Msg("Starting profile UUID cleanup job")
	ctx := context.Background()

	// Calculate the cutoff time for stale UUIDs based on configuration
	inactivityThreshold := time.Duration(j.Config.InactivityDays) * 24 * time.Hour
	cutoffTime := time.Now().Add(-inactivityThreshold)

	// Track metrics for logging
	var totalCleaned int
	var totalErrors int

	// Step 1: Clean up stale UUIDs (based on inactivity)
	j.Log.Info().
		Int("inactivity_days", j.Config.InactivityDays).
		Time("cutoff_time", cutoffTime).
		Msg("Finding stale profile UUIDs")

	staleUUIDs, err := j.Repo.FindStaleProfileUUIDs(ctx, cutoffTime, BatchSize)
	if err != nil {
		j.Log.Error().Err(err).Msg("Failed to find stale profile UUIDs")
		totalErrors++
	} else {
		j.Log.Info().Int("count", len(staleUUIDs)).Msg("Found stale profile UUIDs")

		if len(staleUUIDs) > 0 {
			var cleanedCount int

			if j.Config.UseSoftDelete {
				cleanedCount, err = j.Repo.SoftDeleteProfileUUIDs(ctx, staleUUIDs)
				if err != nil {
					j.Log.Error().Err(err).Msg("Failed to soft delete stale profile UUIDs")
					totalErrors++
				} else {
					j.Log.Info().Int("count", cleanedCount).Msg("Successfully soft deleted stale profile UUIDs")
					totalCleaned += cleanedCount
				}
			} else {
				cleanedCount, err = j.Repo.DeleteProfileUUIDs(ctx, staleUUIDs)
				if err != nil {
					j.Log.Error().Err(err).Msg("Failed to delete stale profile UUIDs")
					totalErrors++
				} else {
					j.Log.Info().Int("count", cleanedCount).Msg("Successfully deleted stale profile UUIDs")
					totalCleaned += cleanedCount
				}
			}
		}
	}

	// Step 2: Clean up orphaned UUIDs (if enabled)
	if j.Config.DeleteOrphanedUUIDs {
		j.Log.Info().Msg("Finding orphaned profile UUIDs")

		orphanedUUIDs, err := j.Repo.FindOrphanedProfileUUIDs(ctx, BatchSize)
		if err != nil {
			j.Log.Error().Err(err).Msg("Failed to find orphaned profile UUIDs")
			totalErrors++
		} else {
			j.Log.Info().Int("count", len(orphanedUUIDs)).Msg("Found orphaned profile UUIDs")

			if len(orphanedUUIDs) > 0 {
				var cleanedCount int

				if j.Config.UseSoftDelete {
					cleanedCount, err = j.Repo.SoftDeleteProfileUUIDs(ctx, orphanedUUIDs)
					if err != nil {
						j.Log.Error().Err(err).Msg("Failed to soft delete orphaned profile UUIDs")
						totalErrors++
					} else {
						j.Log.Info().Int("count", cleanedCount).Msg("Successfully soft deleted orphaned profile UUIDs")
						totalCleaned += cleanedCount
					}
				} else {
					cleanedCount, err = j.Repo.DeleteProfileUUIDs(ctx, orphanedUUIDs)
					if err != nil {
						j.Log.Error().Err(err).Msg("Failed to delete orphaned profile UUIDs")
						totalErrors++
					} else {
						j.Log.Info().Int("count", cleanedCount).Msg("Successfully deleted orphaned profile UUIDs")
						totalCleaned += cleanedCount
					}
				}
			}
		}
	}

	// Log summary
	j.Log.Info().
		Int("total_cleaned", totalCleaned).
		Int("total_errors", totalErrors).
		Bool("soft_delete_used", j.Config.UseSoftDelete).
		Msg("Profile UUID cleanup job completed")
}
