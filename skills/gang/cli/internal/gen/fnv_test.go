package gen

import (
	"encoding/json"
	"hash/fnv"
	"os/exec"
	"strconv"
	"strings"
	"testing"
)

// Known values come from the engine's loop, run in node:
// function fnv(s) { let h = 0x811c9dc5; for (let i = 0; i < s.length; i++) { h ^= s.charCodeAt(i); h = Math.imul(h, 0x01000193) >>> 0 } return h }
func TestFNVKnownValues(t *testing.T) {
	cases := []struct {
		in   string
		want uint32
	}{
		{"", 0x811c9dc5},
		{"a", 0xe40c292c},
		{"foobar", 0xbf9cf968},
		{"\U000000e9", 0x6c0b6c44}, // one UTF-16 unit, two UTF-8 bytes
		{"\U0001F600", 0xcb31c4b8}, // a surrogate pair
		{"T\U000000e9st \U0001F600", 0x274ab436},
	}
	for _, c := range cases {
		if got := FNV(c.in); got != c.want {
			t.Errorf("FNV(%q) = %#x, want %#x", c.in, got, c.want)
		}
	}
}

// For ASCII, UTF-16 units equal bytes, so the standard library must agree.
func TestFNVMatchesStdlibOnASCII(t *testing.T) {
	for _, s := range []string{"a", "T1 gang/T1 - 1", `[{"id":"T1"}]`} {
		h := fnv.New32a()
		h.Write([]byte(s))
		if got, want := FNV(s), h.Sum32(); got != want {
			t.Errorf("FNV(%q) = %#x, stdlib %#x", s, got, want)
		}
	}
}

// The engine computes the checksum in JavaScript. Run the same loop in node.
func TestFNVMatchesNode(t *testing.T) {
	if _, err := exec.LookPath("node"); err != nil {
		t.Skip("node not on PATH")
	}
	inputs := []string{"plain", "caf\U000000e9", "\U0001F600 x \U00004e2d", "tab\tquote\""}
	script := `function fnv(s) { let h = 0x811c9dc5; for (let i = 0; i < s.length; i++) { h ^= s.charCodeAt(i); h = Math.imul(h, 0x01000193) >>> 0 } return h }
for (const s of JSON.parse(process.argv[1])) console.log(fnv(s))`
	arg, err := json.Marshal(inputs)
	if err != nil {
		t.Fatal(err)
	}
	out, err := exec.Command("node", "-e", script, string(arg)).Output()
	if err != nil {
		t.Fatal(err)
	}
	lines := strings.Fields(string(out))
	for i, s := range inputs {
		want, _ := strconv.ParseUint(lines[i], 10, 32)
		if got := FNV(s); uint64(got) != want {
			t.Errorf("FNV(%q) = %d, node %d", s, got, want)
		}
	}
}
