import type { CreateTaskRequest, CreateTaskResponse, TaskItem } from "./types";

const DEFAULT_BASE_URL = "http://127.0.0.1:8080";

const API_BASE_URL = (import.meta.env.VITE_API_BASE_URL as string | undefined)?.replace(/\/$/, "") || DEFAULT_BASE_URL;

async function handleResponse<T>(response: Response): Promise<T> {
  if (!response.ok) {
    const text = await response.text();
    throw new Error(text || `请求失败: ${response.status}`);
  }
  return (await response.json()) as T;
}

export async function createTask(payload: CreateTaskRequest): Promise<CreateTaskResponse> {
  const response = await fetch(`${API_BASE_URL}/api/v1/tasks`, {
    method: "POST",
    headers: {
      "Content-Type": "application/json"
    },
    body: JSON.stringify(payload)
  });
  return handleResponse<CreateTaskResponse>(response);
}

export async function fetchTask(id: string): Promise<TaskItem> {
  const response = await fetch(`${API_BASE_URL}/api/v1/tasks?id=${encodeURIComponent(id)}`);
  return handleResponse<TaskItem>(response);
}

export async function listTasks(limit = 20): Promise<TaskItem[]> {
  const response = await fetch(`${API_BASE_URL}/api/v1/tasks?limit=${limit}`);
  return handleResponse<TaskItem[]>(response);
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
