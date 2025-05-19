package http

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strconv"
	"testing"

	"github.com/flurbudurbur/Shiori/internal/domain"
	"github.com/go-chi/chi/v5"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

// MockNotificationService is a mock for the notificationService interface
type MockNotificationService struct {
	mock.Mock
}

func (m *MockNotificationService) Find(ctx context.Context, params domain.NotificationQueryParams) ([]domain.Notification, int, error) {
	args := m.Called(ctx, params)
	if args.Get(0) == nil {
		return nil, args.Int(1), args.Error(2)
	}
	return args.Get(0).([]domain.Notification), args.Int(1), args.Error(2)
}

func (m *MockNotificationService) FindByID(ctx context.Context, id int) (*domain.Notification, error) {
	args := m.Called(ctx, id)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*domain.Notification), args.Error(1)
}

func (m *MockNotificationService) Store(ctx context.Context, n domain.Notification) (*domain.Notification, error) {
	args := m.Called(ctx, n)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*domain.Notification), args.Error(1)
}

func (m *MockNotificationService) Update(ctx context.Context, n domain.Notification) (*domain.Notification, error) {
	args := m.Called(ctx, n)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*domain.Notification), args.Error(1)
}

func (m *MockNotificationService) Delete(ctx context.Context, id int) error {
	args := m.Called(ctx, id)
	return args.Error(0)
}

func (m *MockNotificationService) Test(ctx context.Context, n domain.Notification) error {
	args := m.Called(ctx, n)
	return args.Error(0)
}

func contextWithUser(user *domain.User) context.Context {
	// Assuming UserContextKey is "user" as defined in middleware.go (or http package const)
	// If UserContextKey is not exported or available, use the string literal.
	// For this test, let's assume `UserContextKey` is an exported const from this http package or accessible.
	// If not, use: return context.WithValue(context.Background(), ContextKey("user"), user)
	return context.WithValue(context.Background(), UserContextKey, user)
}


func TestNotificationHandler_List(t *testing.T) {
	mockService := new(MockNotificationService)
	realEncoder := encoder{}
	handler := newNotificationHandler(realEncoder, mockService)
	router := chi.NewRouter()
	router.Get("/notifications", handler.list)

	testUser := &domain.User{HashedUUID: "user-123"}
	ctx := contextWithUser(testUser)

	expectedNotifications := []domain.Notification{{ID: 1, Name: "Test Notif", UserHashedUUID: testUser.HashedUUID}}
	mockService.On("Find", mock.AnythingOfType("*context.valueCtx"), domain.NotificationQueryParams{UserHashedUUID: &testUser.HashedUUID}).Return(expectedNotifications, len(expectedNotifications), nil)

	req := httptest.NewRequest("GET", "/notifications", nil).WithContext(ctx)
	rr := httptest.NewRecorder()
	router.ServeHTTP(rr, req)

	assert.Equal(t, http.StatusOK, rr.Code)
	var respNotifications []domain.Notification
	err := json.Unmarshal(rr.Body.Bytes(), &respNotifications)
	require.NoError(t, err)
	assert.Equal(t, expectedNotifications, respNotifications)
	mockService.AssertExpectations(t)

	t.Run("service error", func(t *testing.T) {
		mockService := new(MockNotificationService) // New mock for subtest
		handler := newNotificationHandler(realEncoder, mockService)
		router := chi.NewRouter()
		router.Get("/notifications", handler.list)

		mockService.On("Find", mock.AnythingOfType("*context.valueCtx"), domain.NotificationQueryParams{UserHashedUUID: &testUser.HashedUUID}).Return(nil, 0, errors.New("service find error"))
		
		req := httptest.NewRequest("GET", "/notifications", nil).WithContext(ctx)
		rr := httptest.NewRecorder()
		router.ServeHTTP(rr, req)

		assert.Equal(t, http.StatusNotFound, rr.Code) // As per handler logic for Find error
		mockService.AssertExpectations(t)
	})

	t.Run("no user in context", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/notifications", nil) // No user in context
		rr := httptest.NewRecorder()
		router.ServeHTTP(rr, req) // Original router with original mockService
		assert.Equal(t, http.StatusUnauthorized, rr.Code)
	})
}

func TestNotificationHandler_Store(t *testing.T) {
	mockService := new(MockNotificationService)
	realEncoder := encoder{}
	handler := newNotificationHandler(realEncoder, mockService)
	router := chi.NewRouter()
	router.Post("/notifications", handler.store)
	
	testUser := &domain.User{HashedUUID: "user-store-123"}
	ctx := contextWithUser(testUser)

	notificationInput := domain.Notification{Name: "New Discord", Type: "DISCORD"}
	storedNotification := domain.Notification{ID: 1, Name: "New Discord", Type: "DISCORD", UserHashedUUID: testUser.HashedUUID}

	mockService.On("Store", mock.AnythingOfType("*context.valueCtx"), mock.MatchedBy(func(n domain.Notification) bool {
		return n.Name == notificationInput.Name && n.UserHashedUUID == testUser.HashedUUID
	})).Return(&storedNotification, nil)

	bodyBytes, _ := json.Marshal(notificationInput)
	req := httptest.NewRequest("POST", "/notifications", bytes.NewBuffer(bodyBytes)).WithContext(ctx)
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()
	router.ServeHTTP(rr, req)

	assert.Equal(t, http.StatusCreated, rr.Code)
	var respNotification domain.Notification
	err := json.Unmarshal(rr.Body.Bytes(), &respNotification)
	require.NoError(t, err)
	assert.Equal(t, storedNotification, respNotification)
	mockService.AssertExpectations(t)
}

func TestNotificationHandler_Update(t *testing.T) {
	mockService := new(MockNotificationService)
	realEncoder := encoder{}
	handler := newNotificationHandler(realEncoder, mockService)
	router := chi.NewRouter()
	// Note: The route in notification.go is Put("/{notificationID}", h.update)
	// So the test request should target something like /notifications/1
	router.Put("/notifications/{notificationID}", handler.update)


	testUser := &domain.User{HashedUUID: "user-update-123"}
	ctx := contextWithUser(testUser)
	notificationID := 1

	notificationInput := domain.Notification{ID: notificationID, Name: "Updated Discord", Type: "DISCORD", UserHashedUUID: testUser.HashedUUID}
	// UserHashedUUID might be set by handler or service, ensure mock matches what service expects.
	// The handler currently does not explicitly set UserHashedUUID on update from context.
	// It relies on the service.Update to handle authorization if ID implies ownership.
	// For the mock, we match the input data.
	
	updatedNotification := domain.Notification{ID: notificationID, Name: "Updated Discord", Type: "DISCORD", UserHashedUUID: testUser.HashedUUID}

	mockService.On("Update", mock.AnythingOfType("*context.valueCtx"), notificationInput).Return(&updatedNotification, nil)

	bodyBytes, _ := json.Marshal(notificationInput)
	reqPath := "/notifications/" + strconv.Itoa(notificationID)
	req := httptest.NewRequest("PUT", reqPath, bytes.NewBuffer(bodyBytes)).WithContext(ctx)
	req.Header.Set("Content-Type", "application/json")
	
	// Need to wrap request with chi context for URLParam
	chiCtx := chi.NewRouteContext()
	chiCtx.URLParams.Add("notificationID", strconv.Itoa(notificationID))
	req = req.WithContext(context.WithValue(ctx, chi.RouteCtxKey, chiCtx))

	rr := httptest.NewRecorder()
	router.ServeHTTP(rr, req)

	assert.Equal(t, http.StatusOK, rr.Code)
	var respNotification domain.Notification
	err := json.Unmarshal(rr.Body.Bytes(), &respNotification)
	require.NoError(t, err)
	assert.Equal(t, updatedNotification, respNotification)
	mockService.AssertExpectations(t)
}

func TestNotificationHandler_Delete(t *testing.T) {
	mockService := new(MockNotificationService)
	realEncoder := encoder{}
	handler := newNotificationHandler(realEncoder, mockService)
	router := chi.NewRouter()
	router.Delete("/notifications/{notificationID}", handler.delete)

	testUser := &domain.User{HashedUUID: "user-delete-123"} // User context might be relevant for service-side auth
	ctx := contextWithUser(testUser)
	notificationID := 1

	mockService.On("Delete", mock.AnythingOfType("*context.valueCtx"), notificationID).Return(nil)

	reqPath := "/notifications/" + strconv.Itoa(notificationID)
	req := httptest.NewRequest("DELETE", reqPath, nil).WithContext(ctx)
	
	chiCtx := chi.NewRouteContext()
	chiCtx.URLParams.Add("notificationID", strconv.Itoa(notificationID))
	req = req.WithContext(context.WithValue(ctx, chi.RouteCtxKey, chiCtx))
	
	rr := httptest.NewRecorder()
	router.ServeHTTP(rr, req)

	assert.Equal(t, http.StatusNoContent, rr.Code)
	mockService.AssertExpectations(t)
}

func TestNotificationHandler_Test(t *testing.T) {
	mockService := new(MockNotificationService)
	realEncoder := encoder{}
	handler := newNotificationHandler(realEncoder, mockService)
	router := chi.NewRouter()
	router.Post("/notifications/test", handler.test)

	testUser := &domain.User{HashedUUID: "user-test-123"}
	ctx := contextWithUser(testUser)

	notificationInput := domain.Notification{Name: "Test Notif", Type: "DISCORD"}
	// The handler sets UserHashedUUID from context before calling service.Test
	expectedServiceInput := domain.Notification{Name: "Test Notif", Type: "DISCORD", UserHashedUUID: testUser.HashedUUID}


	mockService.On("Test", mock.AnythingOfType("*context.valueCtx"), expectedServiceInput).Return(nil)

	bodyBytes, _ := json.Marshal(notificationInput)
	req := httptest.NewRequest("POST", "/notifications/test", bytes.NewBuffer(bodyBytes)).WithContext(ctx)
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()
	router.ServeHTTP(rr, req)

	assert.Equal(t, http.StatusNoContent, rr.Code)
	mockService.AssertExpectations(t)

	t.Run("service test error", func(t *testing.T) {
		mockService := new(MockNotificationService) // New mock for subtest
		handler := newNotificationHandler(realEncoder, mockService)
		router := chi.NewRouter()
		router.Post("/notifications/test", handler.test)

		mockService.On("Test", mock.AnythingOfType("*context.valueCtx"), expectedServiceInput).Return(errors.New("test failed"))

		req := httptest.NewRequest("POST", "/notifications/test", bytes.NewBuffer(bodyBytes)).WithContext(ctx)
		req.Header.Set("Content-Type", "application/json")
		rr := httptest.NewRecorder()
		router.ServeHTTP(rr, req)
		
		assert.Equal(t, http.StatusInternalServerError, rr.Code)
		assert.Contains(t, rr.Body.String(), "Failed to test notification: test failed")
		mockService.AssertExpectations(t)
	})
}