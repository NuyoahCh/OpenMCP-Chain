import { useCallback, useEffect, useState } from "react";
import { createTask, fetchTask, listTasks, statusLabel, verifyApiConnection } from "./api";
import TaskForm from "./components/TaskForm";
import TaskList from "./components/TaskList";
import TaskDetails from "./components/TaskDetails";
import StatusSummary from "./components/StatusSummary";
import ConnectionSettings from "./components/ConnectionSettings";
import { useApiBaseUrl } from "./hooks/useApiBaseUrl";
import { useCallback, useEffect, useMemo, useState } from "react";
import { createTask, fetchTask, listTasks, statusLabel } from "./api";
import TaskForm from "./components/TaskForm";
import TaskList from "./components/TaskList";
import type { CreateTaskRequest, TaskItem } from "./types";

interface ToastState {
  title: string;
  message: string;
}

function useToast(timeout = 5200) {
  const [toast, setToast] = useState<ToastState | null>(null);

  useEffect(() => {
    if (!toast) {
      return;
    }
    const timer = setTimeout(() => setToast(null), timeout);
    return () => clearTimeout(timer);
  }, [toast, timeout]);

  return { toast, showToast: setToast } as const;
}

export default function App() {
  const [tasks, setTasks] = useState<TaskItem[]>([]);
  const [submitting, setSubmitting] = useState(false);
  const [activeTask, setActiveTask] = useState<TaskItem | null>(null);
  const [isPolling, setIsPolling] = useState(false);
  const [initialLoading, setInitialLoading] = useState(true);
  const [refreshing, setRefreshing] = useState(false);
  const [fetchError, setFetchError] = useState<string | null>(null);
  const [lastSynced, setLastSynced] = useState<number | null>(null);
  const [connectionStatus, setConnectionStatus] = useState<"idle" | "success" | "error">("idle");
  const [testingConnection, setTestingConnection] = useState(false);
  const { baseUrl, defaultBaseUrl, update, reset } = useApiBaseUrl();
  const { toast, showToast } = useToast();

  const refreshTasks = useCallback(
    async (options?: { manual?: boolean; silent?: boolean }) => {
      if (options?.manual) {
        setRefreshing(true);
      }
      try {
        const data = await listTasks(20);
        const normalized = [...data].sort((a, b) => b.updated_at - a.updated_at);
        setTasks(normalized);
        setFetchError(null);
        setLastSynced(Date.now());
        setConnectionStatus("success");
        setActiveTask((current) => {
          if (!current) {
            return normalized[0] ?? null;
          }
          const updated = normalized.find((item) => item.id === current.id);
          return updated ?? normalized[0] ?? current;
        });
        return true;
      } catch (error) {
        const message = error instanceof Error ? error.message : "无法同步任务列表";
        setFetchError(message);
        setConnectionStatus("error");
        if (!options?.silent) {
          showToast({
            title: "同步失败",
            message
          });
        }
        return false;
      } finally {
        if (options?.manual) {
          setRefreshing(false);
        }
        setInitialLoading(false);
      }
    },
    [showToast]
  );

  useEffect(() => {
    refreshTasks({ silent: true });
    const interval = setInterval(() => {
      refreshTasks({ silent: true });
    }, 15000);
    return () => clearInterval(interval);
  }, [refreshTasks]);

  const pollTask = useCallback(
    async (taskId: string) => {
      setIsPolling(true);
      try {
        let attempts = 0;
        const maxAttempts = 40;
        while (attempts < maxAttempts) {
          const task = await fetchTask(taskId);
          setActiveTask(task);
          await refreshTasks({ silent: true });
          if (task.status === "succeeded" || task.status === "failed") {
            showToast({
              title: `任务${task.status === "succeeded" ? "完成" : "结束"}`,
              message: statusLabel(task.status)
            });
            break;
          }
          attempts += 1;
          await new Promise((resolve) => setTimeout(resolve, Math.min(2000 + attempts * 200, 6000)));
        }
      } catch (error) {
        console.error("轮询任务失败", error);
        showToast({
          title: "任务轮询失败",
          message: error instanceof Error ? error.message : "未知错误"
        });
      } finally {
        setIsPolling(false);
        await refreshTasks({ silent: true });
      }
    },
    [refreshTasks, showToast]
  );

  const handleSubmit = useCallback(
    async (payload: CreateTaskRequest) => {
      setSubmitting(true);
  const [loading, setLoading] = useState(false);
  const [activeTask, setActiveTask] = useState<TaskItem | null>(null);
  const [isPolling, setIsPolling] = useState(false);
  const { toast, showToast } = useToast();

  const refreshTasks = useCallback(async () => {
    try {
      const data = await listTasks(20);
      setTasks(data);
      if (activeTask) {
        const updated = data.find((item) => item.id === activeTask.id);
        if (updated) {
          setActiveTask(updated);
        }
      }
    } catch (error) {
      console.error("加载任务失败", error);
    }
  }, [activeTask]);

  useEffect(() => {
    refreshTasks();
    const interval = setInterval(refreshTasks, 15000);
    return () => clearInterval(interval);
  }, [refreshTasks]);

  const pollTask = useCallback(async (taskId: string) => {
    setIsPolling(true);
    try {
      let attempts = 0;
      const maxAttempts = 40;
      while (attempts < maxAttempts) {
        const task = await fetchTask(taskId);
        setActiveTask(task);
        await refreshTasks();
        if (task.status === "succeeded" || task.status === "failed") {
          showToast({
            title: `任务${task.status === "succeeded" ? "完成" : "结束"}`,
            message: statusLabel(task.status)
          });
          break;
        }
        attempts += 1;
        await new Promise((resolve) => setTimeout(resolve, Math.min(2000 + attempts * 200, 6000)));
      }
    } catch (error) {
      console.error("轮询任务失败", error);
      showToast({
        title: "任务轮询失败",
        message: error instanceof Error ? error.message : "未知错误"
      });
    } finally {
      setIsPolling(false);
      await refreshTasks();
    }
  }, [refreshTasks, showToast]);

  const handleSubmit = useCallback(
    async (payload: CreateTaskRequest) => {
      setLoading(true);
      try {
        const response = await createTask(payload);
        showToast({
          title: "任务已提交",
          message: `ID: ${response.task_id}`
        });
        await refreshTasks({ silent: true });
        await refreshTasks();
        pollTask(response.task_id);
      } catch (error) {
        showToast({
          title: "提交失败",
          message: error instanceof Error ? error.message : "未知错误"
        });
      } finally {
        setSubmitting(false);
        setLoading(false);
      }
    },
    [pollTask, refreshTasks, showToast]
  );

  const handleManualRefresh = useCallback(() => {
    refreshTasks({ manual: true });
  }, [refreshTasks]);

  const handleUpdateBaseUrl = useCallback(
    async (value: string) => {
      const next = update(value);
      setConnectionStatus("idle");
      const success = await refreshTasks({ manual: true, silent: true });
      if (!success) {
        throw new Error("无法连接到新的 API 地址");
      }
      showToast({
        title: "服务地址已更新",
        message: next
      });
    },
    [refreshTasks, showToast, update]
  );

  const handleResetBaseUrl = useCallback(async () => {
    const next = reset();
    setConnectionStatus("idle");
    const success = await refreshTasks({ manual: true, silent: true });
    if (!success) {
      throw new Error("无法连接默认地址，请确认后端服务状态");
    }
    showToast({
      title: "已恢复默认地址",
      message: next
    });
  }, [refreshTasks, reset, showToast]);

  const handleTestConnection = useCallback(async () => {
    setTestingConnection(true);
    try {
      await verifyApiConnection();
      setConnectionStatus("success");
      showToast({
        title: "连接正常",
        message: baseUrl
      });
      await refreshTasks({ silent: true });
    } catch (error) {
      const message = error instanceof Error ? error.message : "无法连接后端";
      setConnectionStatus("error");
      showToast({
        title: "连接失败",
        message
      });
    } finally {
      setTestingConnection(false);
    }
  }, [baseUrl, refreshTasks, showToast]);
  const activeTaskId = activeTask?.id ?? null;
  const sortedTasks = useMemo(() => {
    return [...tasks].sort((a, b) => b.updated_at - a.updated_at);
  }, [tasks]);

  const selectedTask = useMemo(() => {
    if (!activeTaskId) {
      return null;
    }
    return tasks.find((task) => task.id === activeTaskId) ?? activeTask;
  }, [activeTask, activeTaskId, tasks]);

  useEffect(() => {
    if (!activeTask && sortedTasks.length) {
      setActiveTask(sortedTasks[0]);
    }
  }, [activeTask, sortedTasks]);

  return (
    <main>
      <header className="header">
        <div>
          <h1 className="gradient-text">OpenMCP Chain 控制台</h1>
          <p>
            通过可视化界面调度智能体任务，追踪大模型推理与链上观测的完整过程。实时掌握执行状态，快速定位异常。
          </p>
        </div>
        <ConnectionSettings
          baseUrl={baseUrl}
          defaultBaseUrl={defaultBaseUrl}
          onUpdate={handleUpdateBaseUrl}
          onReset={handleResetBaseUrl}
          onTest={handleTestConnection}
          testing={testingConnection}
          status={connectionStatus}
          lastSynced={lastSynced}
          refreshing={refreshing}
          onRefresh={handleManualRefresh}
          fetchError={fetchError}
        />
      </header>

      <div className="glow-border" style={{ marginBottom: "2.5rem" }}>
        <TaskForm onSubmit={handleSubmit} submitting={submitting} />
      </div>

      <StatusSummary tasks={tasks} />

      <TaskList
        tasks={tasks}
        activeTaskId={activeTask?.id}
        loading={initialLoading}
        error={fetchError}
        onRetry={handleManualRefresh}
        <div className="card" style={{ padding: "1rem 1.25rem", minWidth: "220px" }}>
          <h3 style={{ margin: 0, fontSize: "1rem" }}>运行提示</h3>
          <p className="helper-text" style={{ marginTop: "0.35rem" }}>
            默认连接 <code>http://127.0.0.1:8080</code>，可通过 <code>VITE_API_BASE_URL</code> 覆盖。
          </p>
          {isPolling ? <p className="helper-text">正在等待最新任务完成...</p> : null}
        </div>
      </header>

      <div className="glow-border" style={{ marginBottom: "2.5rem" }}>
        <TaskForm onSubmit={handleSubmit} loading={loading} />
      </div>

      <TaskList
        tasks={sortedTasks}
        tasks={tasks}
        activeTaskId={selectedTask?.id}
        onSelect={(task) => {
          setActiveTask(task);
          pollTask(task.id);
        }}
      />

      {activeTask ? <TaskDetails task={activeTask} isPolling={isPolling} /> : null}
      {selectedTask ? (
        <div className="card" style={{ marginTop: "2rem" }}>
          <h2 className="section-title">任务详情</h2>
      {selectedTask && selectedTask.result ? (
        <div className="card" style={{ marginTop: "2rem" }}>
          <h2 className="section-title">最新结果</h2>
          <div className="meta-row" style={{ marginBottom: "1rem" }}>
            <span>
              <strong>ID:</strong> {selectedTask.id}
            </span>
            <span>
              <strong>状态:</strong> {statusLabel(selectedTask.status)}
            </span>
            <span>
              <strong>链上操作:</strong> {selectedTask.chain_action || "-"}
            </span>
            <span>
              <strong>地址:</strong> {selectedTask.address || "-"}
            </span>
            <span>
              <strong>尝试:</strong> {selectedTask.attempts}/{selectedTask.max_retries}
            </span>
            <span>
              <strong>更新时间:</strong> {new Date(selectedTask.updated_at * 1000).toLocaleString()}
            </span>
            <span>
              <strong>创建时间:</strong> {new Date(selectedTask.created_at * 1000).toLocaleString()}
            </span>
          </div>
          {selectedTask.result ? (
            <div className="result-panel">
              <h3 style={{ marginTop: 0 }}>思考过程</h3>
              <pre>{selectedTask.result.thought || "(无思考记录)"}</pre>
              <h3>模型回复</h3>
              <pre>{selectedTask.result.reply || "(暂无回复)"}</pre>
              <h3>链上观察</h3>
              <pre>{selectedTask.result.observations || "(暂无链上日志)"}</pre>
              <div className="meta-row" style={{ marginTop: "0.75rem" }}>
                <span>
                  <strong>链 ID:</strong> {selectedTask.result.chain_id || "-"}
                </span>
                <span>
                  <strong>区块:</strong> {selectedTask.result.block_number || "-"}
                </span>
              </div>
            </div>
          ) : (
            <p className="helper-text" style={{ margin: 0 }}>
              该任务尚未产出最终结果，系统会持续轮询状态并在完成后自动更新。
            </p>
          )}
          {selectedTask.status === "failed" && selectedTask.last_error ? (
            <p className="helper-text" style={{ color: "#fca5a5", marginTop: "1rem" }}>
              最近错误：{selectedTask.last_error}
            </p>
          ) : null}
          {selectedTask.status === "failed" && selectedTask.error_code ? (
            <p className="helper-text" style={{ color: "#f87171", marginTop: "0.5rem" }}>
              错误代码：{selectedTask.error_code}
            </p>
          ) : null}
              <strong>更新时间:</strong> {new Date(selectedTask.updated_at * 1000).toLocaleString()}
            </span>
          </div>
          <div className="result-panel">
            <h3 style={{ marginTop: 0 }}>思考过程</h3>
            <pre>{selectedTask.result.thought}</pre>
            <h3>模型回复</h3>
            <pre>{selectedTask.result.reply}</pre>
            <h3>链上观察</h3>
            <pre>{selectedTask.result.observations}</pre>
          </div>
        </div>
      ) : null}

      {toast ? (
        <div className="toast">
          <h3>{toast.title}</h3>
          <span>{toast.message}</span>
        </div>
      ) : null}
    </main>
  );
}
