# 200 Server Config Boot

## User Story

As a developer, I want a server started by `gizclaw serve` to honor the workspace
configuration for networking and storage.

## Covered Behaviors

- the server boots from a story-owned `config.yaml`
- the configured listen address is actually used
- a client context can connect to the resulting server

## Isolation Rules

- This story owns its own virtual `HOME`
- This story owns its own `XDG_CONFIG_HOME`
- This story owns its own server workspace and runtime data
