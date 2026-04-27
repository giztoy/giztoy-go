---

name: gizclaw-cli
version: 1.0.0
description: "Routes GizClaw CLI tasks to the right command-specific skill and defines shared CLI rules. Use when the user asks generally about gizclaw CLI, or when choosing between context, server, admin, and play commands."
metadata:
  requires:
bins: ["gizclaw"]

---

## cliHelp: "gizclaw --help"

# GizClaw CLI

Use this as the entrypoint for GizClaw CLI work. For concrete command usage,
switch to the most specific skill below.

## How To Start

1. Identify the command family from the user's request.
2. Read the corresponding command-specific skill.
3. Use the installed `gizclaw` binary. If unavailable inside this repository,
  use `go run ./cmd/gizclaw`.
4. Run the narrowest relevant `--help` only when exact flags are unclear.

## Skill Routing


| User intent                                                           | Read this skill                                 |
| --------------------------------------------------------------------- | ----------------------------------------------- |
| Create/list/switch/inspect contexts, ping, server-info                | `../gizclaw-context/SKILL.md`                   |
| Start foreground server, service install/start/stop, workspace config | `../gizclaw-server/SKILL.md`                    |
| Manage registered devices/gears                                       | `../gizclaw-admin-gears/SKILL.md`               |
| Manage firmware depots, channels, release, rollback                   | `../gizclaw-admin-firmware/SKILL.md`            |
| Apply/show/delete declarative admin resources                         | `../gizclaw-admin-resources/SKILL.md`           |
| Read provider credentials                                             | `../gizclaw-admin-credentials/SKILL.md`         |
| Read MiniMax tenants                                                  | `../gizclaw-admin-minimax-tenants/SKILL.md`     |
| Read global voices                                                    | `../gizclaw-admin-voices/SKILL.md`              |
| Read workspace template documents                                     | `../gizclaw-admin-workspace-templates/SKILL.md` |
| Read workspace instances                                              | `../gizclaw-admin-workspaces/SKILL.md`          |
| Open Play UI                                                          | `../gizclaw-play/SKILL.md`                      |


## Shared Rules

1. Use the CLI for CLI tasks. Do not bypass `gizclaw` through internal Go packages unless the user explicitly asks for implementation-level work.
2. Use explicit `--context <name>` when the user names a context. Otherwise inspect the current context before server-facing client/admin commands.
3. For mutating commands, verify the target from the user request, current context, or previous command output before running it.
4. For structured admin resource lifecycle work, use `admin apply -f`, `admin show <kind> <name>`, and `admin delete <kind> <name>` through the declarative resource skill.
5. Credential `body` values are returned by the admin API. Use actual user-provided values and do not mask them unless requested.
6. Run long-lived commands such as `serve`, `admin --listen`, and `play --listen` in the background and monitor startup output.
7. Summarize JSON output by key fields unless the user asks for raw output.

## Reporting

When reporting results to the user, include:

1. The exact command that was run.
2. Whether it succeeded.
3. Key output fields or the error reason.
4. The natural next step, only when it helps the current task.

## Source References

Use these only when validating implementation details or updating the CLI itself:

- Command implementation: `cmd/internal/commands/`
- CLI integration stories: `integration/cmd/`