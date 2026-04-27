# Real Service UI Smoke

## User Story

As an operator, I want the Admin and Play UIs to load against a real GizClaw service with seeded data so UI integration coverage proves the browser, proxy, service, and stores work together.

## Scenario

Given a real `gizclaw serve` process, persistent test workspace, seeded Admin resources, and real Admin/Play UI processes, when Playwright opens the product UIs without route mocks, key pages and actions display real backend data.

## Covered Behaviors

- Starts real GizClaw server, Admin UI, and Play UI processes.
- Seeds real device, firmware, credential, MiniMax tenant, voice, workspace template, and workspace data.
- Verifies Admin dashboard, devices, firmware, provider, and AI pages through real `/api/admin` and `/api/public` proxy routes.
- Verifies Play action cards through real `/api/gear` and `/api/public` proxy routes.
- Keeps run artifacts in `integration/.workspace/ui-real-service` for debugging.
