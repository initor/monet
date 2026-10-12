---
name: review-code
description: Use when the user asks to review a Go repository, including Kubernetes code, against official conventions (Effective Go, Go Code Review Comments, K8s best practices). Only applies inside a Go project (go.mod present). Writes findings to reviews.md; never modifies the reviewed code.
---

## Overview

Sample a bounded set of source files, run available Go tooling, and check the code against three official convention sets. Report convention-backed findings in `reviews.md` -- never modify the reviewed code.

## When to use

- The user asks to review a Go repository ("review this repo" inside a Go project).
- The target directory contains a `go.mod`.
- Not for non-Go projects -- this skill is Go/K8s-specific and stops if no `go.mod` is found.
- Not for reviewing prose or design docs.

## Quick reference

| Step | Purpose | Output |
|------|---------|--------|
| Pre | Confirm a Go project exists | stop if no `go.mod` |
| 1 | Detect project type (go-generic / k8s-aware / k8s-controller) | `project_classification` block |
| 2 | Sample 5-10 source files (bounded) | files in context |
| 3 | Run go vet / golangci-lint / go test (optional) | tool output + `tools_run` block |
| 4 | Check Effective Go conventions | violations + good patterns |
| 5 | Check Go Code Review Comments | violations + good patterns |
| 6 | Check K8s conventions (k8s-controller only) | violations + good patterns |
| 7 | Classify findings by severity + attach URLs | Critical / Suggestions / Nits |
| 8 | Write the review | `reviews.md` at repo root |

## Preconditions

This skill only applies inside a Go project. Check for `go.mod` in the working directory:

```bash
test -f go.mod && echo "go_mod_found=true" || echo "go_mod_found=false"
```

If `go.mod` is not found:
- Say you could not detect a Go project in the current directory.
- Recommend navigating to the repository root where `go.mod` lives.
- Stop.

## Step 1: Detect project type

Read the first 30 lines of `go.mod` to capture the module path and key dependencies:

```bash
head -30 go.mod
```

Run layered K8s detection:

```bash
# Strong signals (any one = K8s controller):
grep -q 'sigs.k8s.io/controller-runtime' go.mod 2>/dev/null && echo "k8s_controller_runtime=true"
test -f PROJECT && echo "kubebuilder_project=true"

# Medium signals (two or more together = likely K8s controller):
ls controllers/ 2>/dev/null || ls internal/controller/ 2>/dev/null
grep -q 'k8s.io/client-go' go.mod 2>/dev/null && echo "k8s_client_go=true"

# Weak signals (alone = K8s-aware, not K8s controller):
grep -q 'k8s.io/api' go.mod 2>/dev/null && echo "k8s_api=true"
grep -q 'k8s.io/apimachinery' go.mod 2>/dev/null && echo "k8s_apimachinery=true"
```

Classify the project framework using this priority:

- **`k8s-controller`**: Any strong signal present (controller-runtime in go.mod OR PROJECT file exists), OR two or more medium signals present (controllers/ directory + client-go)
- **`k8s-aware`**: Only weak signals detected (k8s.io/api or k8s.io/apimachinery in go.mod, but no controller-runtime, no PROJECT file, fewer than two medium signals). This means the project uses K8s libraries but is not a controller.
- **`go-generic`**: No K8s signals at all

Set confidence:
- `high` if classification is unambiguous (strong signal present, or no K8s signals at all)
- `medium` if classification relies on medium signals only (no strong signals, but two or more medium signals)

Emit a classification block listing all detected signals (omit signals that were not found):

```yaml
project_classification:
  language: go
  framework: k8s-controller | k8s-aware | go-generic
  confidence: high | medium
  signals:
    - "go.mod present"
    - "module: <module path from go.mod>"
    - "controller-runtime in go.mod"    # if found
    - "PROJECT file present"            # if found
    - "controllers/ directory exists"   # if found
    - "client-go in go.mod"             # if found
    - "k8s.io/api in go.mod"            # if found
    - "k8s.io/apimachinery in go.mod"   # if found
```

## Step 2: Sample code for review (bounded)

List Go source files, excluding vendor, generated, and non-project code:

```bash
find . -name '*.go' \
  -not -path './vendor/*' \
  -not -path './.git/*' \
  -not -name 'zz_generated_*' \
  -not -name '*_generated.go' \
  -not -path '*/testdata/*' \
  -not -path '*/third_party/*' \
  | head -50
```

From the listing, prioritize files in this order:

1. `main.go` or files in `cmd/` (entry points)
2. Files in `controllers/` or `internal/controller/` (reconcile loops -- when `framework: k8s-controller`)
3. Files in `api/` or `apis/` (CRD type definitions -- when `framework: k8s-controller` or `k8s-aware`)
4. Files matching `*_types.go` (K8s type definitions)
5. `PROJECT` file (kubebuilder configuration -- when present)
6. `types.go` or files in `pkg/apis/` (general type definitions)
7. Files in `pkg/` or `internal/` (core logic)
8. Any file with recent changes: `git log --oneline -1 -- <file>` (use `|| true` to handle non-git dirs)

When `framework: go-generic`, skip priorities 2-5 (they are K8s-specific) and use only 1, 6, 7, 8.

**Parallelism:** Per-file reading and convention checking can be parallelized -- dispatch one sub-agent per sampled file. Each agent reads its file and checks conventions. Merge all per-file findings before severity classification.

Select the top **5-10 files** from the prioritized list. For this bounded set, prefer reading each file **in full** with the Read tool so findings reference correct line numbers. If a file is very large and you fall back to `head -300 <file>`, note that the review covers only the first 300 lines of that file and may miss issues below the cut.

Also read `go.mod` in full for dependency review:

```bash
cat go.mod
```

If `go.sum` is present, skim its size but do not read it in full:

```bash
wc -l go.sum || true
```

## Step 3: Run Go analysis tools

Run tool detection, linting, and tests. All tools are optional -- skip gracefully when unavailable and continue the review.

### 3a. Tool detection

Detect which tools are available and whether test files exist:

```bash
command -v go >/dev/null 2>&1 && echo "go_available=true" || echo "go_available=false"
command -v golangci-lint >/dev/null 2>&1 && echo "golangci_lint_available=true" || echo "golangci_lint_available=false"
find . -name '*_test.go' -not -path './vendor/*' -not -path './.git/*' | head -1 | grep -q . && echo "test_files_exist=true" || echo "test_files_exist=false"
```

Record the three boolean results for use in 3b, 3c, and 3d.

**Parallelism:** Steps 3b (linter) and 3c (tests) can run as parallel background processes since they have no dependencies on each other. Launch all available tools concurrently and collect their outputs before proceeding to Step 4.

### 3b. Linter execution

Follow this fallback cascade to avoid duplicate findings:

1. **If golangci-lint is available:** Run it from the project root (same directory as `go.mod`). golangci-lint includes `govet` by default, so do NOT run standalone `go vet` when golangci-lint is available.

   ```bash
   golangci-lint run --output.text.colors=false --output.text.print-issued-lines=false ./... 2>&1
   ```

   golangci-lint respects project `.golangci.yml` / `.golangci.yaml` config files automatically -- do not specify which linters to enable.

2. **If golangci-lint is NOT available but go is available:** Run standalone `go vet` as a fallback:

   ```bash
   go vet ./... 2>&1
   ```

3. **If neither tool is available:** Note "No linting tools available -- skipping static analysis" and continue to 3c.

Capture and retain the full linter output for use in Steps 7 and 8.

### 3c. Test execution

Run tests conditionally based on tool availability and test file presence:

1. **If go is available AND test_files_exist=true:** Run tests:

   ```bash
   go test -v -count=1 -timeout 120s ./... 2>&1
   ```

   - `-v`: verbose output showing individual test results
   - `-count=1`: bypasses test cache for fresh results
   - `-timeout 120s`: 2-minute timeout to prevent hanging on slow integration tests

2. **If go is available but no test files:** Note "No test files found" and continue.

3. **If go is not available:** Note "Go toolchain not available -- skipping tests" and continue.

4. **If test output shows `[build failed]`:** Classify this as a Critical finding (the code does not compile) and note that test results are unavailable due to build failure.

Capture and retain the full test output for use in Steps 7 and 8.

### 3d. Tool status summary

Record a status summary for use in Step 8:

```yaml
tools_run:
  golangci_lint: ran | not_available
  go_vet: ran | covered_by_golangci_lint | not_available
  go_test: ran (N passed, M failed of T total) | no_test_files | not_available | build_failed
```

## Step 4: Apply Effective Go conventions

**Parallelism:** Steps 4, 5, and 6 are independent convention checks against the same sampled code. Dispatch them as 3 parallel sub-agents -- one per convention set. Each agent receives the sampled code and its convention checklist, returns findings and good patterns. Merge all results in Step 7.

Read `reference/effective-go.md` for the 22-category Effective Go checklist and apply each category to the sampled code, noting violations and good patterns.

## Step 5: Apply Go Code Review Comments conventions

Read `reference/code-review-comments.md` for the 31-rule Go Code Review Comments checklist and apply each rule to the sampled code (same approach as Step 4).

## Step 6: Apply K8s conventions (when applicable)

**This step only applies when `framework: k8s-controller`.** Skip it entirely for `go-generic` projects.

When `framework: k8s-aware`, do NOT apply the checklist. Instead, emit this note:

> "This project uses K8s libraries (k8s.io/api, k8s.io/apimachinery) but does not appear to be a controller. K8s controller-specific conventions are not applied. If this project is in fact a controller, check that `sigs.k8s.io/controller-runtime` is in go.mod or that a `PROJECT` file exists at the repo root."

For `framework: k8s-controller`, read `reference/k8s-conventions.md` for the 15-item K8s checklist (with its "Why it matters" rationale) and apply each item to the sampled code (same approach as Steps 4 and 5).

## Step 7: Classify findings by severity

Take ALL findings from Steps 3 (tools), 4 (Effective Go), 5 (Code Review Comments), and 6 (K8s conventions) and classify each into one of three severity buckets: **Critical**, **Suggestions**, or **Nits**.

### Parsing tool findings from Step 3 into classifiable entries

**golangci-lint output:** Each output line follows the format `file.go:line:col: message (lintername)`. Parse each line by:
1. Extract `file:line` from the beginning of the line (everything before the second colon-number pair)
2. Extract the linter name from the parenthesized suffix -- look for `(govet)`, `(staticcheck)`, `(errcheck)`, `(ineffassign)`, `(unused)`, `(typecheck)` at the end of each line
3. Use the linter name to classify severity per the tool severity table below
4. Each parsed line becomes one finding entry in the appropriate severity bucket

**Standalone go vet output:** Each line follows `file.go:line:col: message`. Every go vet finding is classified as Critical.

**go test output:** Look for lines starting with `--- FAIL:` to extract failed test names, and `FAIL` lines to extract the package. Each test failure becomes one Critical finding. If output contains `[build failed]`, create a single Critical finding for the build failure.

### Severity classification rules for tool findings

| Source | Finding Type | Severity |
|--------|-------------|----------|
| golangci-lint: govet | Any finding | Critical |
| golangci-lint: staticcheck SA* | Bug detection | Critical |
| golangci-lint: staticcheck S* | Style | Suggestions |
| golangci-lint: errcheck | Unchecked error | Critical |
| golangci-lint: ineffassign | Unused assignment | Suggestions |
| golangci-lint: unused | Unused code | Suggestions |
| golangci-lint: typecheck | Type error | Critical |
| go vet (standalone) | Any finding | Critical |
| go test | Test failure | Critical |
| go test | Build failed | Critical |

### Severity classification rules for convention findings

| Convention Area | Severity |
|----------------|----------|
| Concurrency safety (goroutine leaks, races) | Critical |
| Error handling (discarded errors, wrong pattern) | Critical |
| K8s controller correctness (non-idempotent reconciler, missing owner refs) | Critical |
| K8s deprecated patterns (Phase field, Requeue:true) | Suggestions |
| Naming conventions (Get prefix, wrong caps) | Nits |
| Doc comments (missing or malformed) | Nits |
| Style/formatting (import grouping, line breaks) | Nits |
| K8s API design (bool in spec, int enum) | Suggestions |
| When in doubt | Suggestions |

### Attach a guideline URL to every finding

Read `reference/guideline-urls.md` for the four lookup tables (Effective Go, Go Code Review Comments, tool findings, K8s conventions). **Every finding MUST have a URL.** If no exact match exists, use the general URL for the tool or convention source.

## Step 8: Write reviews.md

**This step replaces all conversation-only output.** Write the review to `reviews.md` at the repository root (same directory as `go.mod`). This file is overwritten on each run. Use the Write tool.

**Cap findings at 20 maximum** across all severity sections. Prioritize by impact:
1. Correctness issues (including K8s reconciler correctness)
2. Concurrency safety
3. K8s controller patterns (when applicable)
4. API design (including K8s API design)
5. Naming conventions
6. Style/formatting

When `framework: k8s-controller`, mark findings that use deprecated K8s patterns (Phase fields, `Requeue: true` without `RequeueAfter`) with a **"deprecated pattern"** label in the observation.

Apply Tone rules throughout the written output.

**Template for reviews.md content:**

```markdown
# Code Review: <module-name from go.mod>

**Reviewed:** <YYYY-MM-DD>
**Reviewer:** review-code skill (Claude Code)
**Project type:** <language>/<framework> (<confidence> confidence)

## Summary

<Generate the Summary section using the instructions below. This is the first thing a reviewer reads -- it must synthesize all findings into an actionable overview.>

### Part 1: Overall assessment (2-3 sentences)

Synthesize across ALL findings (tool and convention) to produce a concise overall assessment. Include:

1. **Opening sentence** about the overall code quality and maturity level. Frame it constructively -- lead with something positive before areas for improvement.
2. **Key strengths observed** -- draw from the "Good Patterns" findings collected in Steps 4-6. Be specific: reference the pattern and where it was observed (e.g., "Solid use of table-driven tests in `pkg/auth/`").
3. **Highest-impact area for improvement** -- identify which severity bucket has the most critical items and what theme connects them (e.g., "The most impactful area for improvement is error handling, which accounts for 3 of 5 critical findings").
4. **Recurring themes** if any -- when the same convention area appears in multiple findings, call it out (e.g., "Error handling is a recurring theme across 4 findings in different packages").

Tone: Balanced and constructive. Always lead with something positive. Example:

> "This is a well-structured codebase with clean separation of concerns and solid test coverage. The most impactful area for improvement is error handling -- several functions discard errors or use bare returns where wrapping would aid debugging. Consistent use of `fmt.Errorf` with `%w` across the codebase would strengthen reliability."

### Part 2: Tools run checklist

Use the `tools_run` status block from Step 3d to generate the checklist. Format each tool line using [x] for tools that ran and [ ] for tools that were skipped or unavailable:

```
**Tools run:**
- [x] golangci-lint vX.Y (N findings)
- [x] go vet: covered by golangci-lint
- [x] go test (N passed, M failed of T total)
```

Rules for the checklist:
- **golangci-lint ran:** Show `[x] golangci-lint (N findings)` with the actual finding count. If version is detectable from output, include it.
- **golangci-lint not available:** Show `[ ] golangci-lint: not available`
- **go vet when golangci-lint ran:** Show `[x] go vet: covered by golangci-lint` (do NOT show as skipped -- govet is a default golangci-lint linter)
- **go vet ran standalone (golangci-lint not available):** Show `[x] go vet (N findings)`
- **go vet not available (and golangci-lint not available):** Show `[ ] go vet: not available`
- **go test ran:** Show `[x] go test (N passed, M failed of T total)` with actual counts
- **go test skipped -- no test files:** Show `[ ] go test: no test files found`
- **go test skipped -- go not available:** Show `[ ] go test: Go toolchain not available`
- **go test -- build failed:** Show `[x] go test: build failed (see Critical findings)`
- **No tools ran at all** (go not available): Replace the entire checklist with: `**Tools run:** No tools available -- review based on convention analysis only`

## Critical

<findings sorted by impact -- these should be addressed before merge>

### <finding-title>

**Location:** `file.go:42`
**Source:** <tool-name> | <convention-name>
**Guideline:** [<short-name>](<url>)

<description of the issue, why it matters, concrete suggestion using friendly tone>

---

## Suggestions

<findings that improve the code but are not blocking>

### <finding-title>

**Location:** `file.go:88`
**Source:** <tool-name> | <convention-name>
**Guideline:** [<short-name>](<url>)

<description, why it matters, suggestion>

---

## Nits

<minor style/convention items -- compact format, one per line>

- `file.go:12` -- <brief description> ([guideline](<url>))
- `file.go:25` -- <brief description> ([guideline](<url>))

---

## Good Patterns Observed

- <acknowledgment with specific file reference>
- <acknowledgment>

---

*Generated by review-code skill. Convention sources: Effective Go, Go Code Review Comments, K8s Best Practices.*
```

**Key formatting rules:**
- Critical and Suggestions findings use full blocks (title, location, source, guideline, description)
- Nits use compact one-line format to avoid bloating the document
- Tool findings and convention findings appear TOGETHER in the appropriate severity section -- do NOT create separate "Tool Findings" or "Lint Results" sections
- If a severity section has no findings, include the heading with "No findings in this category." beneath it

**After writing the full reviews.md**, print a one-line confirmation to the conversation:

> "Review written to reviews.md -- N total findings (X critical, Y suggestions, Z nits)"

## Tone rules

Apply these rules throughout the entire review output:

- Frame findings as **suggestions, not demands**: "Consider using..." not "You must..."
- **Acknowledge good patterns** when found: "Nice use of table-driven tests here"
- Explain the **WHY** behind every suggestion: "This avoids X because..."
- Use **encouraging language**: "This could be even stronger if..."
- **Never** use "wrong", "bad", "incorrect" -- use "could be improved", "convention suggests", "consider"
- When a finding is minor: "This is a small thing, but..." or "Nit:"
- When a finding is significant: "This one is worth addressing because..."

Reinforce tone at each section boundary. If you notice yourself becoming terse, pause and reframe.

## Common mistakes

- Modifying files in the reviewed project -- this skill is read-only; only `reviews.md` is written.
- Exceeding 20 findings -- keep the review bounded and prioritize by impact.
- Omitting the guideline URL on a finding -- every finding MUST cite a convention (use the general URL if no exact match).
- Applying K8s controller conventions (Step 6) to `k8s-aware` projects -- emit the advisory note instead; these projects use K8s libraries but are not controllers.
- Mentioning K8s at all when `framework: go-generic` -- skip Step 6 entirely.
- Reviewing `vendor/`, generated files (`zz_generated_*`, `*_generated.go`), `testdata/`, or `third_party/` -- skip these.
- Listing "no issues found" for a convention area -- skip silently instead.
- Forgetting the **"deprecated pattern"** label on Phase fields in Status structs or `Requeue: true` without `RequeueAfter`.
- Using harsh language ("wrong", "bad", "incorrect") -- apply the Tone rules throughout.
