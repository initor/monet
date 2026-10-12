package exit

import (
	"errors"
	"fmt"
	"testing"
)

// The values come from Contract 2 of the overview plan.
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
	err := fmt.Errorf("plan commit: %w", New(Conflict, "trunk moved at %s", "abc123"))
	var e *Error
	if !errors.As(err, &e) {
		t.Fatalf("errors.As found no *Error in %v", err)
	}
	if e.Code != Conflict || e.Error() != "trunk moved at abc123" {
		t.Errorf("got code %d msg %q, want 3 %q", e.Code, e.Error(), "trunk moved at abc123")
	}
}
