// Package migrations bundles the d2ip SQLite schema migrations as a Go
// embed.FS so that the cache agent can apply them at runtime without
// depending on the on-disk layout of the binary.
//
// Migration files live next to this Go file and are named with the
// "NNNN_<slug>.sql" convention. They are applied in lexicographic order,
// idempotently, guarded by the schema_version table.
package migrations

import "embed"

// FS holds every *.sql migration file in this directory, in lexicographic
// order. Callers should iterate via fs.ReadDir and filter by ".sql" suffix.
//
//go:embed *.sql
var FS embed.FS
