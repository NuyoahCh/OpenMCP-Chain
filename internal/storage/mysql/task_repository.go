package mysql

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sync"
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
}

// MemoryTaskRepository 使用本地 JSON 文件模拟 MySQL 的效果，方便迭代开发。
type MemoryTaskRepository struct {
	mu       sync.Mutex
	dataFile string
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
	return &MemoryTaskRepository{dataFile: path}, nil
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

	line := fmt.Sprintf("goal=%s|action=%s|address=%s|thought=%s|reply=%s|chain_id=%s|block=%s|observe=%s|created_at=%d\n",
		record.Goal, record.ChainAction, record.Address, record.Thought, record.Reply, record.ChainID, record.BlockNumber, record.Observes, record.CreatedAt)

	if _, err := file.WriteString(line); err != nil {
		return fmt.Errorf("写入任务日志失败: %w", err)
	}
	return nil
}

// ErrUnsupportedDriver 在未来对接真正 MySQL 时使用。
var ErrUnsupportedDriver = errors.New("暂不支持的存储驱动")
