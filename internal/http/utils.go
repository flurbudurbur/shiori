package http

import (
	"bytes"
	"compress/gzip"
	"encoding/json"
	"io"
	"net/http"

	"github.com/google/uuid"
)

var Repo *Server

// handleGetUUID generates a new UUID and returns it as JSON.
func (s *Server) handleGetUUID(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		s.log.Error().Msgf("Method not allowed: %s", r.Method)
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	newUUID := uuid.New().String()
	response := map[string]string{"uuid": newUUID}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	if err := json.NewEncoder(w).Encode(response); err != nil {
		s.log.Error().Err(err).Msg("Failed to encode UUID response")
		http.Error(w, "Failed to encode response", http.StatusInternalServerError)
	}
}

func uncompress(data []byte) ([]byte, error) {
	reader, err := gzip.NewReader(bytes.NewBuffer(data))
	if err != nil {
		return nil, err
	}
	defer func(reader *gzip.Reader) {
		err := reader.Close()
		if err != nil {
			Repo.log.Error().Err(err).Msg("failed to close gzip reader")
		}
	}(reader)

	uncompressedData, err := io.ReadAll(reader)
	if err != nil {
		return nil, err
	}

	return uncompressedData, nil
}
