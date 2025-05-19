package http

import (
	"bytes"
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/flurbudurbur/Shiori/internal/domain"
	// "github.com/flurbudurbur/Shiori/internal/sync" // Removed unused import (type alias used)
	"github.com/go-chi/chi/v5"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

// MockSyncService mocks the local alias http.syncService
type MockSyncService struct {
	mock.Mock
}

func (m *MockSyncService) GetSyncDataETag(ctx context.Context, apiKey string) (*string, error) {
	args := m.Called(ctx, apiKey)
	if args.Get(0) == nil && args.Error(1) == nil { // If both are nil, it means (*string)(nil) was intended
		return nil, nil
	}
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*string), args.Error(1)
}

func (m *MockSyncService) GetSyncDataAndETag(ctx context.Context, apiKey string) ([]byte, *string, error) {
	args := m.Called(ctx, apiKey)
	var data []byte
	var etag *string

	if args.Get(0) != nil {
		data = args.Get(0).([]byte)
	}
	if args.Get(1) != nil {
		etag = args.Get(1).(*string)
	}
	return data, etag, args.Error(2)
}

func (m *MockSyncService) SetSyncData(ctx context.Context, apiKey string, data []byte) (*string, error) {
	args := m.Called(ctx, apiKey, data)
	if args.Get(0) == nil && args.Error(1) == nil {
		return nil, nil
	}
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*string), args.Error(1)
}

func (m *MockSyncService) SetSyncDataIfMatch(ctx context.Context, apiKey string, etag string, data []byte) (*string, error) {
	args := m.Called(ctx, apiKey, etag, data)
	if args.Get(0) == nil && args.Error(1) == nil { // ETag mismatch or other condition returning (nil, nil)
		return nil, nil
	}
	if args.Get(0) == nil {
		return nil, args.Error(1) // Actual error
	}
	return args.Get(0).(*string), args.Error(1)
}

// MockProfileUUIDManager mocks profileUUIDManager
type MockProfileUUIDManager struct {
	mock.Mock
}

func (m *MockProfileUUIDManager) PromoteProfileUUID(ctx context.Context, userID string, profileUUID string) error {
	args := m.Called(ctx, userID, profileUUID)
	return args.Error(0)
}

func (m *MockProfileUUIDManager) GetOrGenerateProfileUUID(ctx context.Context, sessionID string) (string, error) {
	args := m.Called(ctx, sessionID)
	return args.String(0), args.Error(1)
}


func TestSyncHandler_GetContent(t *testing.T) {
	mockSyncSvc := new(MockSyncService)
	// uuidManager is not used by getContent
	handler := newSyncHandler(encoder{}, mockSyncSvc, nil)
	router := chi.NewRouter()
	router.Get("/sync/content", handler.getContent)

	testUser := &domain.User{HashedUUID: "user-sync-get-123"}
	ctxWithUser := contextWithUser(testUser) // Assumes contextWithUser from notification_test.go

	t.Run("success no etag", func(t *testing.T) {
		data := []byte("sync data")
		etag := "etag123"
		mockSyncSvc.On("GetSyncDataAndETag", mock.Anything, testUser.HashedUUID).Return(data, &etag, nil).Once()

		req := httptest.NewRequest("GET", "/sync/content", nil).WithContext(ctxWithUser)
		rr := httptest.NewRecorder()
		router.ServeHTTP(rr, req)

		assert.Equal(t, http.StatusOK, rr.Code)
		assert.Equal(t, etag, rr.Header().Get("ETag"))
		assert.Equal(t, "application/octet-stream", rr.Header().Get("Content-Type"))
		assert.Equal(t, data, rr.Body.Bytes())
		mockSyncSvc.AssertExpectations(t)
	})

	t.Run("success with If-None-Match, no change", func(t *testing.T) {
		clientEtag := "etag123"
		mockSyncSvc.On("GetSyncDataETag", mock.Anything, testUser.HashedUUID).Return(&clientEtag, nil).Once()

		req := httptest.NewRequest("GET", "/sync/content", nil).WithContext(ctxWithUser)
		req.Header.Set("If-None-Match", clientEtag)
		rr := httptest.NewRecorder()
		router.ServeHTTP(rr, req)

		assert.Equal(t, http.StatusNotModified, rr.Code)
		mockSyncSvc.AssertExpectations(t)
	})

	t.Run("success with If-None-Match, data changed", func(t *testing.T) {
		clientEtag := "old-etag"
		serverEtag := "new-etag"
		data := []byte("new data")
		mockSyncSvc.On("GetSyncDataETag", mock.Anything, testUser.HashedUUID).Return(&serverEtag, nil).Once() // Different ETag
		mockSyncSvc.On("GetSyncDataAndETag", mock.Anything, testUser.HashedUUID).Return(data, &serverEtag, nil).Once()


		req := httptest.NewRequest("GET", "/sync/content", nil).WithContext(ctxWithUser)
		req.Header.Set("If-None-Match", clientEtag)
		rr := httptest.NewRecorder()
		router.ServeHTTP(rr, req)

		assert.Equal(t, http.StatusOK, rr.Code)
		assert.Equal(t, serverEtag, rr.Header().Get("ETag"))
		assert.Equal(t, data, rr.Body.Bytes())
		mockSyncSvc.AssertExpectations(t)
	})
	
	t.Run("data not found", func(t *testing.T) {
		mockSyncSvc.On("GetSyncDataAndETag", mock.Anything, testUser.HashedUUID).Return(nil, nil, nil).Once() // nil data, nil etag, nil error

		req := httptest.NewRequest("GET", "/sync/content", nil).WithContext(ctxWithUser)
		rr := httptest.NewRecorder()
		router.ServeHTTP(rr, req)
		
		assert.Equal(t, http.StatusNotFound, rr.Code)
		mockSyncSvc.AssertExpectations(t)
	})
}

func TestSyncHandler_PutContent(t *testing.T) {
	mockSyncSvc := new(MockSyncService)
	mockUUIDMgr := new(MockProfileUUIDManager)
	handler := newSyncHandler(encoder{}, mockSyncSvc, mockUUIDMgr)
	router := chi.NewRouter()
	router.Put("/sync/content", handler.putContent)

	testUser := &domain.User{HashedUUID: "user-sync-put-123"}
	ctxWithUser := contextWithUser(testUser)
	requestData := []byte("new sync data")

	t.Run("success no If-Match", func(t *testing.T) {
		newEtag := "newEtag123"
		profileUUID := "profile-uuid-1"
		mockSyncSvc.On("SetSyncData", mock.Anything, testUser.HashedUUID, requestData).Return(&newEtag, nil).Once()
		mockUUIDMgr.On("GetOrGenerateProfileUUID", mock.Anything, testUser.HashedUUID).Return(profileUUID, nil).Once()
		mockUUIDMgr.On("PromoteProfileUUID", mock.Anything, testUser.HashedUUID, profileUUID).Return(nil).Once()

		req := httptest.NewRequest("PUT", "/sync/content", bytes.NewBuffer(requestData)).WithContext(ctxWithUser)
		rr := httptest.NewRecorder()
		router.ServeHTTP(rr, req)

		assert.Equal(t, http.StatusOK, rr.Code)
		assert.Equal(t, newEtag, rr.Header().Get("ETag"))
		mockSyncSvc.AssertExpectations(t)
		mockUUIDMgr.AssertExpectations(t)
	})

	t.Run("success with If-Match, match", func(t *testing.T) {
		clientEtag := "clientEtag456"
		newServerEtag := "serverEtag789"
		profileUUID := "profile-uuid-2"

		mockSyncSvc.On("SetSyncDataIfMatch", mock.Anything, testUser.HashedUUID, clientEtag, requestData).Return(&newServerEtag, nil).Once()
		mockUUIDMgr.On("GetOrGenerateProfileUUID", mock.Anything, testUser.HashedUUID).Return(profileUUID, nil).Once()
		mockUUIDMgr.On("PromoteProfileUUID", mock.Anything, testUser.HashedUUID, profileUUID).Return(nil).Once()

		req := httptest.NewRequest("PUT", "/sync/content", bytes.NewBuffer(requestData)).WithContext(ctxWithUser)
		req.Header.Set("If-Match", clientEtag)
		rr := httptest.NewRecorder()
		router.ServeHTTP(rr, req)

		assert.Equal(t, http.StatusOK, rr.Code)
		assert.Equal(t, newServerEtag, rr.Header().Get("ETag"))
		mockSyncSvc.AssertExpectations(t)
		mockUUIDMgr.AssertExpectations(t)
	})

	t.Run("precondition failed with If-Match, mismatch", func(t *testing.T) {
		clientEtag := "clientEtagMismatch"
		profileUUID := "profile-uuid-3"
		// SetSyncDataIfMatch returns (nil, nil) for precondition failed
		mockSyncSvc.On("SetSyncDataIfMatch", mock.Anything, testUser.HashedUUID, clientEtag, requestData).Return(nil, nil).Once()
		mockUUIDMgr.On("GetOrGenerateProfileUUID", mock.Anything, testUser.HashedUUID).Return(profileUUID, nil).Once()
		mockUUIDMgr.On("PromoteProfileUUID", mock.Anything, testUser.HashedUUID, profileUUID).Return(nil).Once()


		req := httptest.NewRequest("PUT", "/sync/content", bytes.NewBuffer(requestData)).WithContext(ctxWithUser)
		req.Header.Set("If-Match", clientEtag)
		rr := httptest.NewRecorder()
		router.ServeHTTP(rr, req)

		assert.Equal(t, http.StatusPreconditionFailed, rr.Code)
		mockSyncSvc.AssertExpectations(t)
		mockUUIDMgr.AssertExpectations(t)
	})
	
	t.Run("uuid manager GetOrGenerate error, sync proceeds", func(t *testing.T) {
		newEtag := "newEtagNoIfMatch"
		mockSyncSvc.On("SetSyncData", mock.Anything, testUser.HashedUUID, requestData).Return(&newEtag, nil).Once()
		mockUUIDMgr.On("GetOrGenerateProfileUUID", mock.Anything, testUser.HashedUUID).Return("", errors.New("get uuid error")).Once()
		// PromoteProfileUUID should not be called if GetOrGenerate fails

		req := httptest.NewRequest("PUT", "/sync/content", bytes.NewBuffer(requestData)).WithContext(ctxWithUser)
		rr := httptest.NewRecorder()
		router.ServeHTTP(rr, req)

		assert.Equal(t, http.StatusOK, rr.Code) // Sync itself succeeds
		assert.Equal(t, newEtag, rr.Header().Get("ETag"))
		mockSyncSvc.AssertExpectations(t)
		mockUUIDMgr.AssertExpectations(t) // Promote should not have been called
		mockUUIDMgr.AssertNotCalled(t, "PromoteProfileUUID", mock.Anything, mock.Anything, mock.Anything)
	})

	t.Run("uuid manager Promote error, sync proceeds", func(t *testing.T) {
		newEtag := "newEtagPromoteFail"
		profileUUID := "profile-uuid-promote-fail"
		mockSyncSvc.On("SetSyncData", mock.Anything, testUser.HashedUUID, requestData).Return(&newEtag, nil).Once()
		mockUUIDMgr.On("GetOrGenerateProfileUUID", mock.Anything, testUser.HashedUUID).Return(profileUUID, nil).Once()
		mockUUIDMgr.On("PromoteProfileUUID", mock.Anything, testUser.HashedUUID, profileUUID).Return(errors.New("promote error")).Once()

		req := httptest.NewRequest("PUT", "/sync/content", bytes.NewBuffer(requestData)).WithContext(ctxWithUser)
		rr := httptest.NewRecorder()
		router.ServeHTTP(rr, req)

		assert.Equal(t, http.StatusOK, rr.Code) // Sync itself succeeds
		assert.Equal(t, newEtag, rr.Header().Get("ETag"))
		mockSyncSvc.AssertExpectations(t)
		mockUUIDMgr.AssertExpectations(t)
	})
}