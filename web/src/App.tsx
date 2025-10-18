import { useCallback, useEffect, useMemo, useRef, useState } from "react";
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
import TaskDetails from "./components/TaskDetails";
import StatusSummary from "./components/StatusSummary";
import ConnectionSettings from "./components/ConnectionSettings";
import AuthPanel from "./components/AuthPanel";
import { useAuth, type AuthCredentials } from "./hooks/useAuth";
import { useApiBaseUrl } from "./hooks/useApiBaseUrl";
import { useNetworkStatus } from "./hooks/useNetworkStatus";
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
      await login(credentials);
      setRequiresAuth(false);
      await refreshTasks({ manual: true, silent: true });
    },
    [isOnline, login, refreshTasks, showToast]
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
      </div>

      <StatusSummary tasks={tasks} />

      <TaskList
        tasks={filteredTasks}
        activeTaskId={activeTask?.id}
        loading={initialLoading}
        error={fetchError}
        onRetry={handleManualRefresh}
        onSelect={(task) => {
          setActiveTask(task);
          pollTask(task.id);
        }}
        statusFilter={statusFilter}
        onStatusFilterChange={setStatusFilter}
        onExport={handleExport}
      />

      {activeTask ? <TaskDetails task={activeTask} isPolling={isPolling} /> : null}

      {toast ? (
        <div className="toast">
          <h3>{toast.title}</h3>
          <span>{toast.message}</span>
        </div>
      ) : null}
    </main>
  );
}
