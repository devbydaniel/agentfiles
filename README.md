# agentfiles (`af`)

**agentfiles** is a CLI tool that manages portable context files (skills, agent instructions, plugins, resources) for AI coding agents. It keeps a single source-of-truth store and deploys files into any number of repositories using layout presets for different agents (pi, Claude Code, Cursor). Edit files in any repo and push changes back to the store so every project stays in sync.

## Install

```bash
go install github.com/danielbenner/agentfiles@latest
```

The binary is called `agentfiles`. Alias it to `af` for convenience:

```bash
alias af=agentfiles
```

## Quick Start

```bash
# 1. Create a store (git-managed directory for all your agent files)
af init-store ~/.agentfiles

# 2. Add assets to the store
af add skill   ~/my-skills/browse/
af add agent   ~/instructions.md --name default
af add plugin  ~/my-plugins/formatter/
af add resource ~/shared-configs/

# 3. Create a bundle (manually write a TOML file in the store)
cat > ~/.agentfiles/bundles/fullstack.toml << 'EOF'
[bundle]
name = "fullstack"
agents_md = "default"

[skills]
include = ["browse", "web-search"]

[plugins]
include = ["formatter"]

[resources]
include = ["shared-configs"]
EOF

# 4. Initialize a repo to use the bundle
cd ~/my-project
af init --bundle fullstack --layout pi

# 5. Deploy files into the repo
af apply
```

## Manifest Format (`.agentfiles`)

The manifest is a TOML file at the repo root named `.agentfiles`.

### Bundle mode

Reference a bundle from the store and optionally override skills:

```toml
bundle = "fullstack"
layout = "pi"

# Optional overrides (bundle mode only):
skills_add = ["extra-skill"]
skills_remove = ["browse"]
```

### Cherry-pick mode

Select individual assets without a bundle:

```toml
layout = "pi"
agents_md = "default"
skills = ["browse", "web-search"]
plugins = ["formatter"]
resources = ["shared-configs"]
```

Bundle mode and cherry-pick mode are mutually exclusive.

## Bundle Format

Bundles live in `<store>/bundles/<name>.toml`:

```toml
[bundle]
name = "fullstack"
agents_md = "default"      # references agents/<name>.md in the store

[skills]
include = ["browse", "web-search", "solve-captcha"]
exclude = []               # optional: remove specific skills

[plugins]
include = ["formatter"]
exclude = []

[resources]
include = ["shared-configs"]
exclude = []
```

## Layouts

The `layout` field controls where files are placed in the repo:

| Layout       | Agent file     | Skills dir              | Plugins dir              |
|-------------|----------------|-------------------------|--------------------------|
| `pi`        | `AGENTS.md`    | `.pi/skills/<name>/`    | `.pi/plugins/<name>/`    |
| `claude`    | `CLAUDE.md`    | `.claude/skills/<name>/`| `.claude/plugins/<name>/`|
| `cursor`    | `.cursorrules` | `.cursor/skills/<name>/`| `.cursor/plugins/<name>/`|
| `all`       | All of above   | All of above            | All of above             |

Resources are always copied to the repo root regardless of layout.

The `all` layout creates files for every agent. It uses pi as the primary layout and creates symlinks/pointers for Claude and Cursor.

## Commands Reference

### `af init-store [path]`

Create a new agentfiles store with the required directory structure (`skills/`, `agents/`, `plugins/`, `resources/`, `bundles/`) and initialise a git repository.

```bash
af init-store ~/.agentfiles
af init-store --from https://github.com/user/agent-store.git ~/store
```

### `af add skill <path>`

Copy a skill directory into the store.

```bash
af add skill ~/my-skills/browse/
af add skill ./local-skill/ --force  # overwrite existing
```

### `af add agent <file> --name <name>`

Copy an agent instructions file into the store as `agents/<name>.md`.

```bash
af add agent ./AGENTS.md --name default
```

### `af add plugin <path>`

Copy a plugin directory into the store.

```bash
af add plugin ~/my-plugins/formatter/
```

### `af add resource <path>`

Copy a resource directory into the store. Resource contents are deployed directly to the repo root.

```bash
af add resource ~/shared-configs/
```

### `af init`

Create an `.agentfiles` manifest in the current directory. Interactive mode lists available bundles and layouts; non-interactive mode uses flags:

```bash
af init                              # interactive
af init --bundle fullstack --layout pi  # non-interactive
```

### `af apply`

Deploy files from the store into the current repo according to the manifest and layout.

```bash
af apply                  # deploy all assets
af apply --force          # overwrite existing files
af apply --skill browse   # deploy only the named skill
```

Creates/updates `.agentfiles.lock` to track deployed file hashes.

### `af push`

Compare deployed files to their lock hashes and copy changed files back to the store.

```bash
af push                   # push all changes
af push --dry-run         # show what would be pushed
af push --skill browse    # push only the named skill
```

### `af list <skills|bundles|agents|plugins|resources>`

List assets in the store by type: skills, bundles, agents, plugins, or resources.

### `af diff`

Show differences between deployed files and the store.

### `af status`

Show the sync status of deployed files (changed, unchanged, missing).

### `af version`

Print the agentfiles version.

### Global Flags

| Flag | Description |
|------|-------------|
| `--store <path>` | Path to the agentfiles store (default: `~/.agentfiles` or configured in `~/.config/agentfiles/config.toml`) |

## `.gitignore` Recommendations

Add deployed agent files to `.gitignore` since they can be regenerated with `af apply`:

```gitignore
# agentfiles — pi layout
AGENTS.md
.pi/skills/
.pi/plugins/
.agentfiles.lock

# agentfiles — claude layout
# CLAUDE.md
# .claude/skills/

# agentfiles — cursor layout
# .cursorrules
# .cursor/skills/
# .cursor/plugins/
```

Keep `.agentfiles` (the manifest) in version control. The `.agentfiles.lock` file can be gitignored or committed — it's regenerated on `af apply`.

## Push Workflow

The push workflow lets you edit agent files in context (inside a real project) and propagate changes to all other repos:

```bash
# 1. Edit a skill in your repo
vim .pi/skills/browse/SKILL.md

# 2. Push the change back to the store
af push

# 3. In another repo, pull the update
cd ~/other-project
af apply --force
```

This keeps the store as the single source of truth while allowing edits from any repo.
