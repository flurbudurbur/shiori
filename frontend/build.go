package frontend

import (
	"bytes"
	"embed"
	"fmt"
	"html/template"
	"io"
	"io/fs"
	"net/http"
	"os"
	"path"
	"path/filepath"
	"strings"

	"github.com/go-chi/chi/v5"
)

type defaultFS struct {
	prefix string
	fs     fs.FS
}

type IndexParams struct {
	Title   string
	Version string
	BaseUrl string
}

var (
	//go:embed all:dist/*
	Dist embed.FS

	DistDirFS = MustSubFS(Dist, "dist")
)

func (fs defaultFS) Open(name string) (fs.File, error) {
	if fs.fs == nil {
		return os.Open(name)
	}
	return fs.fs.Open(name)
}

// MustSubFS creates sub FS from current filesystem or panic on failure.
// Panic happens when `fsRoot` contains invalid path according to `fs.ValidPath` rules.
//
// MustSubFS is helpful when dealing with `embed.FS` because for example `//go:embed assets/images` embeds files with
// paths including `assets/images` as their prefix. In that case use `fs := MustSubFS(fs, "rootDirectory") to
// create sub fs which uses necessary prefix for directory path.
func MustSubFS(currentFs fs.FS, fsRoot string) fs.FS {
	subFs, err := subFS(currentFs, fsRoot)
	if err != nil {
		panic(fmt.Errorf("can not create sub FS, invalid root given, err: %w", err))
	}
	return subFs
}

func subFS(currentFs fs.FS, root string) (fs.FS, error) {
	root = filepath.ToSlash(filepath.Clean(root)) // note: fs.FS operates only with slashes. `ToSlash` is necessary for Windows
	if dFS, ok := currentFs.(*defaultFS); ok {
		// we need to make exception for `defaultFS` instances as it interprets root prefix differently from fs.FS.
		// fs.Fs.Open does not like relative paths ("./", "../") and absolute paths.
		if !filepath.IsAbs(root) {
			root = filepath.Join(dFS.prefix, root)
		}
		return &defaultFS{
			prefix: root,
			fs:     os.DirFS(root),
		}, nil
	}
	return fs.Sub(currentFs, root)
}

// FileFS registers a new route with path to serve a file from the provided file system.
func FileFS(r *chi.Mux, path, file string, filesystem fs.FS) {
	r.Get(path, StaticFileHandler(file, filesystem))
}

// StaticFileHandler creates a handler function to serve a file from the provided file system.
func StaticFileHandler(file string, filesystem fs.FS) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		fsFile(w, r, file, filesystem)
	}
}

// StaticFS registers a new route with path prefix to serve static files from the provided file system.
func StaticFS(r *chi.Mux, pathPrefix string, filesystem fs.FS) {
	r.Handle(pathPrefix+"*", http.StripPrefix(pathPrefix, http.FileServer(http.FS(filesystem))))
}

// fsFile is a helper function to serve a file from the provided file system.
func fsFile(w http.ResponseWriter, r *http.Request, file string, filesystem fs.FS) {
	f, err := filesystem.Open(file)
	if err != nil {
		http.Error(w, "File not found", http.StatusNotFound)
		return
	}
	defer f.Close()

	stat, err := f.Stat()
	if err != nil {
		http.Error(w, "File not found", http.StatusNotFound)
		return
	}

	data, err := io.ReadAll(f)
	if err != nil {
		http.Error(w, "Failed to read the file", http.StatusInternalServerError)
		return
	}

	reader := bytes.NewReader(data)
	http.ServeContent(w, r, file, stat.ModTime(), reader)
}

// RegisterHandler register web routes and file serving
func RegisterHandler(c *chi.Mux, version, baseUrl string) {
	// Serve static files without a prefix
	assets, _ := fs.Sub(DistDirFS, "assets")
	static, _ := fs.Sub(DistDirFS, "static")
	StaticFS(c, "/assets", assets)
	StaticFS(c, "/static", static)

	p := IndexParams{
		Title:   "Dashboard",
		Version: version,
		BaseUrl: baseUrl,
	}

	// serve on base route
	c.Get("/", func(w http.ResponseWriter, r *http.Request) {
		Index(w, p)
	})

	// handle all other routes
	c.Get("/*", func(w http.ResponseWriter, r *http.Request) {
		requestedPath := strings.TrimPrefix(r.URL.Path, "/")

		// Handle manifest separately if needed (could also be treated as static)
		if requestedPath == "manifest.webmanifest" {
			Manifest(w, p)
			return
		}

		// Construct the path within the 'browser' subdirectory relative to DistDirFS
		// Assuming DistDirFS points to the 'dist' directory
		filePath := path.Join("browser", requestedPath)

		// Attempt to open the file from the embedded filesystem
		f, err := DistDirFS.Open(filePath)
		if err != nil {
			// If the file doesn't exist, assume it's an Angular route and serve index.html
			if os.IsNotExist(err) {
				Index(w, p) // Serve the main application page (already points to dist/browser/index.html)
				return
			}
			// For other errors (e.g., permission issues), return an internal server error
			http.Error(w, "Internal Server Error", http.StatusInternalServerError)
			// TODO: Log the actual error err
			return
		}
		defer f.Close()

		// Get file stats
		stat, err := f.Stat()
		if err != nil {
			// Handle potential errors getting file stats
			http.Error(w, "Internal Server Error", http.StatusInternalServerError)
			// TODO: Log the actual error err
			return
		}

		// If it's a directory, potentially serve index.html or return an error/redirect
		// For now, we assume direct file access or SPA routing fallback
		if stat.IsDir() {
			// Decide how to handle directory requests, e.g., serve index.html or forbid
			Index(w, p) // Or http.NotFound(w, r) or http.StatusForbidden
			return
		}

		// Serve the static file content
		http.ServeContent(w, r, stat.Name(), stat.ModTime(), f.(io.ReadSeeker)) // f needs to be ReadSeeker for ServeContent
	})
}

func Index(w io.Writer, p IndexParams) error {
	tmpl, err := parseIndex()
	if err != nil {
		// Return the error so the HTTP handler can potentially log it
		// and return an appropriate HTTP error code (e.g., 500).
		// Consider adding logging here if not handled further up the call stack.
		return fmt.Errorf("failed to parse index template: %w", err)
	}
	return tmpl.Execute(w, p)
}

func parseIndex() (*template.Template, error) {
	// Explicitly check the error instead of using template.Must
	tmpl, err := template.New("index.html").ParseFS(Dist, "dist/browser/index.html")
	return tmpl, err // Return the template and the potential error
}

func Manifest(w io.Writer, p IndexParams) error {
	return parseManifest().Execute(w, p)
}

func parseManifest() *template.Template {
	return template.Must(template.New("manifest.webmanifest").ParseFS(Dist, "dist/manifest.webmanifest"))
}
