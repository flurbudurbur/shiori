package http

import (
	"context"
	"io"
	"log"
	"net/http"

	"github.com/flurbudurbur/Shiori/internal/domain"
	"github.com/flurbudurbur/Shiori/internal/sync"
	"github.com/go-chi/chi/v5"
)

type syncService = sync.Service

// profileUUIDManager defines the interface for promoting profile UUIDs
type profileUUIDManager interface {
	PromoteProfileUUID(ctx context.Context, userID string, profileUUID string) error
	GetOrGenerateProfileUUID(ctx context.Context, sessionID string) (string, error)
}

type syncHandler struct {
	encoder     encoder
	syncService syncService
	uuidManager profileUUIDManager
}

func newSyncHandler(encoder encoder, syncService syncService, uuidManager profileUUIDManager) *syncHandler {
	return &syncHandler{
		encoder:     encoder,
		syncService: syncService,
		uuidManager: uuidManager,
	}
}

func (h syncHandler) Routes(r chi.Router) {
	r.Get("/content", h.getContent)
	r.Put("/content", h.putContent)
}

func (h syncHandler) getContent(w http.ResponseWriter, r *http.Request) {
	user, ok := r.Context().Value("user").(*domain.User)
	if !ok || user == nil {
		http.Error(w, "Unauthorized: User not found in context", http.StatusUnauthorized)
		return
	}
	userHashedUUID := user.HashedUUID
	etag := r.Header.Get("If-None-Match")

	if etag != "" {
		etagInDb, err := h.syncService.GetSyncDataETag(r.Context(), userHashedUUID)
		if err != nil {
			log.Println(err)
			h.encoder.StatusInternalError(w)
			return
		}

		if etagInDb != nil && etag == *etagInDb {
			// nothing changed after last request
			// see: https://developer.mozilla.org/en-US/docs/Web/HTTP/Headers/If-None-Match
			w.WriteHeader(http.StatusNotModified)
			return
		}
	}

	syncData, syncDataETag, err := h.syncService.GetSyncDataAndETag(r.Context(), userHashedUUID)

	if err != nil {
		h.encoder.StatusInternalError(w)
		return
	}

	if syncData == nil {
		w.WriteHeader(http.StatusNotFound)
		return
	}

	if syncDataETag != nil {
		w.Header().Set("ETag", *syncDataETag)
	}

	w.Header().Set("Content-Type", "application/octet-stream")
	w.Write(syncData)
	w.WriteHeader(http.StatusOK)
}

func (h syncHandler) putContent(w http.ResponseWriter, r *http.Request) {
	user, ok := r.Context().Value("user").(*domain.User)
	if !ok || user == nil {
		http.Error(w, "Unauthorized: User not found in context", http.StatusUnauthorized)
		return
	}
	userHashedUUID := user.HashedUUID
	etag := r.Header.Get("If-Match")

	// Read data from request body
	requestData, err := io.ReadAll(r.Body)
	if err != nil {
		h.encoder.StatusResponse(r.Context(), w, err.Error(), http.StatusBadRequest)
		return
	}

	var newEtag *string
	if etag != "" {
		newEtag, err = h.syncService.SetSyncDataIfMatch(r.Context(), userHashedUUID, etag, requestData)
	} else {
		newEtag, err = h.syncService.SetSyncData(r.Context(), userHashedUUID, requestData)
	}

	// This is a "data sync" event - promote the profile UUID to the persistent database
	// First, get the profile UUID from the session (or generate one if it doesn't exist)
	profileUUID, uuidErr := h.uuidManager.GetOrGenerateProfileUUID(r.Context(), userHashedUUID)
	if uuidErr != nil {
		// Log the error but don't fail the sync operation
		log.Printf("Failed to get profile UUID for promotion: %v", uuidErr)
	} else {
		// Promote the UUID to the persistent database
		if promoteErr := h.uuidManager.PromoteProfileUUID(r.Context(), userHashedUUID, profileUUID); promoteErr != nil {
			// Log the error but don't fail the sync operation
			log.Printf("Failed to promote profile UUID to persistent database: %v", promoteErr)
		}
	}
	if err != nil {
		h.encoder.StatusInternalError(w)
		// It's important to return here if an error occurs, otherwise, it will proceed to write headers.
		return
	}

	if newEtag == nil {
		// syncdata was changed from other clients
		// see: https://developer.mozilla.org/en-US/docs/Web/HTTP/Headers/If-Match
		w.WriteHeader(http.StatusPreconditionFailed)
	} else {
		w.Header().Set("ETag", *newEtag)
		w.WriteHeader(http.StatusOK)
	}
}
