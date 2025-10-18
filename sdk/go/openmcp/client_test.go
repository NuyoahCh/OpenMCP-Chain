package openmcp

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestAuthenticateStoresToken(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/auth/token" {
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
		if r.Method != http.MethodPost {
			t.Fatalf("unexpected method: %s", r.Method)
		}
		if err := json.NewDecoder(r.Body).Decode(&Credentials{}); err != nil {
			t.Fatalf("unexpected body: %v", err)
		}
		_ = json.NewEncoder(w).Encode(Token{
			AccessToken: "abc123",
			ExpiresAt:   time.Now().UTC(),
		})
	}))
	defer srv.Close()

	client := NewClient(srv.URL, srv.Client())

	_, err := client.Authenticate(context.Background(), Credentials{
		WorkspaceID:     "ws-1",
		WorkspaceSecret: "secret",
	})
	if err != nil {
		t.Fatalf("authenticate: %v", err)
	}

	if got := client.AccessToken(); got != "abc123" {
		t.Fatalf("expected token abc123, got %q", got)
	}
}

func TestSubmitTaskRequiresToken(t *testing.T) {
	taskSubmitted := false
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/v1/auth/token":
			_ = json.NewEncoder(w).Encode(Token{AccessToken: "token"})
		case "/v1/tasks":
			if r.Header.Get("Authorization") != "Bearer token" {
				t.Fatalf("expected bearer token, got %q", r.Header.Get("Authorization"))
			}
			taskSubmitted = true
			_ = json.NewEncoder(w).Encode(TaskSummary{TaskID: "task-1"})
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer srv.Close()

	client := NewClient(srv.URL, srv.Client())

	if _, err := client.Authenticate(context.Background(), Credentials{}); err != nil {
		t.Fatalf("authenticate: %v", err)
	}

	if _, err := client.SubmitTask(context.Background(), TaskSubmission{}); err != nil {
		t.Fatalf("submit task: %v", err)
	}

	if !taskSubmitted {
		t.Fatal("task was not submitted")
	}
}

func TestGetTaskError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/v1/tasks/task-404" {
			w.WriteHeader(http.StatusNotFound)
			_ = json.NewEncoder(w).Encode(struct {
				Error APIError `json:"error"`
			}{Error: APIError{Code: "TASK_NOT_FOUND", Message: "missing"}})
			return
		}
		if r.URL.Path == "/v1/auth/token" {
			_ = json.NewEncoder(w).Encode(Token{AccessToken: "token"})
			return
		}
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer srv.Close()

	client := NewClient(srv.URL, srv.Client())
	if _, err := client.Authenticate(context.Background(), Credentials{}); err != nil {
		t.Fatalf("authenticate: %v", err)
	}

	_, err := client.GetTask(context.Background(), "task-404")
	if err == nil {
		t.Fatal("expected error")
	}
	apiErr, ok := err.(*APIError)
	if !ok {
		t.Fatalf("expected APIError, got %T", err)
	}
	if apiErr.Code != "TASK_NOT_FOUND" {
		t.Fatalf("unexpected error code: %s", apiErr.Code)
	}
}
