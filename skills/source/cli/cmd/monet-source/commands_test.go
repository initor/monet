package main

import (
	"os"
	"path/filepath"
	"slices"
	"sort"
	"strings"
	"testing"
)

// testdata/commands.txt lists each command path that the skill text may write
// after monet-source, one per line and sorted. It must equal the table: each
// plain command, each group subcommand, and the inline "report done".
func TestCommandsFileMatchesTable(t *testing.T) {
	want := []string{"report " + reportDoneWord}
	for name := range commands {
		subs, ok := groups[name]
		if !ok {
			want = append(want, name)
		}
		for sub := range subs {
			want = append(want, name+" "+sub)
		}
	}
	sort.Strings(want)
	raw, err := os.ReadFile(filepath.Join("..", "..", "testdata", "commands.txt"))
	if err != nil {
		t.Fatal(err)
	}
	got := strings.Split(strings.TrimSuffix(string(raw), "\n"), "\n")
	if !slices.Equal(got, want) {
		t.Fatalf("commands.txt:\n%s\nwant:\n%s", strings.Join(got, "\n"), strings.Join(want, "\n"))
	}
}

// groups holds the map that subcommands got, so a subcommand added later shows.
func TestSubcommandsRecordsTheMap(t *testing.T) {
	subs := map[string]func(c *Ctx, args []string) error{}
	subcommands("t-group", subs)
	t.Cleanup(func() { delete(groups, "t-group") })
	subs["late"] = nil
	if _, ok := groups["t-group"]["late"]; !ok {
		t.Fatalf("groups[t-group] is %v, not the map that subcommands got", groups["t-group"])
	}
}
