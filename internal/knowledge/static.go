package knowledge

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// Provider 定义知识库检索的通用接口。
type Provider interface {
	Query(goal, chainAction string) []Snippet
}

// Snippet 描述可供大模型引用的一段知识。
type Snippet struct {
	Title    string   `json:"title"`
	Content  string   `json:"content"`
	Keywords []string `json:"keywords"`
	Tags     []string `json:"tags"`
}

// StaticProvider 通过加载 JSON 文件提供静态知识检索能力。
type StaticProvider struct {
	items      []Snippet
	maxResults int
}

// NewStaticProvider 创建静态知识库实例。
func NewStaticProvider(items []Snippet, maxResults int) *StaticProvider {
	if maxResults <= 0 {
		maxResults = 3
	}
	return &StaticProvider{
		items:      items,
		maxResults: maxResults,
	}
}

// LoadStaticProvider 从 JSON 文件加载知识条目。
func LoadStaticProvider(path string, maxResults int) (*StaticProvider, error) {
	if strings.TrimSpace(path) == "" {
		return nil, fmt.Errorf("知识库文件路径不能为空")
	}

	absPath, err := filepath.Abs(path)
	if err != nil {
		return nil, fmt.Errorf("解析知识库路径失败: %w", err)
	}

	file, err := os.Open(absPath)
	if err != nil {
		return nil, fmt.Errorf("读取知识库文件失败: %w", err)
	}
	defer file.Close()

	var entries []Snippet
	if err := json.NewDecoder(file).Decode(&entries); err != nil {
		return nil, fmt.Errorf("解析知识库文件失败: %w", err)
	}

	return NewStaticProvider(entries, maxResults), nil
}

// Query 根据目标和链上操作进行简单匹配。
func (p *StaticProvider) Query(goal, chainAction string) []Snippet {
	if p == nil {
		return nil
	}

	goal = strings.ToLower(strings.TrimSpace(goal))
	chainAction = strings.ToLower(strings.TrimSpace(chainAction))

	results := make([]Snippet, 0, p.maxResults)
	for _, item := range p.items {
		if matches(item, goal, chainAction) {
			results = append(results, item)
			if len(results) >= p.maxResults {
				break
			}
		}
	}
	return results
}

func matches(snippet Snippet, goal, chainAction string) bool {
	if len(snippet.Keywords) == 0 {
		return true
	}
	for _, keyword := range snippet.Keywords {
		normalized := strings.ToLower(strings.TrimSpace(keyword))
		if normalized == "" {
			continue
		}
		if strings.Contains(goal, normalized) || strings.Contains(chainAction, normalized) {
			return true
		}
	}
	if len(snippet.Tags) == 0 {
		return false
	}
	for _, tag := range snippet.Tags {
		normalized := strings.ToLower(strings.TrimSpace(tag))
		if normalized == "" {
			continue
		}
		if strings.Contains(goal, normalized) || strings.Contains(chainAction, normalized) {
			return true
		}
	}
	return false
}

// Ensure StaticProvider 实现 Provider 接口。
var _ Provider = (*StaticProvider)(nil)
