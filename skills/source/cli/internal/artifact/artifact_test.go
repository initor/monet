package artifact

import (
	"os"
	"path/filepath"
	"reflect"
	"regexp"
	"strings"
	"testing"
	"time"

	"source/internal/charter"
	"source/internal/store"
)

var testNow = time.Date(2026, 11, 5, 9, 0, 0, 0, time.UTC)

func write(t *testing.T, path, body string) string {
	if os.MkdirAll(filepath.Dir(path), 0o755) != nil || os.WriteFile(path, []byte(body), 0o644) != nil {
		t.Fatalf("write %s", path)
	}
	return path
}

// The copies equal their sources, and manifest.json lists them in path order.
func TestFreezeCopiesWithManifest(t *testing.T) {
	src, dest, disk := t.TempDir(), filepath.Join(t.TempDir(), "v1"), Manifest{}
	got, err := Freeze(dest, "research", 1, testNow, []string{write(t, filepath.Join(src, "b.md"), "beta"), write(t, filepath.Join(src, "a.md"), "alpha")})
	derr, read := store.DecodeStrict(filepath.Join(dest, "manifest.json"), &disk), func(n string) string { b, _ := os.ReadFile(filepath.Join(dest, n)); return string(b) }
	want := Manifest{Phase: "research", Version: 1, At: testNow, Files: []File{{Path: filepath.Join(dest, "b.md"), SHA256: charter.Hash([]byte("beta")), Bytes: 4},
		{Path: filepath.Join(dest, "a.md"), SHA256: charter.Hash([]byte("alpha")), Bytes: 5}}}
	if err != nil || derr != nil || read("a.md") != "alpha" || read("b.md") != "beta" || !reflect.DeepEqual(got, want) || !reflect.DeepEqual(disk, want) || strings.Join(regexp.MustCompile(`"\w+":`).FindAllString(read("manifest.json"), -1), " ") != `"phase": "version": "at": "files": "path": "sha256": "bytes": "path": "sha256": "bytes":` {
		t.Fatalf("freeze %v, decode %v, manifest.json %s\n got %+v\ndisk %+v\nwant %+v", err, derr, read("manifest.json"), got, disk, want)
	}
}

// A folder or a repeated base name writes no dest; a failed copy writes no manifest.json.
func TestFreezeRefuses(t *testing.T) {
	x1, x2 := write(t, filepath.Join(t.TempDir(), "x.md"), "one"), write(t, filepath.Join(t.TempDir(), "x.md"), "two")
	for name, paths := range map[string][]string{"folder": {x1, filepath.Dir(x2)}, "same base name": {x1, x2}} {
		dest, bad := filepath.Join(t.TempDir(), "v1"), paths[len(paths)-1]
		_, err := Freeze(dest, "research", 1, testNow, paths)
		if _, serr := os.Stat(dest); err == nil || !strings.Contains(err.Error(), bad) || !os.IsNotExist(serr) {
			t.Errorf("%s: err %v, want one that names %s; dest stat %v, want no folder", name, err, bad, serr)
		}
	}
	full := filepath.Dir(write(t, filepath.Join(t.TempDir(), "v1", "x.md", "keep"), "")) // dest/x.md is a folder
	_, err := Freeze(filepath.Dir(full), "research", 1, testNow, []string{x1})
	if _, serr := os.Stat(filepath.Join(filepath.Dir(full), "manifest.json")); err == nil || !strings.Contains(err.Error(), full) || !os.IsNotExist(serr) {
		t.Errorf("failed copy: err %v, want one that names %s; manifest.json stat %v, want none", err, full, serr)
	}
}
