package lock

import (
	"encoding/json"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"sort"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

var t0 = time.Date(2026, 10, 11, 9, 0, 0, 0, time.UTC)

func readLease(t *testing.T, dir, name string) Lease {
	t.Helper()
	b, err := os.ReadFile(filepath.Join(dir, name+".json"))
	if err != nil {
		t.Fatal(err)
	}
	var l Lease
	if err := json.Unmarshal(b, &l); err != nil {
		t.Fatal(err)
	}
	return l
}

// deadPID returns the pid of a process that has exited and been reaped.
func deadPID(t *testing.T) int {
	t.Helper()
	cmd := exec.Command("true")
	if err := cmd.Run(); err != nil {
		t.Fatal(err)
	}
	return cmd.Process.Pid
}

func TestAcquireRefusesLiveHolder(t *testing.T) {
	dir := t.TempDir()
	if _, err := Acquire(dir, "driver", "s1", 5*time.Minute, t0); err != nil {
		t.Fatal(err)
	}
	_, err := Acquire(dir, "driver", "s2", 5*time.Minute, t0.Add(4*time.Minute+59*time.Second))
	if !errors.Is(err, ErrHeld) {
		t.Fatalf("second holder before expiry: err=%v, want ErrHeld", err)
	}
	if got := readLease(t, dir, "driver").Holder; got != "s1" {
		t.Fatalf("holder changed to %s", got)
	}
}

func TestAcquireTakesOverAtExpiry(t *testing.T) {
	dir := t.TempDir()
	if _, err := Acquire(dir, "driver", "s1", 5*time.Minute, t0); err != nil {
		t.Fatal(err)
	}
	l, err := Acquire(dir, "driver", "s2", time.Minute, t0.Add(5*time.Minute))
	if err != nil || l.Holder != "s2" || !l.Expires.Equal(t0.Add(6*time.Minute)) {
		t.Fatalf("takeover at expiry: %+v %v", l, err)
	}
}

func TestAcquireTakesOverFromDeadPIDOnThisHost(t *testing.T) {
	dir := t.TempDir()
	if _, err := AcquirePID(dir, "slot-1", "job-1", deadPID(t), time.Hour, t0); err != nil {
		t.Fatal(err)
	}
	if l, err := Acquire(dir, "slot-1", "job-2", time.Hour, t0); err != nil || l.Holder != "job-2" {
		t.Fatalf("dead pid: %+v %v, want takeover", l, err)
	}
}

func TestAcquireKeepsLivePIDAndOtherHosts(t *testing.T) {
	dir := t.TempDir()
	if _, err := AcquirePID(dir, "live", "job-1", os.Getpid(), time.Hour, t0); err != nil {
		t.Fatal(err)
	}
	if _, err := Acquire(dir, "live", "job-2", time.Hour, t0); !errors.Is(err, ErrHeld) {
		t.Fatalf("live pid: err=%v, want ErrHeld", err)
	}
	far := Lease{Name: "far", Holder: "job-1", PID: deadPID(t), Host: "another-host", Expires: t0.Add(time.Hour)}
	b, _ := json.Marshal(far)
	if err := os.WriteFile(filepath.Join(dir, "far.json"), b, 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := Acquire(dir, "far", "job-2", time.Hour, t0); !errors.Is(err, ErrHeld) {
		t.Fatalf("dead pid on another host: err=%v, want ErrHeld", err)
	}
}

func TestSameHolderRenewsOnAcquire(t *testing.T) {
	dir := t.TempDir()
	if _, err := Acquire(dir, "driver", "s1", time.Minute, t0); err != nil {
		t.Fatal(err)
	}
	l, err := Acquire(dir, "driver", "s1", time.Minute, t0.Add(30*time.Second))
	if err != nil || !l.Expires.Equal(t0.Add(90*time.Second)) {
		t.Fatalf("%+v %v", l, err)
	}
}

func TestRenew(t *testing.T) {
	dir := t.TempDir()
	if err := Renew(dir, "driver", "s1", time.Minute, t0); !errors.Is(err, ErrHeld) {
		t.Fatalf("renew of an absent lease: %v, want ErrHeld", err)
	}
	if _, err := Acquire(dir, "driver", "s1", time.Minute, t0); err != nil {
		t.Fatal(err)
	}
	if err := Renew(dir, "driver", "s2", time.Minute, t0); !errors.Is(err, ErrHeld) {
		t.Fatalf("renew by a non-holder: %v, want ErrHeld", err)
	}
	if err := Renew(dir, "driver", "s1", 10*time.Minute, t0.Add(time.Minute)); err != nil {
		t.Fatal(err)
	}
	if got := readLease(t, dir, "driver").Expires; !got.Equal(t0.Add(11 * time.Minute)) {
		t.Fatalf("expires %s", got)
	}
}

func TestRelease(t *testing.T) {
	dir := t.TempDir()
	if _, err := Acquire(dir, "driver", "s1", time.Minute, t0); err != nil {
		t.Fatal(err)
	}
	if err := Release(dir, "driver", "s2"); !errors.Is(err, ErrHeld) {
		t.Fatalf("release by a non-holder: %v, want ErrHeld", err)
	}
	if readLease(t, dir, "driver").Holder != "s1" {
		t.Fatal("a non-holder release changed the lease")
	}
	if err := Release(dir, "driver", "s1"); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(dir, "driver.json")); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("lease file still there: %v", err)
	}
	if err := Release(dir, "driver", "s1"); err != nil {
		t.Fatalf("release of a free lock: %v", err)
	}
}

// The file shape is Contract 4: {"name","holder","pid","host","expires"},
// with an RFC 3339 expires.
func TestLeaseFileShape(t *testing.T) {
	dir := t.TempDir()
	if _, err := AcquirePID(dir, "gpu", "job-1", 4242, 5*time.Minute, t0); err != nil {
		t.Fatal(err)
	}
	b, err := os.ReadFile(filepath.Join(dir, "gpu.json"))
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
	if want := []string{"expires", "holder", "host", "name", "pid"}; !reflect.DeepEqual(keys, want) {
		t.Fatalf("keys %v, want %v", keys, want)
	}
	host, _ := os.Hostname()
	exp, err := time.Parse(time.RFC3339, raw["expires"].(string))
	if err != nil || !exp.Equal(t0.Add(5*time.Minute)) || raw["name"] != "gpu" || raw["holder"] != "job-1" || raw["pid"] != float64(4242) || raw["host"] != host {
		t.Fatalf("lease %s (%v)", b, err)
	}
}

func TestRejectsBadInput(t *testing.T) {
	dir := t.TempDir()
	for _, name := range []string{"", "../x", "a/b", ".hidden"} {
		if _, err := Acquire(dir, name, "s1", time.Minute, t0); err == nil {
			t.Errorf("name %q accepted", name)
		}
	}
	if _, err := Acquire(dir, "driver", "", time.Minute, t0); err == nil {
		t.Error("empty holder accepted")
	}
	if _, err := Acquire(dir, "driver", "s1", 0, t0); err == nil {
		t.Error("zero ttl accepted")
	}
}

func TestStaleMutexIsBroken(t *testing.T) {
	dir := t.TempDir()
	m := filepath.Join(dir, "driver.mutex")
	if err := os.WriteFile(m, []byte("999999\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	old := time.Now().Add(-time.Minute)
	if err := os.Chtimes(m, old, old); err != nil {
		t.Fatal(err)
	}
	if _, err := Acquire(dir, "driver", "s1", time.Minute, t0); err != nil {
		t.Fatalf("stale mutex blocked acquire: %v", err)
	}
	if _, err := os.Stat(m); !errors.Is(err, os.ErrNotExist) {
		t.Fatal("mutex file left behind")
	}
}

func TestConcurrentAcquireHasOneWinner(t *testing.T) {
	dir := t.TempDir()
	var wg sync.WaitGroup
	var mu sync.Mutex
	wins := 0
	for i := range 16 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, err := Acquire(dir, "driver", "s"+string(rune('a'+i)), time.Minute, t0)
			if err == nil {
				mu.Lock()
				wins++
				mu.Unlock()
			} else if !errors.Is(err, ErrHeld) {
				t.Error(err)
			}
		}()
	}
	wg.Wait()
	if wins != 1 {
		t.Fatalf("%d winners, want 1", wins)
	}
}

func TestWithMutexSerializes(t *testing.T) {
	path := filepath.Join(t.TempDir(), "count.mutex")
	var n atomic.Int64 // Load and Store apart, so only the file mutex prevents lost updates
	var wg sync.WaitGroup
	for range 20 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if err := WithMutex(path, func() error {
				v := n.Load()
				time.Sleep(time.Millisecond)
				n.Store(v + 1)
				return nil
			}); err != nil {
				t.Error(err)
			}
		}()
	}
	wg.Wait()
	if got := n.Load(); got != 20 {
		t.Fatalf("n = %d, want 20 (lost updates)", got)
	}
}
