package gitnotes

const (
	// RefPrefix is the prefix for all Warrant git notes refs.
	RefPrefix = "refs/notes/warrant"
)

// Valid note types (and ref path suffix).
const (
	TypeDecision = "decision"
	TypeTrace    = "trace"
	TypeIntent   = "intent"
)

// RefForType returns the full ref for the given note type (e.g. "decision" -> "refs/notes/warrant/decision").
// Returns empty string if type is not valid.
func RefForType(noteType string) string {
	switch noteType {
	case TypeDecision, TypeTrace, TypeIntent:
		return RefPrefix + "/" + noteType
	default:
		return ""
	}
}

// AllRefs returns all Warrant note refs for iteration (e.g. show all note types for a commit).
func AllRefs() []string {
	return []string{
		RefPrefix + "/" + TypeDecision,
		RefPrefix + "/" + TypeTrace,
		RefPrefix + "/" + TypeIntent,
	}
}
