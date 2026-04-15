# 001 Ping Reconnect

## User Story

As a developer using one saved CLI context, I want to:

1. Start a real GizClaw server.
2. Create a client context and verify `ping` works.
3. Stop and restart the server with the same workspace.
4. Run `ping` again using the same saved context.

So that I can trust a persisted client context across normal local server restarts.

## Covered Behaviors

- a server workspace can be stopped and restarted by the harness
- the server reuses the same persisted identity after restart
- the same stored CLI context still works after restart
- `gizclaw ping` recovers without recreating the context

## Isolation Rules

- This story owns its own virtual `HOME`.
- This story owns its own `XDG_CONFIG_HOME`.
- This story owns its own server workspace and runtime data.
- Runtime artifacts are temporary and cleaned by the test harness.
