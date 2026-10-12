// Package preflight runs the launch checks of spec section 10.1. It fails closed.
package preflight

import (
	"bytes"
	"errors"
	"fmt"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"syscall"
	"time"

	"gang/internal/gh"
	"gang/internal/gitx"
	"gang/internal/lint"
	"gang/internal/lock"
	"gang/internal/plan"
	"gang/internal/store"
)

// Check is one launch check result. Skip marks a check that does not apply
// (the gh check without repo.gh_account). A skipped check is OK.
type Check struct {
	Name   string
	OK     bool
	Skip   bool
	Detail string
}

// The disk floor and probe. Tests replace freeBytes.
var (
	minFreeBytes uint64 = 10 << 30
	freeBytes           = statfsFree
)

// Run runs every check in order and writes run/cap. The error is for a
// failure to run at all, never for a failed check.
func Run(repo string, d store.Dir, p *plan.Plan, holder string, now time.Time) ([]Check, error) {
	if err := os.MkdirAll(d.Run(), 0o755); err != nil {
		return nil, err
	}
	checks := []Check{
		checkRepo(repo, p),
		checkDriver(d, holder, now),
		checkTools(p),
		checkGH(p),
		checkClean(repo),
		checkIntegration(repo, p),
		checkIgnored(repo, d),
		checkPlan(repo, d, p),
		checkDisk(repo),
		checkWorktrees(repo, p),
	}
	agentCap := max(1, runtime.NumCPU()-2)
	if err := store.WriteFileAtomic(d.Run("cap"), []byte(strconv.Itoa(agentCap)+"\n"), 0o644); err != nil {
		return nil, err
	}
	return checks, nil
}

func pass(name, detail string) Check { return Check{Name: name, OK: true, Detail: detail} }
func fail(name, format string, args ...any) Check {
	return Check{Name: name, Detail: fmt.Sprintf(format, args...)}
}

func checkRepo(repo string, p *plan.Plan) Check {
	wd, err := os.Getwd()
	if err != nil {
		return fail("repo", "%v", err)
	}
	if !samePath(wd, repo) {
		return fail("repo", "cwd %s is not the repo root %s", wd, repo)
	}
	out, err := gitx.Run(repo, "remote", "get-url", "origin")
	if err != nil {
		return fail("repo", "no origin remote")
	}
	origin := strings.TrimSpace(out)
	if !remoteMatches(repo, origin, p.Repo.Remote) {
		return fail("repo", "origin %s does not match repo.remote %s", origin, p.Repo.Remote)
	}
	return pass("repo", origin)
}

// checkDriver takes the driver lease for 12 hours (Contract 0.14). The pid is
// 0 because this CLI call exits at once and a dead pid would free the lease.
func checkDriver(d store.Dir, holder string, now time.Time) Check {
	lease, err := lock.AcquirePID(d.Run("locks"), "driver", holder, 0, 12*time.Hour, now)
	if errors.Is(err, lock.ErrHeld) {
		return fail("driver", "held by another session: %v", err)
	}
	if err != nil {
		return fail("driver", "%v", err)
	}
	return pass("driver", "held by "+holder+" until "+lease.Expires.UTC().Format(time.RFC3339))
}

func checkTools(p *plan.Plan) Check {
	tools := []string{"go", "node", "git"}
	if p.Repo.GHAccount != "" {
		tools = append(tools, "gh")
	}
	var missing []string
	for _, tool := range tools {
		if _, err := exec.LookPath(tool); err != nil {
			missing = append(missing, tool)
		}
	}
	if len(missing) > 0 {
		return fail("tools", "not on PATH: %s", strings.Join(missing, ", "))
	}
	return pass("tools", strings.Join(tools, ", "))
}

// checkGH checks the gh login, or skips without repo.gh_account (Contract 0.9).
func checkGH(p *plan.Plan) Check {
	if p.Repo.GHAccount == "" {
		return Check{Name: "gh", OK: true, Skip: true, Detail: "repo.gh_account is empty"}
	}
	if _, err := exec.LookPath("gh"); err != nil {
		return fail("gh", "gh not on PATH")
	}
	out, err := gh.Run("api", "user", "--jq", ".login")
	if err != nil {
		return fail("gh", "%v", err)
	}
	login := strings.TrimSpace(string(out))
	if !strings.EqualFold(login, p.Repo.GHAccount) {
		return fail("gh", "gh is logged in as %q, the plan wants %q", login, p.Repo.GHAccount)
	}
	return pass("gh", login)
}

// checkClean looks at tracked files only: task worktrees under .claude/worktrees/
// show as untracked, and gang never reads the main checkout's files.
func checkClean(repo string) Check {
	out, err := gitx.Run(repo, "status", "--porcelain", "--untracked-files=no")
	if err != nil {
		return fail("clean checkout", "%v", err)
	}
	if s := strings.TrimSpace(out); s != "" {
		return fail("clean checkout", "uncommitted changes: %s", strings.Join(strings.Fields(s), " "))
	}
	return pass("clean checkout", "no tracked changes")
}

// checkIntegration fails when a worktree has the integration branch checked
// out: gang merge moves the ref with update-ref, and that checkout goes stale.
func checkIntegration(repo string, p *plan.Plan) Check {
	integ, base := p.Repo.Integration, p.Repo.Base
	out, err := gitx.Run(repo, "worktree", "list", "--porcelain")
	if err != nil {
		return fail("integration", "%v", err)
	}
	wt := ""
	for _, l := range strings.Split(out, "\n") {
		if path, ok := strings.CutPrefix(l, "worktree "); ok {
			wt = path
		}
		if l == "branch refs/heads/"+integ {
			return fail("integration", "%s is checked out in %s: switch that checkout to another branch", integ, wt)
		}
	}
	if sha, err := gitx.RevParse(repo, "refs/heads/"+integ); err == nil {
		return pass("integration", integ+" at "+short(strings.TrimSpace(sha)))
	}
	if _, err := gitx.RevParse(repo, "refs/heads/"+base); err != nil {
		return fail("integration", "neither %s nor base %s exists", integ, base)
	}
	if _, err := gitx.Run(repo, "branch", integ, "refs/heads/"+base); err != nil {
		return fail("integration", "create %s from %s: %v", integ, base, err)
	}
	return pass("integration", "created "+integ+" from "+base)
}

// checkIgnored needs the whole .planning/gang/ folder ignored (Contract 0.3).
func checkIgnored(repo string, d store.Dir) Check {
	rel, err := relTo(repo, filepath.Dir(d.Root))
	if err != nil {
		return fail("ignored", "%v", err)
	}
	if _, err := gitx.Run(repo, "check-ignore", "-q", rel); err != nil {
		return fail("ignored", "%s is not git-ignored", rel)
	}
	return pass("ignored", rel)
}

func checkPlan(repo string, d store.Dir, p *plan.Plan) Check {
	if probs := lint.Lint(p, lint.Options{}); len(probs) > 0 {
		return fail("plan", "%d lint problems, first: %+v", len(probs), probs[0])
	}
	disk, err := os.ReadFile(d.Plan())
	if err != nil {
		return fail("plan", "%v", err)
	}
	ref := plan.Ref(p.Slug)
	committed, err := gitx.Show(repo, ref, "plan.json")
	if err != nil {
		return fail("plan", "%s has no plan.json: run gang plan commit", ref)
	}
	if !bytes.Equal(disk, committed) {
		return fail("plan", "plan/plan.json differs from %s: run gang plan commit", ref)
	}
	return pass("plan", "lints clean and matches "+ref)
}

func checkDisk(repo string) Check {
	free, err := freeBytes(repo)
	if err != nil {
		return fail("disk", "%v", err)
	}
	if free < minFreeBytes {
		return fail("disk", "%d GB free, need %d", free>>30, minFreeBytes>>30)
	}
	return pass("disk", fmt.Sprintf("%d GB free", free>>30))
}

func checkWorktrees(repo string, p *plan.Plan) Check {
	out, err := gitx.Run(repo, "worktree", "list", "--porcelain")
	if err != nil {
		return fail("worktrees", "%v", err)
	}
	n := 0
	for _, l := range strings.Split(out, "\n") {
		if strings.HasPrefix(l, "worktree ") {
			n++
		}
	}
	if n >= p.Limits.MaxWorktrees {
		return fail("worktrees", "%d worktrees, limit %d: run gang worktrees", n, p.Limits.MaxWorktrees)
	}
	return pass("worktrees", strconv.Itoa(n))
}

func statfsFree(path string) (uint64, error) {
	var st syscall.Statfs_t
	if err := syscall.Statfs(path, &st); err != nil {
		return 0, err
	}
	return uint64(st.Bavail) * uint64(st.Bsize), nil
}

// remoteMatches is Contract 0.9: repo.remote must equal the normalized origin
// URL. A local-path origin compares as an absolute path (relative to repo).
func remoteMatches(repo, origin, remote string) bool {
	if remote == "" {
		return false
	}
	if filepath.IsAbs(origin) || strings.HasPrefix(origin, "./") || strings.HasPrefix(origin, "../") {
		return absTo(repo, origin) == absTo(repo, remote)
	}
	return normalizeRemote(origin) == normalizeRemote(remote)
}

func absTo(repo, path string) string {
	if !filepath.IsAbs(path) {
		path = filepath.Join(repo, path)
	}
	return filepath.Clean(path)
}

// normalizeRemote drops a scheme, a user@ prefix, and a trailing .git or /,
// and turns host:path into host/path.
func normalizeRemote(raw string) string {
	s := strings.TrimSpace(raw)
	if strings.Contains(s, "://") {
		if u, err := url.Parse(s); err == nil {
			s = u.Hostname() + u.Path
		}
	} else if i := strings.Index(s, ":"); i > 0 && !strings.Contains(s[:i], "/") {
		host := s[:i]
		if j := strings.LastIndex(host, "@"); j >= 0 {
			host = host[j+1:]
		}
		s = host + "/" + strings.TrimPrefix(s[i+1:], "/")
	}
	return strings.TrimSuffix(strings.TrimSuffix(s, "/"), ".git")
}

func samePath(a, b string) bool {
	ra, err1 := filepath.EvalSymlinks(a)
	rb, err2 := filepath.EvalSymlinks(b)
	return err1 == nil && err2 == nil && ra == rb
}

// relTo returns path relative to repo, with symlinks resolved on both sides.
func relTo(repo, path string) (string, error) {
	rr, err := filepath.EvalSymlinks(repo)
	if err != nil {
		return "", err
	}
	dir, err := filepath.EvalSymlinks(filepath.Dir(path))
	if err != nil {
		return "", err
	}
	rel, err := filepath.Rel(rr, filepath.Join(dir, filepath.Base(path)))
	if err != nil || strings.HasPrefix(rel, "..") {
		return "", fmt.Errorf("%s is outside the repo %s", path, repo)
	}
	return filepath.ToSlash(rel), nil
}

func short(sha string) string {
	if len(sha) > 7 {
		return sha[:7]
	}
	return sha
}
