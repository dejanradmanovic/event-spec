package main

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	registryserver "github.com/dejanradmanovic/event-spec/registry/server"
	"github.com/dejanradmanovic/event-spec/spec"
	_ "github.com/lib/pq"
	"github.com/spf13/cobra"
	_ "modernc.org/sqlite"
)

func newServeCmd() *cobra.Command {
	var (
		port int
		dsn  string
	)

	cmd := &cobra.Command{
		Use:   "serve",
		Short: "Start the registry HTTP server",
		RunE: func(cmd *cobra.Command, args []string) error {
			if dsn == "" {
				return fmt.Errorf("--db is required: provide a PostgreSQL (postgres://...) or SQLite (file:./registry.db) DSN")
			}

			workspace := ""
			if cfg, err := spec.LoadWorkspaceConfig("event-spec.yaml"); err == nil {
				workspace = cfg.Workspace
			}

			srv, err := registryserver.NewFromDSN(dsn, registryserver.Config{Port: port})
			if err != nil {
				return fmt.Errorf("init registry server: %w", err)
			}

			if workspace != "" {
				_, _ = fmt.Fprintf(cmd.OutOrStdout(), "workspace: %s\n", workspace)
			}
			_, _ = fmt.Fprintf(cmd.OutOrStdout(), "Listening on :%d (db: %s)\n", port, dsn)

			httpSrv := &http.Server{
				Addr:    fmt.Sprintf(":%d", port),
				Handler: srv,
			}

			stop := make(chan os.Signal, 1)
			signal.Notify(stop, syscall.SIGINT, syscall.SIGTERM)

			errCh := make(chan error, 1)
			go func() {
				if err := httpSrv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
					errCh <- err
				}
				close(errCh)
			}()

			select {
			case sig := <-stop:
				_, _ = fmt.Fprintf(cmd.OutOrStdout(), "received %s, shutting down\n", sig)
			case err := <-errCh:
				return err
			}

			ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
			defer cancel()
			return httpSrv.Shutdown(ctx)
		},
	}

	cmd.Flags().IntVar(&port, "port", 8080, "listening port")
	cmd.Flags().StringVar(&dsn, "db", "", "database DSN (postgres://... or file:./registry.db)")

	return cmd
}
