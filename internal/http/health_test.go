package http

import (
	"net/http"
	"net/http/httptest"
	"errors" // For creating test errors
	"testing"

	// "github.com/flurbudurbur/Shiori/internal/database" // No longer needed directly for mock
	"github.com/go-chi/chi/v5"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock" // For mock object
)

// MockDBPinger is a mock for DBPinger interface
type MockDBPinger struct {
	mock.Mock
}

func (m *MockDBPinger) Ping() error {
	args := m.Called()
	return args.Error(0)
}

func TestHealthHandler_Liveness(t *testing.T) {
	realEncoder := encoder{}
	// dbPinger can be nil for liveness as it's not used by handleLiveness
	handler := newHealthHandler(realEncoder, nil)
	router := chi.NewRouter()
	handler.Routes(router) // This will register /liveness and /readiness

	req := httptest.NewRequest("GET", "/liveness", nil)
	rr := httptest.NewRecorder()
	router.ServeHTTP(rr, req)

	assert.Equal(t, http.StatusOK, rr.Code)
	assert.Equal(t, "text/plain", rr.Header().Get("Content-Type"))
	assert.Equal(t, "OK", rr.Body.String())
}

func TestHealthHandler_Readiness_Healthy(t *testing.T) {
	realEncoder := encoder{}
	mockDbPinger := new(MockDBPinger)

	mockDbPinger.On("Ping").Return(nil) // Simulate successful ping

	handler := newHealthHandler(realEncoder, mockDbPinger)
	router := chi.NewRouter()
	handler.Routes(router)

	req := httptest.NewRequest("GET", "/readiness", nil)
	rr := httptest.NewRecorder()
	router.ServeHTTP(rr, req)

	assert.Equal(t, http.StatusOK, rr.Code)
	assert.Equal(t, "text/plain", rr.Header().Get("Content-Type"))
	assert.Equal(t, "OK", rr.Body.String())

	mockDbPinger.AssertExpectations(t)
}

func TestHealthHandler_Readiness_Unhealthy(t *testing.T) {
	realEncoder := encoder{}
	mockDbPinger := new(MockDBPinger)

	mockDbPinger.On("Ping").Return(errors.New("db ping failed")) // Simulate failed ping

	handler := newHealthHandler(realEncoder, mockDbPinger)
	router := chi.NewRouter()
	handler.Routes(router)

	req := httptest.NewRequest("GET", "/readiness", nil)
	rr := httptest.NewRecorder()
	router.ServeHTTP(rr, req)

	assert.Equal(t, http.StatusInternalServerError, rr.Code)
	assert.Equal(t, "text/plain", rr.Header().Get("Content-Type"))
	assert.Equal(t, "Unhealthy. Database unreachable", rr.Body.String())

	mockDbPinger.AssertExpectations(t)
}