package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

const richSpecYAML = `$schema: "https://event-spec.io/schemas/event/v1"
name: product_viewed
display_name: "Product Viewed"
namespace: ecommerce
version: "1-2-0"
status: active
type: track
event_name: "Product Viewed"
description: "Fired when a user views a product."
changelog: "Added optional coupon_code property"
owner: "growth-team@example.com"
tags: [product, ecommerce]
properties:
  product_id:
    type: string
    required: true
    description: "The SKU or database ID of the product"
  price:
    type: number
    required: true
    minimum: 0
  category:
    type: string
    required: true
    enum: [clothing, electronics, other]
  coupon_code:
    type: string
    required: false
`

func TestDocsCmd_ExplicitDir_HTML(t *testing.T) {
	specDir := t.TempDir()
	outDir := t.TempDir()
	writeSpec(t, specDir, "product_viewed.yaml", richSpecYAML)

	cmd := newDocsCmd()
	var stdout bytes.Buffer
	cmd.SetOut(&stdout)
	cmd.SetErr(&bytes.Buffer{})
	cmd.SetArgs([]string{specDir, "--format", "html", "--out", outDir})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Index file must exist.
	indexPath := filepath.Join(outDir, "index.html")
	if _, err := os.Stat(indexPath); err != nil {
		t.Fatalf("index.html not created: %v", err)
	}

	// Event page must exist.
	eventPath := filepath.Join(outDir, "ecommerce", "product_viewed.html")
	if _, err := os.Stat(eventPath); err != nil {
		t.Fatalf("event page not created: %v", err)
	}

	// Index should link to the event.
	indexContent, _ := os.ReadFile(indexPath)
	if !strings.Contains(string(indexContent), "ecommerce/product_viewed.html") {
		t.Errorf("index.html missing link to event page")
	}
	if !strings.Contains(string(indexContent), "Event Catalog") {
		t.Errorf("index.html missing title")
	}

	// Event page should contain key fields.
	eventContent, _ := os.ReadFile(eventPath)
	body := string(eventContent)
	for _, want := range []string{
		"Product Viewed",
		"1-2-0",
		"active",
		"ecommerce",
		"product_id",
		"coupon_code",
		"growth-team@example.com",
		"Back to Index",
		"<!DOCTYPE html>",
	} {
		if !strings.Contains(body, want) {
			t.Errorf("event page missing %q", want)
		}
	}

	// HTML must be self-contained: no external URLs in href/src.
	for _, forbidden := range []string{"cdn.", "unpkg.com", "jsdelivr.net", "googleapis.com"} {
		if strings.Contains(body, forbidden) {
			t.Errorf("event page contains external dependency: %q", forbidden)
		}
	}

	if !strings.Contains(stdout.String(), "generated 1 event page(s)") {
		t.Errorf("unexpected stdout: %s", stdout.String())
	}
}

func TestDocsCmd_ExplicitDir_Markdown(t *testing.T) {
	specDir := t.TempDir()
	outDir := t.TempDir()
	writeSpec(t, specDir, "product_viewed.yaml", richSpecYAML)

	cmd := newDocsCmd()
	var stdout bytes.Buffer
	cmd.SetOut(&stdout)
	cmd.SetErr(&bytes.Buffer{})
	cmd.SetArgs([]string{specDir, "--format", "markdown", "--out", outDir})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	indexPath := filepath.Join(outDir, "index.md")
	if _, err := os.Stat(indexPath); err != nil {
		t.Fatalf("index.md not created: %v", err)
	}

	eventPath := filepath.Join(outDir, "ecommerce", "product_viewed.md")
	if _, err := os.Stat(eventPath); err != nil {
		t.Fatalf("event page not created: %v", err)
	}

	// Index should use Markdown table format.
	indexContent, _ := os.ReadFile(indexPath)
	if !strings.Contains(string(indexContent), "# Event Catalog") {
		t.Errorf("index.md missing H1 heading")
	}
	if !strings.Contains(string(indexContent), "ecommerce/product_viewed.md") {
		t.Errorf("index.md missing link to event")
	}

	// Event page should contain key fields as GFM.
	eventContent, _ := os.ReadFile(eventPath)
	body := string(eventContent)
	for _, want := range []string{
		"# Product Viewed",
		"1-2-0",
		"active",
		"ecommerce",
		"product_id",
		"Back to Index",
		"## Properties",
	} {
		if !strings.Contains(body, want) {
			t.Errorf("event.md missing %q", want)
		}
	}
}

func TestDocsCmd_DefaultFormatIsHTML(t *testing.T) {
	specDir := t.TempDir()
	outDir := t.TempDir()
	writeSpec(t, specDir, "product_viewed.yaml", richSpecYAML)

	cmd := newDocsCmd()
	cmd.SetOut(&bytes.Buffer{})
	cmd.SetErr(&bytes.Buffer{})
	cmd.SetArgs([]string{specDir, "--out", outDir})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if _, err := os.Stat(filepath.Join(outDir, "index.html")); err != nil {
		t.Error("default format should be html; index.html not found")
	}
}

func TestDocsCmd_InvalidFormat(t *testing.T) {
	specDir := t.TempDir()
	outDir := t.TempDir()
	writeSpec(t, specDir, "product_viewed.yaml", richSpecYAML)

	cmd := newDocsCmd()
	cmd.SetOut(&bytes.Buffer{})
	cmd.SetErr(&bytes.Buffer{})
	cmd.SetArgs([]string{specDir, "--format", "pdf", "--out", outDir})

	err := cmd.Execute()
	if err == nil {
		t.Fatal("expected error for invalid format")
	}
	if !strings.Contains(err.Error(), "html") || !strings.Contains(err.Error(), "markdown") {
		t.Errorf("error should mention valid formats, got: %v", err)
	}
}

func TestDocsCmd_NoEvents(t *testing.T) {
	outDir := t.TempDir()
	emptyDir := t.TempDir()

	cmd := newDocsCmd()
	var stdout bytes.Buffer
	cmd.SetOut(&stdout)
	cmd.SetErr(&bytes.Buffer{})
	cmd.SetArgs([]string{emptyDir, "--out", outDir})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(stdout.String(), "no event specs found") {
		t.Errorf("expected 'no event specs found' message, got: %s", stdout.String())
	}
}

func TestDocsCmd_MultipleNamespaces(t *testing.T) {
	specDir := t.TempDir()
	outDir := t.TempDir()

	writeSpec(t, specDir, "product_viewed.yaml", richSpecYAML)
	writeSpec(t, specDir, "user_signed_up.yaml", `$schema: "https://event-spec.io/schemas/event/v1"
name: user_signed_up
namespace: auth
version: "1-0-0"
status: active
type: track
event_name: "User Signed Up"
`)

	cmd := newDocsCmd()
	cmd.SetOut(&bytes.Buffer{})
	cmd.SetErr(&bytes.Buffer{})
	cmd.SetArgs([]string{specDir, "--format", "markdown", "--out", outDir})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Both namespace dirs must be created.
	for _, p := range []string{
		filepath.Join(outDir, "ecommerce", "product_viewed.md"),
		filepath.Join(outDir, "auth", "user_signed_up.md"),
	} {
		if _, err := os.Stat(p); err != nil {
			t.Errorf("expected %s to exist: %v", p, err)
		}
	}

	// Index must contain both namespaces.
	indexContent, _ := os.ReadFile(filepath.Join(outDir, "index.md"))
	if !strings.Contains(string(indexContent), "## ecommerce") {
		t.Error("index missing ecommerce namespace")
	}
	if !strings.Contains(string(indexContent), "## auth") {
		t.Error("index missing auth namespace")
	}
}

func TestDocsCmd_FromWorkspace(t *testing.T) {
	root := t.TempDir()
	specsDir := filepath.Join(root, "specs")
	outDir := filepath.Join(root, "out")
	if err := os.MkdirAll(specsDir, 0o755); err != nil {
		t.Fatal(err)
	}
	writeSpec(t, specsDir, "product_viewed.yaml", richSpecYAML)
	writeWorkspaceConfig(t, root, "specs")
	withWorkDir(t, root)

	cmd := newDocsCmd()
	var stdout bytes.Buffer
	cmd.SetOut(&stdout)
	cmd.SetErr(&bytes.Buffer{})
	cmd.SetArgs([]string{"--format", "markdown", "--out", outDir})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if _, err := os.Stat(filepath.Join(outDir, "index.md")); err != nil {
		t.Error("index.md not created from workspace config")
	}
}

func TestDocsCmd_WorkspaceDefaultFormat(t *testing.T) {
	root := t.TempDir()
	specsDir := filepath.Join(root, "specs")
	outDir := filepath.Join(root, "out")
	if err := os.MkdirAll(specsDir, 0o755); err != nil {
		t.Fatal(err)
	}
	writeSpec(t, specsDir, "product_viewed.yaml", richSpecYAML)

	// Write workspace config with docs.format = markdown.
	if err := os.WriteFile(filepath.Join(root, "event-spec.yaml"), []byte(`version: 1
workspace: test
registry:
  mode: local
specs_dir: specs
docs:
  format: markdown
`), 0o644); err != nil {
		t.Fatal(err)
	}
	withWorkDir(t, root)

	cmd := newDocsCmd()
	cmd.SetOut(&bytes.Buffer{})
	cmd.SetErr(&bytes.Buffer{})
	cmd.SetArgs([]string{"--out", outDir})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if _, err := os.Stat(filepath.Join(outDir, "index.md")); err != nil {
		t.Error("workspace docs.format=markdown should produce index.md")
	}
}
