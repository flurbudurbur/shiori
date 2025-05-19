package database

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/flurbudurbur/Shiori/internal/domain"
	"github.com/flurbudurbur/Shiori/internal/logger"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	gormlogger "gorm.io/gorm/logger" // Added import for gormlogger
)

func setupTestDB(t *testing.T) *DB {
	t.Helper()
	// Using an in-memory SQLite database for testing
	// Each test function will get a unique database
	dbName := fmt.Sprintf("file:%s_%s?mode=memory&cache=shared", t.Name(), uuid.NewString())
	gormDB, err := gorm.Open(sqlite.Open(dbName), &gorm.Config{
		// Disable logger for GORM during tests unless specifically needed for debugging
		Logger: gormlogger.Default.LogMode(gormlogger.Silent),
	})
	require.NoError(t, err, "Failed to connect to database")

	// Auto-migrate the schema
	err = gormDB.AutoMigrate(&domain.ProfileUUID{}, &domain.User{}) // Added User for FindOrphaned
	require.NoError(t, err, "Failed to migrate database")

	// The DB struct from internal/database/database.go has an unexported 'handler' field.
	// For tests, we are primarily concerned with providing this *gorm.DB instance
	// to the repository, which uses db.Get().
	// The DB struct's own logger isn't directly used by the repo methods being tested here.
	return &DB{
		handler: gormDB,
	}
}

func TestProfileUUIDRepo_NewProfileUUIDRepo(t *testing.T) {
	testCfg := &domain.Config{
		Logging: domain.LoggingConfig{Level: "DEBUG"},
		Version: "dev",
	}
	log := logger.New(testCfg)
	db := setupTestDB(t)
	repo := NewProfileUUIDRepo(log, db)
	assert.NotNil(t, repo)
	assert.NotNil(t, repo.(*ProfileUUIDRepo).db)       // This accesses the DB struct passed in
	assert.NotNil(t, repo.(*ProfileUUIDRepo).db.Get()) // This accesses the gorm.DB handler
	assert.NotNil(t, repo.(*ProfileUUIDRepo).log)
}

func TestProfileUUIDRepo_StoreAndFindByUserID(t *testing.T) {
	ctx := context.Background()
	testCfg := &domain.Config{
		Logging: domain.LoggingConfig{Level: "DEBUG"},
		Version: "dev",
	}
	log := logger.New(testCfg)
	db := setupTestDB(t)
	repo := NewProfileUUIDRepo(log, db)

	userID := "user-123"
	profileUUIDStr := uuid.NewString()

	t.Run("Store_Success", func(t *testing.T) {
		profile := domain.ProfileUUID{
			UserID:      userID,
			ProfileUUID: profileUUIDStr,
		}
		err := repo.Store(ctx, profile)
		require.NoError(t, err)

		// Verify timestamps are set
		storedProfile, err := repo.FindByUserID(ctx, userID)
		require.NoError(t, err)
		require.NotNil(t, storedProfile)
		assert.False(t, storedProfile.CreatedAt.IsZero())
		assert.False(t, storedProfile.LastActivityAt.IsZero())
		assert.Equal(t, storedProfile.CreatedAt, storedProfile.LastActivityAt) // Initially same
	})

	t.Run("FindByUserID_Success", func(t *testing.T) {
		found, err := repo.FindByUserID(ctx, userID)
		require.NoError(t, err)
		require.NotNil(t, found)
		assert.Equal(t, userID, found.UserID)
		assert.Equal(t, profileUUIDStr, found.ProfileUUID)
	})

	t.Run("Store_DuplicateProfileUUID", func(t *testing.T) {
		// Attempt to store another record with the same ProfileUUID but different UserID (should fail on unique constraint)
		// Note: The current FindByUserID is by UserID, so this tests the DB constraint more than repo logic for duplicates.
		// A more direct test would be to try to store the exact same UserID and ProfileUUID again.
		// However, the unique constraint is on ProfileUUID.
		duplicateProfile := domain.ProfileUUID{
			UserID:      "another-user-456",
			ProfileUUID: profileUUIDStr, // Same ProfileUUID
		}
		err := repo.Store(ctx, duplicateProfile)
		assert.Error(t, err, "Expected error when storing duplicate profile UUID")
		// Error message might vary depending on GORM and SQLite version,
		// but it should indicate a unique constraint violation.
		// Example: "UNIQUE constraint failed: profile_uuids.profile_uuid"
		assert.Contains(t, err.Error(), "UNIQUE constraint failed", "Error message should indicate unique constraint violation")
	})

	t.Run("Store_WithPresetTimestamps", func(t *testing.T) {
		presetUserID := "user-preset-ts"
		presetProfileUUID := uuid.NewString()
		now := time.Now().Truncate(time.Second) // Truncate for easier comparison
		profile := domain.ProfileUUID{
			UserID:         presetUserID,
			ProfileUUID:    presetProfileUUID,
			CreatedAt:      now.Add(-1 * time.Hour),
			LastActivityAt: now.Add(-30 * time.Minute),
		}
		err := repo.Store(ctx, profile)
		require.NoError(t, err)

		storedProfile, err := repo.FindByUserID(ctx, presetUserID)
		require.NoError(t, err)
		require.NotNil(t, storedProfile)
		assert.Equal(t, profile.CreatedAt, storedProfile.CreatedAt.Truncate(time.Second))
		assert.Equal(t, profile.LastActivityAt, storedProfile.LastActivityAt.Truncate(time.Second))
	})

	t.Run("FindByUserID_NotFound", func(t *testing.T) {
		found, err := repo.FindByUserID(ctx, "non-existent-user")
		require.NoError(t, err) // Expects nil, nil for not found
		assert.Nil(t, found)
	})
}

func TestProfileUUIDRepo_UpdateLastActivity(t *testing.T) {
	ctx := context.Background()
	testCfg := &domain.Config{
		Logging: domain.LoggingConfig{Level: "DEBUG"},
		Version: "dev",
	}
	log := logger.New(testCfg)
	db := setupTestDB(t)
	repo := NewProfileUUIDRepo(log, db)

	userID := "user-activity-test"
	profileUUIDStr := uuid.NewString()

	initialProfile := domain.ProfileUUID{
		UserID:      userID,
		ProfileUUID: profileUUIDStr,
	}
	err := repo.Store(ctx, initialProfile)
	require.NoError(t, err)

	originalStored, err := repo.FindByUserID(ctx, userID)
	require.NoError(t, err)
	require.NotNil(t, originalStored)
	originalLastActivity := originalStored.LastActivityAt

	// Allow some time to pass to ensure the new timestamp is different
	time.Sleep(10 * time.Millisecond)

	t.Run("UpdateLastActivity_Success", func(t *testing.T) {
		err := repo.UpdateLastActivity(ctx, userID, profileUUIDStr)
		require.NoError(t, err)

		updatedProfile, err := repo.FindByUserID(ctx, userID)
		require.NoError(t, err)
		require.NotNil(t, updatedProfile)
		assert.True(t, updatedProfile.LastActivityAt.After(originalLastActivity), "LastActivityAt should be updated")
		assert.Equal(t, originalStored.CreatedAt, updatedProfile.CreatedAt, "CreatedAt should not change")
	})

	t.Run("UpdateLastActivity_NotFound_WrongUserID", func(t *testing.T) {
		err := repo.UpdateLastActivity(ctx, "wrong-user-id", profileUUIDStr)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "not found", "Error should indicate record not found")
	})

	t.Run("UpdateLastActivity_NotFound_WrongProfileUUID", func(t *testing.T) {
		err := repo.UpdateLastActivity(ctx, userID, "wrong-profile-uuid")
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "not found", "Error should indicate record not found")
	})
}
func TestProfileUUIDRepo_FindExpiredProfileUUIDs(t *testing.T) {
	ctx := context.Background()
	testCfg := &domain.Config{
		Logging: domain.LoggingConfig{Level: "DEBUG"},
		Version: "dev",
	}
	log := logger.New(testCfg)
	db := setupTestDB(t)
	repo := NewProfileUUIDRepo(log, db)

	now := time.Now()
	p1 := domain.ProfileUUID{UserID: "user-exp-1", ProfileUUID: uuid.NewString(), CreatedAt: now.Add(-5 * time.Hour), LastActivityAt: now.Add(-4 * time.Hour)} // Expired
	p2 := domain.ProfileUUID{UserID: "user-exp-2", ProfileUUID: uuid.NewString(), CreatedAt: now.Add(-5 * time.Hour), LastActivityAt: now.Add(-2 * time.Hour)} // Not expired
	p3 := domain.ProfileUUID{UserID: "user-exp-3", ProfileUUID: uuid.NewString(), CreatedAt: now.Add(-5 * time.Hour), LastActivityAt: now.Add(-6 * time.Hour)} // Expired

	require.NoError(t, repo.Store(ctx, p1))
	require.NoError(t, repo.Store(ctx, p2))
	require.NoError(t, repo.Store(ctx, p3))

	t.Run("FindExpired_SomeMatch", func(t *testing.T) {
		olderThan := now.Add(-3 * time.Hour) // p1 and p3 should be older
		expired, err := repo.FindExpiredProfileUUIDs(ctx, olderThan)
		require.NoError(t, err)
		assert.Len(t, expired, 2)

		foundIDs := make(map[string]bool)
		for _, p := range expired {
			foundIDs[p.ProfileUUID] = true
		}
		assert.True(t, foundIDs[p1.ProfileUUID])
		assert.True(t, foundIDs[p3.ProfileUUID])
		assert.False(t, foundIDs[p2.ProfileUUID])
	})

	t.Run("FindExpired_NoneMatch", func(t *testing.T) {
		olderThan := now.Add(-7 * time.Hour) // No profiles should be older
		expired, err := repo.FindExpiredProfileUUIDs(ctx, olderThan)
		require.NoError(t, err)
		assert.Len(t, expired, 0)
	})

	t.Run("FindExpired_AllMatch", func(t *testing.T) {
		olderThan := now.Add(-1 * time.Hour) // All profiles should be older
		expired, err := repo.FindExpiredProfileUUIDs(ctx, olderThan)
		require.NoError(t, err)
		assert.Len(t, expired, 3)
	})
}

func TestProfileUUIDRepo_FindStaleProfileUUIDs(t *testing.T) {
	ctx := context.Background()
	testCfg := &domain.Config{
		Logging: domain.LoggingConfig{Level: "DEBUG"},
		Version: "dev",
	}
	log := logger.New(testCfg)
	db := setupTestDB(t)
	repo := NewProfileUUIDRepo(log, db)

	now := time.Now()
	// Create more profiles for limit testing
	profiles := []domain.ProfileUUID{
		{UserID: "user-stale-1", ProfileUUID: uuid.NewString(), CreatedAt: now.Add(-10 * time.Hour), LastActivityAt: now.Add(-9 * time.Hour)}, // Stale
		{UserID: "user-stale-2", ProfileUUID: uuid.NewString(), CreatedAt: now.Add(-10 * time.Hour), LastActivityAt: now.Add(-8 * time.Hour)}, // Stale
		{UserID: "user-stale-3", ProfileUUID: uuid.NewString(), CreatedAt: now.Add(-10 * time.Hour), LastActivityAt: now.Add(-7 * time.Hour)}, // Stale
		{UserID: "user-stale-4", ProfileUUID: uuid.NewString(), CreatedAt: now.Add(-10 * time.Hour), LastActivityAt: now.Add(-2 * time.Hour)}, // Not stale
		{UserID: "user-stale-5", ProfileUUID: uuid.NewString(), CreatedAt: now.Add(-10 * time.Hour), LastActivityAt: now.Add(-6 * time.Hour)}, // Stale
	}

	for _, p := range profiles {
		require.NoError(t, repo.Store(ctx, p))
	}

	t.Run("FindStale_LimitApplied", func(t *testing.T) {
		olderThan := now.Add(-5 * time.Hour) // p1, p2, p3, p5 are candidates (4 total)
		limit := 2
		stale, err := repo.FindStaleProfileUUIDs(ctx, olderThan, limit)
		require.NoError(t, err)
		assert.Len(t, stale, limit) // Should return only 'limit' number of profiles
	})

	t.Run("FindStale_LimitMoreThanAvailable", func(t *testing.T) {
		olderThan := now.Add(-5 * time.Hour) // 4 candidates
		limit := 10
		stale, err := repo.FindStaleProfileUUIDs(ctx, olderThan, limit)
		require.NoError(t, err)
		assert.Len(t, stale, 4) // Should return all available stale profiles
	})

	t.Run("FindStale_NoneMatch", func(t *testing.T) {
		olderThan := now.Add(-12 * time.Hour) // No profiles should be older
		limit := 5
		stale, err := repo.FindStaleProfileUUIDs(ctx, olderThan, limit)
		require.NoError(t, err)
		assert.Len(t, stale, 0)
	})

	t.Run("FindStale_ZeroLimit", func(t *testing.T) {
		olderThan := now.Add(-5 * time.Hour)
		limit := 0 // GORM might treat 0 as no limit, or it might return 0. Behavior depends on GORM.
		// The implementation uses limit directly, so if GORM treats 0 as no limit, this test might need adjustment
		// or the repo method should explicitly handle limit = 0 if specific behavior is desired.
		// For now, assume GORM's default behavior. If it means "no limit", this test will find all 4.
		// If it means "limit is 0", it will find 0.
		// Let's assume GORM's .Limit(0) means no limit.
		// The code in profile_uuid.go uses .Limit(limit), so if limit is 0, it should be .Limit(0)
		// which typically means no rows. Let's test for 0.
		stale, err := repo.FindStaleProfileUUIDs(ctx, olderThan, limit)
		require.NoError(t, err)
		assert.Len(t, stale, 0) // Expecting 0 if limit is 0
	})
}
func TestProfileUUIDRepo_FindOrphanedProfileUUIDs(t *testing.T) {
	ctx := context.Background()
	testCfg := &domain.Config{
		Logging: domain.LoggingConfig{Level: "DEBUG"},
		Version: "dev",
	}
	log := logger.New(testCfg)
	db := setupTestDB(t) // setupTestDB already migrates User and ProfileUUID
	repo := NewProfileUUIDRepo(log, db)

	// 1. Create Users
	user1 := domain.User{HashedUUID: "user-with-profile-1", Scopes: "read", DeletionDate: time.Now().Add(24 * time.Hour)} // Use HashedUUID
	user2 := domain.User{HashedUUID: "user-no-profile-1", Scopes: "read", DeletionDate: time.Now().Add(24 * time.Hour)}   // This user won't have a profile_uuid initially
	require.NoError(t, db.Get().WithContext(ctx).Create(&user1).Error)
	require.NoError(t, db.Get().WithContext(ctx).Create(&user2).Error)

	// 2. Create ProfileUUIDs
	// Orphaned: UserID does not exist in users table
	orphanPUUID1 := domain.ProfileUUID{UserID: "non-existent-user-1", ProfileUUID: uuid.NewString(), CreatedAt: time.Now(), LastActivityAt: time.Now()}
	// Linked to existing user1
	linkedPUUID1 := domain.ProfileUUID{UserID: user1.HashedUUID, ProfileUUID: uuid.NewString(), CreatedAt: time.Now(), LastActivityAt: time.Now()} // Use user1.HashedUUID
	// Orphaned: UserID does not exist
	orphanPUUID2 := domain.ProfileUUID{UserID: "non-existent-user-2", ProfileUUID: uuid.NewString(), CreatedAt: time.Now(), LastActivityAt: time.Now()}

	require.NoError(t, repo.Store(ctx, orphanPUUID1))
	require.NoError(t, repo.Store(ctx, linkedPUUID1))
	require.NoError(t, repo.Store(ctx, orphanPUUID2))

	t.Run("FindOrphaned_SomeMatch", func(t *testing.T) {
		limit := 5
		orphaned, err := repo.FindOrphanedProfileUUIDs(ctx, limit)
		require.NoError(t, err)
		assert.Len(t, orphaned, 2, "Should find two orphaned ProfileUUIDs")

		foundIDs := make(map[string]bool)
		for _, p := range orphaned {
			foundIDs[p.ProfileUUID] = true
		}
		assert.True(t, foundIDs[orphanPUUID1.ProfileUUID])
		assert.True(t, foundIDs[orphanPUUID2.ProfileUUID])
		assert.False(t, foundIDs[linkedPUUID1.ProfileUUID], "Linked ProfileUUID should not be considered orphaned")
	})

	t.Run("FindOrphaned_LimitApplied", func(t *testing.T) {
		limit := 1
		orphaned, err := repo.FindOrphanedProfileUUIDs(ctx, limit)
		require.NoError(t, err)
		assert.Len(t, orphaned, limit, "Should respect the limit")
	})

	t.Run("FindOrphaned_NoOrphans", func(t *testing.T) {
		// First, delete the orphans created earlier to ensure a clean state for this sub-test
		// This is a bit of a hack for a sub-test; ideally, sub-tests are more isolated.
		// A better way would be to set up a new DB for this specific sub-test or clean up more selectively.
		// For now, let's delete all profile_uuids and re-add only a linked one.
		require.NoError(t, db.Get().WithContext(ctx).Exec("DELETE FROM profile_uuids").Error)

		cleanLinkedPUUID := domain.ProfileUUID{UserID: user1.HashedUUID, ProfileUUID: uuid.NewString(), CreatedAt: time.Now(), LastActivityAt: time.Now()} // Use user1.HashedUUID
		require.NoError(t, repo.Store(ctx, cleanLinkedPUUID))

		limit := 5
		orphaned, err := repo.FindOrphanedProfileUUIDs(ctx, limit)
		require.NoError(t, err)
		assert.Len(t, orphaned, 0, "Should find no orphaned ProfileUUIDs")
	})
}
func TestProfileUUIDRepo_DeleteProfileUUIDs(t *testing.T) {
	ctx := context.Background()
	testCfg := &domain.Config{
		Logging: domain.LoggingConfig{Level: "DEBUG"},
		Version: "dev",
	}
	log := logger.New(testCfg)
	db := setupTestDB(t)
	repo := NewProfileUUIDRepo(log, db)

	p1 := domain.ProfileUUID{UserID: "user-del-multi-1", ProfileUUID: uuid.NewString(), CreatedAt: time.Now(), LastActivityAt: time.Now()}
	p2 := domain.ProfileUUID{UserID: "user-del-multi-2", ProfileUUID: uuid.NewString(), CreatedAt: time.Now(), LastActivityAt: time.Now()}
	p3 := domain.ProfileUUID{UserID: "user-del-multi-3", ProfileUUID: uuid.NewString(), CreatedAt: time.Now(), LastActivityAt: time.Now()} // Will remain

	require.NoError(t, repo.Store(ctx, p1))
	require.NoError(t, repo.Store(ctx, p2))
	require.NoError(t, repo.Store(ctx, p3))

	// Retrieve them to get their IDs
	storedP1, err := repo.FindByUserID(ctx, p1.UserID)
	require.NoError(t, err)
	require.NotNil(t, storedP1)
	storedP2, err := repo.FindByUserID(ctx, p2.UserID)
	require.NoError(t, err)
	require.NotNil(t, storedP2)

	t.Run("DeleteProfileUUIDs_Success", func(t *testing.T) {
		toDelete := []domain.ProfileUUID{*storedP1, *storedP2}
		deletedCount, err := repo.DeleteProfileUUIDs(ctx, toDelete)
		require.NoError(t, err)
		assert.Equal(t, 2, deletedCount)

		// Verify p1 and p2 are deleted
		foundP1, err := repo.FindByUserID(ctx, p1.UserID)
		require.NoError(t, err)
		assert.Nil(t, foundP1)

		foundP2, err := repo.FindByUserID(ctx, p2.UserID)
		require.NoError(t, err)
		assert.Nil(t, foundP2)

		// Verify p3 still exists
		foundP3, err := repo.FindByUserID(ctx, p3.UserID)
		require.NoError(t, err)
		assert.NotNil(t, foundP3)
	})

	t.Run("DeleteProfileUUIDs_EmptyList", func(t *testing.T) {
		deletedCount, err := repo.DeleteProfileUUIDs(ctx, []domain.ProfileUUID{})
		require.NoError(t, err)
		assert.Equal(t, 0, deletedCount)
	})

	t.Run("DeleteProfileUUIDs_NonExistentInList", func(t *testing.T) {
		// Create a new profile to ensure the DB isn't empty for this part
		tempP := domain.ProfileUUID{UserID: "user-temp-del", ProfileUUID: uuid.NewString()}
		require.NoError(t, repo.Store(ctx, tempP))
		storedTempP, _ := repo.FindByUserID(ctx, tempP.UserID)

		// Try to delete a profile that was already deleted (or never existed with that ID in the list)
		// The current implementation deletes by ID found in the list. If an ID doesn't match, it's skipped.
		nonExistentIDProfile := domain.ProfileUUID{ID: 99999, UserID: "ghost", ProfileUUID: "ghost-uuid"}
		toDelete := []domain.ProfileUUID{*storedTempP, nonExistentIDProfile} // storedTempP exists, nonExistentIDProfile does not

		deletedCount, err := repo.DeleteProfileUUIDs(ctx, toDelete)
		require.NoError(t, err)
		assert.Equal(t, 1, deletedCount, "Should only delete the one that exists")

		checkTempP, _ := repo.FindByUserID(ctx, tempP.UserID)
		assert.Nil(t, checkTempP, "TempP should be deleted")
	})
}

func TestProfileUUIDRepo_SoftDeleteProfileUUIDs(t *testing.T) {
	ctx := context.Background()
	testCfg := &domain.Config{
		Logging: domain.LoggingConfig{Level: "DEBUG"},
		Version: "dev",
	}
	log := logger.New(testCfg)
	db := setupTestDB(t)
	repo := NewProfileUUIDRepo(log, db)

	p1 := domain.ProfileUUID{UserID: "user-softdel-1", ProfileUUID: uuid.NewString()}
	p2 := domain.ProfileUUID{UserID: "user-softdel-2", ProfileUUID: uuid.NewString()}
	require.NoError(t, repo.Store(ctx, p1))
	require.NoError(t, repo.Store(ctx, p2))

	storedP1, _ := repo.FindByUserID(ctx, p1.UserID)
	storedP2, _ := repo.FindByUserID(ctx, p2.UserID)

	t.Run("SoftDeleteProfileUUIDs_Success", func(t *testing.T) {
		toSoftDelete := []domain.ProfileUUID{*storedP1, *storedP2}
		deletedCount, err := repo.SoftDeleteProfileUUIDs(ctx, toSoftDelete)
		require.NoError(t, err)
		assert.Equal(t, 2, deletedCount)

		// Verify DeletedAt is set for p1
		var checkP1 domain.ProfileUUID
		err = db.Get().WithContext(ctx).Unscoped().Where("user_id = ?", p1.UserID).First(&checkP1).Error // Use Unscoped to fetch soft-deleted
		require.NoError(t, err)
		assert.NotNil(t, checkP1.DeletedAt)
		assert.False(t, checkP1.DeletedAt.IsZero())

		// Verify DeletedAt is set for p2
		var checkP2 domain.ProfileUUID
		err = db.Get().WithContext(ctx).Unscoped().Where("user_id = ?", p2.UserID).First(&checkP2).Error
		require.NoError(t, err)
		assert.NotNil(t, checkP2.DeletedAt)
		assert.False(t, checkP2.DeletedAt.IsZero())

		// Normal FindByUserID should not find them (as it doesn't use Unscoped)
		foundP1Regular, err := repo.FindByUserID(ctx, p1.UserID)
		require.NoError(t, err)
		assert.Nil(t, foundP1Regular)
	})

	t.Run("SoftDeleteProfileUUIDs_EmptyList", func(t *testing.T) {
		deletedCount, err := repo.SoftDeleteProfileUUIDs(ctx, []domain.ProfileUUID{})
		require.NoError(t, err)
		assert.Equal(t, 0, deletedCount)
	})
}

func TestProfileUUIDRepo_DeleteProfileUUID(t *testing.T) {
	ctx := context.Background()
	testCfg := &domain.Config{
		Logging: domain.LoggingConfig{Level: "DEBUG"},
		Version: "dev",
	}
	log := logger.New(testCfg)
	db := setupTestDB(t)
	repo := NewProfileUUIDRepo(log, db)

	userID := "user-del-single"
	profileUUIDStr := uuid.NewString()
	profile := domain.ProfileUUID{UserID: userID, ProfileUUID: profileUUIDStr}
	require.NoError(t, repo.Store(ctx, profile))

	t.Run("DeleteProfileUUID_Success", func(t *testing.T) {
		err := repo.DeleteProfileUUID(ctx, userID, profileUUIDStr)
		require.NoError(t, err)

		found, err := repo.FindByUserID(ctx, userID)
		require.NoError(t, err)
		assert.Nil(t, found)
	})

	t.Run("DeleteProfileUUID_NotFound_WrongUserID", func(t *testing.T) {
		// Re-add for this test case
		require.NoError(t, repo.Store(ctx, profile))
		err := repo.DeleteProfileUUID(ctx, "wrong-user-id", profileUUIDStr)
		assert.Error(t, err)
		assert.ErrorIs(t, err, gorm.ErrRecordNotFound) // Check for specific error type if possible
	})

	t.Run("DeleteProfileUUID_NotFound_WrongProfileUUID", func(t *testing.T) {
		// Ensure it exists before trying to delete with wrong UUID
		_, findErr := repo.FindByUserID(ctx, userID)
		if findErr != nil || findErr == gorm.ErrRecordNotFound { // If it was deleted in previous subtest
			require.NoError(t, repo.Store(ctx, profile))
		}

		err := repo.DeleteProfileUUID(ctx, userID, "wrong-profile-uuid")
		assert.Error(t, err)
		assert.ErrorIs(t, err, gorm.ErrRecordNotFound)
	})

	t.Run("DeleteProfileUUID_AlreadyDeleted", func(t *testing.T) {
		// Ensure it's deleted first
		_ = repo.DeleteProfileUUID(ctx, userID, profileUUIDStr) // Delete it if it exists

		err := repo.DeleteProfileUUID(ctx, userID, profileUUIDStr) // Try to delete again
		assert.Error(t, err)
		assert.ErrorIs(t, err, gorm.ErrRecordNotFound)
	})
}