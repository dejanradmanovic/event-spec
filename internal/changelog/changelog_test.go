package changelog_test

import (
	"errors"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
	"testing"

	"github.com/dejanradmanovic/event-spec/internal/changelog"
)

// rootChangelog is the real file the release workflow extracts from, two levels up from
// this package.
const rootChangelog = "../../CHANGELOG.md"

func readFixture(t *testing.T) []byte {
	t.Helper()
	doc, err := os.ReadFile(filepath.Join("testdata", "sample-changelog.md"))
	if err != nil {
		t.Fatalf("read fixture: %v", err)
	}
	return doc
}

func readRoot(t *testing.T) []byte {
	t.Helper()
	doc, err := os.ReadFile(rootChangelog)
	if err != nil {
		t.Fatalf("read %s: %v", rootChangelog, err)
	}
	return doc
}

func TestSection(t *testing.T) {
	doc := readFixture(t)

	const middle = "### Added\n\n- middle section, first bullet\n- middle section, second bullet\n\n### Fixed\n\n- middle section, a fix"

	tests := []struct {
		name    string
		version string
		want    string
	}{
		{
			name:    "first released version in the file",
			version: "2.0.0",
			want:    "### Breaking\n\n- the first released section in the file",
		},
		{
			name:    "middle version",
			version: "1.2.3",
			want:    middle,
		},
		{
			name:    "middle version with a v prefix",
			version: "v1.2.3",
			want:    middle,
		},
		{
			name:    "last released version stops before the Earlier releases tail",
			version: "0.9.0",
			want:    "### Added\n\n- the last released section in the file",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := changelog.Section(doc, tt.version)
			if err != nil {
				t.Fatalf("Section(%q): %v", tt.version, err)
			}
			if got != tt.want {
				t.Errorf("Section(%q) =\n%s\n--- want ---\n%s", tt.version, got, tt.want)
			}
		})
	}
}

func TestSectionDoesNotBleedIntoNextHeading(t *testing.T) {
	doc := readFixture(t)

	middle, err := changelog.Section(doc, "1.2.3")
	if err != nil {
		t.Fatalf("Section: %v", err)
	}
	if strings.Contains(middle, "1.0.0") {
		t.Errorf("section for 1.2.3 reaches into the next version heading:\n%s", middle)
	}

	last, err := changelog.Section(doc, "0.9.0")
	if err != nil {
		t.Fatalf("Section: %v", err)
	}
	if strings.Contains(last, "Earlier releases") {
		t.Errorf("section for 0.9.0 reaches into the Earlier releases tail:\n%s", last)
	}
}

func TestSectionErrors(t *testing.T) {
	doc := readFixture(t)

	tests := []struct {
		name         string
		version      string
		wantNotFound bool
		wantInErr    string
	}{
		{name: "absent version", version: "3.1.4", wantNotFound: true, wantInErr: "3.1.4"},
		{name: "absent version with a v prefix", version: "v3.1.4", wantNotFound: true, wantInErr: "3.1.4"},
		{name: "empty section", version: "1.0.0", wantInErr: "empty"},
		{name: "no version given", version: "  ", wantInErr: "no version given"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := changelog.Section(doc, tt.version)
			if err == nil {
				t.Fatalf("Section(%q) = %q, want error", tt.version, got)
			}
			if errors.Is(err, changelog.ErrSectionNotFound) != tt.wantNotFound {
				t.Errorf("errors.Is(err, ErrSectionNotFound) = %v, want %v (err: %v)",
					errors.Is(err, changelog.ErrSectionNotFound), tt.wantNotFound, err)
			}
			if !strings.Contains(err.Error(), tt.wantInErr) {
				t.Errorf("error %q does not mention %q", err, tt.wantInErr)
			}
		})
	}
}

func TestSectionCRLF(t *testing.T) {
	doc := []byte(strings.ReplaceAll(string(readFixture(t)), "\n", "\r\n"))

	got, err := changelog.Section(doc, "0.9.0")
	if err != nil {
		t.Fatalf("Section: %v", err)
	}
	want := "### Added\n\n- the last released section in the file"
	if got != want {
		t.Errorf("Section on CRLF input =\n%q\nwant\n%q", got, want)
	}
}

func TestUnreleased(t *testing.T) {
	got, err := changelog.Unreleased(readFixture(t))
	if err != nil {
		t.Fatalf("Unreleased: %v", err)
	}
	want := "### Added\n\n- a change with no version yet"
	if got != want {
		t.Errorf("Unreleased =\n%s\nwant\n%s", got, want)
	}

	noHeading := []byte("# Changelog\n\n## [1.0.0] - 2026-01-01\n\n- x\n")
	if _, err := changelog.Unreleased(noHeading); !errors.Is(err, changelog.ErrSectionNotFound) {
		t.Errorf("Unreleased on a file without the heading: err = %v, want ErrSectionNotFound", err)
	}
}

// TestUnreleasedEmptyIsNotAnError covers the state a release promotion leaves behind: the
// heading is there with nothing under it yet. Erroring here is what used to make the
// documented promotion recipe fail the test suite.
func TestUnreleasedEmptyIsNotAnError(t *testing.T) {
	promoted := []byte("# Changelog\n\n## [Unreleased]\n\n## [1.0.0] - 2026-01-01\n\n### Breaking\n\n- x\n")

	got, err := changelog.Unreleased(promoted)
	if err != nil {
		t.Fatalf("Unreleased on a freshly promoted file: %v", err)
	}
	if got != "" {
		t.Errorf("Unreleased = %q, want an empty body", got)
	}
}

func TestVersions(t *testing.T) {
	got, err := changelog.Versions(readFixture(t))
	if err != nil {
		t.Fatalf("Versions: %v", err)
	}
	want := []string{"2.0.0", "1.2.3", "1.0.0", "0.9.0"}
	if !slices.Equal(got, want) {
		t.Errorf("Versions = %v, want %v", got, want)
	}
}

func TestVersionsHeadingFormatErrors(t *testing.T) {
	tests := []struct {
		name      string
		doc       string
		wantInErr string
	}{
		{
			name:      "missing date",
			doc:       "# Changelog\n\n## [1.2.3]\n\n- x\n",
			wantInErr: `## [1.2.3]`,
		},
		{
			name:      "two-part version",
			doc:       "# Changelog\n\n## [1.2] - 2026-01-01\n\n- x\n",
			wantInErr: `## [1.2] - 2026-01-01`,
		},
		{
			name:      "duplicate Unreleased heading",
			doc:       "# Changelog\n\n## [Unreleased]\n\n- x\n\n## [Unreleased]\n\n- y\n",
			wantInErr: `duplicate "## [Unreleased]" heading`,
		},
		{
			name:      "duplicate version",
			doc:       "# Changelog\n\n## [1.2.3] - 2026-01-01\n\n- x\n\n## [1.2.3] - 2026-02-01\n\n- y\n",
			wantInErr: "duplicate section for version 1.2.3",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := changelog.Versions([]byte(tt.doc))
			if err == nil {
				t.Fatalf("Versions = %v, want error", got)
			}
			if !strings.Contains(err.Error(), tt.wantInErr) {
				t.Errorf("error %q does not mention %q", err, tt.wantInErr)
			}
		})
	}
}

func TestVersionsCRLF(t *testing.T) {
	doc := []byte(strings.ReplaceAll(string(readFixture(t)), "\n", "\r\n"))

	got, err := changelog.Versions(doc)
	if err != nil {
		t.Fatalf("Versions on CRLF input: %v", err)
	}
	want := []string{"2.0.0", "1.2.3", "1.0.0", "0.9.0"}
	if !slices.Equal(got, want) {
		t.Errorf("Versions = %v, want %v", got, want)
	}
}

// TestRootChangelogStructure guards the real CHANGELOG.md: the release workflow extracts
// from it, and the docs mirror renders it with onBrokenLinks: 'throw'.
func TestRootChangelogStructure(t *testing.T) {
	doc := readRoot(t)
	text := strings.ReplaceAll(string(doc), "\r\n", "\n")
	lines := strings.Split(text, "\n")

	if lines[0] != "# Changelog" {
		t.Errorf("first line = %q, want %q", lines[0], "# Changelog")
	}

	preamble := lines
	for i, line := range lines {
		if strings.HasPrefix(line, "## ") {
			preamble = lines[:i]
			break
		}
	}
	preambleText := strings.Join(preamble, "\n")
	for _, link := range []string{"https://keepachangelog.com/", "https://semver.org/"} {
		if !strings.Contains(preambleText, link) {
			t.Errorf("preamble does not link %s:\n%s", link, preambleText)
		}
	}

	versionHeading := regexp.MustCompile(`^## \[\d+\.\d+\.\d+\] - \d{4}-\d{2}-\d{2}$`)
	var unreleased int
	for i, line := range lines {
		switch {
		case line == "## [Unreleased]":
			unreleased++
		case strings.HasPrefix(line, "## [") && !versionHeading.MatchString(line):
			t.Errorf("line %d: heading %q is neither %q nor %q",
				i+1, line, "## [Unreleased]", "## [x.y.z] - YYYY-MM-DD")
		}
	}
	if unreleased != 1 {
		t.Errorf("found %d %q headings, want exactly 1", unreleased, "## [Unreleased]")
	}

	// Versions applies the same heading rules and additionally rejects duplicates.
	if _, err := changelog.Versions(doc); err != nil {
		t.Errorf("Versions(CHANGELOG.md): %v", err)
	}

	// The file is mirrored into Docusaurus, which resolves links against the docs route,
	// so a relative repository path would break the docs build.
	for _, rel := range []string{"](./", "](../", "](docs/"} {
		if strings.Contains(text, rel) {
			t.Errorf("CHANGELOG.md contains a relative markdown link %q; links must be absolute", rel)
		}
	}
}

// TestBreakingEntryGitEnv pins the migration content a user upgrading past 8c00715
// (issue #184, PR #197) needs before the upgrade, not after it. The entry is found by
// content, not by which section holds it: promoting it from ## [Unreleased] to a version
// heading is a routine release step and must not need a test edit.
func TestBreakingEntryGitEnv(t *testing.T) {
	breaking, ok := gitEnvBreakingEntry(lf(string(readRoot(t))))
	if !ok {
		t.Fatalf("no %q subsection in CHANGELOG.md mentions GIT_SSH_COMMAND", "### Breaking")
	}

	want := []string{
		"GIT_TERMINAL_PROMPT=0",
		"GIT_ALLOW_PROTOCOL=file:git:http:https:ssh",
		"core.sshCommand",
		"core.askPass",
		"core.gitProxy",
		"diff.external",
		// The one secret-bearing migration row needs its caution; see the security
		// review of #204 and of this job.
		"credential.helper",
		"184",
		"197",
	}
	for _, w := range want {
		if !strings.Contains(breaking, w) {
			t.Errorf("Breaking entry does not mention %q", w)
		}
	}

	// Every variable the resolver strips and names in an error has to be answered either
	// by a migration-table row or by the sentence recording that it has no config
	// equivalent (GIT_EXEC_PATH). Same two-place rule as registry/git's own docs test, so
	// this copy and docs/docs/registry/git.md can be checked the same way.
	for _, name := range []string{
		"GIT_SSH_COMMAND",
		"GIT_SSH",
		"GIT_ASKPASS",
		"GIT_PROXY_COMMAND",
		"GIT_EXTERNAL_DIFF",
		"GIT_EXEC_PATH",
		"GIT_CONFIG_COUNT",
		"GIT_CONFIG_KEY_",
	} {
		if !migrationDocumented(breaking, name) {
			t.Errorf("Breaking entry has no migration-table row for %q and no %q sentence naming it",
				name, "no git config equivalent")
		}
	}
}

// gitEnvBreakingEntry returns the "### Breaking" subsection that carries the GIT_* entry,
// wherever in the file it currently lives.
func gitEnvBreakingEntry(text string) (string, bool) {
	for _, s := range breakingSubsections(text) {
		if strings.Contains(s, "GIT_SSH_COMMAND") {
			return s, true
		}
	}
	return "", false
}

// breakingSubsections returns the body of every "### Breaking" subsection in file order.
func breakingSubsections(text string) []string {
	var (
		out   []string
		lines []string
		open  bool
	)
	flush := func() {
		if open {
			out = append(out, strings.TrimSpace(strings.Join(lines, "\n")))
			lines, open = nil, false
		}
	}
	for _, line := range strings.Split(text, "\n") {
		switch {
		case line == "### Breaking":
			flush()
			open = true
		case strings.HasPrefix(line, "## ") || strings.HasPrefix(line, "### "):
			flush()
		case open:
			lines = append(lines, line)
		}
	}
	flush()
	return out
}

// migrationDocumented reports whether name is answered by a migration-table row or by a
// line stating it has no git config equivalent.
func migrationDocumented(section, name string) bool {
	for _, line := range strings.Split(section, "\n") {
		line = strings.TrimSpace(line)
		if !strings.Contains(line, name) {
			continue
		}
		if strings.HasPrefix(line, "|") || strings.Contains(line, "no git config equivalent") {
			return true
		}
	}
	return false
}

// TestEarlierReleases pins the recorded no-backfill decision.
func TestEarlierReleases(t *testing.T) {
	text := strings.ReplaceAll(string(readRoot(t)), "\r\n", "\n")

	tail, ok := subsection(text, "## Earlier releases")
	if !ok {
		t.Fatalf("CHANGELOG.md has no %q section", "## Earlier releases")
	}
	for _, w := range []string{"v0.4.2", "https://github.com/dejanradmanovic/event-spec/releases"} {
		if !strings.Contains(tail, w) {
			t.Errorf("Earlier releases section does not mention %q", w)
		}
	}
}

// subsection returns the body under heading, stopping at the next heading of the same
// level. found reports whether the heading was there at all.
func subsection(text, heading string) (body string, found bool) {
	prefix := strings.SplitN(heading, " ", 2)[0] + " "
	var lines []string
	for _, line := range strings.Split(text, "\n") {
		if !found {
			found = line == heading
			continue
		}
		if strings.HasPrefix(line, prefix) {
			break
		}
		lines = append(lines, line)
	}
	return strings.TrimSpace(strings.Join(lines, "\n")), found
}
