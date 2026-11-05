package exit

import (
	"errors"
	"fmt"
	"os"
	"strings"
	"testing"
)

// go.mod fixes the module path and the go version (PLAN-A1).
func TestGoModPinsModuleAndVersion(t *testing.T) {
	mod, err := os.ReadFile("../../go.mod")
	if err != nil || !strings.HasPrefix(string(mod), "module source\n") || !strings.Contains(string(mod), "\ngo 1.24\n") {
		t.Errorf("go.mod = %q (err %v), want module source and go 1.24", mod, err)
	}
}

// The values are the exit codes of gang (design 5.9), which monet-source shares.
func TestCodesMatchContract(t *testing.T) {
	got := map[string]int{"OK": OK, "User": User, "Refused": Refused, "Conflict": Conflict, "Held": Held, "Usage": Usage, "Pending": Pending}
	want := map[string]int{"OK": 0, "User": 1, "Refused": 2, "Conflict": 3, "Held": 4, "Usage": 64, "Pending": 75}
	for name, code := range want {
		if got[name] != code {
			t.Errorf("%s = %d, want %d", name, got[name], code)
		}
	}
}

func TestNewSurvivesWrapping(t *testing.T) {
	err := fmt.Errorf("charter seal: %w", New(Conflict, "charter changed at %s", "abc123"))
	var e *Error
	if !errors.As(err, &e) {
		t.Fatalf("errors.As found no *Error in %v", err)
	}
	if e.Code != Conflict || e.Msg != "charter changed at abc123" || e.Error() != e.Msg {
		t.Errorf("got code %d msg %q error %q, want 3 %q", e.Code, e.Msg, e.Error(), "charter changed at abc123")
	}
	if got := New(Held, "held").Code; got != Held {
		t.Errorf("New(Held, ...).Code = %d, want %d", got, Held)
	}
}
