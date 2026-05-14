// Package migrations embeds all SQL migration files for use by cmd/migrate.
package migrations

import "embed"

//go:embed *.sql
var FS embed.FS
