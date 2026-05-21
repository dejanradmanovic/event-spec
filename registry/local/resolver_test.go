package local_test

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/dejanradmanovic/event-spec/registry"
	"github.com/dejanradmanovic/event-spec/registry/local"
	"github.com/dejanradmanovic/event-spec/spec"
)

// ---- fixture helpers ----

// writeYAML writes content to dir/relPath, creating parent directories as needed.
func writeYAML(t *testing.T, dir, relPath, content string) {
	t.Helper()
	full := filepath.Join(dir, relPath)
	if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.WriteFile(full, []byte(content), 0o644); err != nil {
		t.Fatalf("write %s: %v", relPath, err)
	}
}

const schemaHeader = `$schema: "https://event-spec.io/schemas/event/v1"` + "\n"

func eventYAML(namespace, name, version, status string, tags ...string) string {
	tagLine := ""
	if len(tags) > 0 {
		tagLine = "tags: ["
		for i, t := range tags {
			if i > 0 {
				tagLine += ", "
			}
			tagLine += t
		}
		tagLine += "]\n"
	}
	return schemaHeader +
		"name: " + name + "\n" +
		"namespace: " + namespace + "\n" +
		"version: " + version + "\n" +
		"status: " + status + "\n" +
		tagLine +
		"event_name: " + name + "\n" +
		"type: track\n" +
		"properties: {}\n"
}

func sourceYAML(name string) string {
	return "name: " + name + "\n" +
		"language: go\n" +
		"events:\n" +
		"  - ecommerce/**\n" +
		"output:\n" +
		"  path: ./generated\n"
}

func destinationYAML(name, provider string) string {
	return "name: " + name + "\n" +
		"provider: " + provider + "\n"
}

// newResolver creates a Resolver backed by a temp directory tree.
func newResolver(t *testing.T, setup func(specsDir, sourcesDir, destsDir string)) *local.Resolver {
	t.Helper()
	root := t.TempDir()
	specsDir := filepath.Join(root, "specs")
	sourcesDir := filepath.Join(root, "sources")
	destsDir := filepath.Join(root, "destinations")
	for _, d := range []string{specsDir, sourcesDir, destsDir} {
		if err := os.MkdirAll(d, 0o755); err != nil {
			t.Fatalf("mkdir %s: %v", d, err)
		}
	}
	setup(specsDir, sourcesDir, destsDir)
	r, err := local.New(local.Config{
		SpecsDir:        specsDir,
		SourcesDir:      sourcesDir,
		DestinationsDir: destsDir,
	})
	if err != nil {
		t.Fatalf("git.New: %v", err)
	}
	return r
}

// ---- tests ----

func TestResolver_Walk_skipsFilesWithoutSchemaHeader(t *testing.T) {
	r := newResolver(t, func(specsDir, _, _ string) {
		writeYAML(t, specsDir, "noise.yaml", "name: not-an-event\n")
		writeYAML(t, specsDir, "ecommerce/product_viewed/1-0-0.yaml",
			eventYAML("ecommerce", "product_viewed", "1-0-0", "active"))
	})

	ctx := context.Background()
	events, err := r.ListEvents(ctx, registry.ListFilter{})
	if err != nil {
		t.Fatalf("ListEvents: %v", err)
	}
	if len(events) != 1 {
		t.Fatalf("want 1 event (schema-valid only), got %d", len(events))
	}
}

func TestResolver_Walk_recursiveSubdirectories(t *testing.T) {
	r := newResolver(t, func(specsDir, _, _ string) {
		writeYAML(t, specsDir, "ecommerce/product_viewed/1-0-0.yaml",
			eventYAML("ecommerce", "product_viewed", "1-0-0", "active"))
		writeYAML(t, specsDir, "auth/user_signed_up/1-0-0.yaml",
			eventYAML("auth", "user_signed_up", "1-0-0", "active"))
	})

	ctx := context.Background()
	events, err := r.ListEvents(ctx, registry.ListFilter{})
	if err != nil {
		t.Fatalf("ListEvents: %v", err)
	}
	if len(events) != 2 {
		t.Errorf("want 2 events, got %d", len(events))
	}
}

func TestResolver_GetEvent_specificVersion(t *testing.T) {
	r := newResolver(t, func(specsDir, _, _ string) {
		writeYAML(t, specsDir, "ecommerce/product_viewed/1-0-0.yaml",
			eventYAML("ecommerce", "product_viewed", "1-0-0", "active"))
		writeYAML(t, specsDir, "ecommerce/product_viewed/1-2-0.yaml",
			eventYAML("ecommerce", "product_viewed", "1-2-0", "active"))
	})

	ctx := context.Background()
	def, err := r.GetEvent(ctx, "ecommerce", "product_viewed", "1-0-0")
	if err != nil {
		t.Fatalf("GetEvent: %v", err)
	}
	if def.Version != "1-0-0" {
		t.Errorf("want version 1-0-0, got %s", def.Version)
	}
}

func TestResolver_GetEvent_noVersion_returnsHighestActive(t *testing.T) {
	r := newResolver(t, func(specsDir, _, _ string) {
		writeYAML(t, specsDir, "ecommerce/product_viewed/1-0-0.yaml",
			eventYAML("ecommerce", "product_viewed", "1-0-0", "active"))
		writeYAML(t, specsDir, "ecommerce/product_viewed/1-2-0.yaml",
			eventYAML("ecommerce", "product_viewed", "1-2-0", "active"))
		writeYAML(t, specsDir, "ecommerce/product_viewed/2-0-0.yaml",
			eventYAML("ecommerce", "product_viewed", "2-0-0", "deprecated"))
	})

	ctx := context.Background()
	def, err := r.GetEvent(ctx, "ecommerce", "product_viewed", "")
	if err != nil {
		t.Fatalf("GetEvent: %v", err)
	}
	if def.Version != "1-2-0" {
		t.Errorf("want highest active version 1-2-0, got %s", def.Version)
	}
}

func TestResolver_GetEvent_multipleActiveVersions_coexist(t *testing.T) {
	// Both 1-0-0 and 1-2-0 are active; the resolver must index both.
	r := newResolver(t, func(specsDir, _, _ string) {
		writeYAML(t, specsDir, "ecommerce/product_viewed/1-0-0.yaml",
			eventYAML("ecommerce", "product_viewed", "1-0-0", "active"))
		writeYAML(t, specsDir, "ecommerce/product_viewed/1-2-0.yaml",
			eventYAML("ecommerce", "product_viewed", "1-2-0", "active"))
	})

	ctx := context.Background()
	v100, err := r.GetEvent(ctx, "ecommerce", "product_viewed", "1-0-0")
	if err != nil {
		t.Fatalf("GetEvent 1-0-0: %v", err)
	}
	v120, err := r.GetEvent(ctx, "ecommerce", "product_viewed", "1-2-0")
	if err != nil {
		t.Fatalf("GetEvent 1-2-0: %v", err)
	}
	if v100.Version != "1-0-0" || v120.Version != "1-2-0" {
		t.Errorf("unexpected versions: %s, %s", v100.Version, v120.Version)
	}
}

func TestResolver_GetEvent_versionPinning_viaExplicitVersion(t *testing.T) {
	// Callers implementing version_pinning pass the pinned version string directly;
	// the resolver must honour any explicit version regardless of active status.
	r := newResolver(t, func(specsDir, _, _ string) {
		writeYAML(t, specsDir, "ecommerce/product_viewed/1-0-0.yaml",
			eventYAML("ecommerce", "product_viewed", "1-0-0", "active"))
		writeYAML(t, specsDir, "ecommerce/product_viewed/1-2-0.yaml",
			eventYAML("ecommerce", "product_viewed", "1-2-0", "active"))
	})

	ctx := context.Background()
	// Simulate a source pinned to 1-0-0 (the older version).
	def, err := r.GetEvent(ctx, "ecommerce", "product_viewed", "1-0-0")
	if err != nil {
		t.Fatalf("GetEvent pinned to 1-0-0: %v", err)
	}
	if def.Version != "1-0-0" {
		t.Errorf("version pinning: want 1-0-0, got %s", def.Version)
	}
}

func TestResolver_GetEvent_notFound(t *testing.T) {
	r := newResolver(t, func(_, _, _ string) {})
	_, err := r.GetEvent(context.Background(), "ns", "missing", "1-0-0")
	if !errors.Is(err, registry.ErrNotFound) {
		t.Errorf("want ErrNotFound, got %v", err)
	}
}

func TestResolver_GetEvent_noActiveVersion_notFound(t *testing.T) {
	r := newResolver(t, func(specsDir, _, _ string) {
		writeYAML(t, specsDir, "ecommerce/product_viewed/1-0-0.yaml",
			eventYAML("ecommerce", "product_viewed", "1-0-0", "deprecated"))
	})
	_, err := r.GetEvent(context.Background(), "ecommerce", "product_viewed", "")
	if !errors.Is(err, registry.ErrNotFound) {
		t.Errorf("all deprecated: want ErrNotFound, got %v", err)
	}
}

func TestResolver_ListEvents_filterByNamespace(t *testing.T) {
	r := newResolver(t, func(specsDir, _, _ string) {
		writeYAML(t, specsDir, "ecommerce/product_viewed/1-0-0.yaml",
			eventYAML("ecommerce", "product_viewed", "1-0-0", "active"))
		writeYAML(t, specsDir, "auth/user_signed_up/1-0-0.yaml",
			eventYAML("auth", "user_signed_up", "1-0-0", "active"))
	})

	events, err := r.ListEvents(context.Background(), registry.ListFilter{Namespace: "auth"})
	if err != nil {
		t.Fatalf("ListEvents: %v", err)
	}
	if len(events) != 1 || events[0].Namespace != "auth" {
		t.Errorf("namespace filter: want 1 auth event, got %d", len(events))
	}
}

func TestResolver_ListEvents_filterByStatus(t *testing.T) {
	r := newResolver(t, func(specsDir, _, _ string) {
		writeYAML(t, specsDir, "ecommerce/product_viewed/1-0-0.yaml",
			eventYAML("ecommerce", "product_viewed", "1-0-0", "active"))
		writeYAML(t, specsDir, "ecommerce/product_viewed/2-0-0.yaml",
			eventYAML("ecommerce", "product_viewed", "2-0-0", "deprecated"))
	})

	events, err := r.ListEvents(context.Background(), registry.ListFilter{Status: spec.StatusActive})
	if err != nil {
		t.Fatalf("ListEvents: %v", err)
	}
	if len(events) != 1 || events[0].Version != "1-0-0" {
		t.Errorf("status filter: want 1 active event, got %d", len(events))
	}
}

func TestResolver_ListEvents_filterByTags(t *testing.T) {
	r := newResolver(t, func(specsDir, _, _ string) {
		writeYAML(t, specsDir, "ecommerce/product_viewed/1-0-0.yaml",
			eventYAML("ecommerce", "product_viewed", "1-0-0", "active", "funnel", "revenue"))
		writeYAML(t, specsDir, "auth/user_signed_up/1-0-0.yaml",
			eventYAML("auth", "user_signed_up", "1-0-0", "active", "auth"))
	})

	events, err := r.ListEvents(context.Background(), registry.ListFilter{Tags: []string{"revenue"}})
	if err != nil {
		t.Fatalf("ListEvents: %v", err)
	}
	if len(events) != 1 || events[0].Name != "product_viewed" {
		t.Errorf("tag filter: want 1 revenue event, got %d", len(events))
	}
}

func TestResolver_ListEvents_deduplicatesMultipleVersions(t *testing.T) {
	r := newResolver(t, func(specsDir, _, _ string) {
		writeYAML(t, specsDir, "ecommerce/product_viewed/1-0-0.yaml",
			eventYAML("ecommerce", "product_viewed", "1-0-0", "active"))
		writeYAML(t, specsDir, "ecommerce/product_viewed/2-0-0.yaml",
			eventYAML("ecommerce", "product_viewed", "2-0-0", "active"))
	})

	ctx := context.Background()

	// ListEvents returns one event (the latest version).
	events, err := r.ListEvents(ctx, registry.ListFilter{})
	if err != nil {
		t.Fatalf("ListEvents: %v", err)
	}
	if len(events) != 1 {
		t.Fatalf("ListEvents: want 1 deduplicated event, got %d", len(events))
	}
	if events[0].Version != "2-0-0" {
		t.Errorf("ListEvents: want latest version 2-0-0, got %s", events[0].Version)
	}

	// ListAllEvents returns all versions without deduplication.
	all, err := r.ListAllEvents(ctx, registry.ListFilter{})
	if err != nil {
		t.Fatalf("ListAllEvents: %v", err)
	}
	if len(all) != 2 {
		t.Fatalf("ListAllEvents: want 2 versions, got %d", len(all))
	}
}

func TestResolver_GetSource_found(t *testing.T) {
	r := newResolver(t, func(_, sourcesDir, _ string) {
		writeYAML(t, sourcesDir, "web-app.yaml", sourceYAML("web-app"))
	})

	src, err := r.GetSource(context.Background(), "web-app")
	if err != nil {
		t.Fatalf("GetSource: %v", err)
	}
	if src.Name != "web-app" {
		t.Errorf("want name web-app, got %s", src.Name)
	}
}

func TestResolver_GetSource_notFound(t *testing.T) {
	r := newResolver(t, func(_, _, _ string) {})
	_, err := r.GetSource(context.Background(), "missing")
	if !errors.Is(err, registry.ErrNotFound) {
		t.Errorf("want ErrNotFound, got %v", err)
	}
}

func TestResolver_GetDestination_found(t *testing.T) {
	r := newResolver(t, func(_, _, destsDir string) {
		writeYAML(t, destsDir, "amplitude.yaml", destinationYAML("amplitude", "amplitude"))
	})

	dst, err := r.GetDestination(context.Background(), "amplitude")
	if err != nil {
		t.Fatalf("GetDestination: %v", err)
	}
	if dst.Name != "amplitude" {
		t.Errorf("want name amplitude, got %s", dst.Name)
	}
}

func TestResolver_GetDestination_notFound(t *testing.T) {
	r := newResolver(t, func(_, _, _ string) {})
	_, err := r.GetDestination(context.Background(), "missing")
	if !errors.Is(err, registry.ErrNotFound) {
		t.Errorf("want ErrNotFound, got %v", err)
	}
}

func TestResolver_PublishEvent_returnsErrReadOnly(t *testing.T) {
	r := newResolver(t, func(_, _, _ string) {})
	err := r.PublishEvent(context.Background(), spec.EventDef{})
	if !errors.Is(err, registry.ErrReadOnly) {
		t.Errorf("want ErrReadOnly, got %v", err)
	}
}

func TestResolver_Diff_bothVersionsMustExist(t *testing.T) {
	r := newResolver(t, func(specsDir, _, _ string) {
		writeYAML(t, specsDir, "ecommerce/product_viewed/1-0-0.yaml",
			eventYAML("ecommerce", "product_viewed", "1-0-0", "active"))
		writeYAML(t, specsDir, "ecommerce/product_viewed/1-2-0.yaml",
			eventYAML("ecommerce", "product_viewed", "1-2-0", "active"))
	})

	ctx := context.Background()
	_, err := r.Diff(ctx, "ecommerce", "product_viewed", "1-0-0", "1-2-0")
	if err != nil {
		t.Errorf("Diff with valid versions: unexpected error: %v", err)
	}

	_, err = r.Diff(ctx, "ecommerce", "product_viewed", "1-0-0", "9-9-9")
	if !errors.Is(err, registry.ErrNotFound) {
		t.Errorf("Diff missing 'to' version: want ErrNotFound, got %v", err)
	}
}
