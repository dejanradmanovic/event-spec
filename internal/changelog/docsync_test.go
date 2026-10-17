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

// mirrorFrontMatter carries no `title`: Docusaurus takes the page title, and with it the
// browser-tab <title>, from the mirrored `# Changelog` heading, the same way every
// sibling page does.
const mirrorFrontMatter = `---
sidebar_label: Changelog
sidebar_position: 99
---

`

// mirrorComment warns whoever opens the docs page that it is generated, and names the
// command that regenerates it.
const mirrorComment = "<!-- generated from CHANGELOG.md; run: go test ./internal/changelog -update -->"

// mirrorPage renders the docs page: front matter, the changelog's `# Changelog` heading,
// the generated-file comment, then the rest of the file verbatim. The comment goes *below*
// the heading because @docusaurus/utils' parseMarkdownContentTitle only recognises the H1
// when it is the first line of content; with the comment above it the page title degraded
// to the route slug and the browser tab read "changelog" instead of "Changelog".
func mirrorPage(t *testing.T, root string) string {
	t.Helper()
	h1, rest, found := strings.Cut(lf(root), "\n")
	if !found || h1 != "# Changelog" {
		t.Fatalf("CHANGELOG.md must start with a %q line, got %q", "# Changelog", h1)
	}
	return mirrorFrontMatter + h1 + "\n\n" + mirrorComment + "\n" + rest
}

func TestDocsMirrorInSync(t *testing.T) {
	want := mirrorPage(t, string(readRoot(t)))

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
