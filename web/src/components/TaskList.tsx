import { formatTimestamp, statusClassName, statusLabel } from "../api";
import type { TaskItem } from "../types";

interface TaskListProps {
  tasks: TaskItem[];
  onSelect?: (task: TaskItem) => void;
  activeTaskId?: string | null;
}

export default function TaskList({ tasks, onSelect, activeTaskId }: TaskListProps) {
  if (!tasks.length) {
    return (
      <div className="card">
        <h2 className="section-title">最新任务</h2>
        <p className="helper-text">暂无历史记录，提交任务后可查看执行轨迹。</p>
      </div>
    );
  }

  return (
    <div className="card">
      <div className="section-title" style={{ display: "flex", justifyContent: "space-between" }}>
        <span>最新任务</span>
        <span className="helper-text">自动同步最近 {tasks.length} 条记录</span>
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
            </article>
          );
        })}
      </div>
    </div>
  );
}
