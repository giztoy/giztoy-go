# 703 Invalid Workspace Config

## User Story

As a developer, I want `gizclaw serve` to fail with a clear validation error when a
workspace config is incomplete or invalid.

## Covered Behaviors

- a story-owned invalid `config.yaml` is loaded
- `gizclaw serve` exits instead of starting
- stderr explains the configuration problem
