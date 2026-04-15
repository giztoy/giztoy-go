# 700 Missing Context

## User Story

As a developer who has not created any CLI context yet, I want GizClaw commands to fail
with clear guidance instead of silently using unexpected global state.

## Covered Behaviors

- `gizclaw context list` reports that no contexts exist.
- `gizclaw ping` without any current context fails with an actionable message.

## Isolation Rules

- This story owns its own virtual `HOME`.
- This story owns its own `XDG_CONFIG_HOME`.
- No shared config from other stories is visible.
