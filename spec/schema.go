package spec

// EventStatus is the lifecycle state of an event spec.
type EventStatus string

// Valid EventStatus values.
const (
	StatusDraft      EventStatus = "draft"
	StatusActive     EventStatus = "active"
	StatusDeprecated EventStatus = "deprecated"
	StatusDeleted    EventStatus = "deleted"
)

// EventType is the analytics call type.
type EventType string

// Valid EventType values.
const (
	TypeTrack    EventType = "track"
	TypePage     EventType = "page"
	TypeIdentify EventType = "identify"
	TypeGroup    EventType = "group"
	TypeAlias    EventType = "alias"
)

// PropertyType is the JSON Schema-compatible property type.
type PropertyType string

// Valid PropertyType values.
const (
	PropertyTypeString  PropertyType = "string"
	PropertyTypeNumber  PropertyType = "number"
	PropertyTypeInteger PropertyType = "integer"
	PropertyTypeBoolean PropertyType = "boolean"
	PropertyTypeObject  PropertyType = "object"
	PropertyTypeArray   PropertyType = "array"
)

// SamplingStrategy determines how sampling decisions are made.
type SamplingStrategy string

// Valid SamplingStrategy values.
const (
	SamplingUserIDHash SamplingStrategy = "user_id_hash"
	SamplingRandom     SamplingStrategy = "random"
	SamplingNone       SamplingStrategy = "none"
)

// PropertyPriority controls context vs event property collision resolution.
type PropertyPriority string

// Valid PropertyPriority values.
const (
	PriorityEventWins   PropertyPriority = "event_wins"
	PriorityContextWins PropertyPriority = "context_wins"
	PriorityMerge       PropertyPriority = "merge"
)

// SchemaVer is a parsed SchemaVer version string (e.g. "1-2-0").
// Hyphens distinguish event versions from SemVer used for CLI/SDK releases.
type SchemaVer struct {
	Major int
	Minor int
	Patch int
	Raw   string
}

// PropertyDef describes a single event property.
type PropertyDef struct {
	Type        PropertyType `yaml:"type"`
	Required    bool         `yaml:"required"`
	Description string       `yaml:"description,omitempty"`
	Enum        []string     `yaml:"enum,omitempty"`
	Pattern     string       `yaml:"pattern,omitempty"`
	Minimum     *float64     `yaml:"minimum,omitempty"`
	Maximum     *float64     `yaml:"maximum,omitempty"`
	Default     any          `yaml:"default,omitempty"`
	Aliases     []string     `yaml:"aliases,omitempty"`
}

// ProviderOverride holds per-provider event name and property name mappings.
type ProviderOverride struct {
	EventName   string            `yaml:"event_name,omitempty"`
	PropertyMap map[string]string `yaml:"property_map,omitempty"`
}

// SamplingConfig declares the default sampling policy for an event.
type SamplingConfig struct {
	Strategy SamplingStrategy `yaml:"strategy"`
	Rate     float64          `yaml:"rate"`
}

// EventDef is the parsed representation of an event spec YAML file.
type EventDef struct {
	Schema            string                      `yaml:"$schema"`
	Name              string                      `yaml:"name"`
	DisplayName       string                      `yaml:"display_name,omitempty"`
	Description       string                      `yaml:"description,omitempty"`
	Version           string                      `yaml:"version"`
	Changelog         string                      `yaml:"changelog,omitempty"`
	Status            EventStatus                 `yaml:"status"`
	Namespace         string                      `yaml:"namespace"`
	Tags              []string                    `yaml:"tags,omitempty"`
	Owner             string                      `yaml:"owner,omitempty"`
	Type              EventType                   `yaml:"type"`
	EventName         string                      `yaml:"event_name"`
	Required          bool                        `yaml:"required,omitempty"`
	Properties        map[string]PropertyDef      `yaml:"properties"`
	ContextProperties []string                    `yaml:"context_properties,omitempty"`
	ProviderOverrides map[string]ProviderOverride `yaml:"provider_overrides,omitempty"`
	Destinations      []string                    `yaml:"destinations,omitempty"`
	Sampling          *SamplingConfig             `yaml:"sampling,omitempty"`
	PropertyPriority  PropertyPriority            `yaml:"property_priority,omitempty"`

	// SourcePath is populated by the loader, not present in YAML.
	SourcePath string `yaml:"-"`
}

// SourceOutput is the codegen output configuration for a source.
type SourceOutput struct {
	Path    string `yaml:"path"`
	Package string `yaml:"package,omitempty"`
}

// SourceDef is the parsed representation of a source YAML file.
type SourceDef struct {
	Name             string            `yaml:"name"`
	Platform         string            `yaml:"platform,omitempty"`
	Language         string            `yaml:"language"`
	Mode             string            `yaml:"mode,omitempty"`
	RuntimeEndpoint  string            `yaml:"runtime_endpoint,omitempty"`
	Events           []string          `yaml:"events"`
	Destinations     []string          `yaml:"destinations,omitempty"`
	Output           SourceOutput      `yaml:"output"`
	VersionPinning   map[string]string `yaml:"version_pinning,omitempty"`
	ClientValidation bool              `yaml:"client_validation,omitempty"`

	// SourcePath is populated by the loader, not present in YAML.
	SourcePath string `yaml:"-"`
}

// DestinationDef is the parsed representation of a destination YAML file.
type DestinationDef struct {
	Name     string         `yaml:"name"`
	Provider string         `yaml:"provider"`
	Config   map[string]any `yaml:"config,omitempty"`

	// SourcePath is populated by the loader, not present in YAML.
	SourcePath string `yaml:"-"`
}

// RegistryConfig configures the event registry backend.
type RegistryConfig struct {
	Mode string `yaml:"mode"` // git | server
	URL  string `yaml:"url,omitempty"`
}

// WorkspaceConfig is the top-level event-spec.yaml configuration.
type WorkspaceConfig struct {
	Version         int            `yaml:"version"`
	Workspace       string         `yaml:"workspace"`
	Registry        RegistryConfig `yaml:"registry"`
	SpecsDir        string         `yaml:"specs_dir"`
	SourcesDir      string         `yaml:"sources_dir"`
	DestinationsDir string         `yaml:"destinations_dir"`
}
