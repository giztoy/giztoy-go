# UI User Story Sets

## Goal

`test/gizclaw-e2e/ui/` contains browser-level UI integration tests driven by
the Playwright Go SDK.

Each story set should verify user-facing workflows by:

- requiring an already running `gizclaw service`
- seeding canonical local test data once through real APIs
- organizing each behavior as a short Go story file loaded by one suite entry point

The default executable suite is real-service backed. The lower-numbered story
files remain as the UI coverage inventory so page coverage is not lost while
real-service coverage expands.

## Story Set Layout

Each story file should contain:

- a file-level comment that states the user story
- one focused story function registered by the root suite
- no top-level `Test*` function outside the suite entry point

Shared real-service seed data lives in:

- `test/gizclaw-e2e/testutil/gizclaw_seed_data/`

## Entry Points

Run UI integration tests from the repository root:

- `go run github.com/playwright-community/playwright-go/cmd/playwright install --only-shell chromium`
- `gizclaw service start`
- `go test ./test/gizclaw-e2e/ui`

Tests run through a single Go suite. The suite connects to the current GizClaw
CLI context and fails fast if the service is not running, seeds one shared data
set, starts embedded Admin and Play UI HTTP servers directly, launches one
Playwright browser, and runs each story as a subtest.

Run artifacts are intentionally preserved under:

- `test/gizclaw-e2e/.workspace/ui-real-service/`

## Case Naming Convention

Use one short Go file per user-facing behavior:

- `900_real_service_smoke.go`

Recommended rule:

- `100-109`: Admin shell and overview smoke
- `110-119`: Admin peer inventory and detail workflows
- `120-129`: Admin firmware workflows
- `130-139`: Admin provider catalog workflows
- `140-149`: Admin AI catalog workflows
- `150-159`: Admin navigation smoke
- `200-209`: Play UI workflows
- `900-999`: real service-backed UI workflows

Each case file should describe:

- the user story in the file comment
- which route or UI surface is opened
- which user-visible controls are exercised
- what user-visible outcome is expected

## Story Coverage Inventory

### `100-*` Admin Shell And Overview

- `100-admin-dashboard`
- `101-admin-legacy-hash-route`

### `110-*` Admin Peer Workflows

- `110-admin-peers-list`
- `111-admin-peer-detail`
- `112-admin-peer-actions`

### `120-*` Admin Firmware Workflows

- `120-admin-firmware-list`
- `121-admin-firmware-upload`
- `122-admin-depot-detail`
- `123-admin-depot-actions`
- `124-admin-channel-detail`

### `130-*` Admin Provider Catalogs

- `130-admin-credentials-list`
- `131-admin-minimax-tenants-list`

### `140-*` Admin AI Catalogs

- `140-admin-voices-list`
- `141-admin-workspace-templates-list`
- `142-admin-workspaces-list`

### `150-*` Admin Navigation Smoke

- `150-admin-sidebar-navigation`

### `200-*` Play UI Workflows

- `200-play-shell`
- `201-play-actions`
- `202-play-all-actions`
- `203-play-action-errors`

## Real Service-Backed Story Sets

### `900-*` Real Service Smoke

Purpose:

- verify the browser UIs against real GizClaw service processes and real seeded stores
- preserve debug artifacts after failures

Cases:

- `900_real_service_smoke.go`
  - require the already running `gizclaw service`
  - seed admin/device registrations, firmware, provider resources, voices, templates, and workspaces
  - start embedded Admin and Play UI test servers without `gizclaw * --listen`
  - verify Admin pages and Play actions without Playwright route mocks or TypeScript specs
  - status: implemented

## Ordering Strategy

Recommended implementation order:

1. Add shared seed data under `test/gizclaw-e2e/testutil/gizclaw_seed_data/`.
2. Add real-service smoke coverage with shared seed data.
3. Run the real-service Go UI suite and scoped Go checks.

## Design Notes

- Keep each Go Playwright subtest focused on one user story.
- Keep only `test/gizclaw-e2e/ui` as the executable UI test package; story files should register story definitions rather than define top-level `Test*` functions.
- Prefer role-based exact locators scoped by landmarks, cards, or tables.
- Use real-service UI tests to verify backend behavior, proxy routing, and seeded data.
- Do not add pure UI route-mock tests for the same flows; one real-service layer keeps the UI suite honest.
- Do not add test-only IDs unless accessible selectors are insufficient.
- `MemoryPage` is intentionally out of scope while it remains unrouted.
- Do not clean `test/gizclaw-e2e/.workspace/ui-real-service/` automatically; it is the retained failure/debug workspace.