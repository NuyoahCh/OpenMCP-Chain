package openai

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"OpenMCP-Chain/internal/llm"
)

func TestNewClientValidation(t *testing.T) {
	if _, err := NewClient(Config{}); err == nil {
		t.Fatalf("expected error when api key is missing")
	}
}

func TestGenerateSuccess(t *testing.T) {
	var captured struct {
		Authorization string
		Body          map[string]any
	}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		captured.Authorization = r.Header.Get("Authorization")
		defer r.Body.Close()
		if err := json.NewDecoder(r.Body).Decode(&captured.Body); err != nil {
			t.Fatalf("failed to decode body: %v", err)
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"choices": []map[string]any{
				{
					"message": map[string]any{
						"content": `{"thought":"分析","reply":"你好"}`,
					},
				},
			},
		})
	}))
	defer srv.Close()

	client, err := NewClient(Config{APIKey: "test", BaseURL: srv.URL, Timeout: time.Second})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	client.httpClient = srv.Client()

	resp, err := client.Generate(context.Background(), llm.Request{Goal: "测试"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if resp.Reply != "你好" || resp.Thought != "分析" {
		t.Fatalf("unexpected response: %+v", resp)
	}

	if !strings.HasPrefix(captured.Authorization, "Bearer ") {
		t.Fatalf("authorization header missing: %q", captured.Authorization)
	}

	if captured.Body["model"] == "" {
		t.Fatalf("model field missing in request")
	}
}

func TestGenerateHTTPError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "boom", http.StatusBadRequest)
	}))
	defer srv.Close()

	client, err := NewClient(Config{APIKey: "test", BaseURL: srv.URL, Timeout: time.Second})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	client.httpClient = srv.Client()

	if _, err := client.Generate(context.Background(), llm.Request{Goal: "test"}); err == nil {
		t.Fatalf("expected error when http status is not success")
	}
}
