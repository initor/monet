// Package lock implements lease locks as files in run/locks/<name>.json.
// A short-lived <name>.mutex file guards each read-check-replace.
package lock

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"syscall"
	"time"

	"gang/internal/store"
)

// Lease is the Contract 4 lock file.
type Lease struct {
	Name    string    `json:"name"`
	Holder  string    `json:"holder"`
	PID     int       `json:"pid"`
	Host    string    `json:"host"`
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

// Acquire takes the lease for a caller with no long-lived process, such as
// one gang CLI call. It records pid 0, so only expiry frees the lease.
func Acquire(dir, name, holder string, ttl time.Duration, now time.Time) (Lease, error) {
	return AcquirePID(dir, name, holder, 0, ttl, now)
}

// AcquirePID takes the lease and records pid. The lease is free when it has
// no file, when holder already has it (a renew), when it expired, or when it
// is on this host and its pid is dead. Otherwise it returns ErrHeld.
func AcquirePID(dir, name, holder string, pid int, ttl time.Duration, now time.Time) (Lease, error) {
	if err := validate(name, holder, ttl); err != nil {
		return Lease{}, err
	}
	host, err := os.Hostname()
	if err != nil {
		return Lease{}, err
	}
	var got Lease
	err = WithMutex(filepath.Join(dir, name+".mutex"), func() error {
		cur, ok, err := read(dir, name)
		if err != nil {
			return err
		}
		if ok && cur.Holder != holder && !free(cur, host, now) {
			return fmt.Errorf("%w: %s by %s until %s", ErrHeld, name, cur.Holder, cur.Expires.Format(time.RFC3339))
		}
		got = Lease{Name: name, Holder: holder, PID: pid, Host: host, Expires: now.Add(ttl)}
		return write(dir, got)
	})
	return got, err
}

// Renew moves the expiry of a lease that holder still has to now+ttl.
func Renew(dir, name, holder string, ttl time.Duration, now time.Time) error {
	if err := validate(name, holder, ttl); err != nil {
		return err
	}
	return WithMutex(filepath.Join(dir, name+".mutex"), func() error {
		cur, ok, err := read(dir, name)
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

// Release removes the lease. Releasing a free lock is a no-op. A non-holder
// gets ErrHeld and the lease stays.
func Release(dir, name, holder string) error {
	if err := validate(name, holder, time.Second); err != nil {
		return err
	}
	return WithMutex(filepath.Join(dir, name+".mutex"), func() error {
		cur, ok, err := read(dir, name)
		if err != nil || !ok {
			return err
		}
		if cur.Holder != holder {
			return fmt.Errorf("%w: %s by %s", ErrHeld, name, cur.Holder)
		}
		return os.Remove(filepath.Join(dir, name+".json"))
	})
}

func free(l Lease, host string, now time.Time) bool {
	if !now.Before(l.Expires) {
		return true
	}
	return l.Host == host && l.PID > 0 && errors.Is(syscall.Kill(l.PID, 0), syscall.ESRCH)
}

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

func read(dir, name string) (Lease, bool, error) {
	b, err := os.ReadFile(filepath.Join(dir, name+".json"))
	if errors.Is(err, os.ErrNotExist) {
		return Lease{}, false, nil
	}
	if err != nil {
		return Lease{}, false, err
	}
	var l Lease
	if err := json.Unmarshal(b, &l); err != nil {
		return Lease{}, false, fmt.Errorf("lease %s: %w", name, err)
	}
	return l, true, nil
}

func write(dir string, l Lease) error {
	b, err := json.Marshal(l)
	if err != nil {
		return err
	}
	return store.WriteFileAtomic(filepath.Join(dir, l.Name+".json"), append(b, '\n'), 0o644)
}
