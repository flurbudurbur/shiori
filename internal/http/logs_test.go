package http

import (
	"encoding/json"
	// "fmt" // Removed unused import
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/flurbudurbur/Shiori/internal/config"
	"github.com/flurbudurbur/Shiori/internal/domain"
	"github.com/go-chi/chi/v5"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func setupTestLogDir(t *testing.T) (logDir string, cleanup func()) {
	t.Helper()
	tmpDir, err := os.MkdirTemp("", "testlogs_*")
	require.NoError(t, err)
	return tmpDir, func() { os.RemoveAll(tmpDir) }
}

func createTestLogFile(t *testing.T, dir, name string, content string, modTime time.Time) string {
	t.Helper()
	filePath := filepath.Join(dir, name)
	err := os.WriteFile(filePath, []byte(content), 0644)
	require.NoError(t, err)
	if !modTime.IsZero() {
		err = os.Chtimes(filePath, modTime, modTime)
		require.NoError(t, err)
	}
	return filePath
}

func TestLogsHandler_Files(t *testing.T) {
	logDir, cleanup := setupTestLogDir(t)
	defer cleanup()

	// Create some test files
	mt := time.Now().Add(-time.Hour).Truncate(time.Second)
	_ = createTestLogFile(t, logDir, "app-2023-01-01.log", "log content 1", mt)
	_ = createTestLogFile(t, logDir, "debug.log", "log content 2", mt.Add(time.Minute))
	_ = createTestLogFile(t, logDir, "other.txt", "not a log file", mt)

	appCfg := &config.AppConfig{Config: &domain.Config{Logging: domain.LoggingConfig{Path: logDir}}}
	handler := newLogsHandler(appCfg)
	router := chi.NewRouter()
	router.Get("/files", handler.files)

	req := httptest.NewRequest("GET", "/files", nil)
	rr := httptest.NewRecorder()
	router.ServeHTTP(rr, req)

	assert.Equal(t, http.StatusOK, rr.Code)
	var resp LogfilesResponse
	err := json.Unmarshal(rr.Body.Bytes(), &resp)
	require.NoError(t, err)
	assert.Equal(t, 2, resp.Count)
	assert.Len(t, resp.Files, 2)

	// Check names (order might vary)
	names := []string{resp.Files[0].Name, resp.Files[1].Name}
	assert.Contains(t, names, "app-2023-01-01.log")
	assert.Contains(t, names, "debug.log")

	for _, f := range resp.Files {
		if f.Name == "app-2023-01-01.log" {
			assert.Equal(t, int64(len("log content 1")), f.SizeBytes)
			assert.Equal(t, mt, f.UpdatedAt)
		}
	}

	t.Run("log path not set", func(t *testing.T) {
		appCfgNoPath := &config.AppConfig{Config: &domain.Config{Logging: domain.LoggingConfig{Path: ""}}}
		handlerNoPath := newLogsHandler(appCfgNoPath)
		routerNoPath := chi.NewRouter()
		routerNoPath.Get("/files", handlerNoPath.files)

		reqNP := httptest.NewRequest("GET", "/files", nil)
		rrNP := httptest.NewRecorder()
		routerNoPath.ServeHTTP(rrNP, reqNP)

		assert.Equal(t, http.StatusOK, rrNP.Code)
		var respNP LogfilesResponse
		errNP := json.Unmarshal(rrNP.Body.Bytes(), &respNP)
		require.NoError(t, errNP)
		assert.Equal(t, 0, respNP.Count)
		assert.Empty(t, respNP.Files)
	})

	t.Run("log dir does not exist", func(t *testing.T) {
		appCfgBadPath := &config.AppConfig{Config: &domain.Config{Logging: domain.LoggingConfig{Path: "/non/existent/path"}}}
		handlerBadPath := newLogsHandler(appCfgBadPath)
		routerBadPath := chi.NewRouter()
		routerBadPath.Get("/files", handlerBadPath.files)

		reqBP := httptest.NewRequest("GET", "/files", nil)
		rrBP := httptest.NewRecorder()
		routerBadPath.ServeHTTP(rrBP, reqBP)
		
		assert.Equal(t, http.StatusOK, rrBP.Code) // Handler returns empty list, not an error status
		var respBP LogfilesResponse
		errBP := json.Unmarshal(rrBP.Body.Bytes(), &respBP)
		require.NoError(t, errBP)
		assert.Equal(t, 0, respBP.Count)
	})
}

func TestSanitizeLogFile(t *testing.T) {
	logDir, cleanup := setupTestLogDir(t)
	defer cleanup()

	t.Run("file does not exist", func(t *testing.T) {
		_, err := SanitizeLogFile(filepath.Join(logDir, "nonexistent.log"))
		assert.Error(t, err)
	})

	t.Run("sanitize content", func(t *testing.T) {
		originalContent := "info: apikey=secret123 and some other data passkey=anotherSecret"
		logFilePath := createTestLogFile(t, logDir, "sensitive.log", originalContent, time.Now())

		sanitizedFilePath, err := SanitizeLogFile(logFilePath)
		require.NoError(t, err)
		defer os.Remove(sanitizedFilePath)

		sanitizedContent, err := os.ReadFile(sanitizedFilePath)
		require.NoError(t, err)

		expectedSanitized := "info: apikey=REDACTED and some other data passkey=REDACTED"
		assert.Equal(t, expectedSanitized, string(sanitizedContent))
	})

	t.Run("no sensitive content", func(t *testing.T) {
		originalContent := "info: just regular log data"
		logFilePath := createTestLogFile(t, logDir, "clean.log", originalContent, time.Now())

		sanitizedFilePath, err := SanitizeLogFile(logFilePath)
		require.NoError(t, err)
		defer os.Remove(sanitizedFilePath)

		sanitizedContent, err := os.ReadFile(sanitizedFilePath)
		require.NoError(t, err)
		assert.Equal(t, originalContent, string(sanitizedContent))
	})
}

func TestLogsHandler_DownloadFile(t *testing.T) {
	logDir, cleanup := setupTestLogDir(t)
	defer cleanup()

	logContent := "apikey=secretvalue log data"
	_ = createTestLogFile(t, logDir, "testdownload.log", logContent, time.Now())

	appCfg := &config.AppConfig{Config: &domain.Config{Logging: domain.LoggingConfig{Path: logDir}}}
	handler := newLogsHandler(appCfg)
	// router := chi.NewRouter() // Removed unused variable
	// Need to use a router that can capture URL params correctly for chi
	r := chi.NewRouter()
	r.Route("/logs", func(router chi.Router) { // Match the structure if Routes adds a group
		handler.Routes(router) // This will register /files and /files/{logFile} under /logs
	})


	req := httptest.NewRequest("GET", "/logs/files/testdownload.log", nil)
	rr := httptest.NewRecorder()
	r.ServeHTTP(rr, req)

	assert.Equal(t, http.StatusOK, rr.Code)
	assert.Equal(t, "application/octet-stream", rr.Header().Get("Content-Type"))
	assert.Equal(t, `attachment; filename="testdownload.log"`, rr.Header().Get("Content-Disposition"))
	
	expectedSanitizedBody := "apikey=REDACTED log data"
	assert.Equal(t, expectedSanitizedBody, rr.Body.String())


	t.Run("log path not set", func(t *testing.T) {
		appCfgNoPath := &config.AppConfig{Config: &domain.Config{Logging: domain.LoggingConfig{Path: ""}}}
		handlerNoPath := newLogsHandler(appCfgNoPath)
		rNP := chi.NewRouter()
		rNP.Route("/logs", func(router chi.Router) { handlerNoPath.Routes(router) })
		
		reqNP := httptest.NewRequest("GET", "/logs/files/any.log", nil)
		rrNP := httptest.NewRecorder()
		rNP.ServeHTTP(rrNP, reqNP)
		assert.Equal(t, http.StatusNotFound, rrNP.Code)
	})

	t.Run("log file not found", func(t *testing.T) {
		reqNF := httptest.NewRequest("GET", "/logs/files/nonexistent.log", nil)
		rrNF := httptest.NewRecorder()
		r.ServeHTTP(rrNF, reqNF) // Use the original router 'r' with the valid logDir
		assert.Equal(t, http.StatusInternalServerError, rrNF.Code) // SanitizeLogFile will error
		assert.Contains(t, rrNF.Body.String(), "no such file or directory")
	})
	
	t.Run("invalid log file name", func(t *testing.T) {
		reqInvalid := httptest.NewRequest("GET", "/logs/files/invalidtxt", nil)
		rrInvalid := httptest.NewRecorder()
		r.ServeHTTP(rrInvalid, reqInvalid)
		assert.Equal(t, http.StatusBadRequest, rrInvalid.Code)
		assert.Contains(t, strings.ToLower(rrInvalid.Body.String()), "invalid file")
	})
}