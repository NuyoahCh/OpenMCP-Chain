import { formatTimestamp } from "../api";
import type { TaskItem, TaskStats } from "../types";

interface StatusSummaryProps {
  tasks: TaskItem[];
  stats?: TaskStats | null;
  loading?: boolean;
  searchQuery?: string;
}

const STATUS_LABELS: Record<string, string> = {
  pending: "排队中",
  running: "执行中",
  succeeded: "已完成",
  failed: "失败"
};

export default function StatusSummary({
  tasks,
  stats,
  loading,
  searchQuery
}: StatusSummaryProps) {
export default function StatusSummary({ tasks, stats, loading }: StatusSummaryProps) {
  const summary = stats
    ? {
        total: stats.total,
        pending: stats.pending,
        running: stats.running,
        succeeded: stats.succeeded,
        failed: stats.failed,
      }
    : tasks.reduce(
        (acc, task) => {
          acc.total += 1;
          acc[task.status] = (acc[task.status] || 0) + 1;
          return acc;
        },
        { total: 0, pending: 0, running: 0, succeeded: 0, failed: 0 } as Record<string, number>,
      );

  const newestTimestamp = (() => {
    if (stats && stats.newest_updated_at && stats.newest_updated_at > 0) {
      return stats.newest_updated_at;
    }
    const latestTask = tasks[0];
    return latestTask?.updated_at ?? null;
  })();

  const lastGoal = tasks[0]?.goal;
  const highlightHelper = (() => {
    if (loading && !stats && tasks.length === 0) {
      return "正在汇总任务统计...";
    }
    const parts: string[] = [];
    if (newestTimestamp) {
      parts.push(`最近更新：${formatTimestamp(newestTimestamp)}`);
    }
    if (lastGoal) {
      parts.push(`最新目标：${lastGoal}`);
    } else {
      parts.push("等待创建新的任务");
    }
    if (searchQuery) {
      parts.push(`关键字：${searchQuery}`);
    }
    return parts.join(" · ");
  })();

  return (
    <div className="summary-grid">
      <div className="summary-card highlight">
        <span className="summary-title">总任务</span>
        <strong className="summary-value">{summary.total}</strong>
        <p className="helper-text">{highlightHelper}</p>
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
