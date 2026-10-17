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
			if len(migrationLinesFor(lines, name)) == 0 {
				t.Errorf("%s: %s appears in no migration-table row and in no line saying %q",
					gitDocsPath, name, noConfigPhrase)
			}
		}
	})

	// The terminal hint and the migration guide are two renderings of
	// execVarFixes, and a user who reads one and then the other must not be
	// told two different things. Editing either side alone fails here.
	t.Run("the guide states the same fix as the hint", func(t *testing.T) {
		body := strings.Join(lines, "\n")

		for _, fix := range execVarFixes {
			found := migrationLinesFor(lines, fix.name)
			if len(found) == 0 {
				continue // already reported by the subtest above
			}

			want := fix.config
			if want == "" {
				want = noConfigPhrase
			}
			var stated bool
			for _, line := range found {
				if strings.Contains(line, want) {
					stated = true
					break
				}
			}
			if !stated {
				t.Errorf("%s: the hint offers %s %q for %s, but no line documenting it says %q:\n%s",
					gitDocsPath, "git config", want, fix.name, want, strings.Join(found, "\n"))
			}

			// The caveats are prose and worded for their medium, so only the
			// phrase that carries the fact is pinned, and anywhere in the page
			// counts: the credential caution sits below the table, not in a row.
			if fix.docsPhrase != "" && !strings.Contains(body, fix.docsPhrase) {
				t.Errorf("%s: the hint tells %s users %q, but the page never says %q",
					gitDocsPath, fix.name, fix.caveat, fix.docsPhrase)
			}
		}
	})

	// The hint points at the published page; if the file moves, the pointer is
	// dead and only a test like this notices.
	t.Run("the hint's docs pointer matches the docs path", func(t *testing.T) {
		want := strings.TrimSuffix(strings.TrimPrefix(gitDocsPath, "../../docs"), ".md")
		if !strings.HasSuffix(gitDocsURL, want) {
			t.Errorf("gitDocsURL %q does not end in %q, the published path of %s",
				gitDocsURL, want, gitDocsPath)
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

// TestStrippedExecVarsHintNamesThePerVariableFix is the regression test for
// the round-1 defect: the hint closed with the same core.sshCommand example
// whatever had been stripped, so five of the seven variables got advice for a
// different variable, and GIT_EXEC_PATH was told to configure an equivalent it
// does not have.
func TestStrippedExecVarsHintNamesThePerVariableFix(t *testing.T) {
	for _, fix := range execVarFixes {
		t.Run(fix.name, func(t *testing.T) {
			if fix.config == "" && fix.caveat == "" {
				t.Fatal("entry offers the user nothing: set config or caveat")
			}

			hint := strippedExecVarsHint([]string{fix.name})

			if !strings.Contains(hint, fix.name) {
				t.Errorf("hint does not name the variable:\n%s", hint)
			}
			if fix.config == "" {
				// Nothing to configure, so the hint must not say otherwise.
				if strings.Contains(hint, "git config --global") {
					t.Errorf("%s has no config equivalent, but the hint suggests one:\n%s", fix.name, hint)
				}
			} else if want := "git config --global " + fix.config; !strings.Contains(hint, want) {
				t.Errorf("hint does not offer %q:\n%s", want, hint)
			}
			if fix.caveat != "" && !strings.Contains(hint, fix.caveat) {
				t.Errorf("hint drops the caveat %q:\n%s", fix.caveat, hint)
			}
			// The round-1 bug, stated directly.
			if fix.config != "core.sshCommand" && strings.Contains(hint, "core.sshCommand") {
				t.Errorf("hint gives the SSH example for %s, whose fix is %q:\n%s", fix.name, fix.config, hint)
			}
			if !strings.Contains(hint, gitDocsURL) {
				t.Errorf("hint does not point at %s:\n%s", gitDocsURL, hint)
			}
		})
	}
}

// migrationLinesFor returns the lines that document a variable: a
// migration-table row, or the sentence for the one variable git has no config
// key for. Word boundaries, so GIT_SSH_COMMAND does not vouch for GIT_SSH.
func migrationLinesFor(lines []string, name string) []string {
	re := regexp.MustCompile(`\b` + regexp.QuoteMeta(name) + `\b`)
	var found []string
	for _, line := range lines {
		if !re.MatchString(line) {
			continue
		}
		if strings.HasPrefix(line, "|") || strings.Contains(line, noConfigPhrase) {
			found = append(found, line)
		}
	}
	return found
}

func readGitDocs(t *testing.T) []string {
	t.Helper()
	b, err := os.ReadFile(gitDocsPath)
	if err != nil {
		t.Fatalf("read migration guide: %v", err)
	}
	return strings.Split(strings.ReplaceAll(string(b), "\r\n", "\n"), "\n")
}
