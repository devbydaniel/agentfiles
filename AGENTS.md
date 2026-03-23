# AGENTS.md

## Architecture

Go CLI (`af`) using cobra. Entry: `main.go` → `cmd/root.go`. All commands in `cmd/`, business logic in `internal/`.

## Key data flow

```
Config (~/.config/agentfiles/config.toml)
  → Named stores (personal = "~/.agentfiles", work = "~/.agentfiles-work")
    Stores contain: instructions/, skills/, resources/, bundles/, agents/
  → Manifest (.agentfiles per repo) or Config repos ([[repos]] entries)
    OR [user] section for user-level deployment
  → manifest.Resolve(m, stores, defaultStore) expands bundle refs + skill/agent overrides
    into flat ResolvedAsset lists (each carries Name + Store provenance)
  → layout.Get() or layout.GetUser() determines file paths per tool
  → apply.Apply(stores, defaultStore, ...) copies files from correct stores
    + converts agents to tool-specific formats (e.g., .md → .toml for Codex)
    + writes .agentfiles.lock (repo) or user.lock (user-level)
  → push.Push(stores, defaultStore, ...) diffs deployed files against lock hashes,
    converts back to canonical format if needed (e.g., .toml → .md for agents),
    routes changes back to the correct store per lock entry
```

## Non-obvious design decisions

- **Multi-store with config-level registry**: Stores are named in `~/.config/agentfiles/config.toml` under `[stores]`. The registry (`[[repos]]`) lives in the same config file (not inside any store). Assets can reference cross-store items with `storename:assetname` syntax.
- **Store provenance in lock**: Each lock `Entry` has a `Store` field recording which named store the asset came from. Empty means default store (backward compat with pre-multi-store lock files).
- **Lock keys for non-default stores**: Assets from non-default stores use `storename:assetname` as the lock key. Default store assets use just the name.
- **Layout "all"**: all three tool paths get full copies. See `internal/layout/all.go`.
- **Entry**: Just a `Path` string. All entries are full copies. Apply's `deployEntry` calls `deployCopy` directly.
- **Lock hashing**: Directories use sorted `file:<rel>\nhash:<sha256>\n` pairs fed into sha256. This means file renames change the hash. See `lock.HashDir`.
- **Stale asset pruning**: Apply compares old lock → new lock and removes deployed files/directories no longer in the manifest. Skipped during `--skill` (single-skill) deploys. `ApplyResult.Removed` tracks count.
- **Lock HashDirMapped**: Used by push to hash using store directory structure but reading from deployed paths (which may differ per layout).
- **Config repo merge**: Named repos in config.toml are skipped if no matching config.local.toml entry exists. Path-only repos work standalone. Local entries can override layout/skills or skip entirely. See `internal/config/config.go` mergeRepos().
- **apply-all always force-writes the manifest**: It overwrites `.agentfiles` in each repo to keep it in sync with the config. Then runs apply with Force=true.
- **exec uses os/exec.Command.Run()**, not syscall.Exec, so it works cross-platform. Looks up by name first, falls back to path basename.
- **Store must be a git repo**: `store.Open` checks for `.git` directory.
- **Bundle vs cherry-pick modes are mutually exclusive** in manifest validation. `skills_add`/`skills_remove` only work with bundle mode.
- **Resources are layout-independent**: copied to repo root preserving internal structure, not placed in `.pi/` etc.
- **Backward compat**: Single-store setups work with `--store <path>` flag. Lock files without store fields default to the default store. The `storename:` prefix is optional — unprefixed assets use the default store.
- **User-level deployment**: `[user]` section in config acts as the manifest (no `.agentfiles` in `$HOME`). Lock file at `~/.config/agentfiles/user.lock`. User layouts produce home-relative paths (`~/.claude/CLAUDE.md`, `~/AGENTS.md`, `~/.pi/skills/`). Resources not supported at user level. `apply-all` includes user deployment. Parameterized via `Options.LockFilePath` in apply/push.
- **User layout variants**: `layout.GetUser()` returns user-level layouts (`internal/layout/user_*.go`). Same tool names (pi/claude/cursor/all) but different paths (home-relative instead of repo-relative).
- **Agents (subagents)**: Single `.md` files in `agents/` store directory. Authored as Markdown with YAML frontmatter (canonical format). Deployed as-is (`.md`) for Claude/Cursor, converted to TOML for Codex (body → `developer_instructions`, frontmatter fields → TOML keys). Pi layouts ignore agents (`AgentEntries` returns nil). Push from Codex `.toml` converts back to `.md` in store. For "all" layout, `.md` is the primary (source of truth for push). `agents/` is optional in `store.Open` for backward compat with pre-agent stores.
- **Agent hashing**: Lock hash is computed on the canonical (parsed → re-serialized) `.md` form, not the deployed format. This ensures hash stability across format conversions (deploy as `.toml`, push back as `.md`, hash matches).
- **Bundle vs cherry-pick for agents**: `agents` field works in cherry-pick mode. `agents_add`/`agents_remove` work with bundle mode (same pattern as skills).

## Config resolution

`config.DefaultConfigPath()` returns `~/.config/agentfiles/config.toml`. The `--config` flag overrides. Config defines named stores, a default store, and repo entries. The `--store` flag selects a named store or accepts a direct path for backward compat.

## Testing patterns

- Tests create temp stores via `store.Init(t.TempDir())` which handles git init + subdirectory creation.
- Integration tests in `internal/integration_test.go` exercise the full lifecycle including multi-store cross-store apply/push.
- Multi-store tests use `map[string]*store.Store{"name": s}` maps passed to apply/push.
