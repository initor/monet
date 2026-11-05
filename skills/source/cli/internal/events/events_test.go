package events

import (
	"os"
	"reflect"
	"sort"
	"strings"
	"testing"
	"time"

	"source/internal/store"
)

// The route table is the step 1 table of design 5.9, key by key and role by role.
func TestRoutesMatchTheDesign(t *testing.T) {
	want := []string{"CLEARED", "DELIVERED", "DEV_RELAYED", "DEV_SILENT", "FINDINGS", "ITEM",
		"PHASE", "REPORT_DUE", "RESPONSE", "RULING", "VERSION", "WALL"}
	got := make([]string, 0, len(Routes))
	for k := range Routes {
		got = append(got, k)
	}
	sort.Strings(got)
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("routes %v, want %v (SHIP_OK comes with step 2)", got, want)
	}
	roles := map[string][]string{"brain": {"VERSION", "RESPONSE", "ITEM", "DEV_RELAYED", "DEV_SILENT", "REPORT_DUE"},
		"dev": {"FINDINGS", "RULING"}, "brain dev": {"PHASE", "WALL", "CLEARED", "DELIVERED"}}
	for r, keys := range roles {
		for _, k := range keys {
			if g := strings.Join(Routes[k], " "); g != r {
				t.Errorf("Routes[%s] = %q, want %q", k, g, r)
			}
		}
	}
}

// Emit appends a line once, with the time in RFC 3339 UTC with nanoseconds, and compares the stored (sanitized) key.
func TestEmitOncePerKey(t *testing.T) {
	d := store.Dir{Root: t.TempDir()}
	now := time.Date(2026, 11, 4, 9, 30, 0, 123456789, time.FixedZone("PST", -8*3600))
	for i, want := range []bool{true, false} {
		if ok, err := Emit(d, "VERSION research v1", now); err != nil || ok != want {
			t.Fatalf("Emit %d = %v, %v, want %v", i, ok, err, want)
		}
	}
	rows, err := store.ReadTSV(d.Events(), Header)
	if err != nil || !reflect.DeepEqual(rows, [][]string{{"VERSION research v1", "2026-11-04T17:30:00.123456789Z"}}) {
		t.Fatalf("rows %q, %v", rows, err)
	}
	if data, err := os.ReadFile(d.Events()); err != nil || !strings.HasPrefix(string(data), "key\tat\n") {
		t.Fatalf("events.tsv %q, %v, want the header key<TAB>at", data, err)
	}
	for i, want := range []bool{true, false} {
		if ok, err := Emit(d, "ITEM a\tb", now); err != nil || ok != want {
			t.Fatalf("Emit tab %d = %v, %v, want %v", i, ok, err, want)
		}
	}
	if _, err := os.Stat(d.Path("locks", "events.mutex")); !os.IsNotExist(err) {
		t.Errorf("events.mutex left behind: %v", err)
	}
}

// Emit waits while another writer holds <run>/locks/events.mutex.
func TestEmitWaitsForTheMutex(t *testing.T) {
	d := store.Dir{Root: t.TempDir()}
	mu := d.Path("locks", "events.mutex")
	if os.MkdirAll(d.Locks(), 0o755) != nil || os.WriteFile(mu, []byte("1\n"), 0o644) != nil {
		t.Fatal("cannot plant the mutex")
	}
	done := make(chan error, 1)
	go func() { _, err := Emit(d, "WALL x", time.Now()); done <- err }()
	time.Sleep(100 * time.Millisecond)
	if _, err := os.Stat(d.Events()); !os.IsNotExist(err) {
		t.Fatalf("Emit wrote events.tsv while the mutex was held: %v", err)
	}
	os.Remove(mu)
	if err := <-done; err != nil {
		t.Fatal(err)
	}
}

// Emit and For report a corrupt events.tsv, and Emit reports false when the append fails.
func TestEmitAndForReportErrors(t *testing.T) {
	bad, ro := store.Dir{Root: t.TempDir()}, store.Dir{Root: t.TempDir()}
	if os.WriteFile(bad.Events(), []byte("not a header\n"), 0o644) != nil || os.WriteFile(ro.Events(), []byte("key\tat\n"), 0o444) != nil {
		t.Fatal("cannot write the fixtures")
	}
	for _, d := range []store.Dir{bad, ro} {
		if ok, err := Emit(d, "WALL x", time.Now()); ok || err == nil {
			t.Errorf("Emit in %s = %v, %v, want false and an error", d.Root, ok, err)
		}
	}
	if _, _, err := For(bad, "dev", 0); err == nil {
		t.Error("For on a bad header gave no error")
	}
}

// For gives the lines from row index from that route to role, and the row count as next.
func TestForRole(t *testing.T) {
	d := store.Dir{Root: t.TempDir()}
	now := time.Now()
	for _, l := range []string{"VERSION research v1", "FINDINGS 1 FIX"} {
		if _, err := Emit(d, l, now); err != nil {
			t.Fatal(err)
		}
	}
	for _, tc := range []struct {
		role string
		from int
		want []string
	}{
		{"brain", 0, []string{"VERSION research v1"}}, {"dev", 0, []string{"FINDINGS 1 FIX"}},
		{"brain", 1, nil}, {"dev", 1, []string{"FINDINGS 1 FIX"}}, {"dev", 2, nil},
	} {
		lines, next, err := For(d, tc.role, tc.from)
		if err != nil || next != 2 || !reflect.DeepEqual(lines, tc.want) {
			t.Errorf("For(%s, %d) = %q, %d, %v, want %q, 2", tc.role, tc.from, lines, next, err, tc.want)
		}
	}
	if _, err := Emit(d, "PHASE build", now); err != nil {
		t.Fatal(err)
	}
	for _, role := range []string{"brain", "dev"} {
		if lines, next, err := For(d, role, 2); err != nil || next != 3 || !reflect.DeepEqual(lines, []string{"PHASE build"}) {
			t.Errorf("For(%s, 2) = %q, %d, %v, want PHASE build, 3", role, lines, next, err)
		}
	}
	for _, tc := range []struct {
		role string
		from int
		want []string
	}{{"owner", 0, []string{`"owner"`, "brain", "dev"}}, {"dev", -1, []string{"-1", "0 to 3"}}, {"dev", 4, []string{"4", "0 to 3"}}} {
		_, _, err := For(d, tc.role, tc.from)
		for _, w := range tc.want {
			if err == nil || !strings.Contains(err.Error(), w) {
				t.Errorf("For(%s, %d) error %v, want it to name %s", tc.role, tc.from, err, w)
			}
		}
	}
}
