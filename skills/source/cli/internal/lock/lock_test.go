package lock

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"testing"
	"time"
)

var testNow = time.Date(2026, 11, 1, 9, 0, 0, 0, time.UTC)

// ageMutex writes a mutex file at path with an mtime age ago.
func ageMutex(t *testing.T, path string, age time.Duration) {
	t.Helper()
	if err := os.WriteFile(path, []byte("1\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	old := time.Now().Add(-age)
	if err := os.Chtimes(path, old, old); err != nil {
		t.Fatal(err)
	}
}

// A mutex file older than 10 seconds is removed at once, and fn runs; a
// younger one holds until it turns 10 seconds old.
func TestWithMutexStale(t *testing.T) {
	for _, tc := range []struct {
		age        time.Duration
		minW, maxW time.Duration
	}{
		{11 * time.Second, 0, 700 * time.Millisecond},
		{9500 * time.Millisecond, 200 * time.Millisecond, 5 * time.Second},
	} {
		m := filepath.Join(t.TempDir(), "brain.mutex")
		ageMutex(t, m, tc.age)
		ran, start := false, time.Now()
		err := WithMutex(m, func() error { ran = true; return nil })
		waited := time.Since(start)
		if err != nil || !ran {
			t.Fatalf("age %s: err %v, ran %v", tc.age, err, ran)
		}
		if waited < tc.minW || waited > tc.maxW {
			t.Errorf("age %s: waited %s, want %s to %s", tc.age, waited, tc.minW, tc.maxW)
		}
		if _, err := os.Stat(m); !errors.Is(err, os.ErrNotExist) {
			t.Errorf("age %s: mutex file left behind (%v)", tc.age, err)
		}
	}
}

// WithMutex returns the error of fn and removes its mutex file.
func TestWithMutexReturnsFnError(t *testing.T) {
	m := filepath.Join(t.TempDir(), "sub", "brain.mutex")
	boom := errors.New("boom")
	if err := WithMutex(m, func() error { return boom }); err != boom {
		t.Fatalf("err %v, want boom", err)
	}
	if _, err := os.Stat(m); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("mutex file left behind (%v)", err)
	}
}

func TestErrHeldText(t *testing.T) {
	if ErrHeld.Error() != "lock held" {
		t.Fatalf("ErrHeld = %q, want lock held", ErrHeld)
	}
}

// A write replaces the lease file by a rename, so a reader of the old file
// never sees a half-written one.
func TestWriteIsAtomic(t *testing.T) {
	dir, l := t.TempDir(), Lease{Name: "brain", Holder: "s1", Expires: testNow}
	if err := write(dir, l); err != nil {
		t.Fatal(err)
	}
	old, err := os.Stat(filepath.Join(dir, "brain.json"))
	if err != nil {
		t.Fatal(err)
	}
	if err := write(dir, l); err != nil {
		t.Fatal(err)
	}
	if cur, err := os.Stat(filepath.Join(dir, "brain.json")); err != nil || os.SameFile(old, cur) {
		t.Fatalf("write rewrote the lease file in place (err %v)", err)
	}
}

func TestValidName(t *testing.T) {
	for name, want := range map[string]bool{
		"brain": true, "dev": true, "a.b_c-9": true, "9x": true,
		"": false, ".hidden": false, "-x": false, "../x": false, "a/b": false, "a b": false,
	} {
		if got := ValidName(name); got != want {
			t.Errorf("ValidName(%q) = %v, want %v", name, got, want)
		}
	}
}

// A written lease reads back whole, and its file holds only name, holder,
// and expires: a lease keeps no PID and no host.
func TestWriteAndRead(t *testing.T) {
	dir := t.TempDir()
	if _, ok, err := Read(dir, "brain"); ok || err != nil {
		t.Fatalf("missing lease: ok %v, err %v", ok, err)
	}
	want := Lease{Name: "brain", Holder: "s1", Expires: testNow.Add(time.Hour)}
	if err := write(dir, want); err != nil {
		t.Fatal(err)
	}
	got, ok, err := Read(dir, "brain")
	if err != nil || !ok || got.Name != want.Name || got.Holder != want.Holder || !got.Expires.Equal(want.Expires) {
		t.Fatalf("Read = %+v, %v, %v, want %+v", got, ok, err, want)
	}
	b, err := os.ReadFile(filepath.Join(dir, "brain.json"))
	if err != nil {
		t.Fatal(err)
	}
	var raw map[string]any
	if err := json.Unmarshal(b, &raw); err != nil {
		t.Fatal(err)
	}
	var keys []string
	for k := range raw {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	if wantK := []string{"expires", "holder", "name"}; !reflect.DeepEqual(keys, wantK) {
		t.Fatalf("keys %v, want %v", keys, wantK)
	}
}

// Read decodes strictly and refuses a name that ValidName refuses.
func TestReadRefuses(t *testing.T) {
	dir := t.TempDir()
	for name, body := range map[string]string{
		"pid":      `{"name":"pid","holder":"s1","pid":7,"expires":"2026-11-01T10:00:00Z"}`,
		"trailing": `{"name":"trailing","holder":"s1","expires":"2026-11-01T10:00:00Z"} {}`,
	} {
		if err := os.WriteFile(filepath.Join(dir, name+".json"), []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
		if _, ok, err := Read(dir, name); err == nil || ok {
			t.Errorf("%s: ok %v, err %v, want an error", name, ok, err)
		}
	}
	if _, _, err := Read(dir, "../x"); err == nil {
		t.Error("bad name accepted")
	}
}
