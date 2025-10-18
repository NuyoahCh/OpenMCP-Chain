import type { CreateTaskRequest, CreateTaskResponse, TaskItem } from "./types";

const DEFAULT_BASE_URL = "http://127.0.0.1:8080";
const STORAGE_KEY = "openmcp.console.apiBaseUrl";
const DEFAULT_TIMEOUT = 20_000;

function normalizeBaseUrl(input: string): string {
  const trimmed = input.trim();
  if (!/^https?:\/\//i.test(trimmed)) {
    throw new Error("API 地址必须以 http:// 或 https:// 开头");
  }
  try {
    const url = new URL(trimmed);
    const normalizedPath = url.pathname.replace(/\/$/, "");
    return `${url.protocol}//${url.host}${normalizedPath}`;
  } catch (error) {
    throw new Error("API 地址格式不正确");
  }
}

const envBaseCandidate = import.meta.env.VITE_API_BASE_URL as string | undefined;
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

interface RequestOptions extends RequestInit {
  timeout?: number;
}

function buildUrl(path: string): string {
  const normalized = path.startsWith("/") ? path : `/${path}`;
  return `${apiBaseUrl}${normalized}`;
}

async function fetchWithTimeout(input: string, options: RequestOptions = {}): Promise<Response> {
  const { timeout = DEFAULT_TIMEOUT, signal, ...init } = options;
  const controller = new AbortController();
  const timer = setTimeout(() => controller.abort(), timeout);
  if (signal) {
    if (signal.aborted) {
      controller.abort();
    } else {
      signal.addEventListener("abort", () => controller.abort(), { once: true });
    }
  }
  try {
    return await fetch(input, { ...init, signal: controller.signal });
  } finally {
    clearTimeout(timer);
  }
}

async function parseJsonResponse<T>(response: Response): Promise<T> {
  const text = await response.text();
  if (!response.ok) {
    let message = text.trim();
    if (!message) {
      message = `请求失败: ${response.status}`;
    } else {
      try {
        const parsed = JSON.parse(text);
        message = parsed.message || parsed.error || message;
      } catch (error) {
        // ignore json parse error
      }
    }
    throw new Error(message);
  }
  if (!text) {
    return {} as T;
  }
  try {
    return JSON.parse(text) as T;
  } catch (error) {
    throw new Error("响应解析失败");
  }
}

export function getDefaultApiBaseUrl(): string {
  return ENV_BASE_URL;
}

export function getApiBaseUrl(): string {
  return apiBaseUrl;
}

export function setApiBaseUrl(value?: string | null): string {
  if (!value || !value.trim()) {
    apiBaseUrl = ENV_BASE_URL;
    if (typeof window !== "undefined") {
      window.localStorage.removeItem(STORAGE_KEY);
    }
    return apiBaseUrl;
  }
  const normalized = normalizeBaseUrl(value);
  apiBaseUrl = normalized;
  if (typeof window !== "undefined") {
    window.localStorage.setItem(STORAGE_KEY, normalized);
  }
  return apiBaseUrl;
}

export async function createTask(payload: CreateTaskRequest): Promise<CreateTaskResponse> {
  const response = await fetchWithTimeout(buildUrl("/api/v1/tasks"), {
    method: "POST",
    headers: {
      "Content-Type": "application/json"
    },
    body: JSON.stringify(payload)
  });
  return parseJsonResponse<CreateTaskResponse>(response);
}

export async function fetchTask(id: string): Promise<TaskItem> {
  const response = await fetchWithTimeout(buildUrl(`/api/v1/tasks?id=${encodeURIComponent(id)}`));
  return parseJsonResponse<TaskItem>(response);
}

export async function listTasks(limit = 20): Promise<TaskItem[]> {
  const response = await fetchWithTimeout(buildUrl(`/api/v1/tasks?limit=${limit}`));
  return parseJsonResponse<TaskItem[]>(response);
}

export async function verifyApiConnection(): Promise<void> {
  await listTasks(1);
}

export function formatTimestamp(timestamp: number): string {
  if (!timestamp) {
    return "-";
  }
  const date = new Date(timestamp * 1000);
  return date.toLocaleString();
}

export function statusLabel(status: string): string {
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

export function statusClassName(status: string): string {
  switch (status) {
    case "pending":
      return "status-badge status-pending";
    case "running":
      return "status-badge status-running";
    case "succeeded":
      return "status-badge status-succeeded";
    case "failed":
      return "status-badge status-failed";
    default:
      return "status-badge";
  }
}
