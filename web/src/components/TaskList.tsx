import { formatTimestamp, statusClassName, statusLabel } from "../api";
import type { TaskItem, TaskStatus } from "../types";

export type TaskStatusFilter = "all" | TaskStatus;

interface TaskListProps {
  tasks: TaskItem[];
  totalCount: number;
  onSelect?: (task: TaskItem) => void;
  activeTaskId?: string | null;
  loading?: boolean;
  error?: string | null;
  onRetry?: () => void;
  statusFilter: TaskStatusFilter;
  onStatusFilterChange: (value: TaskStatusFilter) => void;
  onExport?: () => void;
}

const STATUS_OPTIONS: Array<{ value: TaskStatusFilter; label: string }> = [
  { value: "all", label: "全部" },
  { value: "pending", label: "等待执行" },
  { value: "running", label: "执行中" },
  { value: "succeeded", label: "已完成" },
  { value: "failed", label: "失败" }
];

export default function TaskList({
  tasks,
  totalCount,
  onSelect,
  activeTaskId,
  loading,
  error,
  onRetry,
  statusFilter,
  onStatusFilterChange,
  onExport
}: TaskListProps) {
  if (loading) {
    return (
      <div className="card">
        <h2 className="section-title">最新任务</h2>
        <div className="task-list skeleton">
          {Array.from({ length: 3 }).map((_, index) => (
            <div key={index} className="task-card skeleton-card">
              <div className="skeleton-line" style={{ width: "65%" }} />
              <div className="skeleton-line" style={{ width: "85%" }} />
              <div className="skeleton-line" style={{ width: "45%" }} />
            </div>
          ))}
        </div>
      </div>
    );
  }

  if (error) {
    return (
      <div className="card">
        <h2 className="section-title">最新任务</h2>
        <p className="helper-text" style={{ color: "#fda4af" }}>{error}</p>
        <div className="actions" style={{ marginTop: "1rem" }}>
          <button type="button" className="secondary" onClick={() => onRetry?.()}>
            重试同步
          </button>
        </div>
      </div>
    );
  }

  if (!tasks.length) {
    const hint =
      statusFilter !== "all" && totalCount > 0
        ? "当前筛选条件下暂无记录，尝试切换其他状态。"
        : "暂无历史记录，提交任务后可查看执行轨迹。";
    return (
      <div className="card">
        <h2 className="section-title">最新任务</h2>
        <p className="helper-text">{hint}</p>
        <div className="list-toolbar">
          <label htmlFor="task-status-filter">状态筛选</label>
          <select
            id="task-status-filter"
            value={statusFilter}
            onChange={(event) => onStatusFilterChange(event.target.value as TaskStatusFilter)}
          >
            {STATUS_OPTIONS.map((option) => (
              <option key={option.value} value={option.value}>
                {option.label}
              </option>
            ))}
          </select>
          <button type="button" className="ghost" onClick={onExport} disabled={!totalCount}>
            导出 JSON
          </button>
        </div>
      </div>
    );
  }

  return (
    <div className="card">
      <div
        className="section-title"
        style={{ display: "flex", justifyContent: "space-between", alignItems: "center" }}
      >
        <span>最新任务</span>
        <span className="helper-text">
          {totalCount > tasks.length
            ? `共 ${totalCount} 条记录，展示最近 ${tasks.length} 条`
            : `自动同步最近 ${tasks.length} 条记录`}
        </span>
      </div>
      <div className="list-toolbar">
        <label htmlFor="task-status-filter">状态筛选</label>
        <select
          id="task-status-filter"
          value={statusFilter}
          onChange={(event) => onStatusFilterChange(event.target.value as TaskStatusFilter)}
        >
          {STATUS_OPTIONS.map((option) => (
            <option key={option.value} value={option.value}>
              {option.label}
            </option>
          ))}
        </select>
        <button type="button" className="ghost" onClick={onExport} disabled={!tasks.length}>
          导出 JSON
        </button>
      </div>
      <div className="task-list">
        {tasks.map((task) => {
          const isActive = activeTaskId === task.id;
          return (
            <article
              key={task.id}
              className="task-card"
              style={isActive ? { borderColor: "rgba(59, 130, 246, 0.45)" } : undefined}
              onClick={() => onSelect?.(task)}
            >
              <div style={{ display: "flex", justifyContent: "space-between", alignItems: "center" }}>
                <div>
                  <h3 style={{ margin: "0 0 0.4rem", fontSize: "1.05rem" }}>{task.goal}</h3>
                  <div className="meta-row">
                    <span>
                      <strong>ID:</strong> {task.id}
                    </span>
                    <span>
                      <strong>链上操作:</strong> {task.chain_action || "-"}
                    </span>
                    <span>
                      <strong>地址:</strong> {task.address || "-"}
                    </span>
                    <span>
                      <strong>更新:</strong> {formatTimestamp(task.updated_at)}
                    </span>
                  </div>
                </div>
                <span className={statusClassName(task.status)}>{statusLabel(task.status)}</span>
              </div>
              {task.result ? (
                <div className="result-panel">
                  <div className="meta-row" style={{ marginBottom: "0.75rem" }}>
                    <span>
                      <strong>链 ID:</strong> {task.result.chain_id || "-"}
                    </span>
                    <span>
                      <strong>区块:</strong> {task.result.block_number || "-"}
                    </span>
                    <span>
                      <strong>尝试:</strong> {task.attempts}/{task.max_retries}
                    </span>
                  </div>
                  <pre>{task.result.reply || "(无回复)"}</pre>
                </div>
              ) : null}
              {task.status === "failed" && task.last_error ? (
                <p className="helper-text" style={{ color: "#fca5a5", marginTop: "0.75rem" }}>
                  {task.last_error}
                </p>
              ) : null}
              {task.status === "failed" && task.error_code ? (
                <p className="helper-text" style={{ color: "#f87171", marginTop: "0.25rem" }}>
                  错误代码：{task.error_code}
                </p>
              ) : null}
            </article>
          );
        })}
      </div>
    </div>
  );
}
