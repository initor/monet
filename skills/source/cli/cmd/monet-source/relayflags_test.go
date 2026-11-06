package main

import (
	"reflect"
	"strings"
	"testing"
)

func TestTokenClass(t *testing.T) {
	plain := "ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz0123456789._=-"
	for b := 1; b < 128; b++ {
		s := "a" + string(rune(b)) + "b"
		want := s
		if !strings.ContainsRune(plain, rune(b)) {
			want = shellQuote(s)
		}
		if got := token(s); got != want {
			t.Fatalf("byte %d: got %q, want %q", b, got, want)
		}
	}
	if got := token(""); got != "''" {
		t.Fatalf("empty: got %q", got)
	}
}

func TestNextName(t *testing.T) {
	for _, c := range []struct{ in, want string }{
		{"source demo dev", "source demo dev 2"},
		{"x 9", "x 10"},
		{"x 2 3", "x 2 4"},
		{"x  9", "x  9 2"},
		{"9", "9 2"},
		{" 3", " 3 2"},
		{"x 09", "x 10"},
		{"x 99999999999999999999", "x 100000000000000000000"},
		{"", " 2"},
		{"fix 2 bugs", "fix 2 bugs 2"},
		{"a\nx 9", "a\nx 9 2"},
	} {
		if got := nextName(c.in); got != c.want {
			t.Errorf("nextName(%q) = %q, want %q", c.in, got, c.want)
		}
	}
}

func TestFlagValue(t *testing.T) {
	for _, c := range []struct {
		flags []string
		want  string
	}{
		{[]string{"--effort", "high"}, "high"},
		{[]string{"--effort=high"}, "high"},
		{[]string{"--effort"}, ""},
		{[]string{"--efforts", "x"}, ""},
		{[]string{"--efforts=x"}, ""},
		{[]string{"--model", "opus", "--effort=low", "--effort"}, "low"},
		{[]string{"--effort=", "x"}, ""},
		{nil, ""},
	} {
		if got := flagValue(c.flags, "--effort"); got != c.want {
			t.Errorf("flagValue(%q) = %q, want %q", c.flags, got, c.want)
		}
	}
}

func TestCarriedFlags(t *testing.T) {
	in := strings.Fields("--model opus --name n --resume r -c --plugin-dir /p --permission-mode=plan --session-id=x --effort high -n")
	want := strings.Fields("--model opus --plugin-dir /p --effort high")
	if got := carriedFlags(in); !reflect.DeepEqual(got, want) {
		t.Fatalf("got %q, want %q", got, want)
	}
	// The lists of relay.mjs:52-53.
	if !reflect.DeepEqual(droppedWithValue, strings.Fields("--name -n --permission-mode --session-id --resume -r")) ||
		!reflect.DeepEqual(droppedAlone, strings.Fields("--continue -c --fork-session --inherit-permission-mode")) {
		t.Fatalf("dropped lists: %q and %q", droppedWithValue, droppedAlone)
	}
	// Each dropped flag goes with its value, alone, or in its joined form,
	// and a longer flag that starts with its name stays.
	for _, f := range droppedWithValue {
		in := []string{f, "v", f + "=v", f + "x", "k"}
		if got := carriedFlags(in); !reflect.DeepEqual(got, []string{f + "x", "k"}) {
			t.Errorf("%s: got %q", f, got)
		}
	}
	for _, f := range droppedAlone {
		in := []string{f, "k", f + "=v", f + "x"}
		if got := carriedFlags(in); !reflect.DeepEqual(got, []string{"k", f + "x"}) {
			t.Errorf("%s: got %q", f, got)
		}
	}
}

func TestModelAlias(t *testing.T) {
	for _, c := range []struct{ in, want string }{
		{"claude-opus-5-5", "opus"},
		{"sonnet", "sonnet"},
		{"claude-sonnet-4", "sonnet"},
		{"claude-haiku", "claude-haiku"},
		{"x", "x"},
		{"claude-fable-1", "fable"},
		{"haiku", "haiku"},
		{"-haiku-opus-", "opus"},
		{"opusx", "opusx"},
	} {
		if got := modelAlias(c.in); got != c.want {
			t.Errorf("modelAlias(%q) = %q, want %q", c.in, got, c.want)
		}
	}
	if want := strings.Fields("opus sonnet haiku fable"); !reflect.DeepEqual(modelAliases, want) {
		t.Fatalf("modelAliases = %q, want %q (relay.mjs:54)", modelAliases, want)
	}
}
