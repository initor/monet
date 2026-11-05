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

// Acquire takes the lease for holder until now+ttl. The lease is free when it
// has no file, when holder already has it (a renew), or when now is not
// before its expiry. Otherwise it returns ErrHeld, naming the holder.
func Acquire(dir, name, holder string, ttl time.Duration, now time.Time) (Lease, error) {
	if err := validate(name, holder, ttl); err != nil {
		return Lease{}, err
	}
	var got Lease
	err := WithMutex(filepath.Join(dir, name+".mutex"), func() error {
		cur, ok, err := Read(dir, name)
		if err != nil {
			return err
		}
		if ok && cur.Holder != holder && now.Before(cur.Expires) {
			return fmt.Errorf("%w: %s by %s until %s", ErrHeld, name, cur.Holder, cur.Expires.Format(time.RFC3339))
		}
		got = Lease{Name: name, Holder: holder, Expires: now.Add(ttl)}
		return write(dir, got)
	})
	return got, err
}

// Renew moves the expiry of a lease that holder still has to now+ttl. A
// lease that another holder took, or that has no file, gives ErrHeld.
func Renew(dir, name, holder string, ttl time.Duration, now time.Time) error {
	if err := validate(name, holder, ttl); err != nil {
		return err
	}
	return WithMutex(filepath.Join(dir, name+".mutex"), func() error {
		cur, ok, err := Read(dir, name)
		if err != nil {
			return err
		}
		if !ok || cur.Holder != holder {
			return fmt.Errorf("%w: %s is not held by %s", ErrHeld, name, holder)
		}
		cur.Expires = now.Add(ttl)
		return write(dir, cur)
	})
}

// Release removes the lease. Releasing a free lock is a no-op. Another
// holder gets ErrHeld and the lease stays.
func Release(dir, name, holder string) error {
	if err := validate(name, holder, 1); err != nil {
		return err
	}
	return WithMutex(filepath.Join(dir, name+".mutex"), func() error {
		cur, ok, err := Read(dir, name)
		if err != nil || !ok {
			return err
		}
		if cur.Holder != holder {
			return fmt.Errorf("%w: %s by %s", ErrHeld, name, cur.Holder)
		}
		return os.Remove(filepath.Join(dir, name+".json"))
	})
}

// validate refuses a bad lock name, an empty holder, or a TTL that is not
// positive. Release has no TTL and passes 1.
func validate(name, holder string, ttl time.Duration) error {
	switch {
	case !ValidName(name):
		return fmt.Errorf("lock name %q must match %s", name, nameRE)
	case holder == "":
		return errors.New("lock holder is empty")
	case ttl <= 0:
		return errors.New("lock ttl must be positive")
	}
	return nil
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
