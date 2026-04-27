# UI User Story Sets

## Goal

`integration/ui/` contains browser-level UI integration tests driven by Playwright.

Each story set should verify user-facing workflows by:

- starting real GizClaw server, Admin UI, and Play UI processes
- seeding canonical local test data through real APIs
- organizing each behavior as a story-owned scenario directory

## Story Set Layout

Each story directory should contain:

- `USER_STORIES.md`
- one focused Go harness test
- one focused `*.real.spec.ts` Playwright file
- optional story-owned server fixtures

Shared real-service seed data lives in:

- `integration/testutil/gizclaw_seed_data/`

## Module Entry Points

Run UI integration commands from `integration/ui/`:

- `npm install`
- `npm run test:list`
- `npm run typecheck`
- `npm test`
- `npm run test:ui`

The Playwright config for this story set lives in:

- `integration/ui/playwright.config.ts`

Tests run through `go test ./900-real-service-smoke`; that Go test starts real `gizclaw serve`, `gizclaw admin --listen`, and `gizclaw play --listen` processes, then invokes Playwright with real UI URLs.

Run artifacts are intentionally preserved under:

- `integration/.workspace/ui-real-service/`

## Case Naming Convention

Use one directory per user-facing behavior:

- `900-real-service-smoke`

Recommended rule:

- `900-999`: real service-backed UI workflows

Each case directory should describe:

- which route or UI surface is opened
- which real data is seeded
- which user-visible controls are exercised
- what user-visible outcome is expected

## Real Service-Backed Story Sets

### `900-*` Real Service Smoke

Purpose:

- verify the browser UIs against real GizClaw service processes and real seeded stores
- preserve debug artifacts after failures

Cases:

- `900-real-service-smoke`
  - start real `gizclaw serve`
  - seed admin/device registrations, firmware, provider resources, voices, templates, and workspaces
  - start real Admin and Play UI commands
  - verify Admin pages and Play actions without Playwright route mocks
  - status: implemented

## Ordering Strategy

Recommended implementation order:

1. Add shared seed data under `integration/testutil/gizclaw_seed_data/`.
2. Add real-service smoke coverage with shared seed data.
3. Run Playwright discovery, TypeScript, the real-service UI suite, and scoped Go checks.

## Design Notes

- Keep each Playwright spec focused on one user story.
- Prefer role-based exact locators scoped by landmarks, cards, or tables.
- Use real-service UI tests to verify backend behavior, proxy routing, and seeded data.
- Do not add pure UI route-mock tests for the same flows; one real-service layer keeps the UI suite honest.
- Do not add test-only IDs unless accessible selectors are insufficient.
- `MemoryPage` is intentionally out of scope while it remains unrouted.
- Do not clean `integration/.workspace/ui-real-service/` automatically; it is the retained failure/debug workspace.