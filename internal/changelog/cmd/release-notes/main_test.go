package main

import (
	"bytes"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/dejanradmanovic/event-spec/internal/changelog"
)

const sampleDoc = `# Changelog

## [Unreleased]

### Added

- not released yet

## [1.2.3] - 2026-02-01

### Breaking

- the section the release body should carry

## Earlier releases

Nothing earlier is recorded here.
`

const wantSection = "### Breaking\n\n- the section the release body should carry\n"

// writeChangelog writes sampleDoc into a temp dir and returns its path.
func writeChangelog(t *testing.T) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "CHANGELOG.md")
	if err := os.WriteFile(path, []byte(sampleDoc), 0o644); err != nil {
		t.Fatalf("write changelog: %v", err)
	}
	return path
}

func TestRunWritesSectionToOutFile(t *testing.T) {
	path := writeChangelog(t)
	out := filepath.Join(t.TempDir(), "release-notes.md")

	var stdout bytes.Buffer
	if err := run([]string{"-version", "1.2.3", "-changelog", path, "-out", out}, &stdout); err != nil {
		t.Fatalf("run: %v", err)
	}

	got, err := os.ReadFile(out)
	if err != nil {
		t.Fatalf("read out: %v", err)
	}
	if string(got) != wantSection {
		t.Errorf("out file =\n%q\nwant\n%q", got, wantSection)
	}
	if stdout.Len() != 0 {
		t.Errorf("stdout = %q, want nothing when -out is set", stdout.String())
	}
}

func TestRunWritesToStdoutWhenOutEmpty(t *testing.T) {
	path := writeChangelog(t)

	var stdout bytes.Buffer
	if err := run([]string{"-version", "v1.2.3", "-changelog", path}, &stdout); err != nil {
		t.Fatalf("run: %v", err)
	}
	if stdout.String() != wantSection {
		t.Errorf("stdout =\n%q\nwant\n%q", stdout.String(), wantSection)
	}
}

func TestRunDefaultsToCHANGELOGmd(t *testing.T) {
	// No -changelog flag: the default is CHANGELOG.md in the working directory, which is
	// the repository root when the release workflow runs it.
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "CHANGELOG.md"), []byte(sampleDoc), 0o644); err != nil {
		t.Fatalf("write changelog: %v", err)
	}
	t.Chdir(dir)

	var stdout bytes.Buffer
	if err := run([]string{"-version", "1.2.3"}, &stdout); err != nil {
		t.Fatalf("run: %v", err)
	}
	if stdout.String() != wantSection {
		t.Errorf("stdout =\n%q\nwant\n%q", stdout.String(), wantSection)
	}
}

func TestRunMissingSectionNamesVersionAndFile(t *testing.T) {
	path := writeChangelog(t)

	var stdout bytes.Buffer
	err := run([]string{"-version", "0.4.2", "-changelog", path}, &stdout)
	if err == nil {
		t.Fatal("run with an absent version: err = nil, want error")
	}
	if !errors.Is(err, changelog.ErrSectionNotFound) {
		t.Errorf("errors.Is(err, ErrSectionNotFound) = false, want true (err: %v)", err)
	}
	for _, want := range []string{"0.4.2", path} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("error %q does not mention %q", err, want)
		}
	}
}

func TestRunErrors(t *testing.T) {
	path := writeChangelog(t)
	missing := filepath.Join(t.TempDir(), "no-such-file.md")

	tests := []struct {
		name      string
		args      []string
		wantInErr string
	}{
		{
			name:      "no version",
			args:      []string{"-changelog", path},
			wantInErr: "-version is required",
		},
		{
			name:      "unknown flag",
			args:      []string{"-version", "1.2.3", "-nope"},
			wantInErr: "flag provided but not defined: -nope",
		},
		{
			name:      "missing changelog file",
			args:      []string{"-version", "1.2.3", "-changelog", missing},
			wantInErr: "read changelog",
		},
		{
			name:      "unwritable out path",
			args:      []string{"-version", "1.2.3", "-changelog", path, "-out", filepath.Join(missing, "release-notes.md")},
			wantInErr: "write",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var stdout bytes.Buffer
			err := run(tt.args, &stdout)
			if err == nil {
				t.Fatalf("run(%v) = nil, want error", tt.args)
			}
			if !strings.Contains(err.Error(), tt.wantInErr) {
				t.Errorf("error %q does not mention %q", err, tt.wantInErr)
			}
		})
	}
}
