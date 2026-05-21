package spec_test

import (
	"path/filepath"
	"sort"
	"testing"

	"github.com/dejanradmanovic/event-spec/spec"
)

const testdataDir = "testdata/diff"

func loadFixture(t *testing.T, sub, file string) *spec.EventDef {
	t.Helper()
	path := filepath.Join(testdataDir, sub, file)
	def, err := spec.LoadEventDef(path)
	if err != nil {
		t.Fatalf("load fixture %s: %v", path, err)
	}
	return def
}

// sortChanges sorts a slice of Change by (Kind, Property) for deterministic comparison.
func sortChanges(changes []spec.Change) {
	sort.Slice(changes, func(i, j int) bool {
		if changes[i].Kind != changes[j].Kind {
			return changes[i].Kind < changes[j].Kind
		}
		return changes[i].Property < changes[j].Property
	})
}

func TestDiff(t *testing.T) {
	base := loadFixture(t, "base", "product_viewed_1-0-0.yaml")

	tests := []struct {
		name     string
		fixture  string
		wantLen  int
		wantKind spec.ChangeKind
		wantProp string
		breaking bool
	}{
		{
			name: "identical specs produce no changes",
		},
		{
			name:     "add required property",
			fixture:  "add_required/product_viewed.yaml",
			wantLen:  1,
			wantKind: spec.ChangeAddRequiredProp,
			wantProp: "quantity",
			breaking: true,
		},
		{
			name:     "add optional property",
			fixture:  "add_optional/product_viewed.yaml",
			wantLen:  1,
			wantKind: spec.ChangeAddOptionalProp,
			wantProp: "coupon_code",
			breaking: false,
		},
		{
			name:     "remove property",
			fixture:  "remove_prop/product_viewed.yaml",
			wantLen:  1,
			wantKind: spec.ChangeRemoveProp,
			wantProp: "currency",
			breaking: true,
		},
		{
			name:     "change property type",
			fixture:  "change_type/product_viewed.yaml",
			wantLen:  1,
			wantKind: spec.ChangeTypeChanged,
			wantProp: "price",
			breaking: true,
		},
		{
			name:     "remove enum value",
			fixture:  "remove_enum/product_viewed.yaml",
			wantLen:  1,
			wantKind: spec.ChangeRemoveEnumValue,
			wantProp: "category",
			breaking: true,
		},
		{
			name:     "add enum value",
			fixture:  "add_enum/product_viewed.yaml",
			wantLen:  1,
			wantKind: spec.ChangeAddEnumValue,
			wantProp: "category",
			breaking: false,
		},
		{
			name:     "make optional property required",
			fixture:  "make_required/product_viewed.yaml",
			wantLen:  1,
			wantKind: spec.ChangeMakeRequired,
			wantProp: "currency",
			breaking: true,
		},
		{
			name:     "make required property optional",
			fixture:  "make_optional/product_viewed.yaml",
			wantLen:  1,
			wantKind: spec.ChangeMakeOptional,
			wantProp: "price",
			breaking: false,
		},
		{
			name:     "rename property",
			fixture:  "rename_prop/product_viewed.yaml",
			wantLen:  1,
			wantKind: spec.ChangeRenameProp,
			wantProp: "product_id",
			breaking: true,
		},
		{
			name:     "rename event",
			fixture:  "rename_event/product_viewed.yaml",
			wantLen:  1,
			wantKind: spec.ChangeRenameEvent,
			wantProp: "event_name",
			breaking: true,
		},
		{
			name:     "description only",
			fixture:  "description_only/product_viewed.yaml",
			wantLen:  1,
			wantKind: spec.ChangeDescriptionOnly,
			wantProp: "product_id",
			breaking: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var to *spec.EventDef
			if tt.fixture == "" {
				to = base
			} else {
				to = loadFixture(t, filepath.Dir(tt.fixture), filepath.Base(tt.fixture))
			}

			changes := spec.Diff(base, to)
			sortChanges(changes)

			if tt.wantLen == 0 {
				if len(changes) != 0 {
					t.Errorf("expected no changes, got %d: %v", len(changes), changes)
				}
				return
			}

			if len(changes) != tt.wantLen {
				t.Fatalf("expected %d change(s), got %d: %v", tt.wantLen, len(changes), changes)
			}

			c := changes[0]
			if c.Kind != tt.wantKind {
				t.Errorf("Kind: want %q, got %q", tt.wantKind, c.Kind)
			}
			if c.Property != tt.wantProp {
				t.Errorf("Property: want %q, got %q", tt.wantProp, c.Property)
			}
			if c.Breaking != tt.breaking {
				t.Errorf("Breaking: want %v, got %v", tt.breaking, c.Breaking)
			}
		})
	}
}

func TestDiffRenamePropFromTo(t *testing.T) {
	base := loadFixture(t, "base", "product_viewed_1-0-0.yaml")
	to := loadFixture(t, "rename_prop", "product_viewed.yaml")

	changes := spec.Diff(base, to)
	if len(changes) != 1 {
		t.Fatalf("expected 1 change, got %d", len(changes))
	}
	c := changes[0]
	if c.From != "product_id" {
		t.Errorf("From: want %q, got %q", "product_id", c.From)
	}
	if c.To != "sku" {
		t.Errorf("To: want %q, got %q", "sku", c.To)
	}
}

func TestDiffTypeChangedFromTo(t *testing.T) {
	base := loadFixture(t, "base", "product_viewed_1-0-0.yaml")
	to := loadFixture(t, "change_type", "product_viewed.yaml")

	changes := spec.Diff(base, to)
	if len(changes) != 1 {
		t.Fatalf("expected 1 change, got %d", len(changes))
	}
	c := changes[0]
	if c.From != "number" {
		t.Errorf("From: want %q, got %q", "number", c.From)
	}
	if c.To != "string" {
		t.Errorf("To: want %q, got %q", "string", c.To)
	}
}

func TestSuggestVersion(t *testing.T) {
	tests := []struct {
		name    string
		from    string
		changes []spec.Change
		want    string
	}{
		{
			name:    "no changes returns patch bump",
			from:    "1-0-0",
			changes: nil,
			want:    "1-0-1",
		},
		{
			name:    "breaking change requires MAJOR bump",
			from:    "1-0-0",
			changes: []spec.Change{{Kind: spec.ChangeAddRequiredProp, Breaking: true}},
			want:    "2-0-0",
		},
		{
			name:    "minor change requires MINOR bump",
			from:    "1-2-0",
			changes: []spec.Change{{Kind: spec.ChangeAddOptionalProp, Breaking: false}},
			want:    "1-3-0",
		},
		{
			name:    "description-only requires PATCH bump",
			from:    "1-2-3",
			changes: []spec.Change{{Kind: spec.ChangeDescriptionOnly, Breaking: false}},
			want:    "1-2-4",
		},
		{
			name: "mixed changes use highest severity",
			from: "1-0-0",
			changes: []spec.Change{
				{Kind: spec.ChangeAddOptionalProp, Breaking: false},
				{Kind: spec.ChangeAddRequiredProp, Breaking: true},
			},
			want: "2-0-0",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			fromVer, err := spec.ParseSchemaVer(tt.from)
			if err != nil {
				t.Fatalf("parse from: %v", err)
			}
			got := spec.SuggestVersion(fromVer, tt.changes)
			if got.Raw != tt.want {
				t.Errorf("SuggestVersion(%s, changes): want %q, got %q", tt.from, tt.want, got.Raw)
			}
		})
	}
}

func TestValidateVersionBump(t *testing.T) {
	makeEventDef := func(version string) *spec.EventDef {
		return &spec.EventDef{
			Name:      "product_viewed",
			Namespace: "ecommerce",
			EventName: "Product Viewed",
			Version:   version,
			Status:    spec.StatusActive,
			Type:      spec.TypeTrack,
		}
	}

	tests := []struct {
		name    string
		from    string
		to      string
		changes []spec.Change
		wantErr bool
	}{
		{
			name:    "MAJOR bump with breaking changes is valid",
			from:    "1-0-0",
			to:      "2-0-0",
			changes: []spec.Change{{Kind: spec.ChangeAddRequiredProp, Breaking: true}},
			wantErr: false,
		},
		{
			name:    "MINOR bump with breaking changes is invalid",
			from:    "1-0-0",
			to:      "1-1-0",
			changes: []spec.Change{{Kind: spec.ChangeAddRequiredProp, Breaking: true}},
			wantErr: true,
		},
		{
			name:    "PATCH bump with MINOR changes is invalid",
			from:    "1-0-0",
			to:      "1-0-1",
			changes: []spec.Change{{Kind: spec.ChangeAddOptionalProp, Breaking: false}},
			wantErr: true,
		},
		{
			name:    "downgrade is always invalid",
			from:    "1-2-0",
			to:      "1-0-0",
			changes: nil,
			wantErr: true,
		},
		{
			name:    "MAJOR over-bump for MINOR changes is allowed",
			from:    "1-0-0",
			to:      "2-0-0",
			changes: []spec.Change{{Kind: spec.ChangeAddOptionalProp, Breaking: false}},
			wantErr: false,
		},
		{
			name:    "no changes with version bump is valid",
			from:    "1-0-0",
			to:      "1-0-1",
			changes: nil,
			wantErr: false,
		},
		{
			name:    "PATCH bump for description-only is valid",
			from:    "1-0-0",
			to:      "1-0-1",
			changes: []spec.Change{{Kind: spec.ChangeDescriptionOnly, Breaking: false}},
			wantErr: false,
		},
		{
			name:    "equal versions is invalid (no increase)",
			from:    "1-0-0",
			to:      "1-0-0",
			changes: nil,
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			fromDef := makeEventDef(tt.from)
			toDef := makeEventDef(tt.to)
			err := spec.ValidateVersionBump(fromDef, toDef, tt.changes)
			if (err != nil) != tt.wantErr {
				t.Errorf("ValidateVersionBump(%s→%s): wantErr=%v, got err=%v", tt.from, tt.to, tt.wantErr, err)
			}
		})
	}
}
