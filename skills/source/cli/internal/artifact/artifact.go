// Package artifact freezes a version: copies of files, and a manifest.json.
package artifact

import (
	"cmp"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"source/internal/charter"
	"source/internal/store"
)

// File is one copy in a version.
type File struct {
	Path   string `json:"path"`
	SHA256 string `json:"sha256"`
	Bytes  int64  `json:"bytes"`
}

// Manifest is the manifest.json of a version.
type Manifest struct {
	Phase   string    `json:"phase"`
	Version int       `json:"version"`
	At      time.Time `json:"at"`
	Files   []File    `json:"files"`
}

// Freeze copies paths to dest/<base name>, then writes dest/manifest.json. It
// reads all paths first, so a folder or a repeated base name writes nothing.
func Freeze(dest, phase string, version int, at time.Time, paths []string) (Manifest, error) {
	m, data := Manifest{Phase: phase, Version: version, At: at, Files: []File{}}, map[string][]byte{}
	for _, p := range paths {
		b, err := os.ReadFile(p)
		if _, ok := data[filepath.Base(p)]; ok || err != nil {
			return Manifest{}, cmp.Or(err, fmt.Errorf("%s repeats a base name of the version", p))
		}
		data[filepath.Base(p)] = b
		m.Files = append(m.Files, File{Path: filepath.Join(dest, filepath.Base(p)), SHA256: charter.Hash(b), Bytes: int64(len(b))})
	}
	for _, f := range m.Files {
		if err := store.WriteFileAtomic(f.Path, data[filepath.Base(f.Path)], 0o644); err != nil {
			return Manifest{}, err
		}
	}
	return m, store.WriteJSON(filepath.Join(dest, "manifest.json"), m)
}
