package main

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"time"

	"OpenMCP-Chain/sdk/go/openmcp"
)

func main() {
	mux := http.NewServeMux()
	mux.HandleFunc("/v1/auth/token", func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(openmcp.Token{AccessToken: "demo-token", ExpiresAt: time.Now().Add(time.Hour)})
	})
	mux.HandleFunc("/v1/tasks", func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodPost:
			_ = json.NewEncoder(w).Encode(openmcp.TaskSummary{
				TaskID:      "task-demo",
				Status:      "PENDING",
				SubmittedAt: time.Now().UTC(),
			})
		default:
			w.WriteHeader(http.StatusMethodNotAllowed)
		}
	})
	mux.HandleFunc("/v1/tasks/task-demo", func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(openmcp.TaskDetail{
			TaskSummary: openmcp.TaskSummary{
				TaskID:      "task-demo",
				Status:      "SUCCEEDED",
				SubmittedAt: time.Now().Add(-2 * time.Minute).UTC(),
			},
			Result: map[string]any{
				"artifact_uri": "s3://bucket/output.json",
			},
		})
	})

	srv := httptest.NewServer(mux)
	defer srv.Close()

	client := openmcp.NewClient(srv.URL, srv.Client())

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	token, err := client.Authenticate(ctx, openmcp.Credentials{WorkspaceID: "demo", WorkspaceSecret: "secret"})
	if err != nil {
		panic(err)
	}
	fmt.Printf("authenticated with token %s\n", token.AccessToken)

	summary, err := client.SubmitTask(ctx, openmcp.TaskSubmission{WorkspaceID: "demo", Type: "demo"})
	if err != nil {
		panic(err)
	}
	fmt.Printf("submitted task %s (status=%s)\n", summary.TaskID, summary.Status)

	detail, err := client.GetTask(ctx, summary.TaskID)
	if err != nil {
		panic(err)
	}
	fmt.Printf("retrieved task %s result=%v\n", detail.TaskID, detail.Result)
}
