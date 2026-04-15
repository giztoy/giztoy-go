# 000 Ping

## User Story

As a developer running GizClaw locally, I want to:

1. Start a real server process from the CLI inside an isolated workspace.
2. Create one or more client contexts inside an isolated virtual `HOME`.
3. Repeatedly run `gizclaw ping` against the long-running server.
4. Run multiple `gizclaw ping` commands concurrently from different client contexts.

So that I can trust the CLI workflow end to end, not just the in-process Go APIs.

## Covered Behaviors

- `gizclaw serve <workspace>` starts from a story-owned config fixture.
- `gizclaw context create` works against the started server.
- `gizclaw ping --context <name>` succeeds repeatedly for the same client.
- Multiple client identities can ping the same long-running server concurrently.

## Isolation Rules

- This story owns its own virtual `HOME`.
- This story owns its own `XDG_CONFIG_HOME`.
- This story owns its own server workspace and runtime data.
- Runtime artifacts are temporary and cleaned by the test harness.

