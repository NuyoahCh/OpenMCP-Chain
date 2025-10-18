package mysql

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"sort"
	"strings"
	"time"

	"OpenMCP-Chain/internal/auth"
)

// SQLAuthStore persists users, roles and permissions in MySQL.
type SQLAuthStore struct {
	db *sql.DB
}

// NewSQLAuthStore creates the store using the provided connection settings.
func NewSQLAuthStore(ctx context.Context, cfg Config) (*SQLAuthStore, error) {
	db, err := openDatabase(ctx, cfg)
	if err != nil {
		return nil, err
	}
	if err := runMigrations(ctx, db); err != nil {
		db.Close()
		return nil, err
	}
	return &SQLAuthStore{db: db}, nil
}

// Close releases the underlying database connection pool.
func (s *SQLAuthStore) Close() error {
	if s == nil || s.db == nil {
		return nil
	}
	return s.db.Close()
}

// FindUserByUsername implements auth.Store.
func (s *SQLAuthStore) FindUserByUsername(ctx context.Context, username string) (*auth.User, error) {
	const query = `SELECT id, username, password_hash, disabled FROM auth_users WHERE username = ?`
	row := s.db.QueryRowContext(ctx, query, strings.TrimSpace(username))
	var user auth.User
	var disabled int
	if err := row.Scan(&user.ID, &user.Username, &user.PasswordHash, &disabled); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, err
		}
		return nil, fmt.Errorf("查询用户失败: %w", err)
	}
	user.Disabled = disabled == 1
	return &user, nil
}

// LoadSubject loads the subject details including roles and permissions.
func (s *SQLAuthStore) LoadSubject(ctx context.Context, userID int64) (*auth.Subject, error) {
	const userQuery = `SELECT id, username, disabled FROM auth_users WHERE id = ?`
	row := s.db.QueryRowContext(ctx, userQuery, userID)
	var subject auth.Subject
	var disabled int
	if err := row.Scan(&subject.ID, &subject.Username, &disabled); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, err
		}
		return nil, fmt.Errorf("查询用户信息失败: %w", err)
	}
	subject.Disabled = disabled == 1

	const rolesQuery = `SELECT r.name FROM auth_roles r
JOIN auth_user_roles ur ON ur.role_id = r.id
WHERE ur.user_id = ?`
	roles, err := s.collectStrings(ctx, rolesQuery, subject.ID)
	if err != nil {
		return nil, err
	}
	subject.Roles = roles

	const permsQuery = `SELECT DISTINCT p.name FROM auth_permissions p
JOIN auth_role_permissions rp ON rp.permission_id = p.id
JOIN auth_user_roles ur ON ur.role_id = rp.role_id
WHERE ur.user_id = ?
UNION
SELECT DISTINCT p.name FROM auth_permissions p
JOIN auth_user_permissions up ON up.permission_id = p.id
WHERE up.user_id = ?`
	perms, err := s.collectStrings(ctx, permsQuery, subject.ID, subject.ID)
	if err != nil {
		return nil, err
	}
	subject.Permissions = perms
	subject.Normalise()
	return &subject, nil
}

func (s *SQLAuthStore) collectStrings(ctx context.Context, query string, args ...any) ([]string, error) {
	rows, err := s.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("查询列表失败: %w", err)
	}
	defer rows.Close()
	var result []string
	for rows.Next() {
		var value string
		if err := rows.Scan(&value); err != nil {
			return nil, fmt.Errorf("解析列表失败: %w", err)
		}
		result = append(result, strings.ToLower(strings.TrimSpace(value)))
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("遍历列表失败: %w", err)
	}
	sort.Strings(result)
	return result, nil
}

// ApplySeed upserts default users and permissions.
func (s *SQLAuthStore) ApplySeed(ctx context.Context, seed auth.Seed) error {
	username := strings.TrimSpace(seed.Username)
	if username == "" {
		return errors.New("seed username cannot be empty")
	}
	passwordHash, err := auth.HashPassword(seed.Password)
	if err != nil {
		return err
	}
	now := time.Now().Unix()
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer func() {
		if err != nil {
			tx.Rollback()
		}
	}()

	var userID int64
	const upsertUser = `INSERT INTO auth_users (username, password_hash, disabled, created_at, updated_at)
VALUES (?, ?, ?, ?, ?)
ON DUPLICATE KEY UPDATE password_hash = VALUES(password_hash), disabled = VALUES(disabled), updated_at = VALUES(updated_at), id = LAST_INSERT_ID(id)`
	res, execErr := tx.ExecContext(ctx, upsertUser, username, passwordHash, boolToInt(seed.Disabled), now, now)
	if execErr != nil {
		err = fmt.Errorf("保存用户失败: %w", execErr)
		return err
	}
	userID, execErr = res.LastInsertId()
	if execErr != nil {
		err = fmt.Errorf("获取用户ID失败: %w", execErr)
		return err
	}

	for _, role := range dedupeValues(seed.Roles) {
		const upsertRole = `INSERT INTO auth_roles (name, description, created_at, updated_at)
VALUES (?, '', ?, ?)
ON DUPLICATE KEY UPDATE updated_at = VALUES(updated_at), id = LAST_INSERT_ID(id)`
		res, execErr = tx.ExecContext(ctx, upsertRole, role, now, now)
		if execErr != nil {
			err = fmt.Errorf("保存角色失败: %w", execErr)
			return err
		}
		roleID, e := res.LastInsertId()
		if e != nil {
			err = fmt.Errorf("获取角色ID失败: %w", e)
			return err
		}
		if _, execErr = tx.ExecContext(ctx, `INSERT IGNORE INTO auth_user_roles (user_id, role_id, assigned_at) VALUES (?, ?, ?)`, userID, roleID, now); execErr != nil {
			err = fmt.Errorf("绑定用户角色失败: %w", execErr)
			return err
		}
	}

	for _, perm := range dedupeValues(seed.Permissions) {
		const upsertPerm = `INSERT INTO auth_permissions (name, description, created_at, updated_at)
VALUES (?, '', ?, ?)
ON DUPLICATE KEY UPDATE updated_at = VALUES(updated_at), id = LAST_INSERT_ID(id)`
		res, execErr = tx.ExecContext(ctx, upsertPerm, perm, now, now)
		if execErr != nil {
			err = fmt.Errorf("保存权限失败: %w", execErr)
			return err
		}
		permID, e := res.LastInsertId()
		if e != nil {
			err = fmt.Errorf("获取权限ID失败: %w", e)
			return err
		}
		if _, execErr = tx.ExecContext(ctx, `INSERT IGNORE INTO auth_user_permissions (user_id, permission_id, assigned_at) VALUES (?, ?, ?)`, userID, permID, now); execErr != nil {
			err = fmt.Errorf("绑定用户权限失败: %w", execErr)
			return err
		}
	}

	if err = tx.Commit(); err != nil {
		return fmt.Errorf("提交种子数据失败: %w", err)
	}
	return nil
}

func boolToInt(b bool) int {
	if b {
		return 1
	}
	return 0
}

func dedupeValues(values []string) []string {
	seen := make(map[string]struct{}, len(values))
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value == "" {
			continue
		}
		seen[strings.ToLower(value)] = struct{}{}
	}
	result := make([]string, 0, len(seen))
	for key := range seen {
		result = append(result, key)
	}
	sort.Strings(result)
	return result
}
