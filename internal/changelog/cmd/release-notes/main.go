// Command release-notes extracts one version's section from CHANGELOG.md. The release
// workflow runs it before the tag is pushed, so a release whose version has no changelog
// entry fails while nothing has been published yet.
//
//	go run ./internal/changelog/cmd/release-notes -version 1.2.3 -out release-notes.md
package main

import (
	"errors"
	"flag"
	"fmt"
	"io"
	"os"

	"github.com/dejanradmanovic/event-spec/internal/changelog"
)

func main() {
	if err := run(os.Args[1:], os.Stdout); err != nil {
		fmt.Fprintf(os.Stderr, "release-notes: %v\n", err)
		os.Exit(1)
	}
}

func run(args []string, stdout io.Writer) error {
	fs := flag.NewFlagSet("release-notes", flag.ContinueOnError)
	fs.SetOutput(stdout)
	version := fs.String("version", "", "version to extract, with or without a leading v (required)")
	path := fs.String("changelog", "CHANGELOG.md", "path to the changelog file")
	out := fs.String("out", "", "write the section to this file instead of stdout")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if *version == "" {
		return errors.New("-version is required")
	}

	doc, err := os.ReadFile(*path)
	if err != nil {
		return fmt.Errorf("read changelog: %w", err)
	}
	section, err := changelog.Section(doc, *version)
	if err != nil {
		return fmt.Errorf("%w (in %s)", err, *path)
	}

	body := section + "\n"
	if *out == "" {
		_, err := io.WriteString(stdout, body)
		return err
	}
	if err := os.WriteFile(*out, []byte(body), 0o644); err != nil {
		return fmt.Errorf("write %s: %w", *out, err)
	}
	return nil
}
