package api

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"OpenMCP-Chain/internal/task"
)

func TestHandleTaskDetailSuccess(t *testing.T) {
	store := task.NewMemoryStore()
	svc := task.NewService(store, nil, 3)
	server := NewServer(":0", svc)

	sample := &task.Task{
		ID:         "task-success",
		Goal:       "demo",
		Status:     task.StatusSucceeded,
		Attempts:   1,
		MaxRetries: 3,
		CreatedAt:  1700000000,
		UpdatedAt:  1700000001,
		Result: &task.ExecutionResult{
			Reply: "ok",
		},
	}
	if err := store.Create(context.Background(), sample); err != nil {
		t.Fatalf("create sample task: %v", err)
	}

	req := httptest.NewRequest(http.MethodGet, "/api/v1/tasks/task-success", nil)
	rec := httptest.NewRecorder()

	server.handleTaskDetail(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("unexpected status code: got %d want %d", rec.Code, http.StatusOK)
	}

	var got task.Task
	if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if got.ID != sample.ID {
		t.Fatalf("unexpected task id: got %q want %q", got.ID, sample.ID)
	}
	if got.Result == nil || got.Result.Reply != "ok" {
		t.Fatalf("unexpected task result: %+v", got.Result)
	}
}

func TestHandleTaskDetailErrors(t *testing.T) {
	server := NewServer(":0", task.NewService(task.NewMemoryStore(), nil, 3))

	t.Run("invalid method", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPost, "/api/v1/tasks/task-1", nil)
		rec := httptest.NewRecorder()

		server.handleTaskDetail(rec, req)

		if rec.Code != http.StatusMethodNotAllowed {
			t.Fatalf("expected status %d, got %d", http.StatusMethodNotAllowed, rec.Code)
		}
	})

	t.Run("missing id", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/api/v1/tasks/", nil)
		rec := httptest.NewRecorder()

		server.handleTaskDetail(rec, req)

		if rec.Code != http.StatusBadRequest {
			t.Fatalf("expected status %d, got %d", http.StatusBadRequest, rec.Code)
		}
	})

	t.Run("not found", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/api/v1/tasks/missing", nil)
		rec := httptest.NewRecorder()

		server.handleTaskDetail(rec, req)

		if rec.Code != http.StatusNotFound {
			t.Fatalf("expected status %d, got %d", http.StatusNotFound, rec.Code)
		}
	})
}
