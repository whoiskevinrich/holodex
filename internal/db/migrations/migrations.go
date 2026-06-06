// Package migrations embeds the versioned SQL migration files (ADR-016).
package migrations

import "embed"

//go:embed *.sql
var FS embed.FS
