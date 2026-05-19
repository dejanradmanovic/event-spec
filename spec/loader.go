package spec

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"
)

// LoadEventDef parses a single event spec YAML file.
func LoadEventDef(path string) (*EventDef, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read %s: %w", path, err)
	}
	var def EventDef
	if err := yaml.Unmarshal(data, &def); err != nil {
		return nil, fmt.Errorf("parse %s: %w", path, err)
	}
	def.SourcePath = path
	return &def, nil
}

// LoadSourceDef parses a source YAML file.
func LoadSourceDef(path string) (*SourceDef, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read %s: %w", path, err)
	}
	var def SourceDef
	if err := yaml.Unmarshal(data, &def); err != nil {
		return nil, fmt.Errorf("parse %s: %w", path, err)
	}
	def.SourcePath = path
	return &def, nil
}

// LoadDestinationDef parses a destination YAML file.
func LoadDestinationDef(path string) (*DestinationDef, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read %s: %w", path, err)
	}
	var def DestinationDef
	if err := yaml.Unmarshal(data, &def); err != nil {
		return nil, fmt.Errorf("parse %s: %w", path, err)
	}
	def.SourcePath = path
	return &def, nil
}

// LoadWorkspaceConfig parses an event-spec.yaml workspace config file.
func LoadWorkspaceConfig(path string) (*WorkspaceConfig, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read %s: %w", path, err)
	}
	var cfg WorkspaceConfig
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("parse %s: %w", path, err)
	}
	return &cfg, nil
}

// WalkEventDefs walks specsDir recursively and returns all EventDef files whose
// $schema header is set. Files that fail to parse are collected separately so
// the caller can decide whether to treat them as errors or warnings.
func WalkEventDefs(specsDir string) ([]*EventDef, []error) {
	var defs []*EventDef
	var errs []error

	err := filepath.WalkDir(specsDir, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() || !strings.HasSuffix(path, ".yaml") {
			return nil
		}
		def, err := LoadEventDef(path)
		if err != nil {
			errs = append(errs, err)
			return nil
		}
		if def.Schema == "" {
			// Not an event spec file (e.g. workspace config, source, destination).
			return nil
		}
		defs = append(defs, def)
		return nil
	})
	if err != nil {
		errs = append(errs, fmt.Errorf("walk %s: %w", specsDir, err))
	}
	return defs, errs
}

// ParseSchemaVer parses a SchemaVer string like "1-2-0" into its components.
func ParseSchemaVer(v string) (SchemaVer, error) {
	parts := strings.Split(v, "-")
	if len(parts) != 3 {
		return SchemaVer{}, fmt.Errorf("invalid SchemaVer %q: must be MAJOR-MINOR-PATCH", v)
	}
	sv := SchemaVer{Raw: v}
	if n, err := fmt.Sscanf(parts[0], "%d", &sv.Major); n != 1 || err != nil {
		return SchemaVer{}, fmt.Errorf("invalid SchemaVer %q: major is not an integer", v)
	}
	if n, err := fmt.Sscanf(parts[1], "%d", &sv.Minor); n != 1 || err != nil {
		return SchemaVer{}, fmt.Errorf("invalid SchemaVer %q: minor is not an integer", v)
	}
	if n, err := fmt.Sscanf(parts[2], "%d", &sv.Patch); n != 1 || err != nil {
		return SchemaVer{}, fmt.Errorf("invalid SchemaVer %q: patch is not an integer", v)
	}
	return sv, nil
}

// CompareSchemaVer returns -1, 0, or 1 when comparing a to b.
func CompareSchemaVer(a, b SchemaVer) int {
	if a.Major != b.Major {
		return signum(a.Major - b.Major)
	}
	if a.Minor != b.Minor {
		return signum(a.Minor - b.Minor)
	}
	return signum(a.Patch - b.Patch)
}

func signum(n int) int {
	if n < 0 {
		return -1
	}
	if n > 0 {
		return 1
	}
	return 0
}