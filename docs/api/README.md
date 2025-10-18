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

OpenMCP Chain offers pluggable authentication strategies. The default
configuration issues JWT access tokens using the built-in identity store, while
production deployments can switch to OAuth2/OIDC by changing the server
configuration (`auth.mode`).

Regardless of the mode, API calls must include an `Authorization: Bearer` header
containing a valid access token. Tokens carry the caller's roles and permissions
so the gateway can enforce fine-grained authorization.

### REST token exchange

```http
POST /api/v1/auth/token
Content-Type: application/json

{
  "grant_type": "password",
  "username": "operator",
  "password": "<secret>"
}
```

Response:

```json
{
  "access_token": "eyJhbGciOiJ...",
  "expires_in": 3600,
  "refresh_token": "eyJhbGciOiJ...",
  "refresh_expires_in": 86400,
  "token_type": "Bearer"
}
```

When `auth.mode` is set to `oauth`, the server proxies the request to the
external OAuth2 token endpoint using the configured client credentials. The
structure of the response mirrors the upstream provider.

### Roles & permissions

- **Users** authenticate with username/password (JWT mode) or external identity
  (OAuth mode). Account metadata is stored in `auth_users`.
- **Roles** (`auth_roles`) are collections of permissions. Users gain all
  permissions attached to their roles via the `auth_user_roles` join table.
- **Permissions** (`auth_permissions`) describe allowed operations such as
  `tasks.read` and `tasks.write`. Permissions can be assigned directly to users
  through `auth_user_permissions` for emergency overrides.

The REST API currently uses two permissions:

| Permission   | Description                           |
|--------------|---------------------------------------|
| `tasks.read` | Required for `GET /api/v1/tasks`       |
| `tasks.write`| Required for `POST /api/v1/tasks`      |

Administrators can seed initial accounts via the configuration file or by
running SQL migrations to insert default users and roles.

## Task Lifecycle

1. **Submit** – Create a new task with an execution payload.
2. **Pending** – The task is accepted and queued.
3. **Running** – The task is being processed.
4. **Succeeded/Failed** – Terminal states.

## REST Endpoints

### Submit Task

```http
POST /api/v1/tasks
Authorization: Bearer <token>
Content-Type: application/json

{
  "goal": "Summarise the latest governance proposal",
  "chain_action": "eth_call",
  "address": "0x1234..."
}
```

Response:

```json
{
  "task_id": "task-1f7f",
  "status": "pending",
  "attempts": 0,
  "max_retries": 3
}
```

### Get Task

```http
GET /api/v1/tasks?id={task_id}
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
