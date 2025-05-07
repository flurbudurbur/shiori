package http

import (
	"encoding/json"
	"net/http"

	"github.com/flurbudurbur/Shiori/internal/config"
	"github.com/flurbudurbur/Shiori/internal/domain"
	"github.com/go-chi/chi/v5"
	"github.com/go-chi/render"
)

type configJson struct {
	Host            string `json:"host"`
	Port            int    `json:"port"`
	LogLevel        string `json:"log_level"`
	LogPath         string `json:"log_path"`
	LogMaxSize      int    `json:"log_max_size"`
	LogMaxBackups   int    `json:"log_max_backups"`
	BaseURL         string `json:"base_url"`
	CheckForUpdates bool   `json:"check_for_updates"`
	Version         string `json:"version"`
	Commit          string `json:"commit"`
	Date            string `json:"date"`
}

type configHandler struct {
	encoder encoder

	cfg    *config.AppConfig
	server Server
}

func newConfigHandler(encoder encoder, server Server, cfg *config.AppConfig) *configHandler {
	return &configHandler{
		encoder: encoder,
		cfg:     cfg,
		server:  server,
	}
}

func (h configHandler) Routes(r chi.Router) {
	r.Get("/", h.getConfig)
	r.Patch("/", h.updateConfig)
}

func (h configHandler) getConfig(w http.ResponseWriter, r *http.Request) {
	conf := configJson{
		Host:            h.cfg.Config.Server.Host,            // Access via nested Server struct
		Port:            h.cfg.Config.Server.Port,            // Access via nested Server struct
		LogLevel:        h.cfg.Config.Logging.Level,          // Access via nested Logging struct
		LogPath:         h.cfg.Config.Logging.Path,           // Access via nested Logging struct
		LogMaxSize:      h.cfg.Config.Logging.MaxFileSize,    // Access via nested Logging struct
		LogMaxBackups:   h.cfg.Config.Logging.MaxBackupCount, // Access via nested Logging struct
		BaseURL:         h.cfg.Config.Server.BaseURL,         // Access via nested Server struct
		CheckForUpdates: h.cfg.Config.CheckForUpdates,
		Version:         h.server.version,
		Commit:          h.server.commit,
		Date:            h.server.date,
	}

	render.JSON(w, r, conf)
}

func (h configHandler) updateConfig(w http.ResponseWriter, r *http.Request) {
	var data domain.ConfigUpdate

	if err := json.NewDecoder(r.Body).Decode(&data); err != nil {
		h.encoder.Error(w, err)
		return
	}

	if data.CheckForUpdates != nil {
		h.cfg.Config.CheckForUpdates = *data.CheckForUpdates
	}

	if data.LogLevel != nil {
		h.cfg.Config.Logging.Level = *data.LogLevel // Access via nested Logging struct
	}

	if data.LogPath != nil {
		h.cfg.Config.Logging.Path = *data.LogPath // Access via nested Logging struct
	}

	// NOTE: The UpdateConfig method was removed as it was incompatible with the nested TOML structure.
	// Changes made via this endpoint are currently only applied in-memory and are not persisted to config.toml.
	// if err := h.cfg.UpdateConfig(); err != nil {
	// 	render.Status(r, http.StatusInternalServerError)
	// 	render.JSON(w, r, errorResponse{
	// 		Message: err.Error(),
	// 		Status:  http.StatusInternalServerError,
	// 	})
	// 	return
	// }

	render.NoContent(w, r)
}
