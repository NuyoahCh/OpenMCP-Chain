# API Overview

The OpenMCP-Chain API surface provides programmatic access to agent lifecycle
management, audit trails, and configuration endpoints. Interfaces will be
exposed over both REST and gRPC, with shared protobuf definitions compiled into
language-specific SDKs.

## Planned REST Resources

| Resource | Description |
| --- | --- |
| `POST /v1/tasks` | Submit a new agent task and receive a tracking identifier. |
| `GET /v1/tasks/{id}` | Retrieve task status, intermediate artifacts, and blockchain receipts. |
| `POST /v1/agents` | Provision a new agent profile with policy configuration. |
| `GET /v1/agents/{id}` | Inspect agent metadata, health, and resource usage. |
| `GET /v1/audit/{id}` | Fetch signed inference outputs and provenance data. |

## Authentication

* Initial implementation will rely on API keys scoped to project namespaces.
* Future milestones introduce OAuth2/OpenID Connect and hardware-backed signing
  for high-assurance operations.

## Versioning Strategy

* REST endpoints follow semantic versioning via URL prefix (`/v1`).
* gRPC services embed version information within protobuf package names.
* Backward-compatible changes accumulate until a new major version is required.

## Telemetry

* Each API response includes correlation identifiers for tracing.
* Rate limiting headers communicate remaining quota and reset windows.
* Error payloads follow a consistent schema with machine-readable codes.

Detailed protobuf definitions and OpenAPI specifications will be added as the
service implementation matures.
