# 603 Repeat Command After Partial State

## User Story

As a developer, I want repeated registration commands for an auto-created peer
to be idempotent so retries can safely complete partial device metadata setup.

## Scenario

1. Start a real server with registration enabled.
2. Create one device context.
3. Register that context through the harness API.
4. Repeat the same registration request.
5. Verify the repeated request succeeds and updates the auto-created device info.
6. Verify the context remains usable with `gizclaw ping`.

## Covered Behaviors

- initial registration succeeds
- repeating the same registration updates auto-created metadata safely
- the context remains usable after the retry
- the scenario preserves retry coverage without restoring
  `play register` to the CLI surface
