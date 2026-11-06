package lock

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"strings"
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

// TestLeaseRules walks one lease through a take, a renew by acquire, a
// new take at expiry, a lost renew, and release. After each step Read gives
// the holder and the expiry (as an offset from t0) that the row names.
func TestLeaseRules(t *testing.T) {
	d, t0, hr, m := t.TempDir(), testNow, time.Hour, time.Minute
	acq := func(holder string, at, exp time.Duration) func() error {
		return func() error {
			l, err := Acquire(d, "brain", holder, hr, t0.Add(at))
			if err == nil && (l.Holder != holder || !l.Expires.Equal(t0.Add(exp))) {
				return fmt.Errorf("lease %+v, want %s until t0+%s", l, holder, exp)
			}
			return err
		}
	}
	for _, st := range []struct {
		step   string
		call   func() error
		want   error
		names  string
		holder string
		exp    time.Duration
	}{
		{"step 1", acq("h1", 0, hr), nil, "", "h1", hr},
		{"step 2", acq("h2", 0, hr), ErrHeld, "h1", "h1", hr},
		{"step 3", acq("h1", 30*m, 90*m), nil, "", "h1", 90 * m},
		{"step 4", acq("h2", 90*m, 150*m), nil, "", "h2", 150 * m},
		{"step 5", func() error { return Renew(d, "brain", "h1", hr, t0.Add(91*m)) }, ErrHeld, "", "h2", 150 * m},
		{"takeover by h1", func() error { return Takeover(d, "brain", "h1", "h3", hr, t0.Add(92*m)) }, ErrHeld, "brain is not held by h1", "h2", 150 * m},
		{"renew by h2", func() error { return Renew(d, "brain", "h2", hr, t0.Add(100*m)) }, nil, "", "h2", 160 * m},
		{"step 6", func() error { return Release(d, "brain", "h1") }, ErrHeld, "", "h2", 160 * m},
		{"takeover h2 to h2", func() error { return Takeover(d, "brain", "h2", "h2", 2*hr, t0.Add(101*m)) }, nil, "", "h2", 221 * m},
		{"step 7 first release", func() error { return Release(d, "brain", "h2") }, nil, "", "", 0},
		{"step 7 second release", func() error { return Release(d, "brain", "h2") }, nil, "", "", 0},
	} {
		err := st.call()
		if !errors.Is(err, st.want) || (err != nil && !strings.Contains(err.Error(), st.names)) {
			t.Fatalf("%s: err %v, want %v naming %q", st.step, err, st.want, st.names)
		}
		l, ok, rerr := Read(d, "brain")
		if rerr != nil || ok != (st.holder != "") || l.Holder != st.holder || (ok && !l.Expires.Equal(t0.Add(st.exp))) {
			t.Fatalf("%s: Read = %+v, %v, %v, want %q until t0+%s", st.step, l, ok, rerr, st.holder, st.exp)
		}
	}
}

// A bad name, an empty holder, or a TTL of 0 is an error that is not
// ErrHeld, and it leaves the held lease as it was. The bad name holds a
// slash, so a refusal that came after the mutex would leave a folder.
func TestLeaseValidation(t *testing.T) {
	d, hr := t.TempDir(), time.Hour
	if _, err := Acquire(d, "brain", "h1", hr, testNow); err != nil {
		t.Fatal(err)
	}
	for name, call := range map[string]func() error{
		"acquire bad name":     func() error { _, err := Acquire(d, "a/b", "h1", hr, testNow); return err },
		"acquire empty holder": func() error { _, err := Acquire(d, "brain", "", hr, testNow); return err },
		"acquire zero ttl":     func() error { _, err := Acquire(d, "brain", "h1", 0, testNow); return err },
		"renew bad name":       func() error { return Renew(d, "a/b", "h1", hr, testNow) },
		"renew empty holder":   func() error { return Renew(d, "brain", "", hr, testNow) },
		"renew zero ttl":       func() error { return Renew(d, "brain", "h1", 0, testNow) },
		"release bad name":     func() error { return Release(d, "a/b", "h1") },
		"release empty holder": func() error { return Release(d, "brain", "") },
		"takeover bad name":    func() error { return Takeover(d, "a/b", "h1", "h2", hr, testNow) },
		"takeover empty to":    func() error { return Takeover(d, "brain", "h1", "", hr, testNow) },
		"takeover zero ttl":    func() error { return Takeover(d, "brain", "h1", "h2", 0, testNow) },
	} {
		if err := call(); err == nil || errors.Is(err, ErrHeld) {
			t.Errorf("%s: err %v, want a validation error", name, err)
		}
	}
	l, ok, err := Read(d, "brain")
	if err != nil || !ok || l.Holder != "h1" || !l.Expires.Equal(testNow.Add(hr)) {
		t.Fatalf("lease changed: %+v, %v, %v", l, ok, err)
	}
	if ents, err := os.ReadDir(d); err != nil || len(ents) != 1 {
		t.Fatalf("dir holds %d entries (err %v), want only brain.json", len(ents), err)
	}
}

// A lease file that fails the strict decode is an error for Acquire, Renew,
// Release, and Takeover, not ErrHeld, and the file stays as it was.
func TestLeaseReadErrors(t *testing.T) {
	d, hr := t.TempDir(), time.Hour
	p := filepath.Join(d, "brain.json")
	body := `{"name":"brain","holder":"h1","pid":7,"expires":"2026-11-01T10:00:00Z"}`
	for name, call := range map[string]func() error{
		"acquire": func() error { _, err := Acquire(d, "brain", "h2", hr, testNow); return err },
		"renew":   func() error { return Renew(d, "brain", "h1", hr, testNow) },
		"release": func() error { return Release(d, "brain", "h1") },
		"handoff": func() error { return Takeover(d, "brain", "h1", "h2", hr, testNow) },
	} {
		if err := os.WriteFile(p, []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
		if err := call(); err == nil || errors.Is(err, ErrHeld) {
			t.Errorf("%s: err %v, want the read error", name, err)
		}
		if b, err := os.ReadFile(p); err != nil || string(b) != body {
			t.Errorf("%s: lease file changed (err %v)", name, err)
		}
	}
}

// A takeover moves the lease from d1 only while d1 holds it unexpired. Each
// row starts on a fresh dir, with or without Acquire(d, "dev", "d1", 1h, t0),
// and Read then gives the holder and the expiry (as an offset from t0) that
// the row names. Every refusal leaves the dir as it was.
func TestTakeoverNeedsLiveOldHolder(t *testing.T) {
	t0, hr, m := testNow, time.Hour, time.Minute
	for _, tc := range []struct {
		row      string
		acquire  bool
		from, to string
		at       time.Duration
		held     bool
		want     string
		holder   string
		exp      time.Duration
		files    int
	}{
		{"row 1", true, "d1", "d2", 30 * m, false, "", "d2", 90 * m, 1},
		{"row 1b", true, "d1", "d2", hr - time.Nanosecond, false, "", "d2", 2*hr - time.Nanosecond, 1},
		{"row 2", true, "d1", "d2", hr, true, "lock held: dev is not held by d1", "d1", hr, 1},
		{"row 3", true, "d3", "d2", 0, true, "lock held: dev is not held by d3", "d1", hr, 1},
		{"row 2b", true, "d1", "d1", hr, true, "lock held: dev is not held by d1", "d1", hr, 1},
		{"row 3b", true, "d3", "d1", 30 * m, true, "lock held: dev is not held by d3", "d1", hr, 1},
		{"row 3c", true, "d3", "d3", 0, true, "lock held: dev is not held by d3", "d1", hr, 1},
		{"row 4", false, "d1", "d2", 0, true, "lock held: dev is not held by d1", "", 0, 0},
		{"row 5", false, "d1", "", 0, false, "lock holder is empty", "", 0, 0},
	} {
		d := t.TempDir()
		if tc.acquire {
			if _, err := Acquire(d, "dev", "d1", hr, t0); err != nil {
				t.Fatal(err)
			}
		}
		err := Takeover(d, "dev", tc.from, tc.to, hr, t0.Add(tc.at))
		got := ""
		if err != nil {
			got = err.Error()
		}
		if got != tc.want || errors.Is(err, ErrHeld) != tc.held {
			t.Errorf("%s: err %v, want %q (ErrHeld %v)", tc.row, err, tc.want, tc.held)
		}
		l, ok, rerr := Read(d, "dev")
		if rerr != nil || ok != (tc.holder != "") || l.Holder != tc.holder || (ok && !l.Expires.Equal(t0.Add(tc.exp))) {
			t.Errorf("%s: Read = %+v, %v, %v, want %q until t0+%s", tc.row, l, ok, rerr, tc.holder, tc.exp)
		}
		if ents, err := os.ReadDir(d); err != nil || len(ents) != tc.files {
			t.Errorf("%s: dir holds %d entries (err %v), want %d", tc.row, len(ents), err, tc.files)
		}
	}
}

// Takeover runs under the mutex of the lease: for brain and for dev, a
// 9.5-second-old <name>.mutex holds it until that mutex turns stale.
func TestTakeoverWaitsForMutex(t *testing.T) {
	d := t.TempDir()
	for _, name := range []string{"brain", "dev"} {
		if _, err := Acquire(d, name, "d1", time.Hour, testNow); err != nil {
			t.Fatal(err)
		}
		ageMutex(t, filepath.Join(d, name+".mutex"), 9500*time.Millisecond)
		start := time.Now()
		if err := Takeover(d, name, "d1", "d2", time.Hour, testNow); err != nil || time.Since(start) < 200*time.Millisecond {
			t.Fatalf("%s: err %v after %s, want nil after the mutex turns stale", name, err, time.Since(start))
		}
	}
}
