package task

import (
	"context"
	"database/sql"
	"encoding/json"
	stdErrors "errors"
	"fmt"
	"strings"
	"time"

	xerrors "OpenMCP-Chain/internal/errors"
	"github.com/go-sql-driver/mysql"
)

// MySQLStore 使用 MySQL 记录任务状态。
type MySQLStore struct {
	db *sql.DB
}

// NewMySQLStore 创建一个新的 MySQLStore。
func NewMySQLStore(dsn string) (*MySQLStore, error) {
	if strings.TrimSpace(dsn) == "" {
		return nil, xerrors.New(xerrors.CodeInvalidArgument, "MySQL DSN 不能为空")
	}

	db, err := sql.Open("mysql", dsn)
	if err != nil {
		return nil, xerrors.Wrap(xerrors.CodeStorageFailure, err, "连接 MySQL 失败")
	}

	db.SetMaxOpenConns(20)
	db.SetMaxIdleConns(10)
	db.SetConnMaxLifetime(10 * time.Minute)

	if err := db.Ping(); err != nil {
		return nil, xerrors.Wrap(xerrors.CodeStorageFailure, err, "无法连接到 MySQL")
	}

	store := &MySQLStore{db: db}
	if err := store.initSchema(); err != nil {
		_ = db.Close()
		return nil, err
	}
	return store, nil
}

func (s *MySQLStore) initSchema() error {
	const schema = `CREATE TABLE IF NOT EXISTS task_states (
        id VARCHAR(64) PRIMARY KEY,
        goal TEXT NOT NULL,
        chain_action VARCHAR(255) DEFAULT '',
        address VARCHAR(255) DEFAULT '',
        metadata TEXT,
        status VARCHAR(32) NOT NULL,
        attempts INT NOT NULL DEFAULT 0,
        max_retries INT NOT NULL DEFAULT 3,
        last_error TEXT,
        error_code VARCHAR(64) DEFAULT '',
        result_thought TEXT,
        result_reply TEXT,
        result_chain_id VARCHAR(66) DEFAULT '',
        result_block_number VARCHAR(66) DEFAULT '',
        result_observations TEXT,
        created_at BIGINT NOT NULL,
        updated_at BIGINT NOT NULL,
        INDEX idx_task_status (status),
        INDEX idx_task_updated (updated_at)
)`

	if _, err := s.db.Exec(schema); err != nil {
		return xerrors.Wrap(xerrors.CodeStorageFailure, err, "初始化 task_states 表失败")
	}
	if _, err := s.db.Exec(`ALTER TABLE task_states ADD COLUMN error_code VARCHAR(64) DEFAULT '' AFTER last_error`); err != nil {
		var mysqlErr *mysql.MySQLError
		if !(stdErrors.As(err, &mysqlErr) && mysqlErr.Number == 1060) {
			return xerrors.Wrap(xerrors.CodeStorageFailure, err, "扩展 task_states.error_code 失败")
		}
	}
	if _, err := s.db.Exec(`ALTER TABLE task_states ADD COLUMN metadata TEXT AFTER address`); err != nil {
		var mysqlErr *mysql.MySQLError
		if !(stdErrors.As(err, &mysqlErr) && mysqlErr.Number == 1060) {
			return xerrors.Wrap(xerrors.CodeStorageFailure, err, "扩展 task_states.metadata 失败")
		}
	}
	return nil
}

// Create 插入新的任务记录。
func (s *MySQLStore) Create(ctx context.Context, task *Task) error {
	if task == nil {
		return xerrors.New(xerrors.CodeInvalidArgument, "task 不能为空")
	}
	if strings.TrimSpace(task.ID) == "" {
		return xerrors.New(xerrors.CodeInvalidArgument, "任务 ID 不能为空")
	}

	now := time.Now().Unix()
	task.CreatedAt = now
	task.UpdatedAt = now

	metadataValue, err := marshalMetadata(task.Metadata)
	if err != nil {
		return xerrors.Wrap(xerrors.CodeInvalidArgument, err, "编码任务 metadata 失败")
	}

	const stmt = `INSERT INTO task_states
        (id, goal, chain_action, address, metadata, status, attempts, max_retries, last_error, error_code, created_at, updated_at)
        VALUES (?, ?, ?, ?, ?, ?, ?, ?, '', '', ?, ?)`

	_, err = s.db.ExecContext(ctx, stmt,
		task.ID,
		task.Goal,
		task.ChainAction,
		task.Address,
		metadataValue,
		task.Status,
		task.Attempts,
		task.MaxRetries,
		task.CreatedAt,
		task.UpdatedAt,
	)
	if err != nil {
		var mysqlErr *mysql.MySQLError
		if stdErrors.As(err, &mysqlErr) && mysqlErr.Number == 1062 {
			return ErrTaskConflict
		}
		return xerrors.Wrap(xerrors.CodeStorageFailure, err, "插入任务失败")
	}
	return nil
}

// Get 查询指定任务。
func (s *MySQLStore) Get(ctx context.Context, id string) (*Task, error) {
	const stmt = `SELECT id, goal, chain_action, address, metadata, status, attempts, max_retries, last_error, error_code,
        result_thought, result_reply, result_chain_id, result_block_number, result_observations, created_at, updated_at
        FROM task_states WHERE id = ?`

	row := s.db.QueryRowContext(ctx, stmt, id)

	var task Task
	var result ExecutionResult
	var metadata sql.NullString

	if err := row.Scan(
		&task.ID,
		&task.Goal,
		&task.ChainAction,
		&task.Address,
		&metadata,
		&task.Status,
		&task.Attempts,
		&task.MaxRetries,
		&task.LastError,
		&task.ErrorCode,
		&result.Thought,
		&result.Reply,
		&result.ChainID,
		&result.BlockNumber,
		&result.Observations,
		&task.CreatedAt,
		&task.UpdatedAt,
	); err != nil {
		if stdErrors.Is(err, sql.ErrNoRows) {
			return nil, ErrTaskNotFound
		}
		return nil, xerrors.Wrap(xerrors.CodeStorageFailure, err, "查询任务失败")
	}

	decodedMetadata, err := unmarshalMetadata(metadata)
	if err != nil {
		return nil, xerrors.Wrap(xerrors.CodeStorageFailure, err, "解析任务 metadata 失败")
	}
	task.Metadata = cloneMetadata(decodedMetadata)

	if result.Thought != "" || result.Reply != "" || result.ChainID != "" || result.BlockNumber != "" || result.Observations != "" {
		task.Result = &result
	}
	return &task, nil
}

// Claim 将任务标记为运行中并返回最新状态。
func (s *MySQLStore) Claim(ctx context.Context, id string) (*Task, error) {
	const updateStmt = `UPDATE task_states SET status = ?, attempts = attempts + 1, updated_at = ?, last_error = '', error_code = ''
        WHERE id = ? AND status IN (?, ?) AND attempts < max_retries`

	now := time.Now().Unix()
	res, err := s.db.ExecContext(ctx, updateStmt,
		StatusRunning,
		now,
		id,
		StatusPending,
		StatusFailed,
	)
	if err != nil {
		return nil, xerrors.Wrap(xerrors.CodeStorageFailure, err, "更新任务状态失败")
	}
	affected, err := res.RowsAffected()
	if err != nil {
		return nil, xerrors.Wrap(xerrors.CodeStorageFailure, err, "获取影响行数失败")
	}
	if affected == 0 {
		task, getErr := s.Get(ctx, id)
		if getErr != nil {
			return nil, getErr
		}
		switch task.Status {
		case StatusSucceeded:
			return task, ErrTaskCompleted
		case StatusRunning:
			return task, ErrTaskConflict
		default:
			if task.Attempts >= task.MaxRetries {
				return task, ErrTaskExhausted
			}
			return task, ErrTaskConflict
		}
	}
	task, err := s.Get(ctx, id)
	if err != nil {
		return nil, err
	}
	return task, nil
}

// MarkSucceeded 将任务标记为成功。
func (s *MySQLStore) MarkSucceeded(ctx context.Context, id string, result ExecutionResult) error {
	const stmt = `UPDATE task_states SET status = ?, result_thought = ?, result_reply = ?, result_chain_id = ?,
        result_block_number = ?, result_observations = ?, updated_at = ?, last_error = '', error_code = '' WHERE id = ?`

	now := time.Now().Unix()
	res, err := s.db.ExecContext(ctx, stmt,
		StatusSucceeded,
		result.Thought,
		result.Reply,
		result.ChainID,
		result.BlockNumber,
		result.Observations,
		now,
		id,
	)
	if err != nil {
		return xerrors.Wrap(xerrors.CodeStorageFailure, err, "标记任务成功失败")
	}
	if rows, _ := res.RowsAffected(); rows == 0 {
		return ErrTaskNotFound
	}
	return nil
}

// MarkFailed 将任务标记为失败，并在必要时终止重试。
func (s *MySQLStore) MarkFailed(ctx context.Context, id string, code xerrors.Code, lastError string, terminal bool) error {
	const stmt = `UPDATE task_states SET status = ?, last_error = ?, error_code = ?, updated_at = ? WHERE id = ?`

	status := StatusFailed
	if terminal {
		status = StatusFailed
	}
	now := time.Now().Unix()
	res, err := s.db.ExecContext(ctx, stmt,
		status,
		lastError,
		string(code),
		now,
		id,
	)
	if err != nil {
		return xerrors.Wrap(xerrors.CodeStorageFailure, err, "标记任务失败失败")
	}
	if rows, _ := res.RowsAffected(); rows == 0 {
		return ErrTaskNotFound
	}
	return nil
}

// List 返回最近的任务。
func (s *MySQLStore) List(ctx context.Context, opts ListOptions) ([]*Task, error) {
	opts.applyDefaults()

	query := `SELECT id, goal, chain_action, address, metadata, status, attempts, max_retries, last_error, error_code,
        result_thought, result_reply, result_chain_id, result_block_number, result_observations, created_at, updated_at FROM task_states`

	clause, filterArgs := buildFilterClause(opts)
	if clause != "" {
		query += " WHERE " + clause
	}

	order := " ORDER BY updated_at DESC, created_at DESC, id DESC"
	if opts.Order == SortByUpdatedAsc {
		order = " ORDER BY updated_at ASC, created_at ASC, id ASC"
	}
	query += order + " LIMIT ? OFFSET ?"

	args := append(filterArgs, opts.Limit, opts.Offset)

	rows, err := s.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, xerrors.Wrap(xerrors.CodeStorageFailure, err, "查询任务列表失败")
	}
	defer rows.Close()

	tasks := make([]*Task, 0, opts.Limit)
	for rows.Next() {
		var task Task
		var result ExecutionResult
		var metadata sql.NullString
		if err := rows.Scan(
			&task.ID,
			&task.Goal,
			&task.ChainAction,
			&task.Address,
			&metadata,
			&task.Status,
			&task.Attempts,
			&task.MaxRetries,
			&task.LastError,
			&task.ErrorCode,
			&result.Thought,
			&result.Reply,
			&result.ChainID,
			&result.BlockNumber,
			&result.Observations,
			&task.CreatedAt,
			&task.UpdatedAt,
		); err != nil {
			return nil, xerrors.Wrap(xerrors.CodeStorageFailure, err, "解析任务记录失败")
		}
		decodedMetadata, err := unmarshalMetadata(metadata)
		if err != nil {
			return nil, xerrors.Wrap(xerrors.CodeStorageFailure, err, "解析任务列表 metadata 失败")
		}
		task.Metadata = cloneMetadata(decodedMetadata)
		if result.Thought != "" || result.Reply != "" || result.ChainID != "" || result.BlockNumber != "" || result.Observations != "" {
			task.Result = &result
		}
		taskCopy := task
		taskCopy.Metadata = cloneMetadata(task.Metadata)
		tasks = append(tasks, &taskCopy)
	}
	if err := rows.Err(); err != nil {
		return nil, xerrors.Wrap(xerrors.CodeStorageFailure, err, "遍历任务失败")
	}
	return tasks, nil
}

// Close 关闭底层数据库连接。
func (s *MySQLStore) Close() error {
	if s == nil || s.db == nil {
		return nil
	}
	return s.db.Close()
}

// Stats 返回符合过滤条件的任务聚合信息。
func (s *MySQLStore) Stats(ctx context.Context, opts ListOptions) (TaskStats, error) {
	opts.applyDefaults()

	query := `SELECT
        COUNT(*) AS total,
        SUM(CASE WHEN status = ? THEN 1 ELSE 0 END) AS pending,
        SUM(CASE WHEN status = ? THEN 1 ELSE 0 END) AS running,
        SUM(CASE WHEN status = ? THEN 1 ELSE 0 END) AS succeeded,
        SUM(CASE WHEN status = ? THEN 1 ELSE 0 END) AS failed,
        COALESCE(MIN(updated_at), 0) AS oldest,
        COALESCE(MAX(updated_at), 0) AS newest
        FROM task_states`

	clause, filterArgs := buildFilterClause(opts)
	if clause != "" {
		query += " WHERE " + clause
	}

	args := []any{string(StatusPending), string(StatusRunning), string(StatusSucceeded), string(StatusFailed)}
	args = append(args, filterArgs...)

	row := s.db.QueryRowContext(ctx, query, args...)

	var stats TaskStats
	if err := row.Scan(
		&stats.Total,
		&stats.Pending,
		&stats.Running,
		&stats.Succeeded,
		&stats.Failed,
		&stats.OldestUpdatedAt,
		&stats.NewestUpdatedAt,
	); err != nil {
		return TaskStats{}, xerrors.Wrap(xerrors.CodeStorageFailure, err, "查询任务统计失败")
	}
	if stats.Total == 0 {
		stats.OldestUpdatedAt = 0
		stats.NewestUpdatedAt = 0
	}
	return stats, nil
}

func marshalMetadata(metadata map[string]any) (sql.NullString, error) {
	if metadata == nil || len(metadata) == 0 {
		return sql.NullString{}, nil
	}
	bytes, err := json.Marshal(metadata)
	if err != nil {
		return sql.NullString{}, err
	}
	return sql.NullString{String: string(bytes), Valid: true}, nil
}

func unmarshalMetadata(raw sql.NullString) (map[string]any, error) {
	if !raw.Valid || strings.TrimSpace(raw.String) == "" {
		return nil, nil
	}
	var metadata map[string]any
	if err := json.Unmarshal([]byte(raw.String), &metadata); err != nil {
		return nil, err
	}
	return metadata, nil
}

func buildFilterClause(opts ListOptions) (string, []any) {
	conditions := make([]string, 0, 4)
	args := make([]any, 0, 6)

	if len(opts.Statuses) > 0 {
		placeholders := make([]string, 0, len(opts.Statuses))
		for range opts.Statuses {
			placeholders = append(placeholders, "?")
		}
		conditions = append(conditions, fmt.Sprintf("status IN (%s)", strings.Join(placeholders, ",")))
		for _, status := range opts.Statuses {
			args = append(args, status)
		}
	}
	if opts.UpdatedGTE > 0 {
		conditions = append(conditions, "updated_at >= ?")
		args = append(args, opts.UpdatedGTE)
	}
	if opts.UpdatedLTE > 0 {
		conditions = append(conditions, "updated_at <= ?")
		args = append(args, opts.UpdatedLTE)
	}
	if opts.HasResult != nil {
		if *opts.HasResult {
			conditions = append(conditions, "(result_thought <> '' OR result_reply <> '' OR result_chain_id <> '' OR result_block_number <> '' OR result_observations <> '')")
		} else {
			conditions = append(conditions, "(result_thought = '' AND result_reply = '' AND result_chain_id = '' AND result_block_number = '' AND (result_observations IS NULL OR result_observations = ''))")
		}
	}
	if opts.Query != "" {
		pattern := "%" + opts.Query + "%"
		conditions = append(conditions, "(id LIKE ? OR goal LIKE ? OR chain_action LIKE ? OR address LIKE ? OR metadata LIKE ? OR last_error LIKE ? OR result_thought LIKE ? OR result_reply LIKE ? OR result_chain_id LIKE ? OR result_block_number LIKE ? OR result_observations LIKE ?)")
		args = append(args,
			pattern,
			pattern,
			pattern,
			pattern,
			pattern,
			pattern,
			pattern,
			pattern,
			pattern,
			pattern,
			pattern,
		)
	}

	if len(conditions) == 0 {
		return "", nil
	}
	return strings.Join(conditions, " AND "), args
}

var _ Store = (*MySQLStore)(nil)
