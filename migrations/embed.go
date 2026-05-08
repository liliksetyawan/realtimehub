// Package migrations carries SQL files as an embedded fs.FS, applied by
// the postgres adapter at boot.
package migrations

import "embed"

//go:embed *.sql
var FS embed.FS
