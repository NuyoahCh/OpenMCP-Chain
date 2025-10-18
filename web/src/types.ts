export type TaskStatus = "pending" | "running" | "succeeded" | "failed";

export interface ExecutionResult {
  thought: string;
  reply: string;
  chain_id: string;
  block_number: string;
  observations: string;
}

export interface TaskItem {
  id: string;
  goal: string;
  chain_action: string;
  address: string;
  status: TaskStatus;
  attempts: number;
  max_retries: number;
  last_error?: string;
  error_code?: string;
  result?: ExecutionResult | null;
  created_at: number;
  updated_at: number;
}

export interface CreateTaskRequest {
  goal: string;
  chain_action?: string;
  address?: string;
}

export interface CreateTaskResponse {
  task_id: string;
  status: TaskStatus;
  attempts: number;
  max_retries: number;
}
