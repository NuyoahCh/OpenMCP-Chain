import { useCallback, useMemo, useState } from "react";
import { formatTimestamp, statusLabel } from "../api";
import type { TaskItem } from "../types";

interface TaskDetailsProps {
  task: TaskItem;
  isPolling?: boolean;
}

export default function TaskDetails({ task, isPolling }: TaskDetailsProps) {
  const [copiedId, setCopiedId] = useState(false);
  const [copiedPayload, setCopiedPayload] = useState(false);
  const canCopy = useMemo(
    () => typeof navigator !== "undefined" && Boolean(navigator.clipboard?.writeText),
    []
  );

  const handleCopyId = useCallback(async () => {
    if (!canCopy || !navigator?.clipboard?.writeText) {
      return;
    }
    try {
      await navigator.clipboard.writeText(task.id);
      setCopiedId(true);
      setTimeout(() => setCopiedId(false), 1500);
    } catch (error) {
      console.warn("复制任务 ID 失败", error);
    }
  }, [canCopy, task.id]);

  const handleCopyPayload = useCallback(async () => {
    if (!canCopy || !navigator?.clipboard?.writeText) {
      return;
    }
    try {
      await navigator.clipboard.writeText(JSON.stringify(task, null, 2));
      setCopiedPayload(true);
      setTimeout(() => setCopiedPayload(false), 1500);
    } catch (error) {
      console.warn("复制任务详情失败", error);
    }
  }, [canCopy, task]);

  const blockNumber = task.result?.block_number ?? "-";
  const chainId = task.result?.chain_id ?? "-";

  return (
    <div className="card" style={{ marginTop: "2rem" }}>
      <div className="section-title" style={{ display: "flex", justifyContent: "space-between", alignItems: "center" }}>
        <span>任务详情</span>
        {isPolling ? <span className="status-badge status-running">实时同步中</span> : null}
      </div>

      <p className="helper-text" style={{ marginTop: "-0.5rem", marginBottom: "1.25rem" }}>
        {task.goal}
      </p>

      <div className="meta-row" style={{ marginBottom: "1rem" }}>
        <span>
          <strong>ID:</strong> {task.id}
        </span>
        <button type="button" className="link" onClick={handleCopyId} disabled={!canCopy}>
          {copiedId ? "已复制" : "复制 ID"}
        </button>
        <button type="button" className="link" onClick={handleCopyPayload} disabled={!canCopy}>
          {copiedPayload ? "JSON 已复制" : "复制详情 JSON"}
        </button>
      </div>

      <div className="meta-row" style={{ marginBottom: "1rem" }}>
        <span>
          <strong>状态:</strong> {statusLabel(task.status)}
        </span>
        <span>
          <strong>链上操作:</strong> {task.chain_action || "-"}
        </span>
        <span>
          <strong>地址:</strong> {task.address || "-"}
        </span>
        <span>
          <strong>尝试:</strong> {task.attempts}/{task.max_retries}
        </span>
        <span>
          <strong>更新时间:</strong> {formatTimestamp(task.updated_at)}
        </span>
        <span>
          <strong>创建时间:</strong> {formatTimestamp(task.created_at)}
        </span>
      </div>

      {task.metadata ? (
        <div className="result-panel" style={{ marginBottom: "1rem" }}>
          <h3 style={{ marginTop: 0 }}>Metadata</h3>
          <pre>{JSON.stringify(task.metadata, null, 2)}</pre>
        </div>
      ) : null}

      {task.result ? (
        <div className="result-panel">
          <h3 style={{ marginTop: 0 }}>思考过程</h3>
          <pre>{task.result.thought || "(无思考记录)"}</pre>
          <h3>模型回复</h3>
          <pre>{task.result.reply || "(暂无回复)"}</pre>
          <h3>链上观察</h3>
          <pre>{task.result.observations || "(暂无链上日志)"}</pre>
          <div className="meta-row" style={{ marginTop: "0.75rem" }}>
            <span>
              <strong>链 ID:</strong> {chainId}
            </span>
            <span>
              <strong>区块:</strong> {blockNumber}
            </span>
          </div>
        </div>
      ) : (
        <p className="helper-text" style={{ margin: 0 }}>
          该任务尚未产出最终结果，系统会持续轮询状态并在完成后自动更新。
        </p>
      )}

      {task.status === "failed" && task.last_error ? (
        <p className="helper-text" style={{ color: "#fca5a5", marginTop: "1rem" }}>
          最近错误：{task.last_error}
        </p>
      ) : null}
      {task.status === "failed" && task.error_code ? (
        <p className="helper-text" style={{ color: "#f87171", marginTop: "0.5rem" }}>
          错误代码：{task.error_code}
        </p>
      ) : null}
    </div>
  );
}
