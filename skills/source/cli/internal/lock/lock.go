// Package lock implements session leases as files in <run>/locks/<name>.json.
// A short-lived <name>.mutex file guards each read-check-replace.
package lock

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"time"

	"source/internal/store"
)

// Lease is one lock file. Holders are sessions, not processes, so a lease
// keeps no PID and no host, and only expiry frees it.
type Lease struct {
	Name    string    `json:"name"`
	Holder  string    `json:"holder"`
	Expires time.Time `json:"expires"`
}

// ErrHeld means another holder has the lease, or the caller does not hold it.
var ErrHeld = errors.New("lock held")

// mutexStale is how old a mutex file must be before it counts as abandoned.
const mutexStale = 10 * time.Second

var nameRE = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._-]*$`)

// ValidName reports whether name can name a lock: a letter or digit, then
// letters, digits, '.', '_', or '-'.
func ValidName(name string) bool { return nameRE.MatchString(name) }

// WithMutex runs fn while holding the mutex file at path (O_EXCL create).
// A mutex file older than 10 seconds belongs to a process that died inside
// its critical section, so it is removed and the create is retried.
func WithMutex(path string, fn func() error) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	deadline := time.Now().Add(2 * mutexStale)
	for {
		f, err := os.OpenFile(path, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o644)
		if err == nil {
			fmt.Fprintf(f, "%d\n", os.Getpid())
			f.Close()
			defer os.Remove(path)
			return fn()
		}
		if !errors.Is(err, os.ErrExist) {
			return err
		}
		if st, serr := os.Stat(path); serr == nil && time.Since(st.ModTime()) > mutexStale {
			os.Remove(path)
			continue
		}
		if time.Now().After(deadline) {
			return fmt.Errorf("mutex %s is still busy after %s", path, 2*mutexStale)
		}
		time.Sleep(10 * time.Millisecond)
	}
}

// Read returns the lease in dir/<name>.json, decoded strictly. A missing file
// gives ok false and no error.
func Read(dir, name string) (Lease, bool, error) {
	if !ValidName(name) {
		return Lease{}, false, fmt.Errorf("lock name %q must match %s", name, nameRE)
	}
	var l Lease
	err := store.DecodeStrict(filepath.Join(dir, name+".json"), &l)
	if errors.Is(err, os.ErrNotExist) {
		return Lease{}, false, nil
	}
	if err != nil {
		return Lease{}, false, fmt.Errorf("lease %s: %w", name, err)
	}
	return l, true, nil
}

// write replaces dir/<name>.json with l, atomically.
func write(dir string, l Lease) error {
	b, err := json.Marshal(l)
	if err != nil {
		return err
	}
	return store.WriteFileAtomic(filepath.Join(dir, l.Name+".json"), append(b, '\n'), 0o644)
}
