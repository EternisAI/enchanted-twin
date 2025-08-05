package migrations

import (
	"embed"
)

//go:embed *.sql
var EmbedPostgresMigrations embed.FS
