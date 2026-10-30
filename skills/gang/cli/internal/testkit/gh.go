package testkit

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// fakeGH logs each call and answers from files in $GH_FAKE_DIR. A missing
// answer file makes the call fail, like a gh error. It uses absolute paths for
// cat and cp so it runs under a PATH that holds only the fake bin dir.
const fakeGH = `#!/bin/sh
printf '%s\n' "$*" >> "$GH_FAKE_DIR/calls.log"
if [ "$1 $2" = "pr create" ] || [ "$1 $2" = "pr merge" ]; then
  prev=""
  for a in "$@"; do
    if [ "$prev" = "--body-file" ]; then /bin/cp "$a" "$GH_FAKE_DIR/body.md"; fi
    prev="$a"
  done
fi
case "$*" in
  "api user"*) /bin/cat "$GH_FAKE_DIR/login" ;;
  "pr view"*mergeCommit*) /bin/cat "$GH_FAKE_DIR/merge-commit" ;;
  "pr view"*) /bin/cat "$GH_FAKE_DIR/pr-view.json" ;;
  "pr create"*) /bin/cat "$GH_FAKE_DIR/pr-create" ;;
  "pr merge"*) /bin/cat "$GH_FAKE_DIR/pr-merge" ;;
  "api --hostname"*"/statuses/"*) /bin/cat "$GH_FAKE_DIR/status" ;;
  *) echo "fake gh: unexpected call: $*" >&2; exit 1 ;;
esac
`

// FakeGH puts a fake gh first on PATH. It returns the answer dir (also
// $GH_FAKE_DIR) and the bin dir that holds the script.
func FakeGH(t *testing.T) (state, bin string) {
	t.Helper()
	state, bin = t.TempDir(), t.TempDir()
	if err := os.WriteFile(filepath.Join(bin, "gh"), []byte(fakeGH), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("GH_FAKE_DIR", state)
	t.Setenv("PATH", bin+string(os.PathListSeparator)+os.Getenv("PATH"))
	return state, bin
}

// Answer sets the fake gh output for one answer file (login, pr-view.json,
// merge-commit, pr-create, pr-merge, status).
func Answer(t *testing.T, state, name, body string) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(state, name), []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
}

// Calls returns the fake gh calls so far, one argument string each.
func Calls(t *testing.T, state string) []string {
	t.Helper()
	b, err := os.ReadFile(filepath.Join(state, "calls.log"))
	if os.IsNotExist(err) {
		return nil
	}
	if err != nil {
		t.Fatal(err)
	}
	return strings.Split(strings.TrimRight(string(b), "\n"), "\n")
}
