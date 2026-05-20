package spec

// ChangeKind classifies the type of difference detected between two event versions.
type ChangeKind string

// Valid ChangeKind values and their SchemaVer impact (MAJOR or MINOR).
const (
	ChangeAddRequiredProp ChangeKind = "add_required_prop" // MAJOR
	ChangeRemoveProp      ChangeKind = "remove_prop"       // MAJOR
	ChangeRenameProp      ChangeKind = "rename_prop"       // MAJOR
	ChangeTypeChanged     ChangeKind = "type_changed"      // MAJOR
	ChangeMakeRequired    ChangeKind = "make_required"     // MAJOR
	ChangeRemoveEnumValue ChangeKind = "remove_enum_value" // MAJOR
	ChangeMakeOptional    ChangeKind = "make_optional"     // MINOR
	ChangeAddOptionalProp ChangeKind = "add_optional_prop" // MINOR
	ChangeAddEnumValue    ChangeKind = "add_enum_value"    // MINOR
)

// Change describes a single detected difference between two event spec versions.
type Change struct {
	Kind       ChangeKind
	Property   string
	Breaking   bool
	From, To   string
	Suggestion string // suggested new SchemaVer consistent with the detected change
}
