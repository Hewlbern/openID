package resourcestore

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"solid-go/internal/storage"
)

var (
	ErrNotFound      = errors.New("resource not found")
	ErrConflict      = errors.New("resource conflict")
	ErrNotContainer  = errors.New("not a container")
	ErrIsContainer   = errors.New("is a container")
	ErrPrecondition  = errors.New("precondition failed")
	ErrNotEmpty      = errors.New("container not empty")
)

// Resource is a Solid LDP resource (document or container).
type Resource struct {
	Path        string
	IsContainer bool
	ContentType string
	Body        []byte
	ETag        string
	Modified    time.Time
	Created     time.Time
}

type metaFile struct {
	ContentType string    `json:"contentType"`
	ETag        string    `json:"etag"`
	Modified    time.Time `json:"modified"`
	Created     time.Time `json:"created"`
	IsContainer bool      `json:"isContainer"`
}

// Store provides Solid-oriented resource operations on top of Storage.
type Store struct {
	base storage.Storage
	root string
	mu   sync.RWMutex
}

func New(base storage.Storage, root string) *Store {
	return &Store{base: base, root: root}
}

func (s *Store) normalize(path string) string {
	path = strings.TrimPrefix(path, "/")
	path = filepath.ToSlash(path)
	if path == "." {
		return ""
	}
	return path
}

func (s *Store) metaPath(path string) string {
	if path == "" {
		return ".meta.json"
	}
	if strings.HasSuffix(path, "/") {
		return strings.TrimSuffix(path, "/") + "/.meta.json"
	}
	return path + ".meta.json"
}

func (s *Store) dataPath(path string) string {
	path = s.normalize(path)
	if path == "" {
		return ".root"
	}
	if strings.HasSuffix(path, "/") {
		return strings.TrimSuffix(path, "/") + "/.container"
	}
	return path
}

func etagFor(data []byte) string {
	sum := sha256.Sum256(data)
	return `"` + hex.EncodeToString(sum[:8]) + `"`
}

func (s *Store) readMeta(ctx context.Context, path string) (*metaFile, error) {
	raw, err := s.base.Get(ctx, s.metaPath(path))
	if err != nil {
		if os.IsNotExist(err) || errors.Is(err, os.ErrNotExist) || errors.Is(err, os.ErrNotExist) {
			return nil, ErrNotFound
		}
		var pe *os.PathError
		if errors.As(err, &pe) && os.IsNotExist(pe.Err) {
			return nil, ErrNotFound
		}
		if strings.Contains(err.Error(), "no such file") {
			return nil, ErrNotFound
		}
		return nil, err
	}
	var m metaFile
	if err := json.Unmarshal(raw, &m); err != nil {
		return nil, err
	}
	return &m, nil
}

func (s *Store) writeMeta(ctx context.Context, path string, m *metaFile) error {
	raw, err := json.MarshalIndent(m, "", "  ")
	if err != nil {
		return err
	}
	return s.base.Put(ctx, s.metaPath(path), raw)
}

// Exists reports whether a resource exists.
func (s *Store) Exists(ctx context.Context, path string) (bool, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	path = s.normalize(path)
	_, err := s.readMeta(ctx, path)
	if err == ErrNotFound {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	return true, nil
}

// Get returns a resource.
func (s *Store) Get(ctx context.Context, path string) (*Resource, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	path = s.normalize(path)
	m, err := s.readMeta(ctx, path)
	if err != nil {
		return nil, err
	}
	body, err := s.base.Get(ctx, s.dataPath(path))
	if err != nil && !m.IsContainer {
		return nil, err
	}
	if m.IsContainer && body == nil {
		body = []byte{}
	}
	return &Resource{
		Path:        path,
		IsContainer: m.IsContainer,
		ContentType: m.ContentType,
		Body:        body,
		ETag:        m.ETag,
		Modified:    m.Modified,
		Created:     m.Created,
	}, nil
}

// EnsureContainer creates a container path and parents.
func (s *Store) EnsureContainer(ctx context.Context, path string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.ensureContainerLocked(ctx, path)
}

func (s *Store) ensureContainerLocked(ctx context.Context, path string) error {
	path = s.normalize(path)
	if path != "" && !strings.HasSuffix(path, "/") {
		path += "/"
	}
	parts := strings.Split(strings.Trim(path, "/"), "/")
	cur := ""
	if path == "" || path == "/" {
		return s.putContainerLocked(ctx, "")
	}
	for _, p := range parts {
		if p == "" {
			continue
		}
		if cur == "" {
			cur = p + "/"
		} else {
			cur = cur + p + "/"
		}
		if err := s.putContainerLocked(ctx, cur); err != nil {
			return err
		}
	}
	return nil
}

func (s *Store) putContainerLocked(ctx context.Context, path string) error {
	path = s.normalize(path)
	if path != "" && !strings.HasSuffix(path, "/") {
		path += "/"
	}
	if _, err := s.readMeta(ctx, path); err == nil {
		return nil
	}
	now := time.Now().UTC()
	body := []byte("# container\n")
	m := &metaFile{
		ContentType: "text/turtle",
		ETag:        etagFor(body),
		Modified:    now,
		Created:     now,
		IsContainer: true,
	}
	if err := s.base.Put(ctx, s.dataPath(path), body); err != nil {
		return err
	}
	return s.writeMeta(ctx, path, m)
}

// Put creates or replaces a document resource.
func (s *Store) Put(ctx context.Context, path string, contentType string, body []byte, ifMatch, ifNoneMatch string) (*Resource, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	path = s.normalize(path)
	if strings.HasSuffix(path, "/") {
		return nil, ErrIsContainer
	}
	// ensure parent containers
	parent := filepath.Dir(path)
	if parent != "." && parent != "/" && parent != "" {
		if err := s.ensureContainerLocked(ctx, parent+"/"); err != nil {
			return nil, err
		}
	} else {
		_ = s.putContainerLocked(ctx, "")
	}

	existing, err := s.readMeta(ctx, path)
	exists := err == nil
	if err != nil && err != ErrNotFound {
		return nil, err
	}
	if err := checkPreconditions(exists, existing, ifMatch, ifNoneMatch); err != nil {
		return nil, err
	}

	now := time.Now().UTC()
	created := now
	if exists {
		created = existing.Created
	}
	if contentType == "" {
		contentType = "application/octet-stream"
	}
	m := &metaFile{
		ContentType: contentType,
		ETag:        etagFor(body),
		Modified:    now,
		Created:     created,
		IsContainer: false,
	}
	if err := s.base.Put(ctx, s.dataPath(path), body); err != nil {
		return nil, err
	}
	if err := s.writeMeta(ctx, path, m); err != nil {
		return nil, err
	}
	return &Resource{
		Path:        path,
		IsContainer: false,
		ContentType: contentType,
		Body:        body,
		ETag:        m.ETag,
		Modified:    now,
		Created:     created,
	}, nil
}

// PutContainer creates a container at path.
func (s *Store) PutContainer(ctx context.Context, path string) (*Resource, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	path = s.normalize(path)
	if path != "" && !strings.HasSuffix(path, "/") {
		path += "/"
	}
	parent := filepath.Dir(strings.TrimSuffix(path, "/"))
	if parent != "." && parent != "/" && parent != "" {
		if err := s.ensureContainerLocked(ctx, parent+"/"); err != nil {
			return nil, err
		}
	}
	if err := s.putContainerLocked(ctx, path); err != nil {
		return nil, err
	}
	return s.getLocked(ctx, path)
}

func (s *Store) getLocked(ctx context.Context, path string) (*Resource, error) {
	m, err := s.readMeta(ctx, path)
	if err != nil {
		return nil, err
	}
	body, _ := s.base.Get(ctx, s.dataPath(path))
	return &Resource{
		Path:        path,
		IsContainer: m.IsContainer,
		ContentType: m.ContentType,
		Body:        body,
		ETag:        m.ETag,
		Modified:    m.Modified,
		Created:     m.Created,
	}, nil
}

// Delete removes a resource.
func (s *Store) Delete(ctx context.Context, path string, ifMatch string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	path = s.normalize(path)
	m, err := s.readMeta(ctx, path)
	if err != nil {
		return err
	}
	if ifMatch != "" && ifMatch != "*" && m.ETag != ifMatch {
		return ErrPrecondition
	}
	if m.IsContainer {
		children, err := s.listLocked(ctx, path)
		if err != nil {
			return err
		}
		// filter meta internals
		var real []string
		for _, c := range children {
			base := filepath.Base(strings.TrimSuffix(c, "/"))
			if base == ".meta.json" || base == ".container" || strings.HasSuffix(c, ".meta.json") {
				continue
			}
			// only count resources with meta
			if _, err := s.readMeta(ctx, s.normalize(c)); err == nil {
				real = append(real, c)
			}
		}
		if len(real) > 0 {
			return ErrNotEmpty
		}
	}
	_ = s.base.Delete(ctx, s.dataPath(path))
	_ = s.base.Delete(ctx, s.metaPath(path))
	return nil
}

// List returns child resource paths inside a container.
func (s *Store) List(ctx context.Context, path string) ([]string, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.listLocked(ctx, path)
}

func (s *Store) listLocked(ctx context.Context, path string) ([]string, error) {
	path = s.normalize(path)
	if path != "" && !strings.HasSuffix(path, "/") {
		path += "/"
	}
	m, err := s.readMeta(ctx, path)
	if err != nil {
		return nil, err
	}
	if !m.IsContainer {
		return nil, ErrNotContainer
	}
	dir := strings.TrimSuffix(path, "/")
	entries, err := s.base.List(ctx, dir)
	if err != nil {
		// empty dir
		if strings.Contains(err.Error(), "no such file") {
			return nil, nil
		}
		return nil, err
	}
	seen := map[string]bool{}
	var out []string
	for _, e := range entries {
		e = filepath.ToSlash(e)
		base := filepath.Base(e)
		if base == ".meta.json" || base == ".container" || base == ".root" || strings.HasSuffix(base, ".meta.json") {
			continue
		}
		// map file to resource path
		rel := strings.TrimPrefix(e, strings.TrimPrefix(s.root, "./"))
		rel = s.normalize(e)
		// If it's a directory entry ending with /, treat as container
		candidate := rel
		if strings.HasSuffix(candidate, "/") {
			if _, err := s.readMeta(ctx, candidate); err == nil && !seen[candidate] {
				seen[candidate] = true
				out = append(out, candidate)
			}
			continue
		}
		// document
		if _, err := s.readMeta(ctx, candidate); err == nil && !seen[candidate] {
			seen[candidate] = true
			out = append(out, candidate)
			continue
		}
		// maybe container without trailing slash in list
		if _, err := s.readMeta(ctx, candidate+"/"); err == nil && !seen[candidate+"/"] {
			seen[candidate+"/"] = true
			out = append(out, candidate+"/")
		}
	}
	return out, nil
}

// Post creates a new resource inside a container.
func (s *Store) Post(ctx context.Context, container, slug, contentType string, body []byte, isContainer bool) (*Resource, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	container = s.normalize(container)
	if container != "" && !strings.HasSuffix(container, "/") {
		container += "/"
	}
	if err := s.ensureContainerLocked(ctx, container); err != nil {
		return nil, err
	}
	if slug == "" {
		slug = fmt.Sprintf("resource-%d", time.Now().UnixNano())
	}
	slug = strings.Trim(slug, "/")
	var path string
	if isContainer {
		path = container + slug + "/"
		if err := s.putContainerLocked(ctx, path); err != nil {
			return nil, err
		}
		return s.getLocked(ctx, path)
	}
	path = container + slug
	// unlock pattern - call put internals
	existing, err := s.readMeta(ctx, path)
	if err == nil && existing != nil {
		// conflict - generate unique
		path = fmt.Sprintf("%s%s-%d", container, slug, time.Now().UnixNano())
	}
	now := time.Now().UTC()
	if contentType == "" {
		contentType = "application/octet-stream"
	}
	m := &metaFile{
		ContentType: contentType,
		ETag:        etagFor(body),
		Modified:    now,
		Created:     now,
		IsContainer: false,
	}
	if err := s.base.Put(ctx, s.dataPath(path), body); err != nil {
		return nil, err
	}
	if err := s.writeMeta(ctx, path, m); err != nil {
		return nil, err
	}
	return &Resource{
		Path:        path,
		ContentType: contentType,
		Body:        body,
		ETag:        m.ETag,
		Modified:    now,
		Created:     now,
	}, nil
}

func checkPreconditions(exists bool, existing *metaFile, ifMatch, ifNoneMatch string) error {
	if ifNoneMatch == "*" && exists {
		return ErrPrecondition
	}
	if ifMatch != "" {
		if !exists {
			return ErrPrecondition
		}
		if ifMatch != "*" && existing.ETag != ifMatch {
			return ErrPrecondition
		}
	}
	return nil
}

// AuxPath returns the auxiliary resource path (e.g. .acl, .meta).
func AuxPath(resourcePath, suffix string) string {
	resourcePath = strings.TrimPrefix(resourcePath, "/")
	if strings.HasSuffix(resourcePath, "/") {
		return strings.TrimSuffix(resourcePath, "/") + suffix
	}
	return resourcePath + suffix
}
