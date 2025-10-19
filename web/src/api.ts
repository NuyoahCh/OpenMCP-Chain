import type {
  AuthTokenResponse,
  CreateTaskRequest,
  CreateTaskResponse,
  TaskItem,
  TaskStatus,
  TaskStats,
} from "./types";

export class UnauthorizedError extends Error {
  constructor(message?: string) {
    super(message || "未授权，请重新登录");
    this.name = "UnauthorizedError";
  }
}

const DEFAULT_BASE_URL = "http://127.0.0.1:8080";
const STORAGE_KEY = "openmcp.console.apiBaseUrl";
const AUTH_STORAGE_KEY = "openmcp.console.auth";
const DEFAULT_TIMEOUT = 20_000;

export interface AuthState {
  accessToken: string;
  tokenType: string;
  expiresAt?: number;
  refreshToken?: string;
  refreshExpiresAt?: number;
  username?: string;
  scope?: string[];
}

export interface AuthCredentials {
  username: string;
  password: string;
  scope?: string[];
}

type AuthListener = (state: AuthState | null) => void;

const authListeners = new Set<AuthListener>();
let authState: AuthState | null = null;

function normalizeBaseUrl(input: string): string {
  const trimmed = input.trim();
  if (!trimmed) {
    throw new Error("API 地址不能为空");
  }
  if (!/^https?:\/\//i.test(trimmed)) {
    throw new Error("API 地址必须以 http:// 或 https:// 开头");
  }
  const url = new URL(trimmed);
  const normalizedPath = url.pathname.replace(/\/$/, "");
  return `${url.protocol}//${url.host}${normalizedPath}`;
}

const envBaseCandidate = import.meta.env.VITE_API_BASE_URL as
  | string
  | undefined;
const ENV_BASE_URL = (() => {
  if (!envBaseCandidate) {
    return DEFAULT_BASE_URL;
  }
  try {
    return normalizeBaseUrl(envBaseCandidate);
  } catch (error) {
    console.warn("无效的 VITE_API_BASE_URL，已回退到默认地址", error);
    return DEFAULT_BASE_URL;
  }
})();

let apiBaseUrl = ENV_BASE_URL;

if (typeof window !== "undefined") {
  const stored = window.localStorage.getItem(STORAGE_KEY);
  if (stored) {
    try {
      apiBaseUrl = normalizeBaseUrl(stored);
    } catch (error) {
      console.warn("检测到非法的 API Base URL，已清除", error);
      window.localStorage.removeItem(STORAGE_KEY);
      apiBaseUrl = ENV_BASE_URL;
    }
  }
}

function persistAuthState(next: AuthState | null) {
  if (typeof window === "undefined") {
    return;
  }
  if (!next) {
    window.localStorage.removeItem(AUTH_STORAGE_KEY);
    return;
  }
  window.localStorage.setItem(AUTH_STORAGE_KEY, JSON.stringify(next));
}

function notifyAuthListeners() {
  for (const listener of authListeners) {
    listener(authState);
  }
}

function setAuthState(next: AuthState | null) {
  authState = next;
  persistAuthState(next);
  notifyAuthListeners();
}

function loadAuthState(): AuthState | null {
  if (typeof window === "undefined") {
    return null;
  }
  const stored = window.localStorage.getItem(AUTH_STORAGE_KEY);
  if (!stored) {
    return null;
  }
  try {
    const parsed = JSON.parse(stored) as AuthState;
    if (parsed?.expiresAt && parsed.expiresAt <= Date.now()) {
      window.localStorage.removeItem(AUTH_STORAGE_KEY);
      return null;
    }
    return parsed;
  } catch (error) {
    console.warn("无法解析存储的认证信息，已清除", error);
    window.localStorage.removeItem(AUTH_STORAGE_KEY);
    return null;
  }
}

if (typeof window !== "undefined") {
  authState = loadAuthState();
}

interface RequestOptions extends RequestInit {
  timeout?: number;
  skipAuth?: boolean;
}

function buildUrl(path: string): string {
  const normalized = path.startsWith("/") ? path : `/${path}`;
  return `${apiBaseUrl}${normalized}`;
}

async function fetchWithTimeout(
  input: string,
  options: RequestOptions = {},
): Promise<Response> {
  const {
    timeout = DEFAULT_TIMEOUT,
    signal,
    skipAuth,
    headers,
    ...init
  } = options;
  const controller = new AbortController();
  const timer = setTimeout(() => controller.abort(), timeout);

  if (signal) {
    if (signal.aborted) {
      controller.abort();
    } else {
      signal.addEventListener("abort", () => controller.abort(), {
        once: true,
      });
    }
  }

  const mergedHeaders = new Headers(headers ?? {});
  if (!skipAuth) {
    const state = getAuthState();
    if (state && !isAuthExpired(state)) {
      mergedHeaders.set(
        "Authorization",
        `${state.tokenType || "Bearer"} ${state.accessToken}`,
      );
    }
  }

  try {
    const response = await fetch(input, {
      ...init,
      headers: mergedHeaders,
      signal: controller.signal,
    });
    if (response.status === 401) {
      clearAuth();
    }
    return response;
  } finally {
    clearTimeout(timer);
  }
}

async function parseJsonResponse<T>(response: Response): Promise<T> {
  if (response.status === 204) {
    return {} as T;
  }

  const text = await response.text();
  if (!response.ok) {
    let message = text.trim();
    if (!message && response.statusText) {
      message = response.statusText;
    }
    if (text) {
      try {
        const parsed = JSON.parse(text) as { message?: string; error?: string };
        message = parsed.message || parsed.error || message;
      } catch {
        // ignore parse error
      }
    }
    if (response.status === 401) {
      throw new UnauthorizedError(message || "未授权");
    }
    throw new Error(message || `请求失败: ${response.status}`);
  }
  if (!text) {
    return {} as T;
  }
  try {
    return JSON.parse(text) as T;
  } catch (error) {
    console.warn("响应解析失败", error);
    return {} as T;
  }
}

async function request<T>(
  path: string,
  options: RequestOptions = {},
): Promise<T> {
  const response = await fetchWithTimeout(buildUrl(path), {
    headers: { "Content-Type": "application/json" },
    ...options,
  });
  return parseJsonResponse<T>(response);
}

export function getDefaultApiBaseUrl(): string {
  return ENV_BASE_URL;
}

export function getApiBaseUrl(): string {
  return apiBaseUrl;
}

export function setApiBaseUrl(value: string | null): string {
  if (typeof window !== "undefined") {
    if (value === null) {
      window.localStorage.removeItem(STORAGE_KEY);
      apiBaseUrl = ENV_BASE_URL;
      return apiBaseUrl;
    }
    const normalized = normalizeBaseUrl(value);
    window.localStorage.setItem(STORAGE_KEY, normalized);
    apiBaseUrl = normalized;
    return apiBaseUrl;
  }
  apiBaseUrl = value ? normalizeBaseUrl(value) : ENV_BASE_URL;
  return apiBaseUrl;
}

export async function createTask(
  payload: CreateTaskRequest,
): Promise<CreateTaskResponse> {
  return request<CreateTaskResponse>("/api/v1/tasks", {
    method: "POST",
    body: JSON.stringify(payload),
  });
}

export interface TaskListQuery {
  limit?: number;
  status?: TaskStatus | TaskStatus[];
  since?: string | Date;
  until?: string | Date;
  hasResult?: boolean;
  order?: "asc" | "desc";
}

export type TaskStatsQuery = Omit<TaskListQuery, "limit" | "order">;

function toRFC3339(input: string | Date | undefined): string | undefined {
  if (!input) {
    return undefined;
  }
  if (typeof input === "string") {
    return input;
  }
  return input.toISOString();
}

function buildTaskQueryParams(
  query: TaskListQuery = {},
  options: { includeLimit?: boolean; includeOrder?: boolean } = {},
): URLSearchParams {
  const search = new URLSearchParams();
  const { includeLimit = true, includeOrder = true } = options;
  if (includeLimit && query.limit) {
    search.set("limit", String(query.limit));
  }
  if (query.status) {
    const values = Array.isArray(query.status) ? query.status : [query.status];
    if (values.length > 0) {
      search.set("status", values.join(","));
    }
  }
  const since = toRFC3339(query.since);
  if (since) {
    search.set("since", since);
  }
  const until = toRFC3339(query.until);
  if (until) {
    search.set("until", until);
  }
  if (typeof query.hasResult === "boolean") {
    search.set("has_result", String(query.hasResult));
  }
  if (includeOrder && query.order) {
    search.set("order", query.order);
  }
  return search;
}

export async function listTasks(
  query: TaskListQuery = {},
): Promise<TaskItem[]> {
  const search = buildTaskQueryParams(query);
  const suffix = search.toString();
  const url = suffix ? `/api/v1/tasks?${suffix}` : "/api/v1/tasks";
  return request<TaskItem[]>(url);
}

export async function fetchTaskStats(
  query: TaskStatsQuery = {},
): Promise<TaskStats> {
  const search = buildTaskQueryParams(query, {
    includeLimit: false,
    includeOrder: false,
  });
  const suffix = search.toString();
  const url = suffix ? `/api/v1/tasks/stats?${suffix}` : "/api/v1/tasks/stats";
  return request<TaskStats>(url);
}

export async function fetchTask(id: string): Promise<TaskItem> {
  const search = new URLSearchParams({ id });
  return request<TaskItem>(`/api/v1/tasks?${search.toString()}`);
}

export async function verifyApiConnection(): Promise<void> {
  await listTasks({ limit: 1 });
}

export function statusLabel(status: TaskStatus): string {
  switch (status) {
    case "pending":
      return "等待执行";
    case "running":
      return "执行中";
    case "succeeded":
      return "已完成";
    case "failed":
      return "失败";
    default:
      return status;
  }
}

export function statusClassName(status: TaskStatus): string {
  return `status-badge status-${status}`;
}

export function formatTimestamp(timestamp: number | null | undefined): string {
  if (!timestamp) {
    return "-";
  }
  const date = new Date(timestamp * 1000);
  if (Number.isNaN(date.getTime())) {
    return "-";
  }
  return `${date.toLocaleDateString()} ${date.toLocaleTimeString()}`;
}

export function getAuthState(): AuthState | null {
  return authState;
}

export function clearAuth() {
  setAuthState(null);
}

export function subscribeAuth(listener: AuthListener) {
  authListeners.add(listener);
  return () => {
    authListeners.delete(listener);
  };
}

export function isAuthExpired(state: AuthState | null | undefined): boolean {
  if (!state?.expiresAt) {
    return false;
  }
  return state.expiresAt <= Date.now();
}

export async function authenticate(
  credentials: AuthCredentials,
): Promise<AuthState> {
  const response = await request<AuthTokenResponse>("/api/v1/auth/token", {
    method: "POST",
    body: JSON.stringify({
      grant_type: "password",
      username: credentials.username,
      password: credentials.password,
      scope: credentials.scope,
    }),
    skipAuth: true,
  });

  const expiresAt = response.expires_in
    ? Date.now() + response.expires_in * 1000
    : undefined;
  const refreshExpiresAt = response.refresh_expires_in
    ? Date.now() + response.refresh_expires_in * 1000
    : undefined;
  const scope = Array.isArray(response.scope)
    ? response.scope
    : typeof response.scope === "string"
      ? response.scope.split(/[\s,]+/).filter(Boolean)
      : undefined;

  const next: AuthState = {
    accessToken: response.access_token,
    tokenType: response.token_type || "Bearer",
    expiresAt,
    refreshToken: response.refresh_token,
    refreshExpiresAt,
    scope,
    username: credentials.username,
  };
  setAuthState(next);
  return next;
}

export function logout() {
  clearAuth();
}
