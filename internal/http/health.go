package http

import (
	"net/http"

	// "github.com/flurbudurbur/Shiori/internal/database" // No longer directly needed by healthHandler struct
	"github.com/go-chi/chi/v5"
)

// DBPinger defines an interface for types that can be pinged.
type DBPinger interface {
	Ping() error
}

type healthHandler struct {
	encoder encoder
	dbPinger DBPinger // Changed from *database.DB to DBPinger interface
}

func newHealthHandler(encoder encoder, dbPinger DBPinger) *healthHandler { // Changed parameter type
	return &healthHandler{
		encoder: encoder,
		dbPinger: dbPinger, // Use the interface
	}
}

func (h healthHandler) Routes(r chi.Router) {
	r.Get("/liveness", h.handleLiveness)
	r.Get("/readiness", h.handleReadiness)
}

func (h healthHandler) handleLiveness(w http.ResponseWriter, _ *http.Request) {
	writeHealthy(w)
}

func (h healthHandler) handleReadiness(w http.ResponseWriter, _ *http.Request) {
	if err := h.dbPinger.Ping(); err != nil { // Use the interface method
		writeUnhealthy(w)
		return
	}

	writeHealthy(w)
}

func writeHealthy(w http.ResponseWriter) {
	w.Header().Set("Content-Type", "text/plain")
	w.WriteHeader(http.StatusOK)
	w.Write([]byte("OK"))
}

func writeUnhealthy(w http.ResponseWriter) {
	w.Header().Set("Content-Type", "text/plain")
	w.WriteHeader(http.StatusInternalServerError)
	w.Write([]byte("Unhealthy. Database unreachable"))
}
