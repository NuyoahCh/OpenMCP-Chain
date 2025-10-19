import { useCallback, useEffect, useMemo, useRef, useState } from "react";
import {
  UnauthorizedError,
  createTask,
  fetchTask,
  fetchTaskStats,
  listTasks,
  statusLabel,
  verifyApiConnection,
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
import type { CreateTaskRequest, TaskItem, TaskStats } from "./types";

const POLL_INTERVAL = 3500;
const MAX_POLL_ATTEMPTS = 40;
import type { CreateTaskRequest, TaskItem } from "./types";

const POLL_INTERVAL = 3500;
const MAX_POLL_ATTEMPTS = 40;

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

function mergeTasks(prev: TaskItem[], incoming: TaskItem): TaskItem[] {
  const next = prev.filter((task) => task.id !== incoming.id);
  next.push(incoming);
  next.sort((a, b) => b.updated_at - a.updated_at);
  return next;
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
  const [taskStats, setTaskStats] = useState<TaskStats | null>(null);
  const [connectionStatus, setConnectionStatus] = useState<
    "idle" | "success" | "error"
  >("idle");
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
          showToast({ title: "网络不可用", message });
        }
        if (options?.manual) {
          setRefreshing(false);
        }
        setInitialLoading(false);
        return false;
      }

      if (options?.manual) {
        setRefreshing(true);
      }

      try {
        const query: Parameters<typeof listTasks>[0] = {
          limit: 50,
          order: "desc",
        };
        if (statusFilter !== "all") {
          query.status = statusFilter;
        }
        const [listResult, statsResult] = await Promise.allSettled([
          listTasks(query),
          fetchTaskStats(),
        ]);
        if (listResult.status === "rejected") {
          throw listResult.reason;
        }
        if (statsResult.status === "fulfilled") {
          setTaskStats(statsResult.value);
        } else if (statsResult.status === "rejected") {
          console.warn("获取任务统计失败", statsResult.reason);
        }
        const normalized = [...listResult.value].sort(
          (a, b) => b.updated_at - a.updated_at,
        );
        const data = await listTasks(query);
        const normalized = [...data].sort(
          (a, b) => b.updated_at - a.updated_at,
        );
        const data = await listTasks(50);
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
        let message =
          error instanceof Error ? error.message : "无法同步任务列表";
        let message = error instanceof Error ? error.message : "无法同步任务列表";
        if (error instanceof UnauthorizedError) {
          message = error.message || "后端要求身份认证，请先登录";
          setRequiresAuth(true);
        }
        setFetchError(message);
        setConnectionStatus("error");
        if (!options?.silent) {
          showToast({
            title: error instanceof UnauthorizedError ? "需要登录" : "同步失败",
            message,
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
    [isOnline, showToast, statusFilter],
  );

  const startPollingTask = useCallback(
    async (taskId: string) => {
      setIsPolling(true);
      try {
        for (let attempt = 0; attempt < MAX_POLL_ATTEMPTS; attempt += 1) {
          const latest = await fetchTask(taskId);
          setTasks((prev) => mergeTasks(prev, latest));
          setActiveTask(latest);
          if (latest.status === "succeeded" || latest.status === "failed") {
            showToast({
              title: latest.status === "succeeded" ? "任务完成" : "任务失败",
              message: `${statusLabel(latest.status)} · ID ${latest.id}`,
              message: `${statusLabel(latest.status)} · ID ${latest.id}`
            });
            return;
          }
          await new Promise((resolve) => setTimeout(resolve, POLL_INTERVAL));
        }
        showToast({
          title: "轮询超时",
          message: "任务仍在执行，可稍后手动刷新",
        });
        showToast({ title: "轮询超时", message: "任务仍在执行，可稍后手动刷新" });
      } catch (error) {
        const message = error instanceof Error ? error.message : "轮询任务失败";
        showToast({ title: "轮询失败", message });
        if (error instanceof UnauthorizedError) {
          setRequiresAuth(true);
        }
      } finally {
        setIsPolling(false);
        refreshTasks({ silent: true });
      }
    },
    [refreshTasks, showToast],
  );

  const handleCreateTask = useCallback(
    async (payload: CreateTaskRequest) => {
      if (!isOnline) {
        const message = "当前处于离线状态，无法提交任务";
        showToast({ title: "离线模式", message });
        throw new Error(message);
      }
      setSubmitting(true);
      try {
        const response = await createTask(payload);
        showToast({
          title: "任务已提交",
          message: `任务 ID ${response.task_id} 已进入队列`,
          message: `任务 ID ${response.task_id} 已进入队列`
        });
        await startPollingTask(response.task_id);
      } catch (error) {
        const message = error instanceof Error ? error.message : "提交任务失败";
        if (error instanceof UnauthorizedError) {
          setRequiresAuth(true);
        }
        showToast({ title: "提交失败", message });
        throw new Error(message);
      } finally {
        setSubmitting(false);
      }
    },
    [isOnline, showToast, startPollingTask],
    [isOnline, showToast, startPollingTask]
  );

  const handleSelectTask = useCallback((task: TaskItem) => {
    setActiveTask(task);
  }, []);

  const handleExport = useCallback(() => {
    if (!tasks.length) {
      return;
    }
    const blob = new Blob([JSON.stringify(tasks, null, 2)], {
      type: "application/json;charset=utf-8",
      type: "application/json;charset=utf-8"
    });
    const url = URL.createObjectURL(blob);
    const anchor = document.createElement("a");
    anchor.href = url;
    anchor.download = `openmcp_tasks_${Date.now()}.json`;
    anchor.click();
    URL.revokeObjectURL(url);
    showToast({ title: "已导出", message: "任务列表 JSON 已下载" });
  }, [tasks, showToast]);

  const handleTestConnection = useCallback(async () => {
    setTestingConnection(true);
    try {
      await verifyApiConnection();
      setConnectionStatus("success");
      showToast({ title: "连接正常", message: `已连接 ${baseUrl}` });
      setRequiresAuth(false);
    } catch (error) {
      const message = error instanceof Error ? error.message : "连接测试失败";
      setConnectionStatus("error");
      if (error instanceof UnauthorizedError) {
        setRequiresAuth(true);
      }
      showToast({ title: "连接异常", message });
      throw new Error(message);
    } finally {
      setTestingConnection(false);
    }
  }, [baseUrl, showToast]);

  useEffect(() => {
    refreshTasks({ silent: true });
  }, [refreshTasks]);

  useEffect(() => {
    const interval = setInterval(() => {
      if (!requiresAuth) {
        refreshTasks({ silent: true });
      }
    }, 12_000);
    return () => clearInterval(interval);
  }, [refreshTasks, requiresAuth]);

  useEffect(() => {
    if (!networkChangeRef.current) {
      networkChangeRef.current = true;
      return;
    }
    if (!isOnline) {
      showToast({
        title: "离线模式",
        message: "检测到网络不可用，已暂停自动刷新",
        message: "检测到网络不可用，已暂停自动刷新"
      });
    } else {
      showToast({ title: "网络已恢复", message: "正在重新同步任务" });
      refreshTasks({ silent: true });
    }
  }, [isOnline, refreshTasks, showToast]);

  const handleLogin = useCallback(
    async (credentials: AuthCredentials) => {
      await login(credentials);
      setRequiresAuth(false);
      refreshTasks({ silent: true });
    },
    [login, refreshTasks],
    [login, refreshTasks]
  );

  const offlineHint = useMemo(() => {
    if (!lastChanged) {
      return "";
    }
    const date = new Date(lastChanged);
    return `${date.toLocaleDateString()} ${date.toLocaleTimeString()}`;
  }, [lastChanged]);

  return (
    <main>
      <section className="header">
        <div>
          <h1 className="gradient-text">OpenMCP Chain 控制台</h1>
          <p>
            图形化监控大模型智能体与链上交互的全过程，支持任务排队、实时轮询、链上观测与审计导出。
          </p>
          {requiresAuth ? (
            <div className="banner">
              <strong>后端要求身份认证。</strong>
              <span>
                请使用拥有 tasks.read / tasks.write 权限的账号登录后继续操作。
              </span>
              <span>请使用拥有 tasks.read / tasks.write 权限的账号登录后继续操作。</span>
            </div>
          ) : null}
          {!isOnline ? (
            <div className="banner banner-offline">
              <strong>当前处于离线模式。</strong>
              <span>
                {offlineHint
                  ? `最后在线时间：${offlineHint}`
                  : "恢复联网后会自动同步。"}
              </span>
            </div>
          ) : null}
          <StatusSummary
            tasks={tasks}
            stats={taskStats}
            loading={initialLoading && !taskStats}
          />
              <span>{offlineHint ? `最后在线时间：${offlineHint}` : "恢复联网后会自动同步。"}</span>
            </div>
          ) : null}
          <StatusSummary tasks={tasks} />
        </div>
        <div className="header-widgets">
          <ConnectionSettings
            baseUrl={baseUrl}
            defaultBaseUrl={defaultBaseUrl}
            onUpdate={async (value) => {
              update(value);
              showToast({ title: "地址已更新", message: value });
            }}
            onReset={() => {
              const next = reset();
              showToast({ title: "已恢复默认地址", message: next });
            }}
            onTest={handleTestConnection}
            testing={testingConnection}
            status={connectionStatus}
            lastSynced={lastSynced}
            refreshing={refreshing}
            onRefresh={() => refreshTasks({ manual: true })}
            fetchError={fetchError}
            isOnline={isOnline}
            connectionType={connectionType}
          />
          <AuthPanel
            auth={auth}
            isExpired={isExpired}
            requiresAuth={requiresAuth}
            onLogin={handleLogin}
            onLogout={() => {
              logout();
              showToast({ title: "已退出登录", message: "可在需要时重新认证" });
            }}
          />
        </div>
      </section>

      <section className="content-grid">
        <div className="stack">
          <TaskForm
            onSubmit={handleCreateTask}
            submitting={submitting}
            disabled={requiresAuth}
            disabledReason={requiresAuth ? "请登录后再提交任务" : undefined}
            loading={initialLoading}
          />
          {activeTask ? (
            <TaskDetails task={activeTask} isPolling={isPolling} />
          ) : (
            <div className="card">
              <h2 className="section-title">任务详情</h2>
              <p className="helper-text">
                选择任务后可查看模型思考、链上观测与错误信息。提交新任务后会自动跳转到最新记录。
              </p>
            </div>
          )}
        </div>
        <TaskList
          tasks={filteredTasks}
          totalCount={taskStats?.total ?? tasks.length}
          totalCount={tasks.length}
          onSelect={handleSelectTask}
          activeTaskId={activeTask?.id}
          loading={initialLoading && !tasks.length}
          error={fetchError}
          onRetry={() => refreshTasks({ manual: true })}
          statusFilter={statusFilter}
          onStatusFilterChange={setStatusFilter}
          onExport={handleExport}
        />
      </section>

      {toast ? (
        <div className="toast">
          <h3>{toast.title}</h3>
          <span>{toast.message}</span>
        </div>
      ) : null}
    </main>
  );
}
