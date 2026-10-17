package changelog_test

import (
	"flag"
	"os"
	"strings"
	"testing"
)

var update = flag.Bool("update", false, "write docs/docs/changelog.md from CHANGELOG.md instead of comparing")

// docsMirror is the Docusaurus page that renders the root changelog on event-spec.io. It
// is committed rather than generated at build time so that deploy-docs.yml's
// `paths: docs/**` trigger still fires when the changelog changes.
const docsMirror = "../../docs/docs/changelog.md"

// mirrorHeader is everything the docs page adds on top of CHANGELOG.md. No `title` in the
// front matter: the mirrored `# Changelog` is the page's only H1.
const mirrorHeader = `---
sidebar_label: Changelog
sidebar_position: 99
---

<!-- generated from CHANGELOG.md; run: go test ./internal/changelog -update -->

`

func TestDocsMirrorInSync(t *testing.T) {
	want := mirrorHeader + lf(string(readRoot(t)))

	if *update {
		if err := os.WriteFile(docsMirror, []byte(want), 0o644); err != nil {
			t.Fatalf("write %s: %v", docsMirror, err)
		}
		t.Logf("wrote %s", docsMirror)
		return
	}

	got, err := os.ReadFile(docsMirror)
	if err != nil {
		t.Fatalf("read %s: %v", docsMirror, err)
	}
	if lf(string(got)) != want {
		t.Errorf("%s is out of sync with CHANGELOG.md; run: go test ./internal/changelog -update\n--- want ---\n%s\n--- got ---\n%s",
			docsMirror, want, got)
	}
}

// lf normalises line endings: the repository is developed on Windows, where a working
// tree copy of either file can carry CRLF.
func lf(s string) string {
	return strings.ReplaceAll(s, "\r\n", "\n")
}
