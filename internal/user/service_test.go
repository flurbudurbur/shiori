package user

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/flurbudurbur/Shiori/internal/domain"
	"github.com/flurbudurbur/Shiori/internal/logger"
	"github.com/flurbudurbur/Shiori/internal/valkey"
	"github.com/stretchr/testify/assert"
)

// FakeUserRepo is a fake implementation of the domain.UserRepo interface for testing
type FakeUserRepo struct {
	userCount                int
	userCountError           error
	storedUser               domain.User
	storeError               error
	foundUser                *domain.User
	findByHashedUUIDError    error
	findByAPITokenHashError  error
	updateDeletionDateError  error
	expiredUserIDs           []string
	findExpiredUserIDsError  error
	deleteUserError          error
	updateTokenAndScopesError error
}

func (f *FakeUserRepo) GetUserCount(ctx context.Context) (int, error) {
	return f.userCount, f.userCountError
}

func (f *FakeUserRepo) FindByHashedUUID(ctx context.Context, hashedUUID string) (*domain.User, error) {
	return f.foundUser, f.findByHashedUUIDError
}

func (f *FakeUserRepo) Store(ctx context.Context, user domain.User) error {
	f.storedUser = user
	return f.storeError
}

func (f *FakeUserRepo) UpdateDeletionDate(ctx context.Context, hashedUUID string, newDeletionDate time.Time) error {
	return f.updateDeletionDateError
}

func (f *FakeUserRepo) FindExpiredUserIDs(ctx context.Context, now time.Time) ([]string, error) {
	return f.expiredUserIDs, f.findExpiredUserIDsError
}

func (f *FakeUserRepo) FindByAPITokenHash(ctx context.Context, tokenHash string) (*domain.User, error) {
	return f.foundUser, f.findByAPITokenHashError
}

func (f *FakeUserRepo) DeleteUserAndAssociatedData(ctx context.Context, hashedUUID string) error {
	return f.deleteUserError
}

func (f *FakeUserRepo) UpdateTokenAndScopes(ctx context.Context, hashedUUID string, apiTokenHash string, scopes string) error {
	return f.updateTokenAndScopesError
}

// FakeRateLimiter is a fake implementation of the RateLimiterStore interface for testing
type FakeRateLimiter struct {
	isLockedOutResult    bool
	isLockedOutError     error
	incrementFailureCount int64
	incrementFailureError error
	clearFailuresError    error
	setLockoutError       error
}

func (f *FakeRateLimiter) IsLockedOut(ctx context.Context, key string) (bool, error) {
	return f.isLockedOutResult, f.isLockedOutError
}

func (f *FakeRateLimiter) IncrementFailure(ctx context.Context, key string) (int64, error) {
	return f.incrementFailureCount, f.incrementFailureError
}

func (f *FakeRateLimiter) ClearFailures(ctx context.Context, key string) error {
	return f.clearFailuresError
}

func (f *FakeRateLimiter) SetLockout(ctx context.Context, key string) error {
	return f.setLockoutError
}

// FakeProfileUUIDRepo is a fake implementation of the domain.ProfileUUIDRepo interface for testing
type FakeProfileUUIDRepo struct {
	foundProfileUUID        *domain.ProfileUUID
	findByUserIDError       error
	updateLastActivityError error
	storeError              error
	expiredProfileUUIDs     []domain.ProfileUUID
	findExpiredError        error
	staleProfileUUIDs       []domain.ProfileUUID
	findStaleError          error
	orphanedProfileUUIDs    []domain.ProfileUUID
	findOrphanedError       error
	deleteProfileUUIDsCount int
	deleteProfileUUIDsError error
	softDeleteCount         int
	softDeleteError         error
	deleteProfileUUIDError  error
}

func (f *FakeProfileUUIDRepo) FindByUserID(ctx context.Context, userID string) (*domain.ProfileUUID, error) {
	return f.foundProfileUUID, f.findByUserIDError
}

func (f *FakeProfileUUIDRepo) UpdateLastActivity(ctx context.Context, userID string, profileUUID string) error {
	return f.updateLastActivityError
}

func (f *FakeProfileUUIDRepo) Store(ctx context.Context, profileUUID domain.ProfileUUID) error {
	return f.storeError
}

func (f *FakeProfileUUIDRepo) FindExpiredProfileUUIDs(ctx context.Context, olderThan time.Time) ([]domain.ProfileUUID, error) {
	return f.expiredProfileUUIDs, f.findExpiredError
}

func (f *FakeProfileUUIDRepo) FindStaleProfileUUIDs(ctx context.Context, olderThan time.Time, limit int) ([]domain.ProfileUUID, error) {
	return f.staleProfileUUIDs, f.findStaleError
}

func (f *FakeProfileUUIDRepo) FindOrphanedProfileUUIDs(ctx context.Context, limit int) ([]domain.ProfileUUID, error) {
	return f.orphanedProfileUUIDs, f.findOrphanedError
}

func (f *FakeProfileUUIDRepo) DeleteProfileUUIDs(ctx context.Context, profileUUIDs []domain.ProfileUUID) (int, error) {
	return f.deleteProfileUUIDsCount, f.deleteProfileUUIDsError
}

func (f *FakeProfileUUIDRepo) SoftDeleteProfileUUIDs(ctx context.Context, profileUUIDs []domain.ProfileUUID) (int, error) {
	return f.softDeleteCount, f.softDeleteError
}

func (f *FakeProfileUUIDRepo) DeleteProfileUUID(ctx context.Context, userID string, profileUUID string) error {
	return f.deleteProfileUUIDError
}

func TestNewService(t *testing.T) {
	// Setup
	log := logger.Mock()
	repo := &FakeUserRepo{}
	limiter := &FakeRateLimiter{}
	valkeyService := &valkey.Service{} // Empty service for testing
	profileUUIDRepo := &FakeProfileUUIDRepo{}

	// Create the service
	svc := NewService(repo, limiter, log, valkeyService, profileUUIDRepo)

	// Assert
	assert.NotNil(t, svc, "Service should not be nil")
}

func TestService_GetUserCount(t *testing.T) {
	// Setup
	log := logger.Mock()
	ctx := context.Background()
	limiter := &FakeRateLimiter{}
	valkeyService := &valkey.Service{} // Empty service for testing
	profileUUIDRepo := &FakeProfileUUIDRepo{}

	// Test successful case
	repo := &FakeUserRepo{
		userCount: 5,
	}
	svc := NewService(repo, limiter, log, valkeyService, profileUUIDRepo)

	// Call the method
	count, err := svc.GetUserCount(ctx)

	// Assert
	assert.NoError(t, err)
	assert.Equal(t, 5, count)

	// Test error case
	repo = &FakeUserRepo{
		userCountError: errors.New("database error"),
	}
	svc = NewService(repo, limiter, log, valkeyService, profileUUIDRepo)
	
	count, err = svc.GetUserCount(ctx)
	assert.Error(t, err)
	assert.Equal(t, 0, count)
}

func TestService_RegisterNewUser(t *testing.T) {
	// Setup
	log := logger.Mock()
	ctx := context.Background()
	limiter := &FakeRateLimiter{}
	valkeyService := &valkey.Service{} // Empty service for testing
	profileUUIDRepo := &FakeProfileUUIDRepo{}

	// Test successful case
	repo := &FakeUserRepo{}
	svc := NewService(repo, limiter, log, valkeyService, profileUUIDRepo)

	// Call the method
	hashedUUID, err := svc.RegisterNewUser(ctx)

	// Assert
	assert.NoError(t, err)
	assert.NotEmpty(t, hashedUUID)
	assert.Equal(t, hashedUUID, repo.storedUser.HashedUUID)
	assert.NotEmpty(t, repo.storedUser.APITokenHash)
	assert.NotEmpty(t, repo.storedUser.Scopes)
	assert.True(t, repo.storedUser.DeletionDate.After(time.Now()))

	// Test store error case
	repo = &FakeUserRepo{
		storeError: errors.New("database error"),
	}
	svc = NewService(repo, limiter, log, valkeyService, profileUUIDRepo)
	
	hashedUUID, err = svc.RegisterNewUser(ctx)
	assert.Error(t, err)
	assert.Empty(t, hashedUUID)
}

func TestService_GetUserForAuthentication(t *testing.T) {
	// Setup
	log := logger.Mock()
	ctx := context.Background()
	limiter := &FakeRateLimiter{}
	valkeyService := &valkey.Service{} // Empty service for testing
	profileUUIDRepo := &FakeProfileUUIDRepo{}
	hashedUUID := "test-hashed-uuid"
	
	// Create a test user
	testUser := &domain.User{
		HashedUUID:   hashedUUID,
		APITokenHash: "test-token-hash",
		Scopes:       `{"read": true, "write": false}`,
		DeletionDate: time.Now().Add(24 * time.Hour),
	}

	// Test successful case
	repo := &FakeUserRepo{
		foundUser: testUser,
	}
	svc := NewService(repo, limiter, log, valkeyService, profileUUIDRepo)

	// Call the method
	user, err := svc.GetUserForAuthentication(ctx, hashedUUID)

	// Assert
	assert.NoError(t, err)
	assert.Equal(t, testUser, user)

	// Test user not found case
	repo = &FakeUserRepo{
		foundUser: nil,
	}
	svc = NewService(repo, limiter, log, valkeyService, profileUUIDRepo)
	
	user, err = svc.GetUserForAuthentication(ctx, "non-existent-uuid")
	assert.NoError(t, err) // No error, just nil user
	assert.Nil(t, user)

	// Test error case
	repo = &FakeUserRepo{
		findByHashedUUIDError: errors.New("database error"),
	}
	svc = NewService(repo, limiter, log, valkeyService, profileUUIDRepo)
	
	user, err = svc.GetUserForAuthentication(ctx, "error-uuid")
	assert.Error(t, err)
	assert.Nil(t, user)

	// Test expired user case
	expiredUser := &domain.User{
		HashedUUID:   "expired-uuid",
		APITokenHash: "test-token-hash",
		Scopes:       `{"read": true, "write": false}`,
		DeletionDate: time.Now().Add(-24 * time.Hour), // Past date
	}
	repo = &FakeUserRepo{
		foundUser: expiredUser,
	}
	svc = NewService(repo, limiter, log, valkeyService, profileUUIDRepo)
	
	user, err = svc.GetUserForAuthentication(ctx, "expired-uuid")
	assert.Error(t, err)
	assert.Equal(t, ErrUserExpired, err)
	assert.Nil(t, user)
}

func TestService_ResetAndRetrieveUserToken(t *testing.T) {
	// Setup
	log := logger.Mock()
	ctx := context.Background()
	limiter := &FakeRateLimiter{}
	valkeyService := &valkey.Service{} // Empty service for testing
	profileUUIDRepo := &FakeProfileUUIDRepo{}
	hashedUUID := "test-hashed-uuid"
	
	// Create a test user
	testUser := &domain.User{
		HashedUUID:   hashedUUID,
		APITokenHash: "test-token-hash",
		Scopes:       `{"read": true, "write": false}`,
		DeletionDate: time.Now().Add(24 * time.Hour),
	}

	// Test successful case
	repo := &FakeUserRepo{
		foundUser: testUser,
	}
	svc := NewService(repo, limiter, log, valkeyService, profileUUIDRepo)

	// Call the method
	plainToken, err := svc.ResetAndRetrieveUserToken(ctx, hashedUUID)

	// Assert
	assert.NoError(t, err)
	assert.NotEmpty(t, plainToken)

	// Test user not found case
	repo = &FakeUserRepo{
		foundUser: nil,
	}
	svc = NewService(repo, limiter, log, valkeyService, profileUUIDRepo)
	
	plainToken, err = svc.ResetAndRetrieveUserToken(ctx, "non-existent-uuid")
	assert.Error(t, err)
	assert.Empty(t, plainToken)

	// Test find error case
	repo = &FakeUserRepo{
		findByHashedUUIDError: errors.New("database error"),
	}
	svc = NewService(repo, limiter, log, valkeyService, profileUUIDRepo)
	
	plainToken, err = svc.ResetAndRetrieveUserToken(ctx, "error-uuid")
	assert.Error(t, err)
	assert.Empty(t, plainToken)

	// Test update error case
	repo = &FakeUserRepo{
		foundUser: testUser,
		updateTokenAndScopesError: errors.New("update error"),
	}
	svc = NewService(repo, limiter, log, valkeyService, profileUUIDRepo)
	
	plainToken, err = svc.ResetAndRetrieveUserToken(ctx, hashedUUID)
	assert.Error(t, err)
	assert.Empty(t, plainToken)
}

func TestGenerateApiToken(t *testing.T) {
	// Test with different byte lengths
	lengths := []int{16, 32, 64}
	
	for _, length := range lengths {
		token, err := generateApiToken(length)
		assert.NoError(t, err)
		assert.Len(t, token, length*2) // Hex encoding doubles the length
		
		// Generate another token and ensure they're different
		token2, err := generateApiToken(length)
		assert.NoError(t, err)
		assert.NotEqual(t, token, token2, "Tokens should be random and different")
	}
}

func TestHashApiToken(t *testing.T) {
	// Test hashing a token
	token := "test-token"
	hash, err := hashApiToken(token)
	assert.NoError(t, err)
	assert.NotEmpty(t, hash)
	assert.NotEqual(t, token, hash, "Hash should be different from the original token")
	
	// Test hashing the same token again produces a different hash (due to salt)
	hash2, err := hashApiToken(token)
	assert.NoError(t, err)
	assert.NotEqual(t, hash, hash2, "Bcrypt should generate different hashes for the same input due to salt")
}

func TestGenerateHashedUUID(t *testing.T) {
	// Test generating a UUID
	uuid, err := generateHashedUUID()
	assert.NoError(t, err)
	assert.NotEmpty(t, uuid)
	assert.Len(t, uuid, 36) // Standard UUID string length
	
	// Generate another UUID and ensure they're different
	uuid2, err := generateHashedUUID()
	assert.NoError(t, err)
	assert.NotEqual(t, uuid, uuid2, "UUIDs should be random and different")
}