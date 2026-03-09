# AGENTS.md

## Architecture

Go CLI (`af`) using cobra. Entry: `main.go` → `cmd/root.go`. All commands in `cmd/`, business logic in `internal/`.

## Key data flow

```
Store (~/.agentfiles/)
  → Manifest (.agentfiles per repo) or Registry (registry.toml + registry.local.toml)
  → manifest.Resolve() expands bundle refs + skill overrides into flat asset lists
  → layout.Get() determines file paths per tool (pi/claude/cursor/all)
  → apply.Apply() copies files + writes .agentfiles.lock with content hashes
  → push.Push() diffs deployed files against lock hashes, copies changes back to store
```

## Non-obvious design decisions

- **Layout "all"**: all three tool paths get full copies. See `internal/layout/all.go`.
- **Entry**: Just a `Path` string. All entries are full copies. Apply's `deployEntry` calls `deployCopy` directly.
- **Lock hashing**: Directories use sorted `file:<rel>\nhash:<sha256>\n` pairs fed into sha256. This means file renames change the hash. See `lock.HashDir`.
- **Lock HashDirMapped**: Used by push to hash using store directory structure but reading from deployed paths (which may differ per layout).
- **Registry merge**: Named repos in `registry.toml` are skipped if no matching `registry.local.toml` entry exists. Path-only repos work standalone. Local entries can override layout/skills or skip entirely. See `internal/registry/registry.go` merge().
- **apply-all always force-writes the manifest**: It overwrites `.agentfiles` in each repo to keep it in sync with the registry. Then runs apply with Force=true.
- **exec uses os/exec.Command.Run()**, not syscall.Exec, so it works cross-platform. Looks up by name first, falls back to path basename.
- **Store must be a git repo**: `store.Open` checks for `.git` directory.
- **Bundle vs cherry-pick modes are mutually exclusive** in manifest validation. `skills_add`/`skills_remove` only work with bundle mode.
- **Resources are layout-independent**: copied to repo root preserving internal structure, not placed in `.pi/` etc.

## Store config resolution

`store.DefaultStorePath()` checks `~/.config/agentfiles/config.toml` for `source = "..."`, falls back to `~/.agentfiles`. The `--store` global flag overrides everything.

## Testing patterns

- Tests create temp stores via `store.Init(t.TempDir())` which handles git init + subdirectory creation.
- Integration test in `internal/integration_test.go` exercises the full init→apply→push cycle.
