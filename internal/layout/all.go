package layout

// AllLayout produces paths for all supported layouts combined.
// Every entry is a full copy — no pointers or symlinks.
type AllLayout struct {
	pi     PiLayout
	claude ClaudeLayout
	cursor CursorLayout
}

func (AllLayout) Name() string { return "all" }

// InstructionMdPath returns the primary instruction file path (AGENTS.md).
func (a AllLayout) InstructionMdPath() string { return a.pi.InstructionMdPath() }

// SkillPath returns the primary skill path (pi layout).
func (a AllLayout) SkillPath(name string) string { return a.pi.SkillPath(name) }

// InstructionMdEntries returns entries for the instruction file across all layouts.
// Duplicate paths (e.g. pi and cursor both use AGENTS.md) are deduplicated.
func (a AllLayout) InstructionMdEntries() []Entry {
	return dedup([]Entry{
		{Path: a.pi.InstructionMdPath()},
		{Path: a.claude.InstructionMdPath()},
		{Path: a.cursor.InstructionMdPath()},
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
