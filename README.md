# agentfiles (`af`)

Portable config management for AI coding tools. Multiple named stores, many repos, any tool.

`af` keeps skills, instructions, and resources in git-managed stores and deploys them into repositories using **layouts** that match each tool's expected file structure. Multiple named stores let you separate personal and work assets while composing them freely across repos. Edit files in any repo, push changes back, and every other repo picks them up on the next `af apply`.

Think **chezmoi, but for AI coding context files**.

---

## Concepts

### Source Stores

Each store is a git repository containing assets. You can have multiple named stores — for example, one for personal skills and another for work:

```
~/.agentfiles/              # "personal" store
├── skills/
│   ├── browse/
│   │   └── SKILL.md
│   └── tooling/            # skill group
│       └── web-search/
│           └── SKILL.md
├── instructions/
│   └── assistant.md
├── resources/
│   └── editorconfig/
│       └── .editorconfig
└── bundles/
    └── oss.toml

~/.agentfiles-work/         # "work" store
├── skills/
│   └── git-workflow/
│       ├── SKILL.md
│       └── examples/
├── instructions/
│   └── backend.md
├── resources/
│   └── cursor-config/
│       └── .cursor/
│           └── rules.json
└── bundles/
    ├── backend.toml
    └── frontend.toml
```

Stores are named in `~/.config/agentfiles/config.toml`:

```toml
default_store = "work"

[stores]
personal = "~/.agentfiles"
work = "~/.agentfiles-work"
```

Each store is just a git repo. You manage them with normal git operations (commit, push, pull, branch). A single-store setup works too — just define one store.

### Assets

| Asset | What it is | Store location | Example |
|-------|-----------|----------------|---------|
| **Skill** | A directory with a `SKILL.md` and optional supporting files | `skills/<name>/` or `skills/<group>/<name>/` | `skills/browse/SKILL.md`, `skills/tooling/browse/SKILL.md` |
| **Instruction** | A markdown file with system-level instructions | `instructions/<name>.md` | `instructions/assistant.md` |
| **Resource** | An arbitrary file tree copied to repo root | `resources/<name>/` | `resources/cursor-config/.cursor/rules.json` |

**Resources** are special: their contents are copied directly into the repo root, preserving internal directory structure. For example, `resources/cursor-config/.cursor/rules.json` deploys to `.cursor/rules.json` in the repo. This makes resources useful for tool configs, shared scripts, or any files that need specific paths.

### Skill Groups

Skills can be organized into subdirectories (groups) within the store. Grouping is purely a store-side organizational concern — deployed skills always use the **leaf name** (the last path component).

```
~/.agentfiles/
└── skills/
    ├── browse/              # flat skill → "browse"
    │   └── SKILL.md
    ├── tooling/             # group directory (no SKILL.md)
    │   ├── web-search/      # grouped skill → "tooling/web-search"
    │   │   └── SKILL.md
    │   └── browse/          # grouped skill → "tooling/browse"
    │       └── SKILL.md
    └── infra/
        └── aws/
            └── deploy/      # deeply nested → "infra/aws/deploy"
                └── SKILL.md
```

**Key rules:**

- **A directory is a skill if it contains `SKILL.md`.** This is the marker. Everything inside a skill directory (scripts/, references/, etc.) is part of that skill.
- **A skill directory cannot be a group parent.** The walk stops at `SKILL.md` — subdirectories are skill content, not nested groups.
- **Deploy uses the leaf name.** `skills/tooling/browse/` deploys as `.pi/skills/browse/`, not `.pi/skills/tooling/browse/`. Grouping is invisible to the consuming agent.
- **Leaf-name collisions are an error.** If two resolved skills have the same leaf name (e.g., `tooling/browse` and `legacy/browse`), resolution fails.

**Referencing grouped skills:**

| Syntax | Meaning |
|--------|---------|
| `"browse"` | Bare name — searches all groups, errors if ambiguous |
| `"tooling/browse"` | Group-qualified — direct lookup |
| `"tooling/"` | Trailing-slash glob — all skills under `tooling/` recursively |
| `"infra/aws/"` | Nested glob — all skills under `infra/aws/` |
| `"work:tooling/"` | Cross-store glob — all skills under `tooling/` in the `work` store |

Globs work in bundle `skills.include`, `skills.exclude`, manifest `skills`, `skills_add`, and `skills_remove`.

**Migration:** Existing flat skills work unchanged. To reorganize into groups, move skill directories into subdirectories under `skills/` and update bundle/manifest references.

### Bundles

A bundle is a named grouping of assets defined in a TOML file. Instead of listing every skill individually in each repo, you reference a bundle:

```toml
# ~/.agentfiles/bundles/backend.toml
[bundle]
name = "backend"
instructions = "backend"           # → instructions/backend.md

[skills]
include = ["git-workflow", "nestjs-hexagonal-backend", "tooling/"]  # trailing / = group glob
exclude = ["tooling/deprecated-linter"]   # exclude specific grouped skills

[resources]
include = ["cursor-config", "editorconfig"]
exclude = []
```

### Repo Registry (in config)

The config file optionally maps repositories to their bundle/layout configuration. Instead of `cd`-ing into each repo to run `af init`, you declare all deployments in one place and batch-apply them with `af apply-all`.

The registry uses **two files** to support team workflows:

**`~/.config/agentfiles/config.toml`** — shared team config:

```toml
default_store = "work"

[stores]
personal = "~/.agentfiles"
work = "~/.agentfiles-work"

[[repos]]
name = "api-server"
bundle = "backend"
store = "work"                 # optional: defaults to default_store
layout = "pi"

[[repos]]
name = "web-app"
bundle = "frontend"
layout = "all"
skills_add = ["personal:browse"]  # cross-store: prefix with storename:
exec_args = ["--model", "gpt-5.4"]  # default args for af exec
```

Each repo can specify a `store` to target a specific named store (defaults to `default_store`). The optional `exec_args` field provides default arguments that are prepended when launching the agent via `af exec`.

The registry is **additive** to the per-repo workflow. Repos can still have their own `.agentfiles` managed manually — the registry simply provides a central view and batch operations. When `af apply-all` runs, it writes/updates the `.agentfiles` manifest in each registered repo to keep it in sync with the config.

### Manifest (`.agentfiles`)

Each repo gets one file — `.agentfiles` — declaring what it needs. This is the only file you commit. Everything else is generated by `af apply`.

**Bundle mode** — reference a bundle, optionally override skills:

```toml
bundle = "backend"
layout = "pi"

# Optional overrides (only valid with bundle):
skills_add = ["browse", "personal:my-skill"]  # add skills (cross-store with prefix)
skills_remove = ["typeorm-migrations"]         # remove from bundle's list
```

**Cherry-pick mode** — select individual assets, no bundle:

```toml
instructions = "assistant"
skills = ["browse", "tooling/web-search", "personal:git-workflow", "work:ayunis/"]  # qualified names, globs, cross-store
resources = ["editorconfig"]
layout = "pi"
```

Assets without a `storename:` prefix use the default store. Use the prefix to pull assets from other named stores.

Bundle mode and cherry-pick mode are **mutually exclusive**. The `layout` field defaults to `"pi"` if omitted.

### Layouts

Layouts control where files land in the repo. Different AI tools expect different directory structures:

| Layout | Instruction file | Skills |
|--------|-----------|--------|
| `pi` | `AGENTS.md` | `.pi/skills/<name>/` |
| `claude` | `CLAUDE.md` | `.claude/skills/<name>/` |
| `cursor` | `AGENTS.md` | `.cursor/skills/<name>/` |
| `all` | All of the above | All of the above |

**Resources** are layout-independent — always copied to repo root regardless of layout.

The **`all` layout** deploys for every tool simultaneously — all entries are full copies.

#### User-Level Layouts

When deploying to user-level paths (via `[user]` config or `af apply-user`), layouts produce home-relative paths:

| Layout | Instruction file | Skills |
|--------|-----------|--------|
| `pi` | `~/AGENTS.md` | `~/.pi/skills/<name>/` |
| `claude` | `~/.claude/CLAUDE.md` | `~/.claude/skills/<name>/` |
| `cursor` | `~/.cursor/rules/agentfiles.md` | `~/.cursor/skills/<name>/` |
| `all` | All of the above | All of the above |

The lock file for user-level deployments lives at `~/.config/agentfiles/user.lock`.

### Lock File (`.agentfiles.lock`)

Created/updated by `af apply`. Tracks what was deployed, where, and its content hash. Used by `af push`, `af diff`, and `af status` to detect changes. Auto-generated — gitignore it.

```toml
[deployed.instructions]
source = "instructions/backend.md"
path = "AGENTS.md"
hash = "a1b2c3..."

[deployed.skills.browse]
source = "skills/browse/"
path = ".pi/skills/browse"
hash = "d4e5f6..."

# Grouped skill — key is group-qualified
[deployed.skills."tooling/web-search"]
source = "skills/tooling/web-search/"
path = ".pi/skills/web-search"
hash = "g7h8i9..."
```

---

## Install

### Homebrew

```bash
brew install devbydaniel/tap/agentfiles
```

### Go

```bash
go install github.com/devbydaniel/agentfiles@latest
```

The binary is called `agentfiles`. Create an alias:

```bash
alias af=agentfiles
```

---

## Shell Completion

Enable tab completion for `af` commands and arguments (e.g., repo names for `af exec`).

### Zsh

```bash
echo 'source <(af completion zsh)' >> ~/.zshrc
source ~/.zshrc
```

### Bash

```bash
echo 'source <(af completion bash)' >> ~/.bashrc
source ~/.bashrc
```

### Fish

```bash
af completion fish | source
af completion fish > ~/.config/fish/completions/af.fish
```

After setup, `af exec <TAB>` will autocomplete repo names, and all commands/flags are completable.

---

## Quick Start

```bash
# 1. Create a source store
af init-store

# 2. Add your existing skills and instruction files
af add skill ~/ai/skills/browse/
af add skill ~/ai/skills/web-search/
af add instruction ~/my-project/AGENTS.md --name backend
af add resource ~/shared-configs/cursor-config/

# 3. Create a bundle
cat > ~/.agentfiles/bundles/backend.toml << 'EOF'
[bundle]
name = "backend"
instructions = "backend"

[skills]
include = ["browse", "web-search"]

[resources]
include = ["cursor-config"]
EOF

# 4. Set up a repo
cd ~/my-project
af init --bundle backend --layout pi

# 5. Deploy
af apply

# 6. Verify
af status
```

After this, `~/my-project/` contains:
```
AGENTS.md                      ← from instructions/backend.md
.pi/skills/browse/SKILL.md     ← from skills/browse/
.pi/skills/web-search/SKILL.md ← from skills/web-search/
.cursor/rules.json             ← from resources/cursor-config/
.agentfiles                    ← committed to git
.agentfiles.lock               ← gitignored
```

---

## Commands

### `af init-store [path]`

Create a new source store. Creates `skills/`, `instructions/`, `resources/`, `bundles/` subdirectories and runs `git init`.

```bash
af init-store                                    # default: ~/.agentfiles
af init-store ~/my-store                         # custom path
af init-store --from git@github.com:me/store.git # clone existing
```

New stores can optionally be registered in `~/.config/agentfiles/config.toml` under `[stores]` with a name.

### `af add <type> <path>`

Copy assets from the filesystem into the source store.

```bash
af add skill <dir>                         # copies dir → store/skills/<dirname>/
af add skill <dir> --group tooling         # copies dir → store/skills/tooling/<dirname>/
af add skill <dir> --group infra/aws       # copies dir → store/skills/infra/aws/<dirname>/
af add instruction <file> --name <name>          # copies file → store/instructions/<name>.md
af add resource <dir>                      # copies dir → store/resources/<dirname>/
```

Use `--force` to overwrite existing assets in the store. Without it, existing assets produce an error.
Use `--group` with `af add skill` to place the skill in a group subdirectory.

The `--name` flag is required for `af add instruction` because instruction files (e.g., `AGENTS.md`, `CLAUDE.md`) don't have meaningful filenames — the name you choose becomes the identifier used in bundles and manifests.

### `af init`

Create an `.agentfiles` manifest in the current directory.

```bash
af init                                       # interactive: pick bundle + layout
af init --bundle backend --layout pi          # non-interactive
af init --bundle backend --layout all         # deploy for all tools
```

Prints suggested `.gitignore` additions after creating the manifest. Refuses to overwrite an existing `.agentfiles`.

### `af apply`

Read `.agentfiles`, resolve assets from the store, and deploy files into the current repo.

```bash
af apply                    # deploy all assets
af apply --force            # overwrite existing files without warning
af apply --skill browse     # deploy only one skill
```

**What it does:**
1. Reads `.agentfiles` in the current directory
2. Resolves bundle → flat list of assets (or uses cherry-pick list)
3. Looks up the layout to determine target paths
4. Copies/symlinks each asset from the store to the repo
5. Writes `.agentfiles.lock` with content hashes

Without `--force`, existing files are skipped with a warning. The lock file only records assets that were actually deployed (not skipped).

### `af apply-all`

Deploy instruction files to every repo listed in the config (`[[repos]]` in `config.toml`). For each entry the command:

1. Creates the target directory if it doesn't exist
2. Writes/updates the `.agentfiles` manifest from the registry entry
3. Runs `apply --force` to deploy all assets

```bash
af apply-all              # deploy to all registered repos
af apply-all --dry-run    # show what would be done, don't deploy
```

If a `[user]` section is present in the config, user-level files are also deployed before processing repos.

This is the fastest way to propagate store changes (new skills, updated instructions) to all your projects at once. Repos that aren't in the config are unaffected.

### `af apply-user`

Deploy instruction files to user-level paths (e.g. `~/.claude/`, `~/.pi/`). Reads the `[user]` section from the config file.

```bash
af apply-user               # deploy user-level files
af apply-user --force       # overwrite existing files
af apply-user --dry-run     # show what would be done
```

Requires a `[user]` section in `~/.config/agentfiles/config.toml`:

```toml
[user]
bundle = "personal-base"
layout = "all"
```

The `[user]` section supports the same fields as a manifest: `bundle`, `layout`, `instructions`, `skills`, `skills_add`, `skills_remove`. Resources are not supported at user level.

### `af push-user`

Push locally modified user-level instruction files back to the source store.

```bash
af push-user               # push all user-level changes
af push-user --dry-run     # show what would be pushed
```

Requires a prior `af apply-user` (the user lock file must exist).

### `af exec <repo-name>`

Look up a repo by name in the config and launch the appropriate agent CLI in that repo's directory. The agent is chosen based on the repo's layout:

| Layout | Agent CLI |
|--------|-----------|
| `pi` | `pi` |
| `claude` | `claude` |
| `cursor` | `cursor-agent` |
| `all` | `pi` (primary) |

```bash
af exec api-server                         # launch the agent for this repo
af exec web-app -- -p "fix the tests"      # forward args to the agent
af exec api-server --agent claude           # override agent choice
```

Looks up by name first, then falls back to matching the basename of the repo path.

**Default arguments:** Repos can specify `exec_args` in the config to provide default arguments that are always prepended when launching the agent:

```toml
[[repos]]
name = "api-server"
bundle = "backend"
layout = "pi"
exec_args = ["--model", "gpt-5.4"]
```

With this config, `af exec api-server -- -p "fix tests"` runs `pi --model gpt-5.4 -p "fix tests"`. Arguments passed after `--` on the command line come after the configured defaults.

### `af push`

Detect local edits to deployed files and copy them back to the source store.

```bash
af push                     # push all changes back to store
af push --dry-run           # show what would change, don't copy
af push --skill browse      # push only one skill
```

**How it works:**
1. Reads `.agentfiles.lock`
2. Hashes each deployed file on disk
3. Compares to the hash recorded at deploy time
4. Changed files are copied back to the store
5. Lock file is updated with new hashes

After pushing, commit the changes in your source store (`cd ~/.agentfiles && git add -A && git commit`). Then `af apply --force` in other repos to pick up the changes.

### `af list <type>`

List assets in the source store.

```bash
af list skills              # list all skills (group-qualified paths, sorted)
af list skills --flat       # list only leaf names (for scripting)
af list instructions              # list all instruction files (without .md extension)
af list bundles             # list all bundle files (without .toml extension)
af list resources           # list all resource directories
```

With skill groups, `af list skills` shows group-qualified paths:
```
ayunis/backend
browse
infra/aws/deploy
tooling/web-search
```

### `af diff`

Show a line-by-line diff between deployed files in the current repo and their source in the store. Useful for seeing what you've changed locally before deciding to `af push`.

```bash
af diff
```

### `af status`

Show the sync status of every deployed asset.

```bash
af status
```

Reports one of four states per asset:

| Status | Meaning |
|--------|---------|
| `unchanged` | Deployed file matches both lock and store |
| `modified locally` | Deployed file differs from lock (you edited it) |
| `modified in store` | Store file differs from lock (someone pushed from another repo) |
| `conflict` | Both deployed and store differ from lock |

### `af version`

Print the version string.

### Global Flags

| Flag | Default | Description |
|------|---------|-------------|
| `--store <name\|path>` | default store from config | Named store or direct path. For single-store commands. |
| `--config <path>` | `~/.config/agentfiles/config.toml` | Path to the config file. |

---

## Workflows

### Setting up a new repo

```bash
cd ~/new-project
af init --bundle backend --layout pi
af apply
# Add .agentfiles to git, add deployed paths to .gitignore
```

### Editing a skill in context and propagating

```bash
# Edit in your project (where you can test it)
vim .pi/skills/browse/SKILL.md

# Push back to the store
af push

# Commit in the store
cd ~/.agentfiles && git add -A && git commit -m "update browse skill"

# Update other repos
cd ~/other-project && af apply --force
```

### Setting up a worktree

`.agentfiles` is committed to git, so every worktree gets it automatically:

```bash
git worktree add ../my-project-wt-feature
cd ../my-project-wt-feature
af apply    # deploys from the store using the same manifest
```

### Adding a new skill to multiple repos

```bash
# Add the skill to the store
af add skill ~/ai/skills/new-skill/

# Add it to the bundle
vim ~/.agentfiles/bundles/backend.toml  # add "new-skill" to skills.include

# Re-apply in each repo
cd ~/project-1 && af apply --force
cd ~/project-2 && af apply --force

# Or, if using the registry, just:
af apply-all
```

### Using multiple stores

```bash
# Set up two stores
af init-store ~/.agentfiles           # personal skills
af init-store ~/.agentfiles-work      # work skills

# Configure in ~/.config/agentfiles/config.toml:
# default_store = "work"
# [stores]
# personal = "~/.agentfiles"
# work = "~/.agentfiles-work"

# Add skills to each store
af add skill ~/ai/skills/browse/ --store personal
af add skill ~/work/skills/api-patterns/ --store work

# Use cross-store references in manifests:
# bundle = "backend"                       ← from default (work) store
# skills_add = ["personal:browse"]         ← from personal store

af apply   # deploys from both stores

# Push routes changes to the correct store automatically
af push    # modified personal skills go to personal store
```

### Deploying global/user-level instruction files

```bash
# Add a [user] section to your config
cat >> ~/.config/agentfiles/config.toml << 'EOF'

[user]
bundle = "personal-base"
layout = "all"
skills_add = ["work:git-workflow"]
EOF

# Deploy to ~/.claude/, ~/.pi/, ~/AGENTS.md, etc.
af apply-user

# Or include it in apply-all (automatic if [user] is configured)
af apply-all

# Edit a global skill and push back
vim ~/.pi/skills/browse/SKILL.md
af push-user
```

### Checking for drift across repos

```bash
cd ~/project-1 && af status
cd ~/project-2 && af status
```

### Using with multiple AI tools

```bash
# Deploy for all tools at once
af init --bundle backend --layout all
af apply
```

This creates:
```
AGENTS.md                           ← full copy (pi + cursor share this)
CLAUDE.md                           ← full copy
.pi/skills/browse/SKILL.md          ← full copy
.claude/skills/browse/SKILL.md      ← full copy
.cursor/skills/browse/SKILL.md      ← full copy
```

---

## `.gitignore` Setup

Deployed files are derived content — regenerated by `af apply`. Gitignore them. The manifest (`.agentfiles`) is the source of truth and should be committed.

### Pi layout
```gitignore
AGENTS.md
.pi/skills/
.agentfiles.lock
```

### Claude layout
```gitignore
CLAUDE.md
.claude/skills/
.agentfiles.lock
```

### Cursor layout
```gitignore
AGENTS.md
.cursor/skills/
.agentfiles.lock
```

### All layout
```gitignore
AGENTS.md
CLAUDE.md
.pi/skills/
.claude/skills/
.cursor/skills/
.agentfiles.lock
```

**Always commit:** `.agentfiles`
**Always gitignore:** `.agentfiles.lock`

---

## Store Configuration

Stores are configured in `~/.config/agentfiles/config.toml`:

```toml
default_store = "personal"

[stores]
personal = "~/.agentfiles"
work = "~/.agentfiles-work"
```

**Per-command override:** `--store <name>` selects a named store for commands that target a single store (e.g., `af list`, `af add`). Also accepts a direct path for backward compat.

**Config override:** `--config /path/to/config.toml` uses a custom config file.

---

## File Format Reference

### `.agentfiles` (Manifest)

```toml
# EITHER bundle mode:
bundle = "backend"                # references bundles/<name>.toml in the store
layout = "pi"                     # pi | claude | cursor | all (default: pi)
skills_add = ["extra-skill"]      # optional: add skills on top of bundle
skills_remove = ["unwanted"]      # optional: remove skills from bundle

# OR cherry-pick mode:
instructions = "assistant"           # references instructions/<name>.md in the store
skills = ["browse", "web-search"] # skill directory names in the store
resources = ["cursor-config"]     # resource directory names in the store
layout = "pi"                     # pi | claude | cursor | all (default: pi)
```

### Bundle (`.toml`)

```toml
[bundle]
name = "backend"
instructions = "backend"             # references instructions/<name>.md

[skills]
include = ["browse", "git-workflow", "tooling/"]  # trailing / = group glob (recursive)
exclude = ["tooling/deprecated"]                  # globs work in exclude too

[resources]
include = ["cursor-config"]
exclude = []
```

### Lock File (`.agentfiles.lock`)

Auto-generated by `af apply`. Do not edit manually.

```toml
[deployed.instructions]
store = "work"                   # which named store this came from
source = "instructions/backend.md"     # store-relative path
path = "AGENTS.md"               # repo-relative deployed path
hash = "sha256hex..."            # content hash at deploy time

[deployed.skills.browse]
store = "work"
source = "skills/browse/"
path = ".pi/skills/browse"
hash = "sha256hex..."

# Cross-store skill — key is "personal:my-skill"
[deployed.skills."personal:my-skill"]
store = "personal"
source = "skills/my-skill/"
path = ".pi/skills/my-skill"
hash = "sha256hex..."

[deployed.resources.cursor-config]
store = "work"
source = "resources/cursor-config/"
path = "cursor-config"
hash = "sha256hex..."
```

The `store` field is omitted for backward compat when empty (defaults to the default store).

### Config (`~/.config/agentfiles/config.toml`)

```toml
default_store = "work"

[stores]
personal = "~/.agentfiles"
work = "~/.agentfiles-work"

# Optional: user-level deployment (global instruction files)
[user]
bundle = "personal-base"
layout = "all"
skills_add = ["work:git-workflow"]

# Optional: repo registry for af apply-all
[[repos]]
name = "api-server"
bundle = "backend"
store = "work"
layout = "pi"
skills_add = ["personal:browse"]
exec_args = ["--model", "gpt-5.4"]  # prepended to af exec args
```
