package mysql

import (
	"context"
	"fmt"
	"io/fs"
	"sort"
	"strings"
	"time"

	"OpenMCP-Chain/deploy/migrations"
)

var embeddedMigrations = migrations.Files

type migrationFile struct {
	version    string
	name       string
	statements []string
}

func (s *SQLTaskRepository) runMigrations(ctx context.Context) error {
	if _, err := s.db.ExecContext(ctx, `CREATE TABLE IF NOT EXISTS schema_migrations (
        version VARCHAR(32) NOT NULL PRIMARY KEY,
        applied_at BIGINT NOT NULL
)`); err != nil {
		return fmt.Errorf("创建 schema_migrations 表失败: %w", err)
	}

	applied, err := s.loadAppliedVersions(ctx)
	if err != nil {
		return err
	}

	migrations, err := loadMigrationFiles()
	if err != nil {
		return err
	}

	for _, migration := range migrations {
		if _, ok := applied[migration.version]; ok {
			continue
		}
		if err := s.applyMigration(ctx, migration); err != nil {
			return err
		}
	}
	return nil
}

func (s *SQLTaskRepository) loadAppliedVersions(ctx context.Context) (map[string]struct{}, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT version FROM schema_migrations`)
	if err != nil {
		return nil, fmt.Errorf("查询 schema_migrations 失败: %w", err)
	}
	defer rows.Close()

	applied := make(map[string]struct{})
	for rows.Next() {
		var version string
		if err := rows.Scan(&version); err != nil {
			return nil, fmt.Errorf("解析 schema_migrations 失败: %w", err)
		}
		applied[version] = struct{}{}
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("遍历 schema_migrations 失败: %w", err)
	}
	return applied, nil
}

func (s *SQLTaskRepository) applyMigration(ctx context.Context, migration migrationFile) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("开启迁移事务失败: %w", err)
	}

	for _, stmt := range migration.statements {
		if _, err := tx.ExecContext(ctx, stmt); err != nil {
			tx.Rollback()
			return fmt.Errorf("执行迁移 %s 失败: %w", migration.name, err)
		}
	}

	if _, err := tx.ExecContext(ctx, `INSERT INTO schema_migrations (version, applied_at) VALUES (?, ?)`, migration.version, time.Now().Unix()); err != nil {
		tx.Rollback()
		return fmt.Errorf("记录迁移版本失败: %w", err)
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("提交迁移事务失败: %w", err)
	}
	return nil
}

func loadMigrationFiles() ([]migrationFile, error) {
	entries, err := fs.ReadDir(embeddedMigrations, ".")
	if err != nil {
		return nil, fmt.Errorf("读取迁移目录失败: %w", err)
	}

	var migrations []migrationFile
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		name := entry.Name()
		contentBytes, err := embeddedMigrations.ReadFile(name)
		if err != nil {
			return nil, fmt.Errorf("读取迁移文件 %s 失败: %w", name, err)
		}
		statements := splitSQLStatements(string(contentBytes))
		if len(statements) == 0 {
			continue
		}

		version := parseMigrationVersion(name)
		migrations = append(migrations, migrationFile{
			version:    version,
			name:       name,
			statements: statements,
		})
	}

	sort.Slice(migrations, func(i, j int) bool {
		if migrations[i].version == migrations[j].version {
			return migrations[i].name < migrations[j].name
		}
		return migrations[i].version < migrations[j].version
	})
	return migrations, nil
}

func splitSQLStatements(content string) []string {
	rawStatements := strings.Split(content, ";")
	var statements []string
	for _, stmt := range rawStatements {
		trimmed := strings.TrimSpace(stmt)
		if trimmed == "" {
			continue
		}
		statements = append(statements, trimmed)
	}
	return statements
}

func parseMigrationVersion(name string) string {
	if idx := strings.IndexRune(name, '_'); idx > 0 {
		return name[:idx]
	}
	if dot := strings.IndexRune(name, '.'); dot > 0 {
		return name[:dot]
	}
	return name
}
