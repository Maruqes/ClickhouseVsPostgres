// Package migrations embeds versioned schema assets in the shared server binary.
package migrations

import "embed"

//go:embed *.sql
var Files embed.FS
