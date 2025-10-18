package task

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/go-sql-driver/mysql"
)

// MySQLStore 使用 MySQL 记录任务状态。
type MySQLStore struct {
	db *sql.DB
}

// NewMySQLStore 创建一个新的 MySQLStore。
func NewMySQLStore(dsn string) (*MySQLStore, error) {
	if strings.TrimSpace(dsn) == "" {
		return nil, fmt.Errorf("MySQL DSN 不能为空")
	}

	db, err := sql.Open("mysql", dsn)
	if err != nil {
		return nil, fmt.Errorf("连接 MySQL 失败: %w", err)
	}

	db.SetMaxOpenConns(20)
	db.SetMaxIdleConns(10)
	db.SetConnMaxLifetime(10 * time.Minute)

	if err := db.Ping(); err != nil {
		return nil, fmt.Errorf("无法连接到 MySQL: %w", err)
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
        status VARCHAR(32) NOT NULL,
        attempts INT NOT NULL DEFAULT 0,
        max_retries INT NOT NULL DEFAULT 3,
        last_error TEXT,
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
		return fmt.Errorf("初始化 task_states 表失败: %w", err)
	}
	return nil
}

// Create 插入新的任务记录。
func (s *MySQLStore) Create(ctx context.Context, task *Task) error {
	if task == nil {
		return errors.New("task 不能为空")
	}
	if strings.TrimSpace(task.ID) == "" {
		return errors.New("任务 ID 不能为空")
	}

	now := time.Now().Unix()
	task.CreatedAt = now
	task.UpdatedAt = now

	const stmt = `INSERT INTO task_states
        (id, goal, chain_action, address, status, attempts, max_retries, last_error, created_at, updated_at)
        VALUES (?, ?, ?, ?, ?, ?, ?, '', ?, ?)`

	_, err := s.db.ExecContext(ctx, stmt,
		task.ID,
		task.Goal,
		task.ChainAction,
		task.Address,
		task.Status,
		task.Attempts,
		task.MaxRetries,
		task.CreatedAt,
		task.UpdatedAt,
	)
	if err != nil {
		var mysqlErr *mysql.MySQLError
		if errors.As(err, &mysqlErr) && mysqlErr.Number == 1062 {
			return ErrTaskConflict
		}
		return fmt.Errorf("插入任务失败: %w", err)
	}
	return nil
}

// Get 查询指定任务。
func (s *MySQLStore) Get(ctx context.Context, id string) (*Task, error) {
	const stmt = `SELECT id, goal, chain_action, address, status, attempts, max_retries, last_error,
        result_thought, result_reply, result_chain_id, result_block_number, result_observations, created_at, updated_at
        FROM task_states WHERE id = ?`

	row := s.db.QueryRowContext(ctx, stmt, id)

	var task Task
	var result ExecutionResult
	var hasResult bool

	if err := row.Scan(
		&task.ID,
		&task.Goal,
		&task.ChainAction,
		&task.Address,
		&task.Status,
		&task.Attempts,
		&task.MaxRetries,
		&task.LastError,
		&result.Thought,
		&result.Reply,
		&result.ChainID,
		&result.BlockNumber,
		&result.Observations,
		&task.CreatedAt,
		&task.UpdatedAt,
	); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrTaskNotFound
		}
		return nil, fmt.Errorf("查询任务失败: %w", err)
	}

	if result.Thought != "" || result.Reply != "" || result.ChainID != "" || result.BlockNumber != "" || result.Observations != "" {
		task.Result = &result
		hasResult = true
	}
	if !hasResult {
		task.Result = nil
	}
	return &task, nil
}

// Claim 将任务标记为运行中并返回最新状态。
func (s *MySQLStore) Claim(ctx context.Context, id string) (*Task, error) {
	const updateStmt = `UPDATE task_states SET status = ?, attempts = attempts + 1, updated_at = ?, last_error = ''
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
		return nil, fmt.Errorf("更新任务状态失败: %w", err)
	}
	affected, err := res.RowsAffected()
	if err != nil {
		return nil, fmt.Errorf("获取影响行数失败: %w", err)
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
        result_block_number = ?, result_observations = ?, updated_at = ?, last_error = '' WHERE id = ?`

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
		return fmt.Errorf("标记任务成功失败: %w", err)
	}
	if rows, _ := res.RowsAffected(); rows == 0 {
		return ErrTaskNotFound
	}
	return nil
}

// MarkFailed 将任务标记为失败，并在必要时终止重试。
func (s *MySQLStore) MarkFailed(ctx context.Context, id string, lastError string, terminal bool) error {
	const stmt = `UPDATE task_states SET status = ?, last_error = ?, updated_at = ? WHERE id = ?`

	status := StatusFailed
	if terminal {
		status = StatusFailed
	}
	now := time.Now().Unix()
	res, err := s.db.ExecContext(ctx, stmt,
		status,
		lastError,
		now,
		id,
	)
	if err != nil {
		return fmt.Errorf("标记任务失败失败: %w", err)
	}
	if rows, _ := res.RowsAffected(); rows == 0 {
		return ErrTaskNotFound
	}
	return nil
}

// List 返回最近的任务。
func (s *MySQLStore) List(ctx context.Context, limit int) ([]*Task, error) {
	if limit <= 0 {
		limit = 20
	}
	const stmt = `SELECT id, goal, chain_action, address, status, attempts, max_retries, last_error,
        result_thought, result_reply, result_chain_id, result_block_number, result_observations, created_at, updated_at
        FROM task_states ORDER BY created_at DESC LIMIT ?`

	rows, err := s.db.QueryContext(ctx, stmt, limit)
	if err != nil {
		return nil, fmt.Errorf("查询任务列表失败: %w", err)
	}
	defer rows.Close()

	var tasks []*Task
	for rows.Next() {
		var task Task
		var result ExecutionResult
		if err := rows.Scan(
			&task.ID,
			&task.Goal,
			&task.ChainAction,
			&task.Address,
			&task.Status,
			&task.Attempts,
			&task.MaxRetries,
			&task.LastError,
			&result.Thought,
			&result.Reply,
			&result.ChainID,
			&result.BlockNumber,
			&result.Observations,
			&task.CreatedAt,
			&task.UpdatedAt,
		); err != nil {
			return nil, fmt.Errorf("解析任务记录失败: %w", err)
		}
		if result.Thought != "" || result.Reply != "" || result.ChainID != "" || result.BlockNumber != "" || result.Observations != "" {
			task.Result = &result
		}
		taskCopy := task
		tasks = append(tasks, &taskCopy)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("遍历任务失败: %w", err)
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
