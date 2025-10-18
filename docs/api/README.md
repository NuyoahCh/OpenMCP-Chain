# OpenMCP Chain API Specification

This document describes the canonical APIs for interacting with the OpenMCP Chain
platform. Two transport options are provided: REST over HTTP and gRPC. Both share
the same conceptual resources and authentication model so that SDKs and custom
clients can be implemented consistently.

## Core Concepts

- **Workspace** – An isolated namespace for tenant data and workloads.
- **Task** – A unit of work submitted to the OpenMCP Chain execution service.
- **Result** – The final payload returned once a task has been completed.
- **Event Stream** – Optional, ordered updates that describe task lifecycle
  transitions.

All APIs use ISO-8601 timestamps in UTC and UUID identifiers. Unless noted
otherwise, request and response bodies are encoded as JSON for the REST API
and protobuf messages for gRPC.

## Authentication

OpenMCP Chain uses token-based authentication. Clients must exchange their
workspace credentials for a short-lived bearer token which is then attached to
subsequent requests.

### REST

```http
POST /v1/auth/token
Content-Type: application/json

{
  "workspace_id": "ws-1234",
  "workspace_secret": "<secret>"
}
```

Successful responses return an access token and its expiration timestamp:

```json
{
  "access_token": "eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9...",
  "expires_at": "2024-01-01T00:00:00Z"
}
```

Tokens must be supplied in the `Authorization: Bearer <token>` header.

### gRPC

The gRPC service exposes an `Authenticate` method on the `AuthService`. Clients
send an `AuthRequest` containing the workspace credentials and receive an
`AuthToken` message with the `token` and `expires_at` fields. Tokens are carried
in the `authorization` metadata key for subsequent gRPC calls.

```proto
service AuthService {
  rpc Authenticate(AuthRequest) returns (AuthToken);
}

message AuthRequest {
  string workspace_id = 1;
  string workspace_secret = 2;
}

message AuthToken {
  string token = 1;
  google.protobuf.Timestamp expires_at = 2;
}
```

## Task Lifecycle

1. **Submit** – Create a new task with an execution payload.
2. **Pending** – The task is accepted and queued.
3. **Running** – The task is being processed.
4. **Succeeded/Failed** – Terminal states.

## REST Endpoints

### Submit Task

```http
POST /v1/tasks
Authorization: Bearer <token>
Content-Type: application/json

{
  "workspace_id": "ws-1234",
  "type": "data_pipeline",
  "payload": {
    "input_uri": "s3://bucket/path/input.json"
  },
  "metadata": {
    "priority": "high"
  }
}
```

Response:

```json
{
  "task_id": "task-1f7f",
  "status": "PENDING",
  "submitted_at": "2024-01-01T00:00:00Z"
}
```

### Get Task

```http
GET /v1/tasks/{task_id}
Authorization: Bearer <token>
```

Response:

```json
{
  "task_id": "task-1f7f",
  "status": "SUCCEEDED",
  "submitted_at": "2024-01-01T00:00:00Z",
  "updated_at": "2024-01-01T00:05:12Z",
  "result": {
    "artifact_uri": "s3://bucket/path/output.json",
    "checksum": "sha256:..."
  },
  "error": null
}
```

### Stream Task Events (optional)

```http
GET /v1/tasks/{task_id}/events
Authorization: Bearer <token>
Accept: text/event-stream
```

This endpoint returns Server-Sent Events (SSE) messages for each lifecycle
transition.

## gRPC Services

```proto
service TaskService {
  rpc SubmitTask(SubmitTaskRequest) returns (TaskSummary);
  rpc GetTask(GetTaskRequest) returns (TaskDetail);
  rpc StreamTaskEvents(StreamTaskEventsRequest) returns (stream TaskEvent);
}

message SubmitTaskRequest {
  string workspace_id = 1;
  string type = 2;
  google.protobuf.Struct payload = 3;
  map<string, string> metadata = 4;
}

message TaskSummary {
  string task_id = 1;
  TaskStatus status = 2;
  google.protobuf.Timestamp submitted_at = 3;
}

message GetTaskRequest {
  string task_id = 1;
}

message TaskDetail {
  string task_id = 1;
  TaskStatus status = 2;
  google.protobuf.Timestamp submitted_at = 3;
  google.protobuf.Timestamp updated_at = 4;
  google.protobuf.Struct result = 5;
  string error = 6;
}

message StreamTaskEventsRequest {
  string task_id = 1;
}

message TaskEvent {
  string task_id = 1;
  TaskStatus status = 2;
  google.protobuf.Timestamp occurred_at = 3;
  google.protobuf.Struct context = 4;
}

enum TaskStatus {
  TASK_STATUS_UNSPECIFIED = 0;
  TASK_STATUS_PENDING = 1;
  TASK_STATUS_RUNNING = 2;
  TASK_STATUS_SUCCEEDED = 3;
  TASK_STATUS_FAILED = 4;
}
```

## Error Handling

Errors use standard HTTP status codes. Responses include a machine-readable code
and human-readable message:

```json
{
  "error": {
    "code": "TASK_NOT_FOUND",
    "message": "Task task-1f7f does not exist"
  }
}
```

gRPC methods use canonical status codes. The `status.details` field carries the
same machine-readable error payload serialized as protobuf.

## Rate Limiting

Clients should expect `429 Too Many Requests` responses if they exceed their
workspace quota. A `Retry-After` header indicates when they may retry.

## Versioning

APIs are versioned via the URI prefix (e.g., `/v1/`). Backwards-incompatible
changes result in a new version and corresponding protobuf package namespace
(`openmcp.chain.v2`).

## Security Considerations

- Use HTTPS for all REST calls.
- Rotate workspace secrets regularly and store them securely.
- Tokens are short-lived; refresh them when receiving `401 Unauthorized`.

## Changelog

- **v1.0** – Initial release of REST and gRPC APIs with task submission and
  result retrieval capabilities.
