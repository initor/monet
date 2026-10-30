# Plan schema

This file is the schema of `plan.json`. A task in the plan is its brief. Lint rule 2 names the fields that must not be empty, so lint catches a missing field before any spawn. `<gang> lint` checks a plan against the rules in "Lint".

## Run-wide fields

```json
{
  "version": 1,
  "slug": "demo-phase2",
  "repo": {"remote": "github.com/example/demo", "base": "main", "integration": "phase-2", "gh_account": "example", "wave_title": "chore(gang): land wave {k} of {slug}", "checks": "github"},
  "commands": {"gate": ["make check"], "health": ["make check", "make test-e2e"], "wave_gate": ["make smoke"]},
  "env": {"KUBECONFIG": "./.kube/config"},
  "locks": ["gpu", "kind"],
  "limits": {"task_lines": 100, "split_lines": 300, "fix_rounds": 3, "repair_tries": 2, "blocked_stop": 3, "null_stop": 6, "retries": 2},
  "models": {},
  "forbidden": ["git stash", "git rebase", "force-push", "push to main"],
  "done": "every task is reachable from origin/main and no wave PR is open",
  "requirements": [{"id": "API-1", "source": "specs/api.md:12"}],
  "tasks": []
}
```

- A limit that is missing or `0` takes its default. The size limits are `task_lines` 100 and `split_lines` 300. Lock files and binary files do not count toward a size, and a renamed file counts only its edited lines. The other limits are `fix_rounds` 3, `repair_tries` 2, `blocked_stop` 3, `null_stop` 6, `retries` 2, `gate_slots` 3, `gate_timeout_min` 60, `stall_min` 60, `max_worktrees` 400, and `checks_grace_min` 10.
- An empty `repo.wave_title` takes `chore(gang): land wave {k} of {slug}`.
- An empty `repo.checks` takes `github`. With `gate`, gang runs `commands.gate` at the head of each PR and posts the result as the commit status `gang/gate`.
- `models` maps a role to `{"model": "<alias>", "effort": "<effort>"}`, over the defaults in the skill's `roles.json`. The roles are `implement`, `implement_hard`, `review`, `review_adversarial`, `fix`, `sync`, `merge`, `health`, `repair`, `plan_draft`, and `plan_review`. The efforts are `low`, `medium`, `high`, `xhigh`, and `max`.

## Task fields

| Field | Type | Brief field | Rule |
|---|---|---|---|
| `id` | string, `^[A-Z]+[0-9]+$` | GOAL | unique |
| `title`, `goal` | string | GOAL | `goal` is one sentence a stranger can run |
| `branch` | string | (git) | optional. Defaults to `gang/<id>`. Differs from `repo.base`, `repo.integration`, and every other task's branch |
| `scope.write`, `scope.shared` | globs | SCOPE | `shared` means append-only edits |
| `peers` | ids | SCOPE | tasks allowed to write the same files |
| `depends_on`, `merge_after` | ids | CONTEXT | must exist, no cycles |
| `requirements` | ids | CONTEXT | each id exists in the run-wide list |
| `interfaces.consumes`, `interfaces.produces` | refs (`py:`, `cli:`, `make:`, `file:`, `test:`) | CONTEXT | a consumed ref is produced by a dependency, or already exists |
| `acceptance` | strings, 1 or more | ACCEPTANCE | one checkable line each |
| `verify.tests_first` | test ids, 1 or more | VERIFY | these fail before the change and pass after |
| `verify.commands` | strings | VERIFY | extra commands beyond the run gate |
| `size.lines`, `timebox_min` | ints | TIMEBOX | `size.lines` is at most `limits.task_lines`, or `limits.split_lines` with a `size.reason`. The `budget:` line of TIMEBOX reads `limits.task_lines`, or `limits.split_lines` and the text of `size.reason` when it is not blank |
| `size.reason` | string | TIMEBOX | optional. Says why a task above `limits.task_lines` cannot be split. Lint fails such a task when this field is blank |
| `forbidden` | strings | FORBIDDEN | added to the run-wide list |
| `locks` | names | (scheduling) | each name is in the run-wide `locks` |
| `risk` | `low` or `high` | (review) | `high` adds an adversarial reviewer |
| `wave` | int | (scheduling) | set by `gang schedule`, or kept from an adopted plan |
| `model` | role overrides | (models) | optional, rare. The same shape as the run-wide `models` |
| `notes` | string | (any) | optional. Names the requirement id, with the reason, when a requirement is covered twice. |
| `source` | string | (lint) | optional. In an adopted plan, the id of the source task that this task is a part of. |

REPORT is a fixed return schema that the engine gives each agent. STANDING is `run/standing.md`, pasted into every brief.

## Lint

Lint fails with exit 1 and names every problem. It checks:

1. Schema shape and types. Unknown fields fail. So does a value outside its pattern or range above, a `version` other than 1, a `repo.integration` equal to `repo.base`, a lock name that does not start with a letter or digit or holds a character other than letters, digits, `.`, `_`, and `-`, and a `repo.wave_title` that lacks `{k}` or renders to more than 72 characters for wave 99. So does a `repo.checks` other than `github` or `gate`.
2. Every required field is present and not empty: the run-wide `slug`, `repo.remote`, `repo.base`, `repo.integration`, `done`, and `commands.gate`, the `id` and `source` of each requirement, and the `id`, `title`, `goal`, `scope.write`, `acceptance`, `verify.tests_first`, `size.lines`, and `timebox_min` of each task (a `0` counts as empty). The other list fields may be empty.
3. ids are unique. Every reference resolves. Task branches do not collide. The graph has no cycle across `depends_on` and `merge_after`.
4. Each requirement is covered by at least one task. A requirement covered by two or more tasks must be named by its id, with the reason, in the `notes` of each of them.
5. Two tasks whose `scope.write` globs overlap must list each other in `peers`.
6. `size.lines` is at most `limits.task_lines`, or at most `limits.split_lines` when `size.reason` says why the task cannot be split. A plan may lower both limits, not raise them above the defaults. `limits.split_lines` is at least `limits.task_lines`.
7. Model values are aliases from the allowed set (`opus`, `sonnet`, `haiku`, `fable`). A raw model id fails. Role keys and efforts must be known ones.
8. With `--adopt-source <file>`, the task count and the edge set match the source plan. A task stands for its `source`, or for its own `id` when `source` is empty. An edge between two tasks of one source drops out.
