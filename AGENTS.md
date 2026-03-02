# AGENTS Guide (Repository Review Strategy)

## Repository Context

- This is a **pure Go** repository.
- The project does **not** use Bazel.
- The team does **not** use pull requests for this repo.
- There is currently **no CI pipeline** for automated checks.

Because of this, quality gates must be enforced during local development and manual review.

## Development & Merge Workflow

1. Implement changes locally.
2. Run local validation before merge.
3. Perform code review against task/design requirements.
4. Merge directly after issues are resolved.

No PR metadata (title/body/checks) is required in this repo.

## Review Policy

### 1) Scope & Requirement Compliance

- Changes must match the approved task/design scope.
- Out-of-scope additions require explicit confirmation and documentation updates.

### 2) Static Code Review (Required)

- Verify logic correctness and edge cases.
- Verify error handling (no silent failures on critical paths).
- Reject placeholder implementations (TODO stubs, fake outputs, dead registrations).
- Check dependency hygiene (avoid unintended external/provider coupling).

### 3) Local Validation (Required)

Since there is no CI, developers must run local checks and record results:

- `go test ./...` (or a scoped equivalent with justification)
- Any additional task-specific verification commands

### 3.1) Go Test Coverage Baseline (Per Package)

Coverage must be evaluated **per package** (not only repository-wide average).

- **Minimum passing baseline:** coverage must be **above 80%** for each package.
- **Excellent:** coverage is **90% or above**.
- **Unqualified:** coverage is **below 60%** (must be rejected).

Suggested interpretation for review decisions:

- `>= 90%`: Excellent
- `> 80% and < 90%`: Pass
- `>= 60% and <= 80%`: Below baseline (needs fixes or explicit approval)
- `< 60%`: Fail

### 3.2) Unit Test File Mapping Convention

- Keep a **1:1 mapping** between source and unit-test files whenever possible.
- For `foo.go`, prefer a single corresponding `foo_test.go` (avoid splitting tests for the same source into multiple files without clear reason).

### 4) Commit Hygiene

- Do not commit temporary files, logs, build artifacts, or secrets.
- `openteam/` is local workspace metadata and must remain ignored.

## Reviewer Output Expectations

Review feedback should include:

- Clear pass/fail status
- File/line references for each issue
- Actionable fix guidance
- Priority levels (e.g., P0/P1/P2)

## Non-Goals

- No PR title/description checks (not applicable in this repository workflow).
- No Bazel-related requirements (not used in this repository).
