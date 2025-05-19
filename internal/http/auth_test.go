package http

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/flurbudurbur/Shiori/internal/domain"
	"github.com/flurbudurbur/Shiori/internal/logger"
	"github.com/go-chi/chi/v5"
	"github.com/gorilla/sessions"
	"github.com/stretchr/testify/assert"
)

// MockAuthService is a mock implementation of the authService interface
type MockAuthService struct {
	userCount                int
	userCountError           error
	generatedBookmark        string
	generateBookmarkError    error
	authenticatedUser        *domain.User
	authenticateUserError    error
}

func (m *MockAuthService) GetUserCount(ctx context.Context) (int, error) {
	return m.userCount, m.userCountError
}

func (m *MockAuthService) GenerateUserBookmark(ctx context.Context) (string, error) {
	return m.generatedBookmark, m.generateBookmarkError
}

func (m *MockAuthService) AuthenticateUser(ctx context.Context, hashedUUID string) (*domain.User, error) {
	return m.authenticatedUser, m.authenticateUserError
}

// TestAuthHandler_RegistrationStatus tests the registrationStatus endpoint
func TestAuthHandler_RegistrationStatus(t *testing.T) {
	// Setup
	r := chi.NewRouter()
	logMock := logger.Mock()
	log := logMock.With().Str("module", "http").Logger()

	// Create an actual encoder instance
	actualEncoder := encoder{}
	
	// Create a mock config
	config := &domain.Config{
		Server: domain.ServerConfig{
			BaseURL: "/",
		},
	}
	
	// Create a mock cookie store
	cookieStore := sessions.NewCookieStore([]byte("test-secret"))
	
	// Create a mock auth service
	mockAuthService := &MockAuthService{
		userCount: 5,
	}
	
	// Create the handler
	handler := &authHandler{
		log:         log,
		encoder:     actualEncoder,
		config:      config,
		service:     mockAuthService,
		cookieStore: cookieStore,
	}

	// Register the route
	r.Get("/api/v1/auth/register/status", handler.registrationStatus)

	// Create a request
	req := httptest.NewRequest("GET", "/api/v1/auth/register/status", nil)
	w := httptest.NewRecorder()

	// Serve the request
	r.ServeHTTP(w, req)

	// Assert
	assert.Equal(t, http.StatusNoContent, w.Code)
}

// TestAuthHandler_Register tests the register endpoint
func TestAuthHandler_Register(t *testing.T) {
	// Setup
	r := chi.NewRouter()
	logMock := logger.Mock()
	log := logMock.With().Str("module", "http").Logger()

	// Create an actual encoder instance
	actualEncoder := encoder{}
	
	// Create a mock config
	config := &domain.Config{
		Server: domain.ServerConfig{
			BaseURL: "/",
		},
	}
	
	// Create a mock cookie store
	cookieStore := sessions.NewCookieStore([]byte("test-secret"))
	
	// Test successful case
	mockAuthService := &MockAuthService{
		generatedBookmark: "test-bookmark",
		authenticatedUser: &domain.User{
			HashedUUID: "test-bookmark",
			Scopes:     `{"read": true, "write": false}`,
		},
	}
	
	// Create the handler
	handler := &authHandler{
		log:         log,
		encoder:     actualEncoder,
		config:      config,
		service:     mockAuthService,
		cookieStore: cookieStore,
	}

	// Register the route
	r.Post("/api/v1/auth/register", handler.register)

	// Create a request
	req := httptest.NewRequest("POST", "/api/v1/auth/register", nil)
	w := httptest.NewRecorder()

	// Serve the request
	r.ServeHTTP(w, req)

	// Assert
	assert.Equal(t, http.StatusOK, w.Code)
	
	var response generateBookmarkResponse
	err := json.Unmarshal(w.Body.Bytes(), &response)
	assert.NoError(t, err)
	assert.Equal(t, "test-bookmark", response.UUID)
	assert.NotNil(t, response.User)
	assert.Equal(t, "test-bookmark", response.Token)

	// Test error case
	mockAuthService = &MockAuthService{
		generateBookmarkError: errors.New("generation error"),
	}
	
	// Create the handler
	handler = &authHandler{
		log:         log,
		encoder:     actualEncoder,
		config:      config,
		service:     mockAuthService,
		cookieStore: cookieStore,
	}
	
	r = chi.NewRouter()
	r.Post("/api/v1/auth/register", handler.register)

	req = httptest.NewRequest("POST", "/api/v1/auth/register", nil)
	w = httptest.NewRecorder()

	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusInternalServerError, w.Code)
}

// TestAuthHandler_Login tests the login endpoint
func TestAuthHandler_Login(t *testing.T) {
	// Setup
	r := chi.NewRouter()
	logMock := logger.Mock()
	log := logMock.With().Str("module", "http").Logger()

	// Create an actual encoder instance
	actualEncoder := encoder{}
	
	// Create a mock config
	config := &domain.Config{
		Server: domain.ServerConfig{
			BaseURL: "/",
		},
	}
	
	// Create a mock cookie store
	cookieStore := sessions.NewCookieStore([]byte("test-secret"))
	
	// Test successful case
	mockAuthService := &MockAuthService{
		authenticatedUser: &domain.User{
			HashedUUID: "test-uuid",
			Scopes:     `{"read": true, "write": false}`,
		},
	}
	
	// Create the handler
	handler := &authHandler{
		log:         log,
		encoder:     actualEncoder,
		config:      config,
		service:     mockAuthService,
		cookieStore: cookieStore,
	}

	// Register the route
	r.Post("/api/v1/auth/login", handler.login)

	// Create a request with login data
	loginData := `{"uuid": "test-uuid"}`
	req := httptest.NewRequest("POST", "/api/v1/auth/login", bytes.NewBufferString(loginData))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	// Serve the request
	r.ServeHTTP(w, req)

	// Assert
	assert.Equal(t, http.StatusOK, w.Code)
	
	var response loginResponse
	err := json.Unmarshal(w.Body.Bytes(), &response)
	assert.NoError(t, err)
	assert.NotNil(t, response.User)
	assert.Equal(t, "test-uuid", response.Token)

	// Test authentication error case
	mockAuthService = &MockAuthService{
		authenticateUserError: errors.New("authentication error"),
	}
	
	// Create the handler
	handler = &authHandler{
		log:         log,
		encoder:     actualEncoder,
		config:      config,
		service:     mockAuthService,
		cookieStore: cookieStore,
	}
	
	r = chi.NewRouter()
	r.Post("/api/v1/auth/login", handler.login)

	req = httptest.NewRequest("POST", "/api/v1/auth/login", bytes.NewBufferString(loginData))
	req.Header.Set("Content-Type", "application/json")
	w = httptest.NewRecorder()

	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusInternalServerError, w.Code)
}