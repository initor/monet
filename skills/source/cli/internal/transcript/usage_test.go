package transcript

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"slices"
	"strings"
	"testing"

	"source/internal/testkit"
)

func TestLastUsageAcrossTailSteps(t *testing.T) {
	if want := []int64{2 << 20, 8 << 20, 32 << 20}; !slices.Equal(tailSteps, want) || ErrNoUsage.Error() != "no assistant record with usage" {
		t.Fatalf("tailSteps %v, ErrNoUsage %q; want %v and the acceptance text", tailSteps, ErrNoUsage, want)
	}
	old := tailSteps
	tailSteps = []int64{64, 256, 1024}
	t.Cleanup(func() { tailSteps = old })
	good := jsonLine(t, testkit.Assistant(5, 0, 0))
	pad := `{"pad": "` + strings.Repeat("p", 1100) + `"}`
	nine := jsonLine(t, testkit.Assistant(9, 0, 0))
	for k := 0; k <= 400; k++ {
		// The trap is not one JSON value, but its tail is a record of 9 once a window cuts it.
		trap := `{"type": "user", "n": "` + strings.Repeat("x", k) + `"} ` + nine
		path := writeLines(t, good, pad, trap)
		if got, err := LastUsage(path); err != nil || got != 5 {
			t.Fatalf("k %d: got %d, err %v", k, got, err)
		}
		// A last line of k bytes moves each window cut across the trap, onto the record of 9 too.
		path = writeLines(t, good, pad, `{"type": "user"} `+nine, `{"n": "`+strings.Repeat("y", k)+`"}`)
		if got, err := LastUsage(path); err != nil || got != 5 {
			t.Fatalf("k %d with a last line: got %d, err %v", k, got, err)
		}
	}
	// The newest of 20 records sits inside the 256-byte tail window, so a read of the head or the middle finds an older one.
	var many []string
	for v := 1; v <= 20; v++ {
		many = append(many, jsonLine(t, testkit.Assistant(v, 0, 0)))
	}
	if got, err := LastUsage(writeLines(t, many...)); err != nil || got != 20 {
		t.Fatalf("20 records: got %d, err %v; want 20", got, err)
	}
	// In a file of 257 bytes the 256-byte window starts at offset 1, so it must drop its cut first line too.
	first := "x" + nine
	body := first + "\n" + `{"n": "` + strings.Repeat("y", 257-len(first)-11) + `"}` + "\n"
	if got, err := LastUsage(writeRaw(t, body)); len(body) != 257 || !errors.Is(err, ErrNoUsage) {
		t.Fatalf("offset 1 (size %d): got %d, err %v; want ErrNoUsage", len(body), got, err)
	}
	// A last line with no newline holds every tail window, so each window is one cut line, though its 256-byte tail decodes.
	last := `{"type": "user", "n": "` + strings.Repeat("x", 1100) + `"}` + strings.Repeat(" ", 300) + nine
	if got, err := LastUsage(writeRaw(t, good+"\n"+last)); err != nil || got != 5 {
		t.Fatalf("a window with no newline: got %d, err %v; want 5", got, err)
	}
	// The newest record has no trailing newline and lies in the 256-byte window, so each window must end at the last byte.
	tail := pad + "\n" + jsonLine(t, testkit.Assistant(7, 0, 0)) + "\n" + jsonLine(t, testkit.Assistant(4, 0, 0))
	if got, err := LastUsage(writeRaw(t, tail)); len(tail) <= 1024 || err != nil || got != 4 {
		t.Fatalf("no newline after the newest record (size %d): got %d, err %v; want 4", len(tail), got, err)
	}
	// Each tail window cuts the last line inside its spaces, where the cut tail is a compact record of 9.
	compact := `{"type":"assistant","message":{"usage":{"input_tokens":9}}}`
	if got, err := LastUsage(writeLines(t, good, `{"type": "user"}`+strings.Repeat(" ", 1100)+compact)); err != nil || got != 5 {
		t.Fatalf("every window cuts a line whose tail decodes: got %d, err %v; want 5", got, err)
	}
}

func TestLastUsageRule(t *testing.T) {
	older := jsonLine(t, testkit.Assistant(7, 0, 0))
	usages := []struct {
		present    bool
		in, cw, cr int
	}{{false, 0, 0, 0}, {true, 0, 0, 0}, {true, 1, 0, 0}, {true, 0, 2, 0}, {true, 0, 0, 3}}
	for _, typ := range []string{"assistant", "user", "Assistant", "assistant "} {
		for _, side := range []bool{false, true} {
			for _, u := range usages {
				msg := map[string]any{"model": "m"}
				if u.present {
					msg["usage"] = map[string]any{"input_tokens": u.in, "cache_creation_input_tokens": u.cw, "cache_read_input_tokens": u.cr}
				}
				line := jsonLine(t, map[string]any{"type": typ, "isSidechain": side, "message": msg})
				want, sum := 7, u.in+u.cw+u.cr
				if typ == "assistant" && !side && u.present && sum > 0 {
					want = sum
				}
				if got, err := LastUsage(writeLines(t, older, line)); err != nil || got != want {
					t.Errorf("%s: got %d, err %v; want %d", line, got, err, want)
				}
			}
		}
	}
	for _, lines := range [][]string{{}, {`{"type": "assistant"}`, jsonLine(t, testkit.Assistant(0, 0, 0))}} {
		if got, err := LastUsage(writeLines(t, lines...)); !errors.Is(err, ErrNoUsage) {
			t.Errorf("%q: got %d, err %v; want ErrNoUsage", lines, got, err)
		}
	}
	// Valid JSON with a field of the wrong type does not decode into Entry, so it is skipped.
	wrong := `{"type": "assistant", "uuid": 12, "message": {"usage": {"input_tokens": 5}}}`
	if got, err := LastUsage(writeLines(t, older, wrong)); err != nil || got != 7 {
		t.Errorf("%s: got %d, err %v; want 7", wrong, got, err)
	}
	if got, err := LastUsage(writeRaw(t, "")); !errors.Is(err, ErrNoUsage) {
		t.Errorf("empty file: got %d, err %v; want ErrNoUsage", got, err)
	}
	if got, err := LastUsage(writeRaw(t, older+"\n"+jsonLine(t, testkit.Assistant(4, 0, 0)))); err != nil || got != 4 {
		t.Errorf("no trailing newline: got %d, err %v; want 4", got, err)
	}
	missing := filepath.Join(t.TempDir(), "none.jsonl")
	if _, err := LastUsage(missing); err == nil || errors.Is(err, ErrNoUsage) {
		t.Errorf("LastUsage of a missing file: err %v; want a read error", err)
	}
	// A negative sum is not above 0, so the older record counts.
	if got, err := LastUsage(writeLines(t, older, jsonLine(t, testkit.Assistant(2, -3, 0)))); err != nil || got != 7 {
		t.Errorf("negative sum: got %d, err %v; want 7", got, err)
	}
	// A directory opens but cannot be read; one file inside keeps its size above 0 on every file system.
	dir := filepath.Dir(writeLines(t, older))
	if _, err := LastUsage(dir); err == nil || errors.Is(err, ErrNoUsage) {
		t.Errorf("LastUsage of a directory: err %v; want a read error", err)
	}
	// The newest counting line is the fourth of six, with two user lines after it.
	deep := []string{older, jsonLine(t, testkit.Assistant(8, 0, 0)), jsonLine(t, testkit.Assistant(9, 0, 0)), jsonLine(t, testkit.Assistant(10, 0, 0)), `{"type": "user"}`, `{"type": "user"}`}
	if got, err := LastUsage(writeLines(t, deep...)); err != nil || got != 10 {
		t.Errorf("records 7 to 10, then two user lines: got %d, err %v; want 10", got, err)
	}
	// Each line decodes into a fresh Entry, so the isSidechain of a newer line does not stick to an older line that omits it.
	sidechain := jsonLine(t, map[string]any{"type": "assistant", "isSidechain": true, "message": map[string]any{"usage": map[string]any{"input_tokens": 3}}})
	if got, err := LastUsage(writeLines(t, `{"type": "assistant", "message": {"usage": {"input_tokens": 6}}}`, sidechain)); err != nil || got != 6 {
		t.Errorf("a newer sidechain record: got %d, err %v; want 6", got, err)
	}
	// A real window holds thousands of lines, so the split must keep each line apart.
	long := append(slices.Repeat([]string{`{"type": "user"}`}, 10000), jsonLine(t, testkit.Assistant(9, 0, 0)), `{"type": "user"}`)
	if got, err := LastUsage(writeLines(t, long...)); err != nil || got != 9 {
		t.Errorf("10000 user lines, the record of 9, one user line: got %d, err %v; want 9", got, err)
	}
	// Tokens has a value receiver, so a Usage value has the method; distinct counts pin each term of the sum.
	tokens, ok := reflect.TypeOf(Usage{}).MethodByName("Tokens")
	if !ok || tokens.Func.Call([]reflect.Value{reflect.ValueOf(Usage{Input: 1, CacheCreation: 2, CacheRead: 4})})[0].Int() != 7 {
		t.Errorf("Tokens of a Usage value (a method: %v); want a value method that gives 7", ok)
	}
}

// A sparse file above the largest Go slice can only be read through its tail windows.
func TestLastUsageSparseTail(t *testing.T) {
	const size int64 = 1 << 49
	path := filepath.Join(t.TempDir(), "big.jsonl")
	f, err := os.Create(path)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	if err := f.Truncate(size); err != nil {
		t.Skipf("the file system refuses a sparse file of %d bytes: %v", size, err)
	}
	nine := jsonLine(t, testkit.Assistant(9, 0, 0))
	// A filler line of 0, 3 MB, or 9 MB puts the record of 9 in the 2, 8, or 32 MB window; each longer tail covers the last one.
	for _, fill := range []int{0, 3 << 20, 9 << 20} {
		tail := "\n" + nine + "\n" + strings.Repeat("y", fill) + "\n"
		if _, err := f.WriteAt([]byte(tail), size-int64(len(tail))); err != nil {
			t.Fatal(err)
		}
		if got, err := LastUsage(path); err != nil || got != 9 {
			t.Fatalf("sparse file of %d bytes, filler %d: got %d, err %v; want 9", size, fill, got, err)
		}
	}
}

// jsonLine marshals v to one JSON line.
func jsonLine(t *testing.T, v any) string {
	b, err := json.Marshal(v)
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}

// writeLines writes each line and a newline to a temp file and returns its path.
func writeLines(t *testing.T, lines ...string) string {
	return writeRaw(t, strings.Join(lines, "\n")+"\n")
}

// writeRaw writes body to a temp file and returns its path.
func writeRaw(t *testing.T, body string) string {
	path := filepath.Join(t.TempDir(), "t.jsonl")
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	return path
}
