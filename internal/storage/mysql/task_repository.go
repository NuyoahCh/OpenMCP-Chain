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
	"strings"
	"sync"
	"time"
)

// TaskRecord 表示一次智能体执行的落库结构。
type TaskRecord struct {
	Goal        string
	ChainAction string
	Address     string
	Thought     string
	Reply       string
	ChainID     string
	BlockNumber string
	Observes    string
	CreatedAt   int64
}

// TaskRepository 抽象任务数据的持久化接口。
type TaskRepository interface {
	Save(ctx context.Context, record TaskRecord) error
	ListLatest(ctx context.Context, limit int) ([]TaskRecord, error)
}

// MemoryTaskRepository 使用本地 JSON 文件模拟 MySQL 的效果，方便迭代开发。
type MemoryTaskRepository struct {
	mu       sync.RWMutex
	dataFile string
	records  []TaskRecord
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

// Save 以追加写的方式记录任务结果。
func (m *MemoryTaskRepository) Save(_ context.Context, record TaskRecord) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	file, err := os.OpenFile(m.dataFile, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o644)
	if err != nil {
		return fmt.Errorf("打开任务日志失败: %w", err)
	}
	defer file.Close()

	encoded, err := json.Marshal(record)
	if err != nil {
		return fmt.Errorf("序列化任务记录失败: %w", err)
	}

	if _, err := file.Write(append(encoded, '\n')); err != nil {
		return fmt.Errorf("写入任务日志失败: %w", err)
	}

	m.records = append([]TaskRecord{record}, m.records...)
	if len(m.records) > 512 {
		m.records = m.records[:512]
	}
	return nil
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

func (m *MemoryTaskRepository) loadFromDisk() error {
	file, err := os.OpenFile(m.dataFile, os.O_RDONLY|os.O_CREATE, 0o644)
	if err != nil {
		return fmt.Errorf("读取任务日志失败: %w", err)
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	var restored []TaskRecord
	for scanner.Scan() {
		var record TaskRecord
		if err := json.Unmarshal(scanner.Bytes(), &record); err != nil {
			continue
		}
		restored = append([]TaskRecord{record}, restored...)
	}
	if err := scanner.Err(); err != nil {
		return fmt.Errorf("解析任务日志失败: %w", err)
	}

	if len(restored) > 512 {
		restored = restored[:512]
	}
	if len(restored) > 0 {
		m.records = restored
	}
	return nil
}

// SQLTaskRepository 使用真实的 MySQL 数据库存储任务信息。
type SQLTaskRepository struct {
	db *sql.DB
}

// NewSQLTaskRepository 创建连接池并初始化数据表。
func NewSQLTaskRepository(dsn string) (*SQLTaskRepository, error) {
	if strings.TrimSpace(dsn) == "" {
		return nil, fmt.Errorf("MySQL DSN 不能为空")
	}

	db, err := sql.Open("mysql", dsn)
	if err != nil {
		return nil, fmt.Errorf("连接 MySQL 失败: %w", err)
	}

	db.SetMaxOpenConns(10)
	db.SetMaxIdleConns(5)
	db.SetConnMaxLifetime(5 * time.Minute)

	if err := db.Ping(); err != nil {
		return nil, fmt.Errorf("无法连接到 MySQL: %w", err)
	}

	repo := &SQLTaskRepository{db: db}
	if err := repo.initSchema(); err != nil {
		return nil, err
	}
	return repo, nil
}

func (s *SQLTaskRepository) initSchema() error {
	const schema = `CREATE TABLE IF NOT EXISTS tasks (
        id BIGINT AUTO_INCREMENT PRIMARY KEY,
        goal TEXT NOT NULL,
        chain_action VARCHAR(255) DEFAULT '',
        address VARCHAR(255) DEFAULT '',
        thought TEXT NOT NULL,
        reply TEXT NOT NULL,
        chain_id VARCHAR(66) DEFAULT '',
        block_number VARCHAR(66) DEFAULT '',
        observes TEXT NOT NULL,
        created_at BIGINT NOT NULL,
        INDEX idx_created_at (created_at)
)`

	if _, err := s.db.Exec(schema); err != nil {
		return fmt.Errorf("初始化 tasks 表失败: %w", err)
	}
	return nil
}

// Save 将任务记录写入 MySQL。
func (s *SQLTaskRepository) Save(ctx context.Context, record TaskRecord) error {
	const stmt = `INSERT INTO tasks
        (goal, chain_action, address, thought, reply, chain_id, block_number, observes, created_at)
        VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)`

	if _, err := s.db.ExecContext(ctx, stmt,
		record.Goal,
		record.ChainAction,
		record.Address,
		record.Thought,
		record.Reply,
		record.ChainID,
		record.BlockNumber,
		record.Observes,
		record.CreatedAt,
	); err != nil {
		return fmt.Errorf("写入 MySQL 失败: %w", err)
	}
	return nil
}

// ListLatest 查询最近的若干条任务记录。
func (s *SQLTaskRepository) ListLatest(ctx context.Context, limit int) ([]TaskRecord, error) {
	if limit <= 0 {
		limit = 20
	}

	rows, err := s.db.QueryContext(ctx, `SELECT goal, chain_action, address, thought, reply, chain_id, block_number, observes, created_at
        FROM tasks ORDER BY id DESC LIMIT ?`, limit)
	if err != nil {
		return nil, fmt.Errorf("查询任务记录失败: %w", err)
	}
	defer rows.Close()

	var records []TaskRecord
	for rows.Next() {
		var record TaskRecord
		if err := rows.Scan(&record.Goal, &record.ChainAction, &record.Address, &record.Thought, &record.Reply, &record.ChainID, &record.BlockNumber, &record.Observes, &record.CreatedAt); err != nil {
			return nil, fmt.Errorf("解析任务记录失败: %w", err)
		}
		records = append(records, record)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("遍历任务记录失败: %w", err)
	}

	return records, nil
}

// Close 关闭底层数据库连接。
func (s *SQLTaskRepository) Close() error {
	if s == nil || s.db == nil {
		return nil
	}
	return s.db.Close()
}
