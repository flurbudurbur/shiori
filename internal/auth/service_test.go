package auth

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/flurbudurbur/Shiori/internal/domain"
	"github.com/flurbudurbur/Shiori/internal/logger"
	"github.com/flurbudurbur/Shiori/internal/user"
	"github.com/stretchr/testify/assert"
)

// FakeUserService is a fake implementation of the user.Service interface for testing
type FakeUserService struct {
	userCount                int
	userCountError           error
	registeredUUID           string
	registerError            error
	authenticatedUser        *domain.User
	authenticationError      error
	resetTokenResult         string
	resetTokenError          error
	profileUUID              string
	profileUUIDError         error
	promoteProfileUUIDError  error
}

func (f *FakeUserService) GetUserCount(ctx context.Context) (int, error) {
	return f.userCount, f.userCountError
}

func (f *FakeUserService) RegisterNewUser(ctx context.Context) (string, error) {
	return f.registeredUUID, f.registerError
}

func (f *FakeUserService) AuthenticateUserByToken(ctx context.Context, plainToken string) (*domain.User, error) {
	return f.authenticatedUser, f.authenticationError
}

func (f *FakeUserService) GetUserForAuthentication(ctx context.Context, hashedUUID string) (*domain.User, error) {
	return f.authenticatedUser, f.authenticationError
}

func (f *FakeUserService) ResetAndRetrieveUserToken(ctx context.Context, hashedUUID string) (string, error) {
	return f.resetTokenResult, f.resetTokenError
}

func (f *FakeUserService) GetOrGenerateProfileUUID(ctx context.Context, sessionID string) (string, error) {
	return f.profileUUID, f.profileUUIDError
}

func (f *FakeUserService) PromoteProfileUUID(ctx context.Context, userID string, profileUUID string) error {
	return f.promoteProfileUUIDError
}

func TestNewService(t *testing.T) {
	// Setup
	log := logger.Mock()
	fakeUserSvc := &FakeUserService{}

	// Create the service
	svc := NewService(log, fakeUserSvc)

	// Assert
	assert.NotNil(t, svc, "Service should not be nil")
}

func TestService_GetUserCount(t *testing.T) {
	// Setup
	log := logger.Mock()
	ctx := context.Background()

	// Test successful case
	fakeUserSvc := &FakeUserService{
		userCount: 5,
	}
	svc := NewService(log, fakeUserSvc)

	// Call the method
	count, err := svc.GetUserCount(ctx)

	// Assert
	assert.NoError(t, err)
	assert.Equal(t, 5, count)

	// Test error case
	fakeUserSvc = &FakeUserService{
		userCountError: errors.New("database error"),
	}
	svc = NewService(log, fakeUserSvc)
	
	count, err = svc.GetUserCount(ctx)
	assert.Error(t, err)
	assert.Equal(t, 0, count)
}

func TestService_GenerateUserBookmark(t *testing.T) {
	// Setup
	log := logger.Mock()
	ctx := context.Background()
	expectedUUID := "test-hashed-uuid"

	// Test successful case
	fakeUserSvc := &FakeUserService{
		registeredUUID: expectedUUID,
	}
	svc := NewService(log, fakeUserSvc)

	// Call the method
	hashedUUID, err := svc.GenerateUserBookmark(ctx)

	// Assert
	assert.NoError(t, err)
	assert.Equal(t, expectedUUID, hashedUUID)

	// Test error case
	fakeUserSvc = &FakeUserService{
		registerError: errors.New("registration error"),
	}
	svc = NewService(log, fakeUserSvc)
	
	hashedUUID, err = svc.GenerateUserBookmark(ctx)
	assert.Error(t, err)
	assert.Equal(t, "", hashedUUID)
}

func TestService_AuthenticateUser(t *testing.T) {
	// Setup
	log := logger.Mock()
	ctx := context.Background()
	hashedUUID := "test-hashed-uuid"
	expectedUser := &domain.User{
		HashedUUID:   hashedUUID,
		Scopes:       `{"read": true, "write": false}`,
		DeletionDate: time.Now().Add(24 * time.Hour),
	}

	// Test successful authentication
	fakeUserSvc := &FakeUserService{
		authenticatedUser: expectedUser,
	}
	svc := NewService(log, fakeUserSvc)

	// Call the method
	foundUser, err := svc.AuthenticateUser(ctx, hashedUUID)

	// Assert
	assert.NoError(t, err)
	assert.Equal(t, expectedUser, foundUser)

	// Test user not found case
	fakeUserSvc = &FakeUserService{
		authenticatedUser: nil,
	}
	svc = NewService(log, fakeUserSvc)
	
	foundUser, err = svc.AuthenticateUser(ctx, "non-existent-uuid")
	assert.Error(t, err)
	assert.Equal(t, user.ErrAuthenticationFailed, err)
	assert.Nil(t, foundUser)

	// Test error case
	fakeUserSvc = &FakeUserService{
		authenticationError: errors.New("database error"),
	}
	svc = NewService(log, fakeUserSvc)
	
	foundUser, err = svc.AuthenticateUser(ctx, "error-uuid")
	assert.Error(t, err)
	assert.Nil(t, foundUser)
}

func TestMin(t *testing.T) {
	// Test cases
	testCases := []struct {
		a, b, expected int
	}{
		{5, 10, 5},
		{10, 5, 5},
		{0, 5, 0},
		{-5, 5, -5},
		{5, -5, -5},
		{0, 0, 0},
	}

	// Run tests
	for _, tc := range testCases {
		result := min(tc.a, tc.b)
		assert.Equal(t, tc.expected, result, "min(%d, %d) should be %d", tc.a, tc.b, tc.expected)
	}
}