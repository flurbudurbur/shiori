package http

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/flurbudurbur/Shiori/pkg/version" // For version.Release
	"github.com/go-chi/chi/v5"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

// MockUpdateService mocks the updateService interface
type MockUpdateService struct {
	mock.Mock
}

func (m *MockUpdateService) CheckUpdates(ctx context.Context) {
	m.Called(ctx)
}

func (m *MockUpdateService) GetLatestRelease(ctx context.Context) *version.Release {
	args := m.Called(ctx)
	if args.Get(0) == nil {
		return nil
	}
	return args.Get(0).(*version.Release)
}

func TestUpdateHandler_GetLatest(t *testing.T) {
	mockService := new(MockUpdateService)
	// Encoder is not directly used by getLatest for JSON response (uses chi/render)
	handler := newUpdateHandler(encoder{}, mockService)
	router := chi.NewRouter()
	router.Get("/updates/latest", handler.getLatest)

	t.Run("latest release available", func(t *testing.T) {
		publishedAtTime, _ := time.Parse(time.RFC3339, "2023-01-01T10:00:00Z")
		releaseNote := "New features and bug fixes."
		expectedRelease := &version.Release{
			TagName:     "v1.2.3",
			PublishedAt: publishedAtTime,
			URL:         "http://example.com/release/v1.2.3",
			Body:        &releaseNote,
		}
		mockService.On("GetLatestRelease", mock.Anything).Return(expectedRelease).Once()

		req := httptest.NewRequest("GET", "/updates/latest", nil)
		rr := httptest.NewRecorder()
		router.ServeHTTP(rr, req)

		assert.Equal(t, http.StatusOK, rr.Code)
		var respRelease version.Release
		err := json.Unmarshal(rr.Body.Bytes(), &respRelease)
		require.NoError(t, err)
		assert.Equal(t, *expectedRelease, respRelease)
		mockService.AssertExpectations(t)
	})

	t.Run("no latest release available", func(t *testing.T) {
		mockService.On("GetLatestRelease", mock.Anything).Return(nil).Once()

		req := httptest.NewRequest("GET", "/updates/latest", nil)
		rr := httptest.NewRecorder()
		router.ServeHTTP(rr, req)

		assert.Equal(t, http.StatusNoContent, rr.Code)
		mockService.AssertExpectations(t)
	})
}

func TestUpdateHandler_CheckUpdates(t *testing.T) {
	mockService := new(MockUpdateService)
	// Encoder is not directly used by checkUpdates (uses chi/render for NoContent)
	handler := newUpdateHandler(encoder{}, mockService)
	router := chi.NewRouter()
	router.Get("/updates/check", handler.checkUpdates)

	mockService.On("CheckUpdates", mock.Anything).Return().Once()

	req := httptest.NewRequest("GET", "/updates/check", nil)
	rr := httptest.NewRecorder()
	router.ServeHTTP(rr, req)

	assert.Equal(t, http.StatusNoContent, rr.Code)
	mockService.AssertExpectations(t)
}