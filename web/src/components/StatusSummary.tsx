import type { TaskItem } from "../types";

interface StatusSummaryProps {
  tasks: TaskItem[];
}

const STATUS_LABELS: Record<string, string> = {
  pending: "排队中",
  running: "执行中",
  succeeded: "已完成",
  failed: "失败"
};

export default function StatusSummary({ tasks }: StatusSummaryProps) {
  const summary = tasks.reduce(
    (acc, task) => {
      acc.total += 1;
      acc[task.status] = (acc[task.status] || 0) + 1;
      return acc;
    },
    { total: 0, pending: 0, running: 0, succeeded: 0, failed: 0 } as Record<string, number>
  );

  const lastGoal = tasks[0]?.goal;

  return (
    <div className="summary-grid">
      <div className="summary-card highlight">
        <span className="summary-title">总任务</span>
        <strong className="summary-value">{summary.total}</strong>
        <p className="helper-text">{lastGoal ? `最新目标：${lastGoal}` : "等待创建新的任务"}</p>
      </div>
      {(["pending", "running", "succeeded", "failed"] as const).map((status) => (
        <div key={status} className={`summary-card ${status}`}>
          <span className="summary-title">{STATUS_LABELS[status]}</span>
          <strong className="summary-value">{summary[status]}</strong>
          <p className="helper-text">
            {status === "succeeded"
              ? "成功完成的智能体任务数量"
              : status === "failed"
                ? "需要人工介入的任务"
                : status === "running"
                  ? "后台正在执行的任务"
                  : "等待调度的任务"}
          </p>
        </div>
      ))}
    </div>
  );
}
