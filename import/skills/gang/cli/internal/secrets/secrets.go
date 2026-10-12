// Package secrets scans the added lines of a unified diff for credentials.
package secrets

import (
	"regexp"
	"strconv"
	"strings"
)

// Finding is one rule match on one added line, by new-side line number.
type Finding struct {
	File string
	Line int
	Rule string
}

var rules = []struct {
	name string
	re   *regexp.Regexp
}{
	{"private-key", regexp.MustCompile(`-----BEGIN [A-Z ]*PRIVATE KEY(?: BLOCK)?-----`)},
	{"aws-access-key", regexp.MustCompile(`(?:AKIA|ASIA)[0-9A-Z]{16}`)},
	{"github-token", regexp.MustCompile(`gh[pousr]_[A-Za-z0-9]{36,}`)},
	{"github-pat", regexp.MustCompile(`github_pat_[A-Za-z0-9_]{50,}`)},
	{"slack-token", regexp.MustCompile(`(?:xox[baprs]|xapp)-[A-Za-z0-9-]{10,}`)},
	{"anthropic-key", regexp.MustCompile(`sk-ant-[A-Za-z0-9_-]{20,}`)},
	{"generic-sk", regexp.MustCompile(`sk-[A-Za-z0-9]{32,}`)},
	{"google-api-key", regexp.MustCompile(`AIza[0-9A-Za-z_-]{35}`)},
	{"openai-key", regexp.MustCompile(`sk-(?:proj|svcacct|admin)-[A-Za-z0-9_-]{20,}`)},
	{"stripe-key", regexp.MustCompile(`[sr]k_live_[A-Za-z0-9]{20,}`)},
	{"gitlab-pat", regexp.MustCompile(`glpat-[A-Za-z0-9_-]{20,}`)},
	{"npm-token", regexp.MustCompile(`npm_[A-Za-z0-9]{36}`)},
}

var hunkRe = regexp.MustCompile(`^@@ -\d+(?:,(\d+))? \+(\d+)(?:,(\d+))? @@`)

// ScanDiff checks every added line of diff (one or more files, as from
// git diff or git log -p). Hunk line counts decide what is content, so an
// added line that starts with "++" is still scanned.
func ScanDiff(diff string) []Finding {
	var out []Finding
	file := ""
	line, oldLeft, newLeft := 0, 0, 0
	for _, l := range strings.Split(diff, "\n") {
		if oldLeft > 0 || newLeft > 0 {
			inHunk := true
			switch {
			case strings.HasPrefix(l, "+"):
				for _, r := range rules {
					if r.re.MatchString(l[1:]) {
						out = append(out, Finding{File: file, Line: line, Rule: r.name})
					}
				}
				line++
				newLeft--
			case strings.HasPrefix(l, "-"):
				oldLeft--
			case strings.HasPrefix(l, " "):
				line++
				oldLeft--
				newLeft--
			case strings.HasPrefix(l, `\`):
			default:
				oldLeft, newLeft, inHunk = 0, 0, false
			}
			if inHunk {
				continue
			}
		}
		switch {
		case strings.HasPrefix(l, "diff --git "):
			file = ""
		case strings.HasPrefix(l, "+++ "):
			file = newPath(strings.TrimPrefix(l, "+++ "))
		case strings.HasPrefix(l, "@@ "):
			if m := hunkRe.FindStringSubmatch(l); m != nil {
				oldLeft = count(m[1])
				line, _ = strconv.Atoi(m[2])
				newLeft = count(m[3])
			}
		}
	}
	return out
}

func newPath(s string) string {
	if s == "/dev/null" {
		return ""
	}
	if u, err := strconv.Unquote(s); err == nil {
		s = u
	}
	return strings.TrimPrefix(s, "b/")
}

// count reads a hunk length. A missing length means 1.
func count(s string) int {
	if s == "" {
		return 1
	}
	n, _ := strconv.Atoi(s)
	return n
}
