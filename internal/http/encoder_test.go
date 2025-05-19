package http

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestEncoder_StatusResponse(t *testing.T) {
	enc := encoder{}
	ctx := context.Background()

	t.Run("with response body", func(t *testing.T) {
		rr := httptest.NewRecorder()
		responseBody := map[string]string{"message": "success"}
		enc.StatusResponse(ctx, rr, responseBody, http.StatusOK)

		assert.Equal(t, http.StatusOK, rr.Code)
		assert.Equal(t, "application/json; charset=utf-8", rr.Header().Get("Content-Type"))

		var actualBody map[string]string
		err := json.Unmarshal(rr.Body.Bytes(), &actualBody)
		require.NoError(t, err)
		assert.Equal(t, responseBody, actualBody)
	})

	t.Run("nil response body", func(t *testing.T) {
		rr := httptest.NewRecorder()
		enc.StatusResponse(ctx, rr, nil, http.StatusAccepted)
		assert.Equal(t, http.StatusAccepted, rr.Code)
		assert.Empty(t, rr.Body.String())
	})
}

func TestEncoder_StatusCreated(t *testing.T) {
	enc := encoder{}
	rr := httptest.NewRecorder()
	enc.StatusCreated(rr)
	assert.Equal(t, http.StatusCreated, rr.Code)
	assert.Empty(t, rr.Body.String())
}

func TestEncoder_StatusCreatedData(t *testing.T) {
	enc := encoder{}
	rr := httptest.NewRecorder()
	data := map[string]string{"id": "123"}
	enc.StatusCreatedData(rr, data)

	assert.Equal(t, http.StatusCreated, rr.Code)
	assert.Equal(t, "application/json; charset=utf-8", rr.Header().Get("Content-Type"))
	var actualBody map[string]string
	err := json.Unmarshal(rr.Body.Bytes(), &actualBody)
	require.NoError(t, err)
	assert.Equal(t, data, actualBody)
}

func TestEncoder_NoContent(t *testing.T) {
	enc := encoder{}
	rr := httptest.NewRecorder()
	enc.NoContent(rr)
	assert.Equal(t, http.StatusNoContent, rr.Code)
	assert.Empty(t, rr.Body.String())
}

func TestEncoder_StatusNotFound(t *testing.T) {
	enc := encoder{}
	ctx := context.Background()
	rr := httptest.NewRecorder()
	enc.StatusNotFound(ctx, rr)
	assert.Equal(t, http.StatusNotFound, rr.Code)
	assert.Empty(t, rr.Body.String())
}

func TestEncoder_StatusInternalError(t *testing.T) {
	enc := encoder{}
	rr := httptest.NewRecorder()
	enc.StatusInternalError(rr)
	assert.Equal(t, http.StatusInternalServerError, rr.Code)
	assert.Empty(t, rr.Body.String())
}

func TestEncoder_Error(t *testing.T) {
	enc := encoder{}
	rr := httptest.NewRecorder()
	testError := errors.New("this is a test error")
	enc.Error(rr, testError)

	assert.Equal(t, http.StatusInternalServerError, rr.Code)
	assert.Equal(t, "application/json; charset=utf-8", rr.Header().Get("Content-Type"))

	var actualBody errorResponse
	err := json.Unmarshal(rr.Body.Bytes(), &actualBody)
	require.NoError(t, err)
	assert.Equal(t, testError.Error(), actualBody.Message)
	// Status field in errorResponse is omitempty and not set by encoder.Error
	assert.Equal(t, 0, actualBody.Status)
}