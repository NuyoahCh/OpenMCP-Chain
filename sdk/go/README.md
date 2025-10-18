# OpenMCP Chain Go SDK

The Go SDK streamlines communication with the OpenMCP Chain REST API. It manages
authentication, task submission, and result retrieval while exposing a small and
composable API surface.

## Installation

The SDK is distributed as part of the monorepo. From another Go module you can
use a replace directive or, after publishing, import it via:

```
go get github.com/openmcp-chain/openmcp-chain/sdk/go/openmcp
```

## Usage

```go
package main

import (
    "context"
    "log"
    "time"

    "OpenMCP-Chain/sdk/go/openmcp"
)

func main() {
    client := openmcp.NewClient("https://api.openmcp-chain.io", nil)

    ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
    defer cancel()

    token, err := client.Authenticate(ctx, openmcp.Credentials{
        WorkspaceID:     "ws-1234",
        WorkspaceSecret: "top-secret",
    })
    if err != nil {
        log.Fatal(err)
    }

    task, err := client.SubmitTask(ctx, openmcp.TaskSubmission{
        WorkspaceID: "ws-1234",
        Type:        "data_pipeline",
        Payload: map[string]any{
            "input_uri": "s3://bucket/input.json",
        },
    })
    if err != nil {
        log.Fatal(err)
    }

    detail, err := client.GetTask(ctx, task.TaskID)
    if err != nil {
        log.Fatal(err)
    }

    log.Printf("task status: %s", detail.Status)
}
```

Additional examples are available in [`examples`](examples/main.go).

## Testing

From the repository root run:

```
go test ./sdk/go/...
```

The tests use an in-memory HTTP server and do not require external services.
