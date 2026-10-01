// Package migrations embeds versioned schema assets in the shared server binary.
package migrations

import "embed"

//go:embed postgres/*.sql clickhouse/*.sql
var Files embed.FS
