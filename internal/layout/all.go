package layout

// AllLayout produces paths for all supported layouts combined.
// Every entry is a full copy — no pointers or symlinks.
type AllLayout struct {
	pi     PiLayout
	claude ClaudeLayout
	cursor CursorLayout
}

func (AllLayout) Name() string { return "all" }

// AgentMdPath returns the primary agent file path (AGENTS.md).
func (a AllLayout) AgentMdPath() string { return a.pi.AgentMdPath() }

// SkillPath returns the primary skill path (pi layout).
func (a AllLayout) SkillPath(name string) string { return a.pi.SkillPath(name) }

// AgentMdEntries returns entries for the agent file across all layouts.
// Duplicate paths (e.g. pi and cursor both use AGENTS.md) are deduplicated.
func (a AllLayout) AgentMdEntries() []Entry {
	return dedup([]Entry{
		{Path: a.pi.AgentMdPath()},
		{Path: a.claude.AgentMdPath()},
		{Path: a.cursor.AgentMdPath()},
	})
}

// dedup removes entries with duplicate paths, keeping the first occurrence.
func dedup(entries []Entry) []Entry {
	seen := make(map[string]bool)
	out := make([]Entry, 0, len(entries))
	for _, e := range entries {
		if !seen[e.Path] {
			seen[e.Path] = true
			out = append(out, e)
		}
	}
	return out
}

// SkillEntries returns entries for a skill across all layouts.
func (a AllLayout) SkillEntries(name string) []Entry {
	return []Entry{
		{Path: a.pi.SkillPath(name)},
		{Path: a.claude.SkillPath(name)},
		{Path: a.cursor.SkillPath(name)},
	}
}
