package spec

import (
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"github.com/santhosh-tekuri/jsonschema/v5"
)

// ValidationError represents a single validation problem in a spec or event payload.
type ValidationError struct {
	File    string
	Field   string
	Message string
}

func (e ValidationError) Error() string {
	if e.File != "" {
		return fmt.Sprintf("%s: %s: %s", e.File, e.Field, e.Message)
	}
	return fmt.Sprintf("%s: %s", e.Field, e.Message)
}

// ValidateEventDef checks that an EventDef is structurally valid.
// Returns one ValidationError per problem found; nil slice means valid.
func ValidateEventDef(def *EventDef) []ValidationError {
	var errs []ValidationError
	add := func(field, msg string) {
		errs = append(errs, ValidationError{File: def.SourcePath, Field: field, Message: msg})
	}

	if def.Name == "" {
		add("name", "required")
	}
	if def.Version == "" {
		add("version", "required")
	} else if _, err := ParseSchemaVer(def.Version); err != nil {
		add("version", err.Error())
	}
	if def.Namespace == "" {
		add("namespace", "required")
	}
	if def.EventName == "" {
		add("event_name", "required")
	}

	switch def.Status {
	case StatusDraft, StatusActive, StatusDeprecated, StatusDeleted:
	case "":
		add("status", "required; must be one of: draft, active, deprecated, deleted")
	default:
		add("status", fmt.Sprintf("invalid value %q; must be one of: draft, active, deprecated, deleted", def.Status))
	}

	switch def.Type {
	case TypeTrack, TypePage, TypeIdentify, TypeGroup, TypeAlias:
	case "":
		add("type", "required; must be one of: track, page, identify, group, alias")
	default:
		add("type", fmt.Sprintf("invalid value %q; must be one of: track, page, identify, group, alias", def.Type))
	}

	validPropTypes := map[PropertyType]bool{
		PropertyTypeString:  true,
		PropertyTypeNumber:  true,
		PropertyTypeInteger: true,
		PropertyTypeBoolean: true,
		PropertyTypeObject:  true,
		PropertyTypeArray:   true,
	}
	for propName, prop := range def.Properties {
		if !validPropTypes[prop.Type] {
			add(
				fmt.Sprintf("properties.%s.type", propName),
				fmt.Sprintf("invalid value %q; must be one of: string, number, integer, boolean, object, array", prop.Type),
			)
		}
	}

	if def.Sampling != nil {
		switch def.Sampling.Strategy {
		case SamplingUserIDHash, SamplingRandom, SamplingNone:
		default:
			add("sampling.strategy", fmt.Sprintf("invalid value %q; must be one of: user_id_hash, random, none", def.Sampling.Strategy))
		}
		if def.Sampling.Rate < 0 || def.Sampling.Rate > 1 {
			add("sampling.rate", fmt.Sprintf("must be between 0.0 and 1.0, got %g", def.Sampling.Rate))
		}
	}

	if def.PropertyPriority != "" {
		switch def.PropertyPriority {
		case PriorityEventWins, PriorityContextWins, PriorityMerge:
		default:
			add("property_priority", fmt.Sprintf("invalid value %q; must be one of: event_wins, context_wins, merge", def.PropertyPriority))
		}
	}

	return errs
}

// BuildJSONSchema converts an EventDef's properties into a JSON Schema Draft-07
// document suitable for runtime payload validation.
func BuildJSONSchema(def *EventDef) map[string]any {
	properties := make(map[string]any, len(def.Properties))
	var required []string

	for name, prop := range def.Properties {
		properties[name] = buildPropertySchema(prop)
		if prop.Required {
			required = append(required, name)
		}
	}

	schema := map[string]any{
		"$schema":              "http://json-schema.org/draft-07/schema#",
		"title":                def.EventName,
		"type":                 "object",
		"properties":           properties,
		"additionalProperties": true,
	}
	if len(required) > 0 {
		schema["required"] = required
	}
	return schema
}

// ValidateEventPayload validates an event properties map against the spec's
// JSON Schema Draft-07 definition. Returns nil when the payload is valid.
func ValidateEventPayload(def *EventDef, properties map[string]any) []ValidationError {
	compiled, err := compileSchema(def)
	if err != nil {
		return []ValidationError{{File: def.SourcePath, Field: "schema", Message: "failed to compile schema: " + err.Error()}}
	}

	// jsonschema/v5 requires a JSON-decoded value; marshal+unmarshal normalises Go types.
	raw, err := json.Marshal(properties)
	if err != nil {
		return []ValidationError{{File: def.SourcePath, Field: "properties", Message: "failed to marshal properties: " + err.Error()}}
	}
	var instance any
	if err := json.Unmarshal(raw, &instance); err != nil {
		return []ValidationError{{File: def.SourcePath, Field: "properties", Message: "failed to unmarshal properties: " + err.Error()}}
	}

	if err := compiled.Validate(instance); err != nil {
		var verr *jsonschema.ValidationError
		if errors.As(err, &verr) {
			return flattenValidationErrors(verr, def.SourcePath)
		}
		return []ValidationError{{File: def.SourcePath, Field: "properties", Message: err.Error()}}
	}
	return nil
}

// compileSchema builds and compiles a jsonschema.Schema from the EventDef.
func compileSchema(def *EventDef) (*jsonschema.Schema, error) {
	doc := BuildJSONSchema(def)
	b, err := json.Marshal(doc)
	if err != nil {
		return nil, err
	}
	c := jsonschema.NewCompiler()
	c.Draft = jsonschema.Draft7
	if err := c.AddResource("schema.json", strings.NewReader(string(b))); err != nil {
		return nil, err
	}
	return c.Compile("schema.json")
}

// flattenValidationErrors recursively collects leaf-level validation errors.
func flattenValidationErrors(verr *jsonschema.ValidationError, file string) []ValidationError {
	if len(verr.Causes) == 0 {
		return []ValidationError{{
			File:    file,
			Field:   verr.InstanceLocation,
			Message: verr.Message,
		}}
	}
	var out []ValidationError
	for _, cause := range verr.Causes {
		out = append(out, flattenValidationErrors(cause, file)...)
	}
	return out
}

// ValidateWorkspaceConfig checks that a WorkspaceConfig is structurally valid.
func ValidateWorkspaceConfig(cfg *WorkspaceConfig, path string) []ValidationError {
	var errs []ValidationError
	add := func(field, msg string) {
		errs = append(errs, ValidationError{File: path, Field: field, Message: msg})
	}

	if cfg.Version == 0 {
		add("version", "required; must be >= 1")
	}
	if cfg.Workspace == "" {
		add("workspace", "required")
	}

	switch cfg.Registry.Mode {
	case RegistryModeLocal, RegistryModeGit, RegistryModeServer, "":
		// empty defaults to local — valid
	default:
		add("registry.mode", fmt.Sprintf("invalid value %q; must be one of: local, git, server", cfg.Registry.Mode))
	}
	if cfg.Registry.Mode == RegistryModeGit && cfg.Registry.Remote == "" {
		add("registry.remote", `required when registry.mode is "git"`)
	}
	if cfg.Registry.Mode == RegistryModeServer && cfg.Registry.URL == "" {
		add("registry.url", `required when registry.mode is "server"`)
	}

	return errs
}

// ValidateSourceDef checks that a SourceDef is structurally valid.
func ValidateSourceDef(def *SourceDef) []ValidationError {
	var errs []ValidationError
	add := func(field, msg string) {
		errs = append(errs, ValidationError{File: def.SourcePath, Field: field, Message: msg})
	}

	if def.Name == "" {
		add("name", "required")
	}
	if def.Language == "" {
		add("language", "required")
	} else {
		valid := map[string]bool{
			"go": true, "typescript": true, "swift": true, "kotlin": true,
			"python": true, "java": true, "rust": true, "dart": true, "dotnet": true,
		}
		if !valid[def.Language] {
			add("language", fmt.Sprintf(
				"unknown value %q; supported: go, typescript, swift, kotlin, python, java, rust, dart, dotnet",
				def.Language,
			))
		}
	}
	if def.Output.Path == "" {
		add("output.path", "required")
	}
	if def.Mode != "" {
		switch def.Mode {
		case "embedded", "server_proxied", "hybrid":
		default:
			add("mode", fmt.Sprintf("invalid value %q; must be one of: embedded, server_proxied, hybrid", def.Mode))
		}
	}
	if def.Mode == "server_proxied" && def.RuntimeEndpoint == "" {
		add("runtime_endpoint", `required when mode is "server_proxied"`)
	}

	return errs
}

// ValidateDestinationDef checks that a DestinationDef is structurally valid.
func ValidateDestinationDef(def *DestinationDef) []ValidationError {
	var errs []ValidationError
	add := func(field, msg string) {
		errs = append(errs, ValidationError{File: def.SourcePath, Field: field, Message: msg})
	}

	if def.Name == "" {
		add("name", "required")
	}
	if def.Provider == "" {
		add("provider", "required")
	}

	return errs
}

func buildPropertySchema(prop PropertyDef) map[string]any {
	s := map[string]any{"type": string(prop.Type)}
	if prop.Description != "" {
		s["description"] = prop.Description
	}
	if len(prop.Enum) > 0 {
		s["enum"] = prop.Enum
	}
	if prop.Pattern != "" {
		s["pattern"] = prop.Pattern
	}
	if prop.Minimum != nil {
		s["minimum"] = *prop.Minimum
	}
	if prop.Maximum != nil {
		s["maximum"] = *prop.Maximum
	}
	if prop.Default != nil {
		s["default"] = prop.Default
	}
	return s
}
