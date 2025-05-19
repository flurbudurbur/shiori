package http

import (
	"bytes"
	// "context" // Removed unused import
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/flurbudurbur/Shiori/internal/config"
	"github.com/flurbudurbur/Shiori/internal/domain"
	"github.com/go-chi/chi/v5"
	"github.com/stretchr/testify/assert"
	// "github.com/stretchr/testify/mock" // Removed unused import
	"github.com/stretchr/testify/require"
)

// MockEncoder is not needed, we will use the real http.encoder
// MockServer is not needed, we will use a real http.Server with relevant fields set

func TestGetConfigHandler(t *testing.T) {
	realEncoder := encoder{} // Use real encoder
	mockAppConfig := &config.AppConfig{
		Config: &domain.Config{
			Server: domain.ServerConfig{
				Host:    "localhost",
				Port:    8080,
				BaseURL: "/shiori",
			},
			Logging: domain.LoggingConfig{
				Level:          "DEBUG",
				Path:           "/logs",
				MaxFileSize:    100,
				MaxBackupCount: 5,
			},
			CheckForUpdates: true,
		},
	}
	// Create a real http.Server instance and set the fields configHandler reads
	realHttpServer := Server{
		version: "1.0.0",
		commit:  "abcdef",
		date:    "2023-01-01",
		// Other fields can be zero/nil as configHandler doesn't use them
	}

	handler := newConfigHandler(realEncoder, realHttpServer, mockAppConfig)
	router := chi.NewRouter()
	handler.Routes(router)

	req := httptest.NewRequest("GET", "/", nil)
	rr := httptest.NewRecorder()

	// No need to mock encoder.Write as render.JSON is used directly by the handler.
	// If handler used encoder.Write, we would test its output.

	router.ServeHTTP(rr, req)

	assert.Equal(t, http.StatusOK, rr.Code)

	var respJson configJson
	err := json.Unmarshal(rr.Body.Bytes(), &respJson)
	require.NoError(t, err)

	assert.Equal(t, "localhost", respJson.Host)
	assert.Equal(t, 8080, respJson.Port)
	assert.Equal(t, "DEBUG", respJson.LogLevel)
	assert.Equal(t, "/logs", respJson.LogPath)
	assert.Equal(t, 100, respJson.LogMaxSize)
	assert.Equal(t, 5, respJson.LogMaxBackups)
	assert.Equal(t, "/shiori", respJson.BaseURL)
	assert.True(t, respJson.CheckForUpdates)
	assert.Equal(t, "1.0.0", respJson.Version)
	assert.Equal(t, "abcdef", respJson.Commit)
	assert.Equal(t, "2023-01-01", respJson.Date)
}

func TestUpdateConfigHandler(t *testing.T) {
	realEncoder := encoder{} // Use real encoder
	initialLogLevel := "INFO"
	initialLogPath := "/var/log"
	initialCheckUpdates := true

	mockAppConfig := &config.AppConfig{
		Config: &domain.Config{
			Logging: domain.LoggingConfig{
				Level: initialLogLevel,
				Path:  initialLogPath,
			},
			CheckForUpdates: initialCheckUpdates,
		},
	}
	// realHttpServer is not strictly needed by updateConfig's logic, but newConfigHandler requires it.
	realHttpServer := Server{} // Can be a minimal instance

	handler := newConfigHandler(realEncoder, realHttpServer, mockAppConfig)
	router := chi.NewRouter()
	handler.Routes(router)

	t.Run("Update specific fields", func(t *testing.T) {
		newLogLevel := "DEBUG"
		newLogPath := "/tmp/logs"
		newCheckUpdates := false

		updatePayload := domain.ConfigUpdate{
			LogLevel:        &newLogLevel,
			LogPath:         &newLogPath,
			CheckForUpdates: &newCheckUpdates,
		}
		body, _ := json.Marshal(updatePayload)
		req := httptest.NewRequest("PATCH", "/", bytes.NewBuffer(body))
		req.Header.Set("Content-Type", "application/json")
		rr := httptest.NewRecorder()

		router.ServeHTTP(rr, req)

		assert.Equal(t, http.StatusNoContent, rr.Code)
		assert.Equal(t, newLogLevel, mockAppConfig.Config.Logging.Level)
		assert.Equal(t, newLogPath, mockAppConfig.Config.Logging.Path)
		assert.Equal(t, newCheckUpdates, mockAppConfig.Config.CheckForUpdates)
	})

	t.Run("Update only one field", func(t *testing.T) {
		// Reset to known state for this sub-test if necessary, or use different AppConfig instance
		mockAppConfig.Config.Logging.Level = "INFO" // Reset
		newLogLevelOnly := "WARN"
		updatePayload := domain.ConfigUpdate{
			LogLevel: &newLogLevelOnly,
		}
		body, _ := json.Marshal(updatePayload)
		req := httptest.NewRequest("PATCH", "/", bytes.NewBuffer(body))
		req.Header.Set("Content-Type", "application/json")
		rr := httptest.NewRecorder()

		router.ServeHTTP(rr, req)

		assert.Equal(t, http.StatusNoContent, rr.Code)
		assert.Equal(t, newLogLevelOnly, mockAppConfig.Config.Logging.Level)
		// Other fields should remain unchanged from their previous state in mockAppConfig
		assert.Equal(t, "/tmp/logs", mockAppConfig.Config.Logging.Path) // From previous sub-test
		assert.False(t, mockAppConfig.Config.CheckForUpdates)          // From previous sub-test
	})

	t.Run("Invalid JSON body", func(t *testing.T) {
		req := httptest.NewRequest("PATCH", "/", bytes.NewBufferString("invalid json"))
		req.Header.Set("Content-Type", "application/json")
		rr := httptest.NewRecorder()

		// The realEncoder.Error will be called. We check its effect on rr.
		router.ServeHTTP(rr, req)

		assert.Equal(t, http.StatusInternalServerError, rr.Code)
		assert.Contains(t, rr.Body.String(), "error", "Error response should contain 'error'")
		// We don't need mockEncoder.AssertCalled anymore
	})
}