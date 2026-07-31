//go:build cssstub

package middleware

import (
	"errors"
	"fmt"
	"io"
	"log"
	"mime"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"time"
)

// StaticAssetEntry links a relative URL to a file path.
type StaticAssetEntry struct {
	RelativeURL string
	FilePath    string
}

// StaticAssetHandler serves static resources on specific paths.
type StaticAssetHandler struct {
	mappings    map[string]string
	pathMatcher *regexp.Regexp
	expires     int
	baseUrl     string
}

// NewStaticAssetHandler creates a handler for the provided static resources.
func NewStaticAssetHandler(assets []StaticAssetEntry, baseUrl string, expires int) (*StaticAssetHandler, error) {
	mappings := make(map[string]string)
	rootPath := ensureTrailingSlash(strings.TrimPrefix(baseUrl, "http://")) // crude, but for demo
	for _, entry := range assets {
		mappings[trimLeadingSlashes(entry.RelativeURL)] = resolveAssetPath(entry.FilePath)
	}
	pathMatcher, err := createPathMatcher(rootPath, mappings)
	if err != nil {
		return nil, err
	}
	return &StaticAssetHandler{
		mappings:    mappings,
		pathMatcher: pathMatcher,
		expires:     expires,
		baseUrl:     baseUrl,
	}, nil
}

// ServeHTTP implements http.Handler for static assets.
func (h *StaticAssetHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet && r.Method != http.MethodHead {
		http.Error(w, "Only GET and HEAD requests are supported", http.StatusNotImplemented)
		return
	}
	filePath, err := h.getFilePath(r.URL.Path)
	if err != nil {
		log.Printf("StaticAssetHandler: %v", err)
		http.NotFound(w, r)
		return
	}
	log.Printf("Serving %s via static asset %s", r.URL.Path, filePath)
	file, err := os.Open(filePath)
	if err != nil {
		log.Printf("StaticAssetHandler: file open error: %v", err)
		http.NotFound(w, r)
		return
	}
	defer file.Close()
	stat, err := file.Stat()
	if err != nil || stat.IsDir() {
		log.Printf("StaticAssetHandler: file stat error or is dir: %v", err)
		http.NotFound(w, r)
		return
	}
	contentType := mime.TypeByExtension(filepath.Ext(filePath))
	if contentType == "" {
		contentType = "application/octet-stream"
	}
	for k, v := range h.getCacheHeaders() {
		w.Header().Set(k, v)
	}
	w.Header().Set("Content-Type", contentType)
	w.WriteHeader(http.StatusOK)
	if r.Method == http.MethodGet {
		_, err = io.Copy(w, file)
		if err != nil {
			log.Printf("StaticAssetHandler: error copying file: %v", err)
		}
	}
}

// getFilePath matches the URL to a file path.
func (h *StaticAssetHandler) getFilePath(url string) (string, error) {
	match := h.pathMatcher.FindStringSubmatch(url)
	if match == nil || strings.Contains(match[0], "/..") {
		return "", errors.New("no static resource configured at " + url)
	}
	// The mapping is either a known document, or a file within a folder
	document, folder, file := match[1], match[2], match[3]
	if document != "" {
		return h.mappings[document], nil
	}
	if folder != "" && file != "" {
		return filepath.Join(h.mappings[folder], file), nil
	}
	return "", errors.New("no static resource configured at " + url)
}

// getCacheHeaders returns cache headers.
func (h *StaticAssetHandler) getCacheHeaders() map[string]string {
	if h.expires <= 0 {
		return map[string]string{}
	}
	expiry := time.Now().Add(time.Duration(h.expires) * time.Second).UTC().Format(http.TimeFormat)
	return map[string]string{
		"Cache-Control": fmt.Sprintf("max-age=%d", h.expires),
		"Expires":       expiry,
	}
}

// --- Utility functions ---
func ensureTrailingSlash(path string) string {
	if len(path) == 0 || path[len(path)-1] == '/' {
		return path
	}
	return path + "/"
}

func trimLeadingSlashes(s string) string {
	return strings.TrimLeft(s, "/")
}

func resolveAssetPath(path string) string {
	// For demo, just return the path as is
	return path
}

func createPathMatcher(rootPath string, mappings map[string]string) (*regexp.Regexp, error) {
	// Sort longest paths first to ensure the longest match has priority
	var paths []string
	for k := range mappings {
		paths = append(paths, k)
	}
	sort.Slice(paths, func(i, j int) bool { return len(paths[i]) > len(paths[j]) })
	files := []string{".^"}
	folders := []string{".^"}
	for _, path := range paths {
		filePath := mappings[path]
		if strings.HasSuffix(filePath, "/") && !strings.HasSuffix(path, "/") {
			return nil, fmt.Errorf("Server is misconfigured: StaticAssetHandler cannot have a file path ending on a slash if the URL does not, but received %s and %s", path, filePath)
		}
		if strings.HasSuffix(filePath, "/") {
			folders = append(folders, regexp.QuoteMeta(path))
		} else {
			files = append(files, regexp.QuoteMeta(path))
		}
	}
	pattern := fmt.Sprintf(`^(?:%s|%s([^?]+))(?:\?.*)?$`, strings.Join(files, "|"), strings.Join(folders, "|"))
	return regexp.Compile(pattern)
}
