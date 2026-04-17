# 003 Ping RPC Client Reuse

## User Story

As a developer debugging a live GizClaw server, I want to:

1. Start one real server process from the CLI in an isolated workspace.
2. Create a real client context from the CLI in an isolated config home.
3. Reuse one connected client session to issue multiple RPC ping requests.

So that I can trust long-lived client connections, not only one-shot CLI subprocesses.

## Covered Behaviors

- `gizclaw serve <workspace>` keeps the server available for the whole story.
- `gizclaw context create` produces reusable connection metadata and identity files.
- A client loaded from that context can connect once and keep sending repeated RPC `Ping` calls.
- Repeated RPC requests on the same connected client do not fail with premature EOF.

## Isolation Rules

- This story owns its own virtual `HOME`.
- This story owns its own `XDG_CONFIG_HOME`.
- This story owns its own server workspace and runtime data.
- Runtime artifacts are temporary and cleaned by the test harness.
