package http

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	// "time" // Removed potentially unused import

	"github.com/flurbudurbur/Shiori/internal/domain"
	"github.com/flurbudurbur/Shiori/internal/logger"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

// MockUserServiceForUserHandler mocks userservice.Service for user handler tests
type MockUserServiceForUserHandler struct {
	mock.Mock
}

func (m *MockUserServiceForUserHandler) GetUserCount(ctx context.Context) (int, error) {
	args := m.Called(ctx)
	return args.Int(0), args.Error(1)
}

func (m *MockUserServiceForUserHandler) RegisterNewUser(ctx context.Context) (string, error) {
	args := m.Called(ctx)
	return args.String(0), args.Error(1)
}

func (m *MockUserServiceForUserHandler) AuthenticateUserByToken(ctx context.Context, plainToken string) (*domain.User, error) {
	args := m.Called(ctx, plainToken)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*domain.User), args.Error(1)
}

func (m *MockUserServiceForUserHandler) GetUserForAuthentication(ctx context.Context, hashedUUID string) (*domain.User, error) {
	args := m.Called(ctx, hashedUUID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*domain.User), args.Error(1)
}

func (m *MockUserServiceForUserHandler) ResetAndRetrieveUserToken(ctx context.Context, hashedUUID string) (string, error) {
	args := m.Called(ctx, hashedUUID)
	return args.String(0), args.Error(1)
}

func (m *MockUserServiceForUserHandler) GetOrGenerateProfileUUID(ctx context.Context, sessionID string) (string, error) {
	args := m.Called(ctx, sessionID)
	return args.String(0), args.Error(1)
}

func (m *MockUserServiceForUserHandler) PromoteProfileUUID(ctx context.Context, userID string, profileUUID string) error {
	args := m.Called(ctx, userID, profileUUID)
	return args.Error(0)
}


func TestUserResource_HandleResetAndGetUserToken(t *testing.T) {
	mockUserSvc := new(MockUserServiceForUserHandler)
	testLogger := logger.Mock().With().Logger() // Get a zerolog.Logger instance
	realEncoder := encoder{}
	userResource := NewUserResource(mockUserSvc, testLogger, realEncoder)

	// No chi router needed as we call the handler directly.
	// Routes are defined on s.Server in server.go, not directly on UserResource.
	// We test the handler method itself.

	testUser := &domain.User{HashedUUID: "user-reset-token-123"}
	ctxWithUser := contextWithUser(testUser) // Assumes contextWithUser from other _test.go files

	t.Run("success", func(t *testing.T) {
		expectedToken := "new-plain-api-token"
		mockUserSvc.On("ResetAndRetrieveUserToken", mock.AnythingOfType("*context.valueCtx"), testUser.HashedUUID).Return(expectedToken, nil).Once()

		req := httptest.NewRequest("POST", "/profile/api-token", nil).WithContext(ctxWithUser)
		rr := httptest.NewRecorder()
		userResource.handleResetAndGetUserToken(rr, req)

		assert.Equal(t, http.StatusOK, rr.Code)
		var resp map[string]string
		err := json.Unmarshal(rr.Body.Bytes(), &resp)
		require.NoError(t, err)
		assert.Equal(t, expectedToken, resp["api_token"])
		mockUserSvc.AssertExpectations(t)
	})

	t.Run("user not in context", func(t *testing.T) {
		req := httptest.NewRequest("POST", "/profile/api-token", nil) // No user in context
		rr := httptest.NewRecorder()
		userResource.handleResetAndGetUserToken(rr, req)

		assert.Equal(t, http.StatusUnauthorized, rr.Code)
		// Add assertion for error response body if needed
	})

	t.Run("service error", func(t *testing.T) {
		mockUserSvc.On("ResetAndRetrieveUserToken", mock.AnythingOfType("*context.valueCtx"), testUser.HashedUUID).Return("", errors.New("service error")).Once()

		req := httptest.NewRequest("POST", "/profile/api-token", nil).WithContext(ctxWithUser)
		rr := httptest.NewRecorder()
		userResource.handleResetAndGetUserToken(rr, req)

		assert.Equal(t, http.StatusInternalServerError, rr.Code)
		mockUserSvc.AssertExpectations(t)
	})
}


func TestUserResource_HandleGetProfile(t *testing.T) {
	mockUserSvc := new(MockUserServiceForUserHandler)
	testLogger := logger.Mock().With().Logger()
	realEncoder := encoder{}
	userResource := NewUserResource(mockUserSvc, testLogger, realEncoder)

	baseUser := &domain.User{HashedUUID: "user-get-profile-123"}

	t.Run("success with sessionID from context", func(t *testing.T) {
		sessionID := "real-session-id"
		expectedProfileUUID := "profile-uuid-from-session"
		
		ctxWithUserAndSession := context.WithValue(contextWithUser(baseUser), ContextKey("sessionID"), sessionID)


		mockUserSvc.On("GetOrGenerateProfileUUID", mock.Anything, sessionID).Return(expectedProfileUUID, nil).Once()
		mockUserSvc.On("PromoteProfileUUID", mock.Anything, baseUser.HashedUUID, expectedProfileUUID).Return(nil).Once()

		req := httptest.NewRequest("GET", "/profile", nil).WithContext(ctxWithUserAndSession)
		rr := httptest.NewRecorder()
		userResource.handleGetProfile(rr, req)

		assert.Equal(t, http.StatusOK, rr.Code)
		var resp map[string]interface{}
		err := json.Unmarshal(rr.Body.Bytes(), &resp)
		require.NoError(t, err)
		assert.Equal(t, expectedProfileUUID, resp["profile_uuid"])
		assert.Equal(t, baseUser.HashedUUID, resp["user_id"])
		mockUserSvc.AssertExpectations(t)
	})

	t.Run("success fallback to HashedUUID for sessionID", func(t *testing.T) {
		expectedProfileUUID := "profile-uuid-from-hashed"
		ctxWithOnlyUser := contextWithUser(baseUser) // No sessionID in context

		mockUserSvc.On("GetOrGenerateProfileUUID", mock.Anything, baseUser.HashedUUID).Return(expectedProfileUUID, nil).Once()
		mockUserSvc.On("PromoteProfileUUID", mock.Anything, baseUser.HashedUUID, expectedProfileUUID).Return(nil).Once()

		req := httptest.NewRequest("GET", "/profile", nil).WithContext(ctxWithOnlyUser)
		rr := httptest.NewRecorder()
		userResource.handleGetProfile(rr, req)

		assert.Equal(t, http.StatusOK, rr.Code)
		var resp map[string]interface{}
		err := json.Unmarshal(rr.Body.Bytes(), &resp)
		require.NoError(t, err)
		assert.Equal(t, expectedProfileUUID, resp["profile_uuid"])
		mockUserSvc.AssertExpectations(t)
	})
	
	t.Run("GetOrGenerateProfileUUID error", func(t *testing.T) {
		ctxWithOnlyUser := contextWithUser(baseUser)
		mockUserSvc.On("GetOrGenerateProfileUUID", mock.Anything, baseUser.HashedUUID).Return("", errors.New("uuid gen error")).Once()
		// Promote should not be called

		req := httptest.NewRequest("GET", "/profile", nil).WithContext(ctxWithOnlyUser)
		rr := httptest.NewRecorder()
		userResource.handleGetProfile(rr, req)

		assert.Equal(t, http.StatusInternalServerError, rr.Code)
		mockUserSvc.AssertExpectations(t)
		mockUserSvc.AssertNotCalled(t, "PromoteProfileUUID", mock.Anything, mock.Anything, mock.Anything)
	})

	t.Run("PromoteProfileUUID error, request still succeeds", func(t *testing.T) {
		expectedProfileUUID := "profile-uuid-promote-fail"
		ctxWithOnlyUser := contextWithUser(baseUser)

		mockUserSvc.On("GetOrGenerateProfileUUID", mock.Anything, baseUser.HashedUUID).Return(expectedProfileUUID, nil).Once()
		mockUserSvc.On("PromoteProfileUUID", mock.Anything, baseUser.HashedUUID, expectedProfileUUID).Return(errors.New("promote error")).Once()

		req := httptest.NewRequest("GET", "/profile", nil).WithContext(ctxWithOnlyUser)
		rr := httptest.NewRecorder()
		userResource.handleGetProfile(rr, req)

		assert.Equal(t, http.StatusOK, rr.Code) // Request succeeds even if promotion fails
		var resp map[string]interface{}
		err := json.Unmarshal(rr.Body.Bytes(), &resp)
		require.NoError(t, err)
		assert.Equal(t, expectedProfileUUID, resp["profile_uuid"])
		mockUserSvc.AssertExpectations(t)
	})

	t.Run("user not in context", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/profile", nil) // No user context
		rr := httptest.NewRecorder()
		userResource.handleGetProfile(rr, req)
		assert.Equal(t, http.StatusUnauthorized, rr.Code)
	})
}