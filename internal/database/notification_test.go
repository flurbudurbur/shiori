package database

import (
	"context"
	"testing"
	// "time" // No longer needed directly in this file after changes

	"github.com/flurbudurbur/Shiori/internal/domain"
	"github.com/flurbudurbur/Shiori/internal/logger"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm" // Added for gorm.ErrRecordNotFound
	"github.com/flurbudurbur/Shiori/pkg/errors" // For errors.Is, consistent with main package
)

func TestNewNotificationRepo(t *testing.T) {
	log := logger.Mock()
	db, cleanup := setupTestDBInstance(t, true) // true for in-memory shared
	defer cleanup()

	repo := NewNotificationRepo(log, db)
	assert.NotNil(t, repo)

	// Type assertion to check if it's the expected concrete type, though interface satisfaction is key
	concreteRepo, ok := repo.(*NotificationRepo)
	require.True(t, ok, "NewNotificationRepo should return a concrete *NotificationRepo")
	assert.NotNil(t, concreteRepo.log)
	assert.NotNil(t, concreteRepo.db)
	assert.Equal(t, db, concreteRepo.db)
}

// Helper function to create a sample notification for testing
func createSampleNotification(name string, webhook string, enabled bool, icon string, userUUID string, nType domain.NotificationType, events []string) domain.Notification {
	return domain.Notification{
		UserHashedUUID: userUUID,
		Name:           name,
		Type:           nType,
		Webhook:        webhook,
		Enabled:        enabled,
		Events:         events,
		Icon:           icon,
		// Other fields like Token, Title, Username etc. can be added if needed for specific tests
		// ID, CreatedAt, UpdatedAt will be set by GORM
	}
}

func TestNotificationRepo_Store(t *testing.T) {
	log := logger.Mock()
	db, cleanup := setupTestDBInstance(t, true)
	defer cleanup()

	// Ensure user table exists for foreign key constraint if not using a mock DB that ignores it.
	// For this test, we assume migrations have run.
	// We might need to create a dummy user if FK is enforced strictly by the test DB setup.
	// For now, let's use a dummy UUID.
	dummyUserUUID := "test-user-uuid-for-notification"
	// Create a dummy user for FK if necessary - for now, assume it's not strictly checked or handled by test setup
	// _, err := db.Get().Create(&domain.User{HashedUUID: dummyUserUUID, Username: "testuser"}).Error
	// require.NoError(t, err, "Failed to create dummy user for notification test")


	repo := NewNotificationRepo(log, db)

	ctx := context.Background()
	sampleEvents := []string{string(domain.NotificationEventTest), string(domain.NotificationEventSyncSuccess)}
	sampleNotif := createSampleNotification(
		"Test Store Notif",
		"http://store.test/webhook",
		true,
		"store-icon.png",
		dummyUserUUID,
		domain.NotificationTypeDiscord,
		sampleEvents,
	)

	storedNotif, err := repo.Store(ctx, sampleNotif)
	require.NoError(t, err)
	require.NotNil(t, storedNotif)

	assert.NotZero(t, storedNotif.ID, "Stored notification ID should not be zero")
	assert.Equal(t, sampleNotif.UserHashedUUID, storedNotif.UserHashedUUID)
	assert.Equal(t, sampleNotif.Name, storedNotif.Name)
	assert.Equal(t, sampleNotif.Type, storedNotif.Type)
	assert.Equal(t, sampleNotif.Webhook, storedNotif.Webhook)
	assert.Equal(t, sampleNotif.Enabled, storedNotif.Enabled)
	assert.EqualValues(t, sampleNotif.Events, storedNotif.Events) // Use EqualValues for slices
	assert.Equal(t, sampleNotif.Icon, storedNotif.Icon)
	assert.False(t, storedNotif.CreatedAt.IsZero(), "CreatedAt should be set")
	assert.False(t, storedNotif.UpdatedAt.IsZero(), "UpdatedAt should be set")

	// Verify by finding it
	foundNotif, err := repo.FindByID(ctx, storedNotif.ID)
	require.NoError(t, err)
	require.NotNil(t, foundNotif)
	assert.Equal(t, storedNotif.ID, foundNotif.ID)
	assert.Equal(t, storedNotif.Name, foundNotif.Name)
	assert.Equal(t, storedNotif.UserHashedUUID, foundNotif.UserHashedUUID)
	assert.Equal(t, storedNotif.Type, foundNotif.Type)
	assert.EqualValues(t, storedNotif.Events, foundNotif.Events)
}

func TestNotificationRepo_FindByID(t *testing.T) {
	log := logger.Mock()
	db, cleanup := setupTestDBInstance(t, true)
	defer cleanup()
	repo := NewNotificationRepo(log, db)
	ctx := context.Background()

	dummyUserUUID := "test-user-uuid-findbyid"
	// Create a dummy user for FK if necessary
	// _, err := db.Get().Create(&domain.User{HashedUUID: dummyUserUUID, Username: "testuserfind"}).Error
	// require.NoError(t, err)


	// 1. Test finding an existing notification
	sampleEvents := []string{string(domain.NotificationEventTest)}
	sampleNotif := createSampleNotification(
		"Find Me Notif",
		"http://find.me/webhook",
		true,
		"find-icon.png",
		dummyUserUUID,
		domain.NotificationTypeTelegram,
		sampleEvents,
	)
	storedNotif, err := repo.Store(ctx, sampleNotif)
	require.NoError(t, err)
	require.NotNil(t, storedNotif)

	foundNotif, err := repo.FindByID(ctx, storedNotif.ID)
	require.NoError(t, err)
	require.NotNil(t, foundNotif)
	assert.Equal(t, storedNotif.ID, foundNotif.ID)
	assert.Equal(t, storedNotif.Name, foundNotif.Name)
	assert.Equal(t, storedNotif.Webhook, foundNotif.Webhook)
	assert.Equal(t, storedNotif.Enabled, foundNotif.Enabled)
	assert.EqualValues(t, storedNotif.Events, foundNotif.Events)
	assert.Equal(t, storedNotif.Icon, foundNotif.Icon)
	assert.Equal(t, storedNotif.UserHashedUUID, foundNotif.UserHashedUUID)
	assert.Equal(t, storedNotif.Type, foundNotif.Type)

	// 2. Test finding a non-existent notification
	nonExistentID := 99999
	notFoundNotif, err := repo.FindByID(ctx, nonExistentID)
	require.Error(t, err, "Expected an error when finding non-existent notification")
	// The actual error from the repo is `errors.Wrap(gorm.ErrRecordNotFound, "notification with id %d not found", id)`
	// So we check if gorm.ErrRecordNotFound is the cause.
	isRecordNotFound := errors.Is(err, gorm.ErrRecordNotFound)
	assert.True(t, isRecordNotFound, "Error should wrap gorm.ErrRecordNotFound")
	assert.Nil(t, notFoundNotif, "Found notification should be nil for non-existent ID")
}

func TestNotificationRepo_Update(t *testing.T) {
	log := logger.Mock()
	db, cleanup := setupTestDBInstance(t, true)
	defer cleanup()
	repo := NewNotificationRepo(log, db)
	ctx := context.Background()

	dummyUserUUID := "test-user-uuid-update"
	// _, err := db.Get().Create(&domain.User{HashedUUID: dummyUserUUID, Username: "testuserupdate"}).Error
	// require.NoError(t, err)


	// 1. Store an initial notification
	initialEvents := []string{string(domain.NotificationEventTest)}
	initialNotif := createSampleNotification(
		"Initial Update Notif",
		"http://initial.update/webhook",
		true,
		"initial-icon.png",
		dummyUserUUID,
		domain.NotificationTypeDiscord,
		initialEvents,
	)
	storedNotif, err := repo.Store(ctx, initialNotif)
	require.NoError(t, err)
	require.NotNil(t, storedNotif)
	originalUpdatedAt := storedNotif.UpdatedAt

	// 2. Test successful update
	updatedEvents := []string{string(domain.NotificationEventSyncSuccess), string(domain.NotificationEventSyncFailed)}
	notifToUpdate := *storedNotif // Make a copy
	notifToUpdate.Name = "Successfully Updated Notif"
	notifToUpdate.Webhook = "http://successful.update/webhook"
	notifToUpdate.Enabled = false
	notifToUpdate.Icon = "updated-icon.png"
	notifToUpdate.Type = domain.NotificationTypeTelegram
	notifToUpdate.Events = updatedEvents

	updatedNotif, err := repo.Update(ctx, notifToUpdate)
	require.NoError(t, err)
	require.NotNil(t, updatedNotif)

	assert.Equal(t, notifToUpdate.ID, updatedNotif.ID)
	assert.Equal(t, "Successfully Updated Notif", updatedNotif.Name)
	assert.Equal(t, "http://successful.update/webhook", updatedNotif.Webhook)
	assert.False(t, updatedNotif.Enabled)
	assert.Equal(t, "updated-icon.png", updatedNotif.Icon)
	assert.Equal(t, domain.NotificationTypeTelegram, updatedNotif.Type)
	assert.EqualValues(t, updatedEvents, updatedNotif.Events)
	assert.True(t, updatedNotif.UpdatedAt.After(originalUpdatedAt), "UpdatedAt should be more recent after update")

	// Verify by finding it again
	foundAfterUpdate, err := repo.FindByID(ctx, storedNotif.ID)
	require.NoError(t, err)
	require.NotNil(t, foundAfterUpdate)
	assert.Equal(t, "Successfully Updated Notif", foundAfterUpdate.Name)
	assert.False(t, foundAfterUpdate.Enabled)

	// 3. Test update with zero ID
	zeroIDNotif := createSampleNotification("Zero ID", "http://zero.id", true, "zero.png", dummyUserUUID, domain.NotificationTypeDiscord, nil)
	zeroIDNotif.ID = 0 // Explicitly set ID to 0
	_, err = repo.Update(ctx, zeroIDNotif)
	require.Error(t, err, "Expected error when updating notification with zero ID")
	assert.Contains(t, err.Error(), "cannot update notification with zero ID")

	// 4. Test update for a non-existent notification (GORM's Save might create it if not found, or affect 0 rows)
	// The current implementation of Update in notification.go doesn't explicitly return gorm.ErrRecordNotFound
	// if RowsAffected is 0, but logs a warning. Let's test this behavior.
	// If GORM's default is to upsert, this test might need adjustment or the Update method behavior clarified.
	// For now, we assume Save will not error if the record doesn't exist but will affect 0 rows if it's not an upsert.
	// The `Update` method in `notification.go` has a comment about `RowsAffected == 0`.
	// GORM's `Save` will insert if primary key is blank, or update if primary key exists.
	// If ID is provided but doesn't exist, it might still insert if GORM is configured for "upsert" behavior with Save,
	// or it might do nothing and result in RowsAffected == 0 without an error.
	// The code has `if result.RowsAffected == 0` and logs a warning. It does not return an error in that specific block.
	// Let's try to update a notification with a non-existent ID.
	nonExistentNotif := domain.Notification{
		ID:             99999,
		UserHashedUUID: dummyUserUUID,
		Name:           "Non Existent",
		Webhook:        "http://non.existent",
		Enabled:        true,
		Icon:           "non.png",
		Type:           domain.NotificationTypeDiscord,
		Events:         []string{"TEST"},
	}
	updatedNonExistent, err := repo.Update(ctx, nonExistentNotif)
	// GORM's Save behavior: if the record with the given ID doesn't exist, it will INSERT it.
	// So, `err` should be nil, and `updatedNonExistent` should be the newly created record.
	// This means the `if result.RowsAffected == 0` in the main code might not be hit if ID is non-zero and record doesn't exist,
	// because GORM will insert.
	// If the intention of the `Update` method is to *only* update existing records,
	// it should first check if the record exists.
	// Given the current `Save` usage, it will effectively upsert.
	require.NoError(t, err, "Update with non-existent ID should not error due to GORM Save behavior (upsert)")
	require.NotNil(t, updatedNonExistent)
	assert.Equal(t, nonExistentNotif.ID, updatedNonExistent.ID) // It should have been inserted with this ID

	// Let's verify it was indeed created
	foundNonExistent, err := repo.FindByID(ctx, nonExistentNotif.ID)
	require.NoError(t, err)
	require.NotNil(t, foundNonExistent)
	assert.Equal(t, "Non Existent", foundNonExistent.Name)
}

func TestNotificationRepo_Delete(t *testing.T) {
	log := logger.Mock()
	db, cleanup := setupTestDBInstance(t, true)
	defer cleanup()
	repo := NewNotificationRepo(log, db)
	ctx := context.Background()

	dummyUserUUID := "test-user-uuid-delete"
	// _, err := db.Get().Create(&domain.User{HashedUUID: dummyUserUUID, Username: "testuserdelete"}).Error
	// require.NoError(t, err)

	// 1. Store a notification to be deleted
	notifToDelete := createSampleNotification(
		"Delete Me Notif",
		"http://delete.me/webhook",
		true,
		"delete-icon.png",
		dummyUserUUID,
		domain.NotificationTypeDiscord,
		[]string{string(domain.NotificationEventTest)},
	)
	storedNotif, err := repo.Store(ctx, notifToDelete)
	require.NoError(t, err)
	require.NotNil(t, storedNotif)
	require.NotZero(t, storedNotif.ID)

	// 2. Test successful deletion
	err = repo.Delete(ctx, storedNotif.ID)
	require.NoError(t, err)

	// Verify it's gone
	_, err = repo.FindByID(ctx, storedNotif.ID)
	require.Error(t, err, "Expected error when finding a deleted notification")
	assert.True(t, errors.Is(err, gorm.ErrRecordNotFound), "Error should wrap gorm.ErrRecordNotFound for deleted item")

	// 3. Test deleting a non-existent notification
	nonExistentID := 98765
	err = repo.Delete(ctx, nonExistentID)
	require.Error(t, err, "Expected error when deleting non-existent notification")
	// The Delete method in notification.go returns a wrapped gorm.ErrRecordNotFound if RowsAffected is 0
	assert.True(t, errors.Is(err, gorm.ErrRecordNotFound), "Error should wrap gorm.ErrRecordNotFound for non-existent item deletion attempt")
}

func TestNotificationRepo_List(t *testing.T) {
	log := logger.Mock()
	db, cleanup := setupTestDBInstance(t, true)
	defer cleanup()
	repo := NewNotificationRepo(log, db)
	ctx := context.Background()

	dummyUserUUID := "test-user-uuid-list"
	// _, err := db.Get().Create(&domain.User{HashedUUID: dummyUserUUID, Username: "testuserlist"}).Error
	// require.NoError(t, err)

	// 1. Test list when empty
	notifications, err := repo.List(ctx)
	require.NoError(t, err)
	assert.Empty(t, notifications, "Notification list should be empty initially")

	// 2. Store some notifications
	notifC := createSampleNotification("Charlie Notif", "http://c.list", true, "c.png", dummyUserUUID, domain.NotificationTypeDiscord, nil)
	notifA := createSampleNotification("Alpha Notif", "http://a.list", true, "a.png", dummyUserUUID, domain.NotificationTypeTelegram, nil)
	notifB := createSampleNotification("Bravo Notif", "http://b.list", false, "b.png", dummyUserUUID, domain.NotificationTypeNotifiarr, nil)

	_, err = repo.Store(ctx, notifC)
	require.NoError(t, err)
	_, err = repo.Store(ctx, notifA)
	require.NoError(t, err)
	_, err = repo.Store(ctx, notifB)
	require.NoError(t, err)

	// 3. Test list with items, checking order (default is "name asc")
	notifications, err = repo.List(ctx)
	require.NoError(t, err)
	require.Len(t, notifications, 3, "Should retrieve 3 notifications")

	assert.Equal(t, "Alpha Notif", notifications[0].Name)
	assert.Equal(t, "Bravo Notif", notifications[1].Name)
	assert.Equal(t, "Charlie Notif", notifications[2].Name)

	// Check some details of one notification
	assert.Equal(t, "http://a.list", notifications[0].Webhook)
	assert.True(t, notifications[0].Enabled)
	assert.Equal(t, domain.NotificationTypeTelegram, notifications[0].Type)
	assert.Equal(t, dummyUserUUID, notifications[0].UserHashedUUID)
}

func TestNotificationRepo_Find(t *testing.T) {
	log := logger.Mock()
	db, cleanup := setupTestDBInstance(t, true)
	defer cleanup()
	repo := NewNotificationRepo(log, db)
	ctx := context.Background()

	dummyUserUUID := "test-user-uuid-find"
	// _, err := db.Get().Create(&domain.User{HashedUUID: dummyUserUUID, Username: "testuserfindmethod"}).Error
	// require.NoError(t, err)

	// Store some notifications
	names := []string{"Delta", "Alpha", "Charlie", "Bravo", "Echo"}
	for _, name := range names {
		notif := createSampleNotification(
			name+" Notif",
			"http://"+name+".find/webhook",
			true,
			name+".png",
			dummyUserUUID,
			domain.NotificationTypeDiscord,
			[]string{string(domain.NotificationEventTest)},
		)
		_, err := repo.Store(ctx, notif)
		require.NoError(t, err)
	}
	totalNotifications := len(names) // 5

	// 1. Test Find with no parameters (should return all, ordered by name, with correct total count)
	paramsAll := domain.NotificationQueryParams{}
	foundAll, countAll, errAll := repo.Find(ctx, paramsAll)
	require.NoError(t, errAll)
	assert.Equal(t, totalNotifications, countAll, "Total count should match number of stored notifications")
	require.Len(t, foundAll, totalNotifications, "Should retrieve all notifications")
	assert.Equal(t, "Alpha Notif", foundAll[0].Name) // Default sort is "name asc"
	assert.Equal(t, "Bravo Notif", foundAll[1].Name)
	assert.Equal(t, "Charlie Notif", foundAll[2].Name)
	assert.Equal(t, "Delta Notif", foundAll[3].Name)
	assert.Equal(t, "Echo Notif", foundAll[4].Name)

	// 2. Test Find with pagination (Limit)
	paramsLimit := domain.NotificationQueryParams{Limit: 2}
	foundLimit, countLimit, errLimit := repo.Find(ctx, paramsLimit)
	require.NoError(t, errLimit)
	assert.Equal(t, totalNotifications, countLimit, "Total count should remain the same with pagination")
	require.Len(t, foundLimit, 2, "Should retrieve 2 notifications due to limit")
	assert.Equal(t, "Alpha Notif", foundLimit[0].Name)
	assert.Equal(t, "Bravo Notif", foundLimit[1].Name)

	// 3. Test Find with pagination (Limit and Offset)
	paramsOffset := domain.NotificationQueryParams{Limit: 2, Offset: 1}
	foundOffset, countOffset, errOffset := repo.Find(ctx, paramsOffset)
	require.NoError(t, errOffset)
	assert.Equal(t, totalNotifications, countOffset, "Total count should remain the same with pagination")
	require.Len(t, foundOffset, 2, "Should retrieve 2 notifications due to limit and offset")
	assert.Equal(t, "Bravo Notif", foundOffset[0].Name) // Offset 1, so starts from Bravo
	assert.Equal(t, "Charlie Notif", foundOffset[1].Name)

	// 4. Test Find with pagination (Offset beyond total, GORM handles this gracefully by returning empty slice)
	paramsOffsetBeyond := domain.NotificationQueryParams{Limit: 2, Offset: uint64(totalNotifications)}
	foundOffsetBeyond, countOffsetBeyond, errOffsetBeyond := repo.Find(ctx, paramsOffsetBeyond)
	require.NoError(t, errOffsetBeyond)
	assert.Equal(t, totalNotifications, countOffsetBeyond, "Total count should remain the same")
	assert.Empty(t, foundOffsetBeyond, "Should retrieve no notifications if offset is beyond total")

	// 5. Test Find with pagination (Limit greater than remaining)
	paramsLimitGreater := domain.NotificationQueryParams{Limit: 10, Offset: uint64(totalNotifications - 2)} // Offset to last 2 items
	foundLimitGreater, countLimitGreater, errLimitGreater := repo.Find(ctx, paramsLimitGreater)
	require.NoError(t, errLimitGreater)
	assert.Equal(t, totalNotifications, countLimitGreater, "Total count should remain the same")
	require.Len(t, foundLimitGreater, 2, "Should retrieve the remaining 2 notifications")
	assert.Equal(t, "Delta Notif", foundLimitGreater[0].Name)
	assert.Equal(t, "Echo Notif", foundLimitGreater[1].Name)

	// Note: Filtering by UserHashedUUID, Search, Sort, and Filters.Indexers/PushStatus
	// are not explicitly tested here as the current implementation in notification.go
	// does not fully implement them (comments indicate this).
	// The default sort is "name asc".
	// If UserHashedUUID filtering were implemented, we'd add a test for that.
}