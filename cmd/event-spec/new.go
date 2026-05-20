package main

import (
	"bufio"
	"context"
	"crypto/sha256"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	serverclient "github.com/dejanradmanovic/event-spec/registry/server/client"
	"github.com/dejanradmanovic/event-spec/spec"
	"github.com/spf13/cobra"
)

func newNewCmd() *cobra.Command {
	var (
		eventType   string
		status      string
		owner       string
		description string
		displayName string
		workspace   string
	)

	cmd := &cobra.Command{
		Use:   "new <namespace/event_name>",
		Short: "Scaffold a new event spec YAML file",
		Long: `Scaffold a new event spec YAML at the correct registry path.

  event-spec new ecommerce/checkout_started [--type track] [--status draft] [--owner team@example.com]

If no argument is given the command enters interactive mode and prompts for the
required information. The workspace root is located by walking up from the
current directory, or can be specified explicitly with --workspace.`,
		Args: cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			interactive := len(args) == 0
			reader := bufio.NewReader(cmd.InOrStdin())

			nsName, err := resolveNSNameInteractive(interactive, args, reader, cmd.OutOrStdout())
			if err != nil {
				return err
			}

			if interactive {
				if !cmd.Flags().Changed("display-name") {
					displayName, _ = promptLine(reader, "Display name (blank for auto): ", cmd.OutOrStdout())
				}
				if !cmd.Flags().Changed("description") {
					description, _ = promptLine(reader, "Description (optional): ", cmd.OutOrStdout())
				}
				if !cmd.Flags().Changed("owner") {
					owner, _ = promptLine(reader, "Owner (optional, e.g. team email): ", cmd.OutOrStdout())
				}
				if !cmd.Flags().Changed("type") {
					if v, _ := promptLine(reader, "Type [track|page|identify|group|alias] (default: track): ", cmd.OutOrStdout()); v != "" {
						eventType = v
					}
				}
				if !cmd.Flags().Changed("status") {
					if v, _ := promptLine(reader, "Status [draft|active] (default: draft): ", cmd.OutOrStdout()); v != "" {
						status = v
					}
				}
			}

			namespace, name, err := splitNSName(nsName)
			if err != nil {
				return err
			}

			opts := spec.ScaffoldOpts{
				Type:        spec.EventType(eventType),
				Status:      spec.EventStatus(status),
				Owner:       owner,
				Description: description,
				DisplayName: displayName,
			}

			wsDir := workspace
			if wsDir == "" {
				wd, err := os.Getwd()
				if err != nil {
					return fmt.Errorf("getwd: %w", err)
				}
				wsDir = findWorkspaceDir(wd)
			}
			if wsDir == "" {
				wsDir = "."
			}

			cfg, _ := spec.LoadWorkspaceConfig(filepath.Join(wsDir, "event-spec.yaml"))

			content, err := spec.ScaffoldEventDef(namespace, name, opts)
			if err != nil {
				return fmt.Errorf("scaffold: %w", err)
			}

			if cfg != nil && cfg.Registry.Mode == spec.RegistryModeServer {
				return publishEventToServer(cmd, cfg, content, namespace, name)
			}

			specsBase, err := resolveNewSpecsDir(cfg, wsDir)
			if err != nil {
				return err
			}
			outPath := filepath.Join(specsBase, namespace, name, "1-0-0.yaml")

			if err := spec.WriteScaffoldFile(outPath, content); err != nil {
				return err
			}

			relPath := relPathFrom(wsDir, outPath)
			_, _ = fmt.Fprintf(cmd.OutOrStdout(), "Created %s\n", filepath.ToSlash(relPath))
			_, _ = fmt.Fprintf(cmd.OutOrStdout(),
				"Next: edit properties, then run  event-spec validate %s\n",
				filepath.ToSlash(relPath))

			if cfg != nil && cfg.Registry.Mode == spec.RegistryModeGit {
				_, _ = fmt.Fprintf(cmd.OutOrStdout(),
					"Note: git mode — commit the new file and push to %s to publish.\n",
					cfg.Registry.Remote)
			}
			return nil
		},
	}

	cmd.Flags().StringVar(&eventType, "type", "track", "event type: track | page | identify | group | alias")
	cmd.Flags().StringVar(&status, "status", "draft", "event status: draft | active")
	cmd.Flags().StringVar(&owner, "owner", "", "owner string (e.g. team email)")
	cmd.Flags().StringVar(&description, "description", "", "event description")
	cmd.Flags().StringVar(&displayName, "display-name", "", "human-readable display name (defaults to Title Case of event name)")
	cmd.Flags().StringVar(&workspace, "workspace", "", "explicit path to workspace root containing event-spec.yaml")
	return cmd
}

// resolveNSNameInteractive returns the namespace/event_name from args or stdin prompt.
func resolveNSNameInteractive(interactive bool, args []string, reader *bufio.Reader, w io.Writer) (string, error) {
	if !interactive {
		return args[0], nil
	}
	v, err := promptLine(reader, "Namespace/event name (e.g. ecommerce/checkout_started): ", w)
	if err != nil {
		return "", err
	}
	if v == "" {
		return "", fmt.Errorf("namespace/event_name is required")
	}
	return v, nil
}

// promptLine prints prompt to w and returns the trimmed line read from reader.
func promptLine(reader *bufio.Reader, prompt string, w io.Writer) (string, error) {
	_, _ = fmt.Fprint(w, prompt)
	line, err := reader.ReadString('\n')
	if err != nil && err != io.EOF {
		return "", fmt.Errorf("read input: %w", err)
	}
	return strings.TrimSpace(line), nil
}

// findWorkspaceDir walks up from startDir looking for a directory containing
// event-spec.yaml. Returns "" when not found.
func findWorkspaceDir(startDir string) string {
	dir := startDir
	for {
		if _, err := os.Stat(filepath.Join(dir, "event-spec.yaml")); err == nil {
			return dir
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return ""
		}
		dir = parent
	}
}

// resolveNewSpecsDir returns the absolute path of the specs directory to write into.
// For git mode this is the local cache clone's specs dir.
func resolveNewSpecsDir(cfg *spec.WorkspaceConfig, wsDir string) (string, error) {
	if cfg == nil {
		return filepath.Join(wsDir, "specs"), nil
	}
	switch cfg.Registry.Mode {
	case spec.RegistryModeGit:
		cacheDir := cfg.Registry.CacheDir
		if cacheDir == "" {
			home, err := os.UserHomeDir()
			if err != nil {
				return "", fmt.Errorf("resolve home dir: %w", err)
			}
			h := sha256.Sum256([]byte(cfg.Registry.Remote))
			cacheDir = filepath.Join(home, ".event-spec", "cache", fmt.Sprintf("%x", h[:6]))
		}
		specsDir := cfg.Registry.SpecsDir
		if specsDir == "" {
			specsDir = "specs"
		}
		return filepath.Join(cacheDir, specsDir), nil
	default:
		specsDir := cfg.SpecsDir
		if specsDir == "" {
			specsDir = "specs"
		}
		if filepath.IsAbs(specsDir) {
			return specsDir, nil
		}
		return filepath.Join(wsDir, specsDir), nil
	}
}

// relPathFrom returns outPath relative to base, falling back to outPath on error.
func relPathFrom(base, outPath string) string {
	rel, err := filepath.Rel(base, outPath)
	if err != nil {
		return outPath
	}
	return rel
}

// publishEventToServer scaffolds and publishes the event via the registry server REST API.
func publishEventToServer(cmd *cobra.Command, cfg *spec.WorkspaceConfig, content []byte, namespace, name string) error {
	def, err := spec.ParseEventDefBytes(content, namespace+"/"+name+"/1-0-0.yaml")
	if err != nil {
		return fmt.Errorf("parse scaffold: %w", err)
	}
	reg := serverclient.New(serverclient.Config{BaseURL: cfg.Registry.URL})
	if err := reg.PublishEvent(context.Background(), *def); err != nil {
		return fmt.Errorf("publish to %s: %w", cfg.Registry.URL, err)
	}
	_, _ = fmt.Fprintf(cmd.OutOrStdout(), "Published %s/%s 1-0-0 to %s\n", namespace, name, cfg.Registry.URL)
	_, _ = fmt.Fprintf(cmd.OutOrStdout(), "Next: edit the event spec and run  event-spec validate\n")
	return nil
}
