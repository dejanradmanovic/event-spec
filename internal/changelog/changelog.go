// Package changelog reads the repository CHANGELOG.md and extracts the body of a single
// version's section. The file follows Keep a Changelog 1.1.0, whose headings are regular
// enough that a line scan beats pulling in a markdown parser.
package changelog

import (
	"errors"
	"fmt"
	"regexp"
	"strings"
)

// ErrSectionNotFound is returned by Section when the changelog has no heading for the
// requested version. The release workflow relies on it to fail before a tag is pushed.
var ErrSectionNotFound = errors.New("changelog section not found")

// unreleasedHeading collects changes that have no version yet. The releaser promotes it
// to a version heading in a separate pull request before dispatching release.yml.
const unreleasedHeading = "## [Unreleased]"

// versionHeadingFormat is the only accepted shape of a released version heading, quoted
// verbatim in parse errors so a contributor sees what to type.
const versionHeadingFormat = "## [x.y.z] - YYYY-MM-DD"

var versionHeadingRe = regexp.MustCompile(`^## \[(\d+\.\d+\.\d+)\] - \d{4}-\d{2}-\d{2}$`)

// Section returns the body of the "## [<version>] - <date>" block with the heading
// excluded and surrounding whitespace trimmed, stopping at the next "## " heading.
// version is accepted with or without a leading "v".
func Section(doc []byte, version string) (string, error) {
	v := strings.TrimPrefix(strings.TrimSpace(version), "v")
	if v == "" {
		return "", errors.New("changelog: no version given")
	}
	body, found := sectionBody(doc, func(line string) bool {
		m := versionHeadingRe.FindStringSubmatch(line)
		return len(m) > 1 && m[1] == v
	})
	if !found {
		return "", fmt.Errorf("%w: %s", ErrSectionNotFound, v)
	}
	if body == "" {
		return "", fmt.Errorf("changelog: section for %s is empty", v)
	}
	return body, nil
}

// Unreleased returns the body of the "## [Unreleased]" block, the section a contributor
// adds to. An empty body is a legitimate state — it is what the heading looks like
// immediately after a release promotion — so only a missing heading is an error. Section
// errors on an empty section instead, because an empty release body is not legitimate.
func Unreleased(doc []byte) (string, error) {
	body, found := sectionBody(doc, func(line string) bool { return line == unreleasedHeading })
	if !found {
		return "", fmt.Errorf("%w: %s", ErrSectionNotFound, unreleasedHeading)
	}
	return body, nil
}

// Versions returns the released version numbers in file order. Every bracketed "## "
// heading is validated on the way through: a released heading must match
// versionHeadingFormat, and "## [Unreleased]" may appear only once. Other "## " headings
// (such as "## Earlier releases") are ignored.
func Versions(doc []byte) ([]string, error) {
	var (
		versions   []string
		seen       = make(map[string]struct{})
		unreleased int
	)
	for i, line := range splitLines(doc) {
		if !strings.HasPrefix(line, "## [") {
			continue
		}
		if line == unreleasedHeading {
			unreleased++
			if unreleased > 1 {
				return nil, fmt.Errorf("changelog: line %d: duplicate %q heading", i+1, unreleasedHeading)
			}
			continue
		}
		m := versionHeadingRe.FindStringSubmatch(line)
		if m == nil {
			return nil, fmt.Errorf("changelog: line %d: heading %q is neither %q nor %q", i+1, line, unreleasedHeading, versionHeadingFormat)
		}
		if _, dup := seen[m[1]]; dup {
			return nil, fmt.Errorf("changelog: line %d: duplicate section for version %s", i+1, m[1])
		}
		seen[m[1]] = struct{}{}
		versions = append(versions, m[1])
	}
	return versions, nil
}

// sectionBody returns the lines between the first heading match and the next "## "
// heading. found reports whether the heading itself was there, so callers can tell an
// absent section from an empty one.
func sectionBody(doc []byte, match func(line string) bool) (body string, found bool) {
	var lines []string
	for _, line := range splitLines(doc) {
		if !found {
			found = match(line)
			continue
		}
		if strings.HasPrefix(line, "## ") {
			break
		}
		lines = append(lines, line)
	}
	return strings.TrimSpace(strings.Join(lines, "\n")), found
}

// splitLines normalises CRLF before splitting: the repository is developed on Windows,
// and a changelog read straight off a working tree may carry \r line endings.
func splitLines(doc []byte) []string {
	return strings.Split(strings.ReplaceAll(string(doc), "\r\n", "\n"), "\n")
}
