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

var taskLine = regexp.MustCompile(`(?m)^Task: ([A-Z]+[0-9]+)\r?$`) // a whole line, as in internal/merge

type commit struct{ sha, parents, subject, msg string }

// commits lists the commits of a revision range, oldest first.
func commits(repo string, rng ...string) ([]commit, error) {
	args := append([]string{"log", "-z", "--reverse", "--format=%H%x01%P%x01%s%x01%B"}, rng...)
	out, err := gitx.Run(repo, append(args, "--")...)
	if err != nil {
		return nil, err
	}
	var cs []commit
	for _, rec := range strings.Split(out, "\x00") {
		if f := strings.SplitN(rec, "\x01", 4); len(f) == 4 {
			cs = append(cs, commit{sha: f[0], parents: f[1], subject: f[2], msg: strings.TrimRight(f[3], "\n")})
		}
	}
	return cs, nil
}

// Landings reads the first-parent chain of base..trunk, oldest first. A landing
// is a commit with one parent and a "Task: <id>" line, the first if there are
// several. Any other commit is returned in stray as "<sha7> <subject>".
func Landings(repo, base, trunk string) (landings []Landing, stray []string, err error) {
	chain, err := commits(repo, "--first-parent", base+".."+trunk)
	if err != nil {
		return nil, nil, err
	}
	for _, c := range chain {
		parents := strings.Fields(c.parents)
		m := taskLine.FindStringSubmatch(c.msg)
		if len(parents) != 1 || m == nil {
			stray = append(stray, c.sha[:7]+" "+c.subject)
			continue
		}
		landings = append(landings, Landing{ID: m[1], Commit: c.sha, Parent: parents[0], Tip: c.sha, Subject: c.subject, Message: c.msg})
	}
	return landings, stray, nil
}
