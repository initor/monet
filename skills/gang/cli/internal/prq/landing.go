package prq

import (
	"regexp"
	"strings"

	"gang/internal/gitx"
)

// Landing is one task on the trunk, in the order it landed.
type Landing struct {
	ID      string // the task id
	Commit  string // the trunk commit on the first-parent chain
	Parent  string // its first parent: gitx.Replay applies Parent..Commit
	Tip     string // the reviewed task tip: the second parent of a gang merge, or Commit
	Subject string // the subject of the commit that holds the line "Task: <ID>"
	Message string // the full message of that commit
}

var (
	gangMerge = regexp.MustCompile(`^merge ([A-Z]+[0-9]+): `)
	taskLine  = regexp.MustCompile(`(?m)^Task: ([A-Z]+[0-9]+)\r?$`) // a whole line, as in internal/merge
	fixLine   = regexp.MustCompile(`(?m)^Fixes: [A-Z]+[0-9]+\r?$`)  // gang pr fix needs it in the task commit
)

type commit struct{ sha, parents, tree, subject, msg string }

// commits lists the commits of a revision range, oldest first.
func commits(repo string, rng ...string) ([]commit, error) {
	args := append([]string{"log", "-z", "--reverse", "--topo-order", "--format=%H%x01%P%x01%T%x01%s%x01%B"}, rng...)
	out, err := gitx.Run(repo, append(args, "--")...)
	if err != nil {
		return nil, err
	}
	var cs []commit
	for _, rec := range strings.Split(out, "\x00") {
		if f := strings.SplitN(rec, "\x01", 5); len(f) == 5 {
			cs = append(cs, commit{sha: f[0], parents: f[1], tree: f[2], subject: f[3], msg: strings.TrimRight(f[4], "\n")})
		}
	}
	return cs, nil
}

// Landings reads the first-parent chain of base..trunk, oldest first. A landing
// is a commit with one parent and a "Task: <id>" line, the first if there are
// several, or a 0.4.0 gang merge. Any other commit is a stray, as "<sha7> <subject>".
func Landings(repo, base, trunk string) (landings []Landing, stray []string, err error) {
	chain, err := commits(repo, "--first-parent", base+".."+trunk)
	if err != nil {
		return nil, nil, err
	}
	for _, c := range chain {
		l, err := landing(repo, c)
		switch {
		case err != nil:
			return nil, nil, err
		case l.ID == "":
			stray = append(stray, c.sha[:7]+" "+c.subject)
		default:
			landings = append(landings, l)
		}
	}
	return landings, stray, nil
}

// landing reads c as a landing, or returns the zero Landing. A gang merge has
// the subject "merge <id>: ", two parents, and the clean merge-tree of them as
// its tree. Its task commit is the oldest in Parent..Tip with the line "Task: <id>".
func landing(repo string, c commit) (Landing, error) {
	parents := strings.Fields(c.parents)
	if len(parents) == 1 {
		if m := taskLine.FindStringSubmatch(c.msg); m != nil {
			return Landing{ID: m[1], Commit: c.sha, Parent: parents[0], Tip: c.sha, Subject: c.subject, Message: c.msg}, nil
		}
		return Landing{}, nil
	}
	m := gangMerge.FindStringSubmatch(c.subject)
	if len(parents) != 2 || m == nil {
		return Landing{}, nil
	}
	tree, conflicts, err := gitx.MergeTree(repo, parents[0], parents[1])
	if err != nil || len(conflicts) > 0 || tree != c.tree {
		return Landing{}, err
	}
	tasks, err := commits(repo, parents[0]+".."+parents[1])
	if err != nil {
		return Landing{}, err
	}
	for _, t := range tasks {
		for _, l := range taskLine.FindAllStringSubmatch(t.msg, -1) {
			if l[1] == m[1] {
				return Landing{ID: m[1], Commit: c.sha, Parent: parents[0], Tip: parents[1], Subject: t.subject, Message: t.msg}, nil
			}
		}
	}
	return Landing{}, nil
}
