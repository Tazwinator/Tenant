# tenant: notes for contributors and agents

A slow-burn horror story that runs inside a real shell (bash first, then zsh). It's
one CGO-free Go binary plus a hook script per shell. Start with the
[README](README.md), then [docs/](docs/).

Work from `master`.

## Never break these (from docs/SECURITY.md)

- Never read file contents: only directory names under `$HOME`, plus tenant's own
  state. Never read `$HISTFILE`.
- Never write outside `$XDG_STATE_HOME/tenant`. No rc-file or `$HISTFILE` edits, no
  `print -s`, and no `history -s`.
- No `net` and no `os/exec` imports. Only `internal/audit` touches the filesystem.
- Hooks never `eval` binary output: fixed verbs plus integers only.
- Never wrap, alias or replace user commands.
- `confess` output is never altered by the story. `evict` is always out of character.
- Hooks are inert without the `active` sentinel, as root, and with `TENANT_OFF=1`.
- Ghost text must pass `mech.HarmlessCommand`: it is one Enter away from running.
- Hook functions return 0 or the status they were given (bash `set -eu`), and zsh
  functions set their own `localoptions`. Both are covered by integration tests.

## Build and test

```sh
CGO_ENABLED=0 go build -o tenant ./cmd/tenant
go test ./...                               # bash and zsh integration tests; -short skips them
shellcheck -s bash internal/shell/bash.sh && zsh -n internal/shell/zsh.zsh
TENANT_DEV=1 ./tenant _sim --profile=daily  # pacing on a fake clock (spoilers)
```

Changing pacing means re-running `TestPacingTargets` and updating the table in
docs/SCHEDULER.md. Changing a hook script means both shells, plus
internal/shell/integration_test.go.

## Conventions

- Go 1.24+, `CGO_ENABLED=0`, standard library only (D18). Ask before adding a dependency.
- Docs are in British English. Keep them short, and update the relevant doc in the
  same change as the code.
- Spoilers (story text) go in `story/` only. Docs describe structure, not script.
- Record decisions in `docs/DECISIONS.md` (append; don't rewrite history).
- No GUI or new platforms before launch (D16). New surfaces go into the same binary.
