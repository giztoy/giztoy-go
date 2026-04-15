# 101 Context Current Default

## User Story

As a developer with more than one CLI context, I want commands without `--context`
to use the current context.

So that I can switch once with `gizclaw context use` and then run follow-up commands naturally.

## Covered Behaviors

- `gizclaw context use <name>` changes the current context.
- `gizclaw ping` without `--context` uses the current context.
- Switching to a broken context changes the behavior of default commands.
- Switching back to a valid context restores the default command flow.

## Isolation Rules

- This story owns its own virtual `HOME`.
- This story owns its own `XDG_CONFIG_HOME`.
- This story owns its own server workspace and runtime data.
- Runtime artifacts are temporary and cleaned by the test harness.
