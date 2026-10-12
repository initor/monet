package testkit

import (
	"testing"
)

// GangMerge lands tip on the integration branch the way gang merge does
// (Contract 6): a merge commit with parents (trunk, tip), the merge-tree
// result as its tree, and the message "merge <id>: <title>". It returns the
// merge commit.
func GangMerge(t *testing.T, repo, integ, id, title, tip string) string {
	t.Helper()
	trunk := Git(t, repo, "rev-parse", "refs/heads/"+integ)
	tree := Git(t, repo, "merge-tree", "--write-tree", trunk, tip)
	merge := Git(t, repo, "commit-tree", tree, "-p", trunk, "-p", tip, "-m", "merge "+id+": "+title)
	Git(t, repo, "update-ref", "refs/heads/"+integ, merge)
	return merge
}

// MergeTask commits files on gang/<id>, cut from the integration tip, with a
// "Task: <id>" line, then lands it with GangMerge. It returns the task tip.
func MergeTask(t *testing.T, repo, integ, id string, files map[string]string) string {
	t.Helper()
	trunk := Git(t, repo, "rev-parse", "refs/heads/"+integ)
	Git(t, repo, "update-ref", "refs/heads/gang/"+id, trunk)
	tip := CommitOn(t, repo, "gang/"+id, id+": work\n\nTask: "+id+"\n\nTesting Done: unit tests", files)
	GangMerge(t, repo, integ, id, "Build "+id, tip)
	return tip
}
