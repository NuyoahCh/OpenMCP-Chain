import { useCallback, useEffect, useMemo, useRef, useState } from "react";
import { useCallback, useEffect, useState } from "react";
import {
  UnauthorizedError,
  createTask,
  fetchTask,
  listTasks,
  statusLabel,
  verifyApiConnection
} from "./api";
import TaskForm from "./components/TaskForm";
import TaskList, { type TaskStatusFilter } from "./components/TaskList";
import { createTask, fetchTask, listTasks, statusLabel, verifyApiConnection } from "./api";
import TaskForm from "./components/TaskForm";
import TaskList from "./components/TaskList";
import TaskDetails from "./components/TaskDetails";
import StatusSummary from "./components/StatusSummary";
import ConnectionSettings from "./components/ConnectionSettings";
import AuthPanel from "./components/AuthPanel";
import { useAuth, type AuthCredentials } from "./hooks/useAuth";
import { useApiBaseUrl } from "./hooks/useApiBaseUrl";
import { useNetworkStatus } from "./hooks/useNetworkStatus";
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
  const [requiresAuth, setRequiresAuth] = useState(false);
  const [statusFilter, setStatusFilter] = useState<TaskStatusFilter>("all");
  const { baseUrl, defaultBaseUrl, update, reset } = useApiBaseUrl();
  const { toast, showToast } = useToast();
  const { auth, login, logout, isExpired } = useAuth();
  const { isOnline, lastChanged, connectionType } = useNetworkStatus();
  const networkChangeRef = useRef(false);

  const filteredTasks = useMemo(() => {
    if (statusFilter === "all") {
      return tasks;
    }
    return tasks.filter((task) => task.status === statusFilter);
  }, [statusFilter, tasks]);

  useEffect(() => {
    setActiveTask((current) => {
      if (!filteredTasks.length) {
        return null;
      }
      if (current && filteredTasks.some((task) => task.id === current.id)) {
        return current;
      }
      return filteredTasks[0];
    });
  }, [filteredTasks]);

  const refreshTasks = useCallback(
    async (options?: { manual?: boolean; silent?: boolean }) => {
      if (!isOnline) {
        const message = "当前设备处于离线状态，无法刷新数据";
        setFetchError(message);
        setConnectionStatus("error");
        if (!options?.silent) {
          showToast({
            title: "网络不可用",
            message
          });
        }
        setInitialLoading(false);
        if (options?.manual) {
          setRefreshing(false);
        }
        return false;
      }
  const { baseUrl, defaultBaseUrl, update, reset } = useApiBaseUrl();
  const { toast, showToast } = useToast();
  const { auth, login, logout, isExpired } = useAuth();
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
        setRequiresAuth(false);
        setActiveTask((current) => {
          if (!current) {
            return normalized[0] ?? null;
          }
          const updated = normalized.find((item) => item.id === current.id);
          return updated ?? normalized[0] ?? current;
        });
        return true;
      } catch (error) {
        if (error instanceof UnauthorizedError) {
          const message = error.message || "后端要求身份认证，请先登录";
          setFetchError(message);
          setRequiresAuth(true);
          setConnectionStatus("error");
          if (!options?.silent) {
            showToast({
              title: "需要登录",
              message
            });
          }
        } else {
          const message = error instanceof Error ? error.message : "无法同步任务列表";
          setFetchError(message);
          setConnectionStatus("error");
          if (!options?.silent) {
            showToast({
              title: "同步失败",
              message
            });
          }
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
    [isOnline, showToast]
  );

  useEffect(() => {
    if (!networkChangeRef.current) {
      networkChangeRef.current = true;
      return;
    }
    if (!isOnline) {
      showToast({
        title: "离线模式",
        message: "检测到网络不可用，功能将暂时受限"
      });
    } else {
      showToast({
        title: "网络已恢复",
        message: "自动重新同步任务列表"
      });
      refreshTasks({ silent: true });
    }
  }, [isOnline, refreshTasks, showToast]);

  useEffect(() => {
    [showToast]
  );

  useEffect(() => {
    refreshTasks({ silent: true });
    const interval = setInterval(() => {
      if (requiresAuth && (!auth || isExpired)) {
        return;
      }
      if (!isOnline) {
        return;
      }
      refreshTasks({ silent: true });
    }, 15000);
    return () => clearInterval(interval);
  }, [auth, isExpired, isOnline, refreshTasks, requiresAuth]);
      refreshTasks({ silent: true });
    }, 15000);
    return () => clearInterval(interval);
  }, [auth, isExpired, refreshTasks, requiresAuth]);
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
        if (error instanceof UnauthorizedError) {
          setRequiresAuth(true);
          setConnectionStatus("error");
          showToast({
            title: "需要登录",
            message: error.message || "会话失效，请重新登录"
          });
        } else {
          showToast({
            title: "任务轮询失败",
            message: error instanceof Error ? error.message : "未知错误"
          });
        }
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
      try {
        if (!isOnline) {
          showToast({
            title: "网络不可用",
            message: "当前处于离线状态，无法提交任务"
          });
          return;
        }
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
        pollTask(response.task_id);
      } catch (error) {
        if (error instanceof UnauthorizedError) {
          setRequiresAuth(true);
          setConnectionStatus("error");
          showToast({
            title: "需要登录",
            message: error.message || "会话失效，请重新登录"
          });
        } else {
          showToast({
            title: "提交失败",
            message: error instanceof Error ? error.message : "未知错误"
          });
        }
      } finally {
        setSubmitting(false);
      }
    },
    [isOnline, pollTask, refreshTasks, showToast]
  );

  const handleManualRefresh = useCallback(() => {
    if (!isOnline) {
      showToast({
        title: "网络不可用",
        message: "恢复网络后再尝试刷新"
      });
      return;
    }
    refreshTasks({ manual: true });
  }, [isOnline, refreshTasks, showToast]);

  const handleUpdateBaseUrl = useCallback(
    async (value: string) => {
      if (!isOnline) {
        throw new Error("当前离线，请连接网络后重试");
      }
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
    [isOnline, refreshTasks, showToast, update]
  );

  const handleResetBaseUrl = useCallback(async () => {
    if (!isOnline) {
      throw new Error("当前离线，请连接网络后重试");
    }
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
  }, [isOnline, refreshTasks, reset, showToast]);

  const handleTestConnection = useCallback(async () => {
    if (!isOnline) {
      showToast({
        title: "网络不可用",
        message: "请检查网络连接后再试"
      });
      return;
    }
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
      if (error instanceof UnauthorizedError) {
        setRequiresAuth(true);
        showToast({
          title: "需要登录",
          message
        });
      } else {
        showToast({
          title: "连接失败",
          message
        });
      }
    } finally {
      setTestingConnection(false);
    }
  }, [baseUrl, isOnline, refreshTasks, showToast]);

  const handleLogin = useCallback(
    async (credentials: AuthCredentials) => {
      if (!isOnline) {
        showToast({
          title: "网络不可用",
          message: "当前离线，无法登录"
        });
        throw new Error("当前处于离线状态");
      }
      showToast({
        title: "连接失败",
        message
      });
    } finally {
      setTestingConnection(false);
    }
  }, [baseUrl, refreshTasks, showToast]);

  const handleLogin = useCallback(
    async (credentials: AuthCredentials) => {
      await login(credentials);
      setRequiresAuth(false);
      await refreshTasks({ manual: true, silent: true });
    },
    [isOnline, login, refreshTasks, showToast]
    [login, refreshTasks]
  );

  const isSessionReady = Boolean(auth) && !isExpired;
  const showAuthWarning = requiresAuth && (!auth || isExpired);
  const formDisabled = (requiresAuth && !isSessionReady) || !isOnline;
  const offlineReason = !isOnline
    ? `当前设备已离线${
        lastChanged ? `（${new Date(lastChanged).toLocaleTimeString()} 检测）` : ""
      }`
    : undefined;
  const formDisabledReason = offlineReason
    ? offlineReason
    : !auth
        ? "后端要求身份认证，请先登录"
        : isExpired
          ? "登录已过期，请重新获取访问令牌"
          : undefined;

  const handleExport = useCallback(() => {
    const targetTasks = statusFilter === "all" ? tasks : tasks.filter((task) => task.status === statusFilter);
    if (!targetTasks.length || typeof window === "undefined") {
      showToast({
        title: "暂无数据",
        message: "没有可导出的任务记录"
      });
      return;
    }
    const blob = new Blob([JSON.stringify(targetTasks, null, 2)], {
      type: "application/json;charset=utf-8"
    });
    const url = URL.createObjectURL(blob);
    const anchor = document.createElement("a");
    const timestamp = new Date().toISOString().replace(/[:.]/g, "-");
    anchor.href = url;
    anchor.download = `openmcp-tasks-${timestamp}.json`;
    anchor.click();
    URL.revokeObjectURL(url);
    showToast({
      title: "已导出任务", 
      message: `共 ${targetTasks.length} 条记录`
    });
  }, [showToast, statusFilter, tasks]);

  const lastNetworkChange = useMemo(() => {
    if (!lastChanged) {
      return null;
    }
    const date = new Date(lastChanged);
    return `${date.toLocaleDateString()} ${date.toLocaleTimeString()}`;
  }, [lastChanged]);

  return (
    <main>
      {!isOnline ? (
        <div className="banner banner-offline">
          <strong>离线模式：</strong> 检测到网络不可用，任务轮询与提交将暂停。
          {lastNetworkChange ? <span>最近检测：{lastNetworkChange}</span> : null}
        </div>
      ) : null}
  const formDisabled = requiresAuth && !isSessionReady;
  const formDisabledReason = !auth
    ? "后端要求身份认证，请先登录"
    : isExpired
      ? "登录已过期，请重新获取访问令牌"
      : undefined;
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
        <div className="header-widgets">
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
            isOnline={isOnline}
            connectionType={connectionType}
          />
          <AuthPanel
            auth={auth}
            isExpired={isExpired}
            requiresAuth={requiresAuth}
            onLogin={handleLogin}
            onLogout={logout}
          />
        </div>
      </header>

      <div className="glow-border" style={{ marginBottom: "2.5rem" }}>
        <TaskForm
          onSubmit={handleSubmit}
          submitting={submitting}
          disabled={formDisabled}
          disabledReason={showAuthWarning ? formDisabledReason : undefined}
        />
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
        tasks={filteredTasks}
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
        statusFilter={statusFilter}
        onStatusFilterChange={setStatusFilter}
        onExport={handleExport}
      />

      {activeTask ? <TaskDetails task={activeTask} isPolling={isPolling} /> : null}
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
