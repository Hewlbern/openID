package replica

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"time"
)

type FileMeta struct {
	ContentType string    `json:"contentType"`
	Modified    time.Time `json:"modified"`
	IsContainer bool      `json:"isContainer"`
}

type Resource struct {
	Path        string
	ContentType string
	Body        []byte
	IsContainer bool
	Modified    time.Time
}

// WalkPod lists LDP resources under storageRoot/handle, skipping store metadata files.
func WalkPod(storageRoot, handle string) ([]Resource, error) {
	handle = strings.Trim(handle, "/")
	root := filepath.Join(storageRoot, handle)
	info, err := os.Stat(root)
	if err != nil {
		return nil, err
	}
	if !info.IsDir() {
		return nil, os.ErrNotExist
	}
	var out []Resource
	err = filepath.WalkDir(root, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		name := d.Name()
		if name == ".meta.json" || name == ".container" || name == ".root" || strings.HasSuffix(name, ".meta.json") {
			return nil
		}
		rel, err := filepath.Rel(storageRoot, path)
		if err != nil {
			return err
		}
		rel = filepath.ToSlash(rel)
		if d.IsDir() {
			out = append(out, Resource{
				Path:        strings.TrimSuffix(rel, "/") + "/",
				ContentType: "text/turtle",
				IsContainer: true,
				Modified:    fileMod(path),
			})
			return nil
		}
		body, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		ct := "application/octet-stream"
		mod := fileMod(path)
		if raw, err := os.ReadFile(path + ".meta.json"); err == nil {
			var m FileMeta
			if json.Unmarshal(raw, &m) == nil {
				if m.ContentType != "" {
					ct = m.ContentType
				}
				if !m.Modified.IsZero() {
					mod = m.Modified
				}
			}
		}
		out = append(out, Resource{
			Path:        rel,
			ContentType: ct,
			Body:        body,
			Modified:    mod,
		})
		return nil
	})
	return out, err
}

func fileMod(path string) time.Time {
	st, err := os.Stat(path)
	if err != nil {
		return time.Time{}
	}
	return st.ModTime().UTC()
}
