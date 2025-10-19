export type TaskStatus = "pending" | "running" | "succeeded" | "failed";

export interface ExecutionResult {
  thought?: string | null;
  reply?: string | null;
  chain_id?: string | null;
  block_number?: string | number | null;
  observations?: string | null;
}

export interface TaskItem {
  id: string;
  goal: string;
  chain_action?: string | null;
  address?: string | null;
  status: TaskStatus;
  attempts: number;
  max_retries: number;
  last_error?: string | null;
  error_code?: string | null;
  result?: ExecutionResult | null;
  metadata?: Record<string, unknown> | null;
  created_at: number;
  updated_at: number;
}

export interface CreateTaskRequest {
  goal: string;
  chain_action?: string;
  address?: string;
  metadata?: Record<string, unknown>;
}

export interface CreateTaskResponse {
  task_id: string;
  status: TaskStatus;
  attempts: number;
  max_retries: number;
  goal?: string;
  chain_action?: string | null;
  address?: string | null;
  metadata?: Record<string, unknown> | null;
  created_at: number;
  updated_at: number;
}

export interface AuthTokenResponse {
  access_token: string;
  expires_in?: number;
  refresh_token?: string;
  refresh_expires_in?: number;
  token_type?: string;
  scope?: string | string[];
}

export interface TaskStats {
  total: number;
  pending: number;
  running: number;
  succeeded: number;
  failed: number;
  oldest_updated_at?: number | null;
  newest_updated_at?: number | null;
}
