package mysql

import (
	"bufio"
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"sync"
	"time"
)

// TaskRecord 表示一次智能体执行的落库结构。
type TaskRecord struct {
	ID          int64  `json:"id"`
	Goal        string `json:"goal"`
	ChainAction string `json:"chain_action"`
	Address     string `json:"address"`
	Thought     string `json:"thought"`
	Reply       string `json:"reply"`
	ChainID     string `json:"chain_id"`
	BlockNumber string `json:"block_number"`
	Observes    string `json:"observes"`
	CreatedAt   int64  `json:"created_at"`
	UpdatedAt   int64  `json:"updated_at"`
}

// TaskRepository 抽象任务数据的持久化接口。
type TaskRepository interface {
	Create(ctx context.Context, record *TaskRecord) error
	GetByID(ctx context.Context, id int64) (*TaskRecord, error)
	Update(ctx context.Context, record TaskRecord) error
	Delete(ctx context.Context, id int64) error
	ListLatest(ctx context.Context, limit int) ([]TaskRecord, error)
	WithTransaction(ctx context.Context, fn func(context.Context, TaskRepository) error) error
}

// MemoryTaskRepository 使用本地 JSON 文件模拟 MySQL 的效果，方便迭代开发。
type MemoryTaskRepository struct {
	mu       sync.RWMutex
	dataFile string
	records  []TaskRecord
	lastID   int64
}

// NewMemoryTaskRepository 创建一个内存任务仓库。
func NewMemoryTaskRepository(dataDir string) (*MemoryTaskRepository, error) {
	if dataDir == "" {
		dataDir = "."
	}
	if err := os.MkdirAll(dataDir, 0o755); err != nil {
		return nil, fmt.Errorf("创建数据目录失败: %w", err)
	}
	path := filepath.Join(dataDir, "tasks.log")
	repo := &MemoryTaskRepository{dataFile: path}
	if err := repo.loadFromDisk(); err != nil {
		return nil, err
	}
	return repo, nil
}

// Create 以追加写的方式记录任务结果。
func (m *MemoryTaskRepository) Create(_ context.Context, record *TaskRecord) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if record == nil {
		return errors.New("record 不能为空")
	}

	m.lastID++
	record.ID = m.lastID
	if record.UpdatedAt == 0 {
		record.UpdatedAt = record.CreatedAt
	}

	m.records = append(m.records, *record)
	m.sortRecords()
	return m.persistToDisk()
}

func (m *MemoryTaskRepository) GetByID(_ context.Context, id int64) (*TaskRecord, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	for i := range m.records {
		if m.records[i].ID == id {
			record := m.records[i]
			return &record, nil
		}
	}
	return nil, sql.ErrNoRows
}

func (m *MemoryTaskRepository) Update(_ context.Context, record TaskRecord) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	updated := false
	for i := range m.records {
		if m.records[i].ID == record.ID {
			if record.UpdatedAt == 0 {
				record.UpdatedAt = time.Now().Unix()
			}
			if record.CreatedAt == 0 {
				record.CreatedAt = m.records[i].CreatedAt
			}
			m.records[i] = record
			updated = true
			break
		}
	}
	if !updated {
		return sql.ErrNoRows
	}
	m.sortRecords()
	return m.persistToDisk()
}

func (m *MemoryTaskRepository) Delete(_ context.Context, id int64) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	idx := -1
	for i := range m.records {
		if m.records[i].ID == id {
			idx = i
			break
		}
	}
	if idx == -1 {
		return sql.ErrNoRows
	}
	m.records = append(m.records[:idx], m.records[idx+1:]...)
	return m.persistToDisk()
}

// ErrUnsupportedDriver 在未来对接真正 MySQL 时使用。
var ErrUnsupportedDriver = errors.New("暂不支持的存储驱动")

// ListLatest 返回最近的任务记录，按时间倒序排列。
func (m *MemoryTaskRepository) ListLatest(_ context.Context, limit int) ([]TaskRecord, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	if limit <= 0 || limit > len(m.records) {
		limit = len(m.records)
	}

	results := make([]TaskRecord, limit)
	copy(results, m.records[:limit])
	return results, nil
}

func (m *MemoryTaskRepository) WithTransaction(ctx context.Context, fn func(context.Context, TaskRepository) error) error {
	if fn == nil {
		return errors.New("事务函数不能为空")
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	tx := &memoryTxRepository{
		records: append([]TaskRecord(nil), m.records...),
		lastID:  m.lastID,
	}

	if err := fn(ctx, tx); err != nil {
		return err
	}

	m.records = append([]TaskRecord(nil), tx.records...)
	m.lastID = tx.lastID
	m.sortRecords()
	return m.persistToDisk()
}

func (m *MemoryTaskRepository) loadFromDisk() error {
	file, err := os.OpenFile(m.dataFile, os.O_RDONLY|os.O_CREATE, 0o644)
	if err != nil {
		return fmt.Errorf("读取任务日志失败: %w", err)
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	var restored []TaskRecord
	var maxID int64
	var fallbackID int64
	for scanner.Scan() {
		var record TaskRecord
		if err := json.Unmarshal(scanner.Bytes(), &record); err != nil {
			continue
		}
		if record.ID == 0 {
			fallbackID++
			record.ID = fallbackID
		}
		if record.CreatedAt == 0 {
			record.CreatedAt = time.Now().Unix()
		}
		if record.UpdatedAt == 0 {
			record.UpdatedAt = record.CreatedAt
		}
		if record.ID > maxID {
			maxID = record.ID
		}
		restored = append(restored, record)
	}
	if err := scanner.Err(); err != nil {
		return fmt.Errorf("解析任务日志失败: %w", err)
	}

	if len(restored) > 0 {
		sortTaskRecords(restored)
		if len(restored) > 512 {
			trimmed := make([]TaskRecord, 512)
			copy(trimmed, restored[:512])
			restored = trimmed
		}
		m.records = restored
		m.lastID = maxID
	}
	return nil
}

func (m *MemoryTaskRepository) persistToDisk() error {
	file, err := os.OpenFile(m.dataFile, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o644)
	if err != nil {
		return fmt.Errorf("打开任务日志失败: %w", err)
	}
	defer file.Close()

	encoder := json.NewEncoder(file)
	for i := len(m.records) - 1; i >= 0; i-- {
		if err := encoder.Encode(m.records[i]); err != nil {
			return fmt.Errorf("写入任务日志失败: %w", err)
		}
	}
	return nil
}

func (m *MemoryTaskRepository) sortRecords() {
	sortTaskRecords(m.records)
	if len(m.records) > 512 {
		records := make([]TaskRecord, 512)
		copy(records, m.records[:512])
		m.records = records
	}
}

func sortTaskRecords(records []TaskRecord) {
	sort.Slice(records, func(i, j int) bool {
		if records[i].CreatedAt == records[j].CreatedAt {
			return records[i].ID > records[j].ID
		}
		return records[i].CreatedAt > records[j].CreatedAt
	})
}

type memoryTxRepository struct {
	records []TaskRecord
	lastID  int64
}

func (t *memoryTxRepository) Create(_ context.Context, record *TaskRecord) error {
	if record == nil {
		return errors.New("record 不能为空")
	}

	t.lastID++
	record.ID = t.lastID
	if record.UpdatedAt == 0 {
		record.UpdatedAt = record.CreatedAt
	}

	t.records = append(t.records, *record)
	sortTaskRecords(t.records)
	if len(t.records) > 512 {
		trimmed := make([]TaskRecord, 512)
		copy(trimmed, t.records[:512])
		t.records = trimmed
	}

	return nil
}

func (t *memoryTxRepository) GetByID(_ context.Context, id int64) (*TaskRecord, error) {
	for i := range t.records {
		if t.records[i].ID == id {
			record := t.records[i]
			return &record, nil
		}
	}
	return nil, sql.ErrNoRows
}

func (t *memoryTxRepository) Update(_ context.Context, record TaskRecord) error {
	for i := range t.records {
		if t.records[i].ID == record.ID {
			if record.UpdatedAt == 0 {
				record.UpdatedAt = time.Now().Unix()
			}
			if record.CreatedAt == 0 {
				record.CreatedAt = t.records[i].CreatedAt
			}
			t.records[i] = record
			sortTaskRecords(t.records)
			if len(t.records) > 512 {
				trimmed := make([]TaskRecord, 512)
				copy(trimmed, t.records[:512])
				t.records = trimmed
			}
			return nil
		}
	}
	return sql.ErrNoRows
}

func (t *memoryTxRepository) Delete(_ context.Context, id int64) error {
	idx := -1
	for i := range t.records {
		if t.records[i].ID == id {
			idx = i
			break
		}
	}
	if idx == -1 {
		return sql.ErrNoRows
	}
	t.records = append(t.records[:idx], t.records[idx+1:]...)
	return nil
}

func (t *memoryTxRepository) ListLatest(_ context.Context, limit int) ([]TaskRecord, error) {
	if limit <= 0 || limit > len(t.records) {
		limit = len(t.records)
	}
	results := make([]TaskRecord, limit)
	copy(results, t.records[:limit])
	return results, nil
}

func (t *memoryTxRepository) WithTransaction(ctx context.Context, fn func(context.Context, TaskRepository) error) error {
	if fn == nil {
		return errors.New("事务函数不能为空")
	}
	return fn(ctx, t)
}

// SQLTaskRepository 使用真实的 MySQL 数据库存储任务信息。
type SQLTaskRepository struct {
	db *sql.DB
}

// Config 控制 MySQL 仓库的连接参数与连接池配置。
type Config struct {
	DSN             string
	MaxOpenConns    int
	MaxIdleConns    int
	ConnMaxLifetime time.Duration
	ConnMaxIdleTime time.Duration
}

// NewSQLTaskRepository 创建连接池并初始化数据表。
func NewSQLTaskRepository(ctx context.Context, cfg Config) (*SQLTaskRepository, error) {
	db, err := openDatabase(ctx, cfg)
	if err != nil {
		return nil, err
	}

	if err := runMigrations(ctx, db); err != nil {
		db.Close()
		return nil, err
	}
	repo := &SQLTaskRepository{db: db}
	return repo, nil
}

func (s *SQLTaskRepository) Create(ctx context.Context, record *TaskRecord) error {
	if record == nil {
		return errors.New("record 不能为空")
	}
	if record.UpdatedAt == 0 {
		record.UpdatedAt = record.CreatedAt
	}

	const stmt = `INSERT INTO tasks
    (goal, chain_action, address, thought, reply, chain_id, block_number, observes, created_at, updated_at)
    VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`

	res, err := s.db.ExecContext(ctx, stmt,
		record.Goal,
		record.ChainAction,
		record.Address,
		record.Thought,
		record.Reply,
		record.ChainID,
		record.BlockNumber,
		record.Observes,
		record.CreatedAt,
		record.UpdatedAt,
	)
	if err != nil {
		return fmt.Errorf("写入 MySQL 失败: %w", err)
	}
	id, err := res.LastInsertId()
	if err != nil {
		return fmt.Errorf("获取插入主键失败: %w", err)
	}
	record.ID = id
	return nil
}

func (s *SQLTaskRepository) GetByID(ctx context.Context, id int64) (*TaskRecord, error) {
	const stmt = `SELECT id, goal, chain_action, address, thought, reply, chain_id, block_number, observes, created_at, updated_at
    FROM tasks WHERE id = ?`

	row := s.db.QueryRowContext(ctx, stmt, id)
	var record TaskRecord
	if err := row.Scan(
		&record.ID,
		&record.Goal,
		&record.ChainAction,
		&record.Address,
		&record.Thought,
		&record.Reply,
		&record.ChainID,
		&record.BlockNumber,
		&record.Observes,
		&record.CreatedAt,
		&record.UpdatedAt,
	); err != nil {
		return nil, err
	}
	return &record, nil
}

func (s *SQLTaskRepository) Update(ctx context.Context, record TaskRecord) error {
	if record.ID == 0 {
		return errors.New("更新任务需要提供 ID")
	}
	if record.UpdatedAt == 0 {
		record.UpdatedAt = time.Now().Unix()
	}

	const stmt = `UPDATE tasks SET goal = ?, chain_action = ?, address = ?, thought = ?, reply = ?, chain_id = ?, block_number = ?, observes = ?, created_at = ?, updated_at = ?
    WHERE id = ?`

	res, err := s.db.ExecContext(ctx, stmt,
		record.Goal,
		record.ChainAction,
		record.Address,
		record.Thought,
		record.Reply,
		record.ChainID,
		record.BlockNumber,
		record.Observes,
		record.CreatedAt,
		record.UpdatedAt,
		record.ID,
	)
	if err != nil {
		return fmt.Errorf("更新任务失败: %w", err)
	}
	affected, err := res.RowsAffected()
	if err != nil {
		return fmt.Errorf("获取更新结果失败: %w", err)
	}
	if affected == 0 {
		return sql.ErrNoRows
	}
	return nil
}

func (s *SQLTaskRepository) Delete(ctx context.Context, id int64) error {
	const stmt = `DELETE FROM tasks WHERE id = ?`
	res, err := s.db.ExecContext(ctx, stmt, id)
	if err != nil {
		return fmt.Errorf("删除任务失败: %w", err)
	}
	affected, err := res.RowsAffected()
	if err != nil {
		return fmt.Errorf("获取删除结果失败: %w", err)
	}
	if affected == 0 {
		return sql.ErrNoRows
	}
	return nil
}

// ListLatest 查询最近的若干条任务记录。
func (s *SQLTaskRepository) ListLatest(ctx context.Context, limit int) ([]TaskRecord, error) {
	if limit <= 0 {
		limit = 20
	}

	rows, err := s.db.QueryContext(ctx, `SELECT id, goal, chain_action, address, thought, reply, chain_id, block_number, observes, created_at, updated_at
    FROM tasks ORDER BY created_at DESC, id DESC LIMIT ?`, limit)
	if err != nil {
		return nil, fmt.Errorf("查询任务记录失败: %w", err)
	}
	defer rows.Close()

	var records []TaskRecord
	for rows.Next() {
		var record TaskRecord
		if err := rows.Scan(
			&record.ID,
			&record.Goal,
			&record.ChainAction,
			&record.Address,
			&record.Thought,
			&record.Reply,
			&record.ChainID,
			&record.BlockNumber,
			&record.Observes,
			&record.CreatedAt,
			&record.UpdatedAt,
		); err != nil {
			return nil, fmt.Errorf("解析任务记录失败: %w", err)
		}
		records = append(records, record)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("遍历任务记录失败: %w", err)
	}

	return records, nil
}

func (s *SQLTaskRepository) WithTransaction(ctx context.Context, fn func(context.Context, TaskRepository) error) error {
	if fn == nil {
		return errors.New("事务函数不能为空")
	}

	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("开启事务失败: %w", err)
	}

	repo := &sqlTxRepository{tx: tx}
	if err := fn(ctx, repo); err != nil {
		if rbErr := tx.Rollback(); rbErr != nil {
			return fmt.Errorf("事务回滚失败: %v, 原始错误: %w", rbErr, err)
		}
		return err
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("提交事务失败: %w", err)
	}
	return nil
}

// Close 关闭底层数据库连接。
func (s *SQLTaskRepository) Close() error {
	if s == nil || s.db == nil {
		return nil
	}
	return s.db.Close()
}

type sqlTxRepository struct {
	tx *sql.Tx
}

func (t *sqlTxRepository) Create(ctx context.Context, record *TaskRecord) error {
	if record == nil {
		return errors.New("record 不能为空")
	}
	if record.UpdatedAt == 0 {
		record.UpdatedAt = record.CreatedAt
	}

	const stmt = `INSERT INTO tasks
    (goal, chain_action, address, thought, reply, chain_id, block_number, observes, created_at, updated_at)
    VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`

	res, err := t.tx.ExecContext(ctx, stmt,
		record.Goal,
		record.ChainAction,
		record.Address,
		record.Thought,
		record.Reply,
		record.ChainID,
		record.BlockNumber,
		record.Observes,
		record.CreatedAt,
		record.UpdatedAt,
	)
	if err != nil {
		return fmt.Errorf("写入 MySQL 失败: %w", err)
	}
	id, err := res.LastInsertId()
	if err != nil {
		return fmt.Errorf("获取插入主键失败: %w", err)
	}
	record.ID = id
	return nil
}

func (t *sqlTxRepository) GetByID(ctx context.Context, id int64) (*TaskRecord, error) {
	const stmt = `SELECT id, goal, chain_action, address, thought, reply, chain_id, block_number, observes, created_at, updated_at
    FROM tasks WHERE id = ? FOR UPDATE`

	row := t.tx.QueryRowContext(ctx, stmt, id)
	var record TaskRecord
	if err := row.Scan(
		&record.ID,
		&record.Goal,
		&record.ChainAction,
		&record.Address,
		&record.Thought,
		&record.Reply,
		&record.ChainID,
		&record.BlockNumber,
		&record.Observes,
		&record.CreatedAt,
		&record.UpdatedAt,
	); err != nil {
		return nil, err
	}
	return &record, nil
}

func (t *sqlTxRepository) Update(ctx context.Context, record TaskRecord) error {
	if record.ID == 0 {
		return errors.New("更新任务需要提供 ID")
	}
	if record.UpdatedAt == 0 {
		record.UpdatedAt = time.Now().Unix()
	}

	const stmt = `UPDATE tasks SET goal = ?, chain_action = ?, address = ?, thought = ?, reply = ?, chain_id = ?, block_number = ?, observes = ?, created_at = ?, updated_at = ?
    WHERE id = ?`

	res, err := t.tx.ExecContext(ctx, stmt,
		record.Goal,
		record.ChainAction,
		record.Address,
		record.Thought,
		record.Reply,
		record.ChainID,
		record.BlockNumber,
		record.Observes,
		record.CreatedAt,
		record.UpdatedAt,
		record.ID,
	)
	if err != nil {
		return fmt.Errorf("更新任务失败: %w", err)
	}
	affected, err := res.RowsAffected()
	if err != nil {
		return fmt.Errorf("获取更新结果失败: %w", err)
	}
	if affected == 0 {
		return sql.ErrNoRows
	}
	return nil
}

func (t *sqlTxRepository) Delete(ctx context.Context, id int64) error {
	const stmt = `DELETE FROM tasks WHERE id = ?`
	res, err := t.tx.ExecContext(ctx, stmt, id)
	if err != nil {
		return fmt.Errorf("删除任务失败: %w", err)
	}
	affected, err := res.RowsAffected()
	if err != nil {
		return fmt.Errorf("获取删除结果失败: %w", err)
	}
	if affected == 0 {
		return sql.ErrNoRows
	}
	return nil
}

func (t *sqlTxRepository) ListLatest(ctx context.Context, limit int) ([]TaskRecord, error) {
	if limit <= 0 {
		limit = 20
	}

	rows, err := t.tx.QueryContext(ctx, `SELECT id, goal, chain_action, address, thought, reply, chain_id, block_number, observes, created_at, updated_at
    FROM tasks ORDER BY created_at DESC, id DESC LIMIT ?`, limit)
	if err != nil {
		return nil, fmt.Errorf("查询任务记录失败: %w", err)
	}
	defer rows.Close()

	var records []TaskRecord
	for rows.Next() {
		var record TaskRecord
		if err := rows.Scan(
			&record.ID,
			&record.Goal,
			&record.ChainAction,
			&record.Address,
			&record.Thought,
			&record.Reply,
			&record.ChainID,
			&record.BlockNumber,
			&record.Observes,
			&record.CreatedAt,
			&record.UpdatedAt,
		); err != nil {
			return nil, fmt.Errorf("解析任务记录失败: %w", err)
		}
		records = append(records, record)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("遍历任务记录失败: %w", err)
	}

	return records, nil
}

func (t *sqlTxRepository) WithTransaction(ctx context.Context, fn func(context.Context, TaskRepository) error) error {
	if fn == nil {
		return errors.New("嵌套事务函数不能为空")
	}
	return fn(ctx, t)
}
