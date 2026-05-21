// Package ui provides the web admin interface for the registry server.
// All templates and static assets are embedded at compile time so the server
// binary is self-contained with zero external file dependencies.
package ui

import "embed"

// FS holds the embedded templates and static assets for the admin UI.
//
//go:embed templates static
var FS embed.FS
