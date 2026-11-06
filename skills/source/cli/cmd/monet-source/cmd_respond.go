package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"strings"

	"source/internal/exit"
	"source/internal/round"
	"source/internal/state"
	"source/internal/store"
)

func init() { commands["respond"] = cmdRespond }

// answer is the answer of the developer to one finding (R22).
type answer struct {
	Finding string `json:"finding"`
	Answer  string `json:"answer"`
	Text    string `json:"text"`
}

// response is the stdin of respond.
type response struct {
	Answers []answer `json:"answers"`
}

// answerSet is the literal set of answer values.
var answerSet = map[string]bool{"fixed": true, "evidence": true, "declined": true}

// decodeStdin decodes one JSON value of c.Stdin into v, with unknown fields
// refused and no data after the value. Each failure is exit 2.
func decodeStdin(c *Ctx, v any) error {
	dec := json.NewDecoder(c.Stdin)
	dec.DisallowUnknownFields()
	if err := dec.Decode(v); err != nil {
		return exit.New(exit.Refused, "stdin: %v", err)
	}
	if err := dec.Decode(&json.RawMessage{}); !errors.Is(err, io.EOF) {
		return exit.New(exit.Refused, "stdin: trailing data after the JSON value")
	}
	return nil
}

// cmdRespond renews dev, then under the state mutex needs a closed round with no response.md, a strict
// stdin, and each answer in answerSet. Each refusal is exit 2 and writes nothing. It writes
// rounds/<n>/response.md once and records RESPONSE <n>.
func cmdRespond(c *Ctx, args []string) error {
	fs := newFlagSet("respond")
	holder := fs.String("holder", "", "the dev lease holder")
	if pos, err := parseFlags(fs, args); err != nil {
		return err
	} else if len(pos) != 0 {
		return exit.New(exit.Usage, "usage: monet-source respond --holder <session>")
	}
	if err := requireLease(c, "dev", *holder); err != nil {
		return err
	}
	d := store.Dir{Root: c.Run}
	var n int
	err := state.Update(d, func(s *state.State) error {
		n = s.Round
		p := round.Path(c.Run, n, "response.md")
		if _, err := os.Lstat(p); n == 0 || s.RoundOpen {
			return exit.New(exit.Refused, "respond: no round is closed")
		} else if err == nil {
			return exit.New(exit.Refused, "round %d already has a response", n)
		} else if !errors.Is(err, os.ErrNotExist) {
			return err
		}
		var r response
		if err := decodeStdin(c, &r); err != nil {
			return err
		}
		var b strings.Builder
		fmt.Fprintf(&b, "# Response to round %d\n", n)
		for _, a := range r.Answers {
			if !answerSet[a.Answer] {
				return exit.New(exit.Refused, "answer %s: want fixed, evidence, or declined", a.Finding)
			}
			fmt.Fprintf(&b, "\n## %s: %s\n\n%s\n", a.Finding, a.Answer, a.Text)
		}
		return store.WriteFileAtomic(p, []byte(b.String()), 0o644)
	})
	if err == nil {
		err = record(c, d, fmt.Sprintf("RESPONSE %d", n))
	}
	return err
}
