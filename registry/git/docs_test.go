package git

// The two user-facing halves of the #184 environment scrub are the migration
// table in docs/docs/registry/git.md and the diagnostic runGit appends to a
// failed git command. Both are pinned here rather than in resolver_test.go
// because both read unexported identifiers (strippedExecVars,
// strippedExecVarsHint) and resolver_test.go is package git_test.

import (
	"os"
	"regexp"
	"strings"
	"testing"
	"unicode/utf8"
)

const (
	// gitDocsPath is the migration guide this package's error message points
	// the user at. The test fails rather than skips when it is missing: a
	// moved or renamed docs file should break loudly, not rot quietly.
	gitDocsPath = "../../docs/docs/registry/git.md"

	// noConfigPhrase marks the one stripped variable git has no config key
	// for, so it can be documented without inventing a key.
	noConfigPhrase = "no git config equivalent"

	// migrationTableHeader is the header row of the GIT_* migration table.
	migrationTableHeader = "| Instead of | Use |"
)

var (
	gitVarRe   = regexp.MustCompile(`GIT_[A-Z_]+`)
	tableSepRe = regexp.MustCompile(`^\|-{4,}\|-{4,}\|$`)
)

// TestGitDocsCoverStrippedExecVars pins the migration guide against the list
// the code actually strips: every variable documented with a replacement, no
// run-on sentence of variable names, and the house form of the separator row.
func TestGitDocsCoverStrippedExecVars(t *testing.T) {
	lines := readGitDocs(t)

	t.Run("every stripped variable is documented", func(t *testing.T) {
		for _, name := range strippedExecVars {
			// Word boundaries so GIT_SSH_COMMAND does not vouch for GIT_SSH.
			re := regexp.MustCompile(`\b` + regexp.QuoteMeta(name) + `\b`)
			var documented bool
			for _, line := range lines {
				if !re.MatchString(line) {
					continue
				}
				if strings.HasPrefix(line, "|") || strings.Contains(line, noConfigPhrase) {
					documented = true
					break
				}
			}
			if !documented {
				t.Errorf("%s: %s appears in no migration-table row and in no line saying %q",
					gitDocsPath, name, noConfigPhrase)
			}
		}
	})

	// Three per line is the most any prose line needs: the paragraph after the
	// table names GIT_SSH_COMMAND, GIT_TERMINAL_PROMPT and GIT_ALLOW_PROTOCOL.
	t.Run("no run-on sentence of variable names", func(t *testing.T) {
		const maxVarsPerProseLine = 3

		for i, line := range lines {
			if strings.HasPrefix(line, "|") || strings.HasPrefix(line, "- ") {
				continue
			}
			if n := len(gitVarRe.FindAllString(line, -1)); n > maxVarsPerProseLine {
				t.Errorf("%s:%d: prose line names %d GIT_* variables (max %d); use a list item or a table row:\n%s",
					gitDocsPath, i+1, n, maxVarsPerProseLine, line)
			}
		}
	})

	t.Run("separator row is padded", func(t *testing.T) {
		for i, line := range lines {
			if line != migrationTableHeader {
				continue
			}
			if i+1 >= len(lines) {
				t.Fatalf("%s:%d: %q is the last line, no separator row follows",
					gitDocsPath, i+1, migrationTableHeader)
			}
			if sep := lines[i+1]; !tableSepRe.MatchString(sep) {
				t.Errorf("%s:%d: separator row %q does not match %s", gitDocsPath, i+2, sep, tableSepRe)
			}
			return
		}
		t.Fatalf("%s: no migration table found (expected a header line %q)",
			gitDocsPath, migrationTableHeader)
	})
}

// TestStrippedExecVarsHintFitsEightyColumns is the mechanical replacement for
// eyeballing the message in an 80-column terminal: it makes no claim about how
// any emulator wraps, only that no line is long enough to need wrapping.
func TestStrippedExecVarsHintFitsEightyColumns(t *testing.T) {
	const maxCols = 80

	tests := []struct {
		name     string
		stripped []string
	}{
		{name: "none set", stripped: nil},
		{name: "one set", stripped: []string{"GIT_SSH_COMMAND"}},
		{name: "all set", stripped: strippedExecVars},
		{name: "synthetic 40-rune name", stripped: []string{strings.Repeat("G", 40)}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			hint := strippedExecVarsHint(tt.stripped)

			for i, line := range strings.Split(hint, "\n") {
				if n := utf8.RuneCountInString(line); n > maxCols {
					t.Errorf("line %d is %d runes, want <= %d:\n%s", i+1, n, maxCols, line)
				}
				if strings.Contains(line, "\r") {
					t.Errorf("line %d contains a carriage return: %q", i+1, line)
				}
				if trimmed := strings.TrimRight(line, " \t"); trimmed != line {
					t.Errorf("line %d has trailing whitespace: %q", i+1, line)
				}
			}

			// ST1005: the hint is spliced onto the end of an error string.
			if last := hint[len(hint)-1:]; strings.ContainsAny(last, ".:\n") {
				t.Errorf("hint ends in %q; an error string must not end in punctuation or a newline", last)
			}
			if !strings.HasPrefix(hint, "event-spec deliberately removed") {
				t.Errorf("hint does not open with the documented phrase:\n%s", hint)
			}
			for _, name := range tt.stripped {
				if !strings.Contains(hint, "\n  "+name+"\n") {
					t.Errorf("hint does not name %s on its own indented line:\n%s", name, hint)
				}
			}
		})
	}
}

func readGitDocs(t *testing.T) []string {
	t.Helper()
	b, err := os.ReadFile(gitDocsPath)
	if err != nil {
		t.Fatalf("read migration guide: %v", err)
	}
	return strings.Split(strings.ReplaceAll(string(b), "\r\n", "\n"), "\n")
}
