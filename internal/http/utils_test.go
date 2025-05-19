package http

import (
	"bytes"
	"compress/gzip"
	"encoding/json"
	"errors" // Added import for errors.Is
	"io"
	"net/http"
	"net/http/httptest"
	"strings" // Added import for strings.Contains
	"testing"

	"github.com/flurbudurbur/Shiori/internal/logger" // For logger.Mock()
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestHandleGetUUID(t *testing.T) {
	// Minimal server setup for logging
	testLogger := logger.Mock().With().Logger()
	serverInstance := &Server{
		log: testLogger,
		// Other fields can be nil as they are not used by handleGetUUID
	}

	handler := http.HandlerFunc(serverInstance.handleGetUUID)

	t.Run("POST request success", func(t *testing.T) {
		req := httptest.NewRequest("POST", "/utils/uuid", nil)
		rr := httptest.NewRecorder()
		handler.ServeHTTP(rr, req)

		assert.Equal(t, http.StatusOK, rr.Code)
		assert.Equal(t, "application/json", rr.Header().Get("Content-Type"))

		var resp map[string]string
		err := json.Unmarshal(rr.Body.Bytes(), &resp)
		require.NoError(t, err)
		assert.NotEmpty(t, resp["uuid"])
		_, err = uuid.Parse(resp["uuid"]) // Check if it's a valid UUID
		assert.NoError(t, err)
	})

	t.Run("GET request method not allowed", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/utils/uuid", nil)
		rr := httptest.NewRecorder()
		handler.ServeHTTP(rr, req)

		assert.Equal(t, http.StatusMethodNotAllowed, rr.Code)
	})
}

func TestUncompress(t *testing.T) {
	// Setup global Repo for the logger in uncompress's defer
	originalRepo := Repo // Save original global Repo if any
	testLogger := logger.Mock().With().Logger()
	Repo = &Server{log: testLogger}
	defer func() { Repo = originalRepo }() // Restore original Repo

	t.Run("valid gzipped data", func(t *testing.T) {
		originalData := []byte("hello, world - this is some test data for gzip")
		var b bytes.Buffer
		gz := gzip.NewWriter(&b)
		_, err := gz.Write(originalData)
		require.NoError(t, err)
		err = gz.Close()
		require.NoError(t, err)
		compressedData := b.Bytes()

		uncompressed, err := uncompress(compressedData)
		require.NoError(t, err)
		assert.Equal(t, originalData, uncompressed)
	})

	t.Run("invalid non-gzipped data", func(t *testing.T) {
		invalidData := []byte("this is not gzipped")
		_, err := uncompress(invalidData)
		assert.Error(t, err)
		assert.Equal(t, gzip.ErrHeader, err, "Expected gzip header error")
	})

	t.Run("empty data", func(t *testing.T) {
		_, err := uncompress([]byte{})
		assert.Error(t, err)
		// gzip.NewReader returns io.EOF for empty input
		assert.Equal(t, io.EOF, err, "Expected EOF for empty data")
	})

	t.Run("corrupt gzipped data", func(t *testing.T) {
		originalData := []byte("hello world")
		var b bytes.Buffer
		gz := gzip.NewWriter(&b)
		_, err := gz.Write(originalData)
		require.NoError(t, err)
		// Deliberately DO NOT Close the gzip writer to make it incomplete/corrupt
		// or just take a valid header and append garbage
		
		var validHeaderBuf bytes.Buffer
		gzValid := gzip.NewWriter(&validHeaderBuf)
		_, err = gzValid.Write([]byte("h")) // Write something small
		require.NoError(t, err)
		err = gzValid.Close()
		require.NoError(t, err)
		
		// Take header from valid, append garbage
		corruptData := append(validHeaderBuf.Bytes()[:10], []byte("not valid gzip stream data beyond header")...)


		_, err = uncompress(corruptData)
		assert.Error(t, err)
		// The error could be io.ErrUnexpectedEOF or another stream error
		// For example, "gzip: invalid header" if header itself is corrupt,
		// or "unexpected EOF" if stream ends prematurely.
		// "gzip: invalid checksum" is also possible.
		// Let's check for a non-nil error.
		// A more specific error might be "unexpected EOF" or similar depending on corruption.
		// If the header is valid but data is short, it's often io.ErrUnexpectedEOF.
		// If checksum fails, it's gzip.ErrChecksum.
		// If the stream is just truncated, io.ReadAll might return io.ErrUnexpectedEOF.
		assert.True(t, errors.Is(err, io.ErrUnexpectedEOF) || strings.Contains(err.Error(), "checksum"), "Expected a stream error like EOF or checksum")
	})
}