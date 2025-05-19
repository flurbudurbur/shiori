package database

import (
	"context"
	"testing"
	"time"

	"github.com/flurbudurbur/Shiori/internal/domain"
	"github.com/flurbudurbur/Shiori/internal/logger"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	// "gorm.io/driver/sqlite" // Removed unused import
	// "gorm.io/gorm" // Removed unused import
)

// Old setupTestDB removed. Tests will use setupTestDBInstance from database_test.go

func TestUserRepo_GetUserCount(t *testing.T) {
	// Setup
	db, cleanup := setupTestDBInstance(t, false) // Using file-based temp DB
	defer cleanup()

	repo := NewUserRepo(logger.Mock(), db)
	ctx := context.Background()

	// Test with empty database
	count, err := repo.GetUserCount(ctx)
	assert.NoError(t, err)
	assert.Equal(t, 0, count)

	// Add a user
	user := domain.User{
		HashedUUID:   "test-uuid-1",
		APITokenHash: "test-token-hash-1",
		Scopes:       `{"read": true, "write": false}`,
		DeletionDate: time.Now().Add(24 * time.Hour),
	}
	err = db.Get().Create(&user).Error
	require.NoError(t, err)

	// Test with one user
	count, err = repo.GetUserCount(ctx)
	assert.NoError(t, err)
	assert.Equal(t, 1, count)

	// Add another user
	user2 := domain.User{
		HashedUUID:   "test-uuid-2",
		APITokenHash: "test-token-hash-2",
		Scopes:       `{"read": true, "write": false}`,
		DeletionDate: time.Now().Add(24 * time.Hour),
	}
	err = db.Get().Create(&user2).Error
	require.NoError(t, err)

	// Test with two users
	count, err = repo.GetUserCount(ctx)
	assert.NoError(t, err)
	assert.Equal(t, 2, count)
}

func TestUserRepo_FindByHashedUUID(t *testing.T) {
	// Setup
	db, cleanup := setupTestDBInstance(t, false) // Using file-based temp DB
	defer cleanup()

	repo := NewUserRepo(logger.Mock(), db)
	ctx := context.Background()

	// Test with non-existent user
	user, err := repo.FindByHashedUUID(ctx, "non-existent-uuid")
	assert.NoError(t, err)
	assert.Nil(t, user)

	// Add a user
	expectedUser := domain.User{
		HashedUUID:   "test-uuid-1",
		APITokenHash: "test-token-hash-1",
		Scopes:       `{"read": true, "write": false}`,
		DeletionDate: time.Now().Add(24 * time.Hour).Round(time.Second).UTC(), // Round to avoid microsecond differences and set to UTC
	}
	err = db.Get().Create(&expectedUser).Error
	require.NoError(t, err)

	// Test with existing user
	foundUser, err := repo.FindByHashedUUID(ctx, "test-uuid-1")
	assert.NoError(t, err)
	require.NotNil(t, foundUser)
	
	// Round the time and convert to UTC to avoid microsecond/location differences in comparison
	foundUser.DeletionDate = foundUser.DeletionDate.Round(time.Second).UTC()
	
	assert.Equal(t, expectedUser.HashedUUID, foundUser.HashedUUID)
	assert.Equal(t, expectedUser.APITokenHash, foundUser.APITokenHash)
	assert.Equal(t, expectedUser.Scopes, foundUser.Scopes)
	assert.Equal(t, expectedUser.DeletionDate, foundUser.DeletionDate)
}

func TestUserRepo_Store(t *testing.T) {
	// Setup
	db, cleanup := setupTestDBInstance(t, false) // Using file-based temp DB
	defer cleanup()

	repo := NewUserRepo(logger.Mock(), db)
	ctx := context.Background()

	// Create a user to store
	user := domain.User{
		HashedUUID:   "test-uuid-1",
		APITokenHash: "test-token-hash-1",
		Scopes:       `{"read": true, "write": false}`,
		DeletionDate: time.Now().Add(24 * time.Hour).Round(time.Second).UTC(),
	}

	// Store the user
	err := repo.Store(ctx, user)
	assert.NoError(t, err)

	// Verify the user was stored
	var storedUser domain.User
	err = db.Get().Where("hashed_uuid = ?", "test-uuid-1").First(&storedUser).Error
	assert.NoError(t, err)
	
	// Round the time and convert to UTC to avoid microsecond/location differences in comparison
	storedUser.DeletionDate = storedUser.DeletionDate.Round(time.Second).UTC()
	
	assert.Equal(t, user.HashedUUID, storedUser.HashedUUID)
	assert.Equal(t, user.APITokenHash, storedUser.APITokenHash)
	assert.Equal(t, user.Scopes, storedUser.Scopes)
	assert.Equal(t, user.DeletionDate, storedUser.DeletionDate)

	// Test storing a user with duplicate HashedUUID
	duplicateUser := domain.User{
		HashedUUID:   "test-uuid-1", // Same HashedUUID
		APITokenHash: "test-token-hash-2",
		Scopes:       `{"read": true, "write": true}`,
		DeletionDate: time.Now().Add(48 * time.Hour),
	}

	// Attempt to store the duplicate user
	err = repo.Store(ctx, duplicateUser)
	assert.Error(t, err) // Should fail due to primary key constraint
}

func TestUserRepo_FindByAPITokenHash(t *testing.T) {
	// Setup
	db, cleanup := setupTestDBInstance(t, false) // Using file-based temp DB
	defer cleanup()

	repo := NewUserRepo(logger.Mock(), db)
	ctx := context.Background()

	// Test with non-existent token hash
	user, err := repo.FindByAPITokenHash(ctx, "non-existent-token-hash")
	assert.NoError(t, err)
	assert.Nil(t, user)

	// Add a user
	expectedUser := domain.User{
		HashedUUID:   "test-uuid-1",
		APITokenHash: "test-token-hash-1",
		Scopes:       `{"read": true, "write": false}`,
		DeletionDate: time.Now().Add(24 * time.Hour).Round(time.Second).UTC(),
	}
	err = db.Get().Create(&expectedUser).Error
	require.NoError(t, err)

	// Test with existing token hash
	foundUser, err := repo.FindByAPITokenHash(ctx, "test-token-hash-1")
	assert.NoError(t, err)
	require.NotNil(t, foundUser)
	
	// Round the time and convert to UTC to avoid microsecond/location differences in comparison
	foundUser.DeletionDate = foundUser.DeletionDate.Round(time.Second).UTC()
	
	assert.Equal(t, expectedUser.HashedUUID, foundUser.HashedUUID)
	assert.Equal(t, expectedUser.APITokenHash, foundUser.APITokenHash)
	assert.Equal(t, expectedUser.Scopes, foundUser.Scopes)
	assert.Equal(t, expectedUser.DeletionDate, foundUser.DeletionDate)
}

func TestUserRepo_UpdateDeletionDate(t *testing.T) {
	// Setup
	db, cleanup := setupTestDBInstance(t, false) // Using file-based temp DB
	defer cleanup()

	repo := NewUserRepo(logger.Mock(), db)
	ctx := context.Background()

	// Test with non-existent user
	newDate := time.Now().Add(48 * time.Hour).Round(time.Second).UTC()
	err := repo.UpdateDeletionDate(ctx, "non-existent-uuid", newDate)
	assert.Error(t, err) // Should return error for non-existent user

	// Add a user
	user := domain.User{
		HashedUUID:   "test-uuid-1",
		APITokenHash: "test-token-hash-1",
		Scopes:       `{"read": true, "write": false}`,
		DeletionDate: time.Now().Add(24 * time.Hour).Round(time.Second).UTC(),
	}
	err = db.Get().Create(&user).Error
	require.NoError(t, err)

	// Update the deletion date
	err = repo.UpdateDeletionDate(ctx, "test-uuid-1", newDate)
	assert.NoError(t, err)

	// Verify the deletion date was updated
	var updatedUser domain.User
	err = db.Get().Where("hashed_uuid = ?", "test-uuid-1").First(&updatedUser).Error
	assert.NoError(t, err)
	
	// Round the time and convert to UTC to avoid microsecond/location differences in comparison
	updatedUser.DeletionDate = updatedUser.DeletionDate.Round(time.Second).UTC()
	
	assert.Equal(t, newDate, updatedUser.DeletionDate)
}

func TestUserRepo_FindExpiredUserIDs(t *testing.T) {
	// Setup
	db, cleanup := setupTestDBInstance(t, false) // Using file-based temp DB
	defer cleanup()

	repo := NewUserRepo(logger.Mock(), db)
	ctx := context.Background()

	// Add users with different deletion dates
	now := time.Now()
	
	// User 1: Expired yesterday
	user1 := domain.User{
		HashedUUID:   "test-uuid-1",
		APITokenHash: "test-token-hash-1",
		Scopes:       `{"read": true, "write": false}`,
		DeletionDate: now.Add(-24 * time.Hour),
	}
	err := db.Get().Create(&user1).Error
	require.NoError(t, err)
	
	// User 2: Expires tomorrow
	user2 := domain.User{
		HashedUUID:   "test-uuid-2",
		APITokenHash: "test-token-hash-2",
		Scopes:       `{"read": true, "write": false}`,
		DeletionDate: now.Add(24 * time.Hour),
	}
	err = db.Get().Create(&user2).Error
	require.NoError(t, err)
	
	// User 3: Expired an hour ago
	user3 := domain.User{
		HashedUUID:   "test-uuid-3",
		APITokenHash: "test-token-hash-3",
		Scopes:       `{"read": true, "write": false}`,
		DeletionDate: now.Add(-1 * time.Hour),
	}
	err = db.Get().Create(&user3).Error
	require.NoError(t, err)

	// Find expired user IDs
	expiredIDs, err := repo.FindExpiredUserIDs(ctx, now)
	assert.NoError(t, err)
	assert.Len(t, expiredIDs, 2)
	assert.Contains(t, expiredIDs, "test-uuid-1")
	assert.Contains(t, expiredIDs, "test-uuid-3")
	assert.NotContains(t, expiredIDs, "test-uuid-2")
}

func TestUserRepo_DeleteUserAndAssociatedData(t *testing.T) {
	// Setup
	db, cleanup := setupTestDBInstance(t, false) // Using file-based temp DB
	defer cleanup()

	repo := NewUserRepo(logger.Mock(), db)
	ctx := context.Background()

	// Test with non-existent user
	err := repo.DeleteUserAndAssociatedData(ctx, "non-existent-uuid")
	assert.Error(t, err) // Should return error for non-existent user

	// Add a user
	user := domain.User{
		HashedUUID:   "test-uuid-1",
		APITokenHash: "test-token-hash-1",
		Scopes:       `{"read": true, "write": false}`,
		DeletionDate: time.Now().Add(24 * time.Hour),
	}
	err = db.Get().Create(&user).Error
	require.NoError(t, err)

	// Delete the user
	err = repo.DeleteUserAndAssociatedData(ctx, "test-uuid-1")
	assert.NoError(t, err)

	// Verify the user was deleted
	var count int64
	err = db.Get().Model(&domain.User{}).Where("hashed_uuid = ?", "test-uuid-1").Count(&count).Error
	assert.NoError(t, err)
	assert.Equal(t, int64(0), count)
}

func TestUserRepo_UpdateTokenAndScopes(t *testing.T) {
	// Setup
	db, cleanup := setupTestDBInstance(t, false) // Using file-based temp DB
	defer cleanup()

	repo := NewUserRepo(logger.Mock(), db)
	ctx := context.Background()

	// Test with non-existent user
	err := repo.UpdateTokenAndScopes(ctx, "non-existent-uuid", "new-token-hash", `{"read": true, "write": true}`)
	assert.Error(t, err) // Should return error for non-existent user

	// Add a user
	user := domain.User{
		HashedUUID:   "test-uuid-1",
		APITokenHash: "test-token-hash-1",
		Scopes:       `{"read": true, "write": false}`,
		DeletionDate: time.Now().Add(24 * time.Hour),
	}
	err = db.Get().Create(&user).Error
	require.NoError(t, err)

	// Update the token and scopes
	newTokenHash := "new-token-hash"
	newScopes := `{"read": true, "write": true, "admin": true}`
	err = repo.UpdateTokenAndScopes(ctx, "test-uuid-1", newTokenHash, newScopes)
	assert.NoError(t, err)

	// Verify the token and scopes were updated
	var updatedUser domain.User
	err = db.Get().Where("hashed_uuid = ?", "test-uuid-1").First(&updatedUser).Error
	assert.NoError(t, err)
	assert.Equal(t, newTokenHash, updatedUser.APITokenHash)
	assert.Equal(t, newScopes, updatedUser.Scopes)
}