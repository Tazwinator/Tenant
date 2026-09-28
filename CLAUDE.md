# tenant: notes for contributors and agents

A slow-burn horror story that runs inside a real zsh. It's one CGO-free Go binary
plus a zsh hook. Start with the [README](README.md), then [docs/](docs/).

## Never break these (from docs/SECURITY.md)

- Never read file contents. Only directory names under `$HOME`, plus tenant's own state.
- Never write outside `$XDG_STATE_HOME/tenant`. No `.zshrc` or `$HISTFILE` edits,
  and no `print -s`.
- No `net` and no `os/exec` imports. Only `internal/audit` touches the filesystem.
- The zsh hook never `eval`s binary output: fixed verbs plus integers only.
- Never wrap, alias or replace user commands.
- `confess` output is never altered by the story. `evict` is always out of character.
- Hooks are inert without the `active` sentinel, as root, and with `TENANT_OFF=1`.

## Conventions

- Go 1.24+, `CGO_ENABLED=0`, standard library first. Ask before adding a dependency.
- Docs are in British English. Keep them short, and update the relevant doc in the
  same change as the code.
- Spoilers (story text) go in `story/` only. Docs describe structure, not script.
- Record decisions in `docs/DECISIONS.md` (append; don't rewrite history).
