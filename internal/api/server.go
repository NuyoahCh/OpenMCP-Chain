package api

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"strconv"
	"time"

	"OpenMCP-Chain/internal/agent"
)

// Server 负责暴露 REST 接口，供外部驱动智能体执行。
type Server struct {
	addr  string
	agent *agent.Agent
}

// NewServer 构造 API 服务实例。
func NewServer(addr string, ag *agent.Agent) *Server {
	return &Server{addr: addr, agent: ag}
}

// Start 启动 HTTP 服务，直到上下文取消或出现错误。
func (s *Server) Start(ctx context.Context) error {
	mux := http.NewServeMux()
	mux.HandleFunc("/api/v1/tasks", s.handleTasks)

	// 配置 HTTP 服务器。
	server := &http.Server{
		Addr:              s.addr,
		Handler:           withContext(ctx, mux),
		ReadHeaderTimeout: 5 * time.Second,
	}

	// 启动服务器并监听关闭信号。
	errCh := make(chan error, 1)
	go func() {
		if err := server.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			errCh <- err
		}
		close(errCh)
	}()

	select {
	case <-ctx.Done():
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_ = server.Shutdown(shutdownCtx)
		return ctx.Err()
	case err := <-errCh:
		return err
	}
}

func (s *Server) handleTasks(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodPost:
		s.handleCreateTask(w, r)
	case http.MethodGet:
		s.handleListTasks(w, r)
	default:
		http.Error(w, "仅支持 GET/POST", http.StatusMethodNotAllowed)
	}
}

func (s *Server) handleCreateTask(w http.ResponseWriter, r *http.Request) {
	if s.agent == nil {
		http.Error(w, "Agent 未初始化", http.StatusServiceUnavailable)
		return
	}

// handleCreateTask 处理创建智能体任务的请求。
func (s *Server) handleCreateTask(w http.ResponseWriter, r *http.Request) {
	// 仅支持 POST 方法。
	if r.Method != http.MethodPost {
		http.Error(w, "仅支持 POST", http.StatusMethodNotAllowed)
		return
	}

	// 解析请求体。
	var req agent.TaskRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "请求体解析失败", http.StatusBadRequest)
		return
	}

	// 调用智能体执行任务。
	ctx := r.Context()
	result, err := s.agent.Execute(ctx, req)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	// 返回结果。
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(result)
}

func (s *Server) handleListTasks(w http.ResponseWriter, r *http.Request) {
	limit := 20
	if raw := r.URL.Query().Get("limit"); raw != "" {
		if parsed, err := strconv.Atoi(raw); err == nil && parsed > 0 {
			limit = parsed
		}
	}

	ctx := r.Context()
	if s.agent == nil {
		http.Error(w, "Agent 未初始化", http.StatusServiceUnavailable)
		return
	}
	results, err := s.agent.ListHistory(ctx, limit)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(results)
}

// withContext 确保请求处理能够感知根上下文取消。
func withContext(ctx context.Context, handler http.Handler) http.Handler {
	// 包装处理器以检查上下文状态。
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		select {
		case <-ctx.Done():
			http.Error(w, "服务已关闭", http.StatusServiceUnavailable)
			return
		default:
		}
		handler.ServeHTTP(w, r)
	})
}
