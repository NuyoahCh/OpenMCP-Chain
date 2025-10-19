package task

// TaskStats 聚合了任务状态的统计信息，常用于仪表盘或健康检查。
type TaskStats struct {
	Total           int   `json:"total"`
	Pending         int   `json:"pending"`
	Running         int   `json:"running"`
	Succeeded       int   `json:"succeeded"`
	Failed          int   `json:"failed"`
	OldestUpdatedAt int64 `json:"oldest_updated_at,omitempty"`
	NewestUpdatedAt int64 `json:"newest_updated_at,omitempty"`
}
