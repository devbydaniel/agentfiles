package layout

// UserAllLayout produces user-level paths for all supported layouts combined.
// Every entry is a full copy — no pointers or symlinks.
type UserAllLayout struct {
	pi     UserPiLayout
	claude UserClaudeLayout
	cursor UserCursorLayout
}

func (UserAllLayout) Name() string { return "user-all" }

// InstructionMdPath returns the primary instruction file path (pi layout).
func (a UserAllLayout) InstructionMdPath() string { return a.pi.InstructionMdPath() }

// SkillPath returns the primary skill path (pi layout).
func (a UserAllLayout) SkillPath(name string) string { return a.pi.SkillPath(name) }

// InstructionMdEntries returns entries for the instruction file across all user layouts.
func (a UserAllLayout) InstructionMdEntries() []Entry {
	return []Entry{
		{Path: a.pi.InstructionMdPath()},
		{Path: a.claude.InstructionMdPath()},
		{Path: a.cursor.InstructionMdPath()},
	}
}

// SkillEntries returns entries for a skill across all user layouts.
func (a UserAllLayout) SkillEntries(name string) []Entry {
	return []Entry{
		{Path: a.pi.SkillPath(name)},
		{Path: a.claude.SkillPath(name)},
		{Path: a.cursor.SkillPath(name)},
	}
}
