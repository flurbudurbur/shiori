package http

import (
	"fmt"
	"io/fs"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/dustin/go-humanize"
	"github.com/flurbudurbur/Shiori/internal/config"
	"github.com/go-chi/chi/v5"
	"github.com/go-chi/render"
)

type logsHandler struct {
	cfg *config.AppConfig
}

func newLogsHandler(cfg *config.AppConfig) *logsHandler {
	return &logsHandler{cfg: cfg}
}

func (h logsHandler) Routes(r chi.Router) {
	r.Get("/files", h.files)
	r.Get("/files/{logFile}", h.downloadFile)
}

func (h logsHandler) files(w http.ResponseWriter, r *http.Request) {
	response := LogfilesResponse{
		Files: []logFile{},
		Count: 0,
	}

	if h.cfg.Config.Logging.Path == "" { // Access via nested Logging struct
		render.JSON(w, r, response)
		return
	}

	logsDir := h.cfg.Config.Logging.Path // Use the directory path directly

	// check if dir exists before walkDir
	if _, err := os.Stat(logsDir); os.IsNotExist(err) {
		render.JSON(w, r, response)
		return
	}

	var walk = func(path string, d fs.DirEntry, err error) error {
		if d.IsDir() {
			return nil
		}

		if filepath.Ext(path) == ".log" {
			i, err := d.Info()
			if err != nil {
				return err
			}

			response.Files = append(response.Files, logFile{
				Name:      d.Name(),
				SizeBytes: i.Size(),
				Size:      humanize.Bytes(uint64(i.Size())),
				UpdatedAt: i.ModTime(),
			})
		}

		return nil
	}

	if err := filepath.WalkDir(logsDir, walk); err != nil {
		render.Status(r, http.StatusInternalServerError)
		render.JSON(w, r, errorResponse{
			Message: err.Error(),
			Status:  http.StatusInternalServerError,
		})
		return
	}

	response.Count = len(response.Files)

	render.JSON(w, r, response)
}

var ( // regexes for sanitizing log files
	keyValueRegex = regexp.MustCompile(`(passkey|authkey|secret_key|apikey)=([a-zA-Z0-9]+)`)
)

func SanitizeLogFile(filePath string) (string, error) {
	data, err := os.ReadFile(filePath)
	if err != nil {
		return "", err
	}

	sanitizedData := string(data)

	// passkey| authkey |secret_key|apikey)=([a-zA-Z0-9]+)
	sanitizedData = keyValueRegex.ReplaceAllString(sanitizedData, "${1}=REDACTED")

	tmpFile, err := os.CreateTemp("", "sanitized-log-*.log")
	if err != nil {
		return "", err
	}

	_, err = tmpFile.WriteString(sanitizedData)
	if err != nil {
		err := tmpFile.Close()
		if err != nil {
			return "", err
		}
		return "", err
	}

	err = tmpFile.Close()
	if err != nil {
		return "", err
	}

	return tmpFile.Name(), nil
}

func (h logsHandler) downloadFile(w http.ResponseWriter, r *http.Request) {
	if h.cfg.Config.Logging.Path == "" { // Access via nested Logging struct
		render.Status(r, http.StatusNotFound)
		return
	}

	logsDir := h.cfg.Config.Logging.Path // Use the directory path directly

	// check if dir exists before walkDir
	if _, err := os.Stat(logsDir); os.IsNotExist(err) {
		render.Status(r, http.StatusNotFound)
		render.JSON(w, r, errorResponse{
			Message: "log directory not found or inaccessible",
			Status:  http.StatusNotFound,
		})
		return
	}

	logFile := chi.URLParam(r, "logFile")
	if logFile == "" {
		render.Status(r, http.StatusBadRequest)
		render.JSON(w, r, errorResponse{
			Message: "empty log file",
			Status:  http.StatusBadRequest,
		})
		return
	} else if !strings.Contains(logFile, ".log") {
		render.Status(r, http.StatusBadRequest)
		render.JSON(w, r, errorResponse{
			Message: "invalid file",
			Status:  http.StatusBadRequest,
		})
		return
	}

	filePath := filepath.Join(logsDir, logFile)

	// Sanitize the log file
	sanitizedFilePath, err := SanitizeLogFile(filePath)
	if err != nil {
		render.Status(r, http.StatusInternalServerError)
		render.JSON(w, r, errorResponse{
			Message: err.Error(),
			Status:  http.StatusInternalServerError,
		})
		return
	}
	defer func(name string) {
		err := os.Remove(name)
		if err != nil {
			fmt.Printf("failed to remove temp file: %s", name)
		}
	}(sanitizedFilePath)

	w.Header().Set("Content-Disposition", "attachment; filename="+strconv.Quote(logFile))
	w.Header().Set("Content-Type", "application/octet-stream")

	http.ServeFile(w, r, sanitizedFilePath)
}

type logFile struct {
	Name      string    `json:"filename"`
	SizeBytes int64     `json:"size_bytes"`
	Size      string    `json:"size"`
	UpdatedAt time.Time `json:"updated_at"`
}

type LogfilesResponse struct {
	Files []logFile `json:"files"`
	Count int       `json:"count"`
}
