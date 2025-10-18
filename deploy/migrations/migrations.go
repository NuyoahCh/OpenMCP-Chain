package migrations

import "embed"

// Files 暴露所有 SQL 迁移文件。
//
//go:embed *.sql
var Files embed.FS
