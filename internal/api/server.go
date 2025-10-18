package api

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"strconv"
	"time"

	"OpenMCP-Chain/internal/agent"
	"OpenMCP-Chain/internal/observability/metrics"
	"OpenMCP-Chain/internal/task"
	"OpenMCP-Chain/pkg/logger"
)

// Server 负责暴露 REST 接口，供外部驱动智能体执行。
type Server struct {
	addr           string
	tasks          *task.Service
	metricsEnabled bool
}

// NewServer 构造 API 服务实例。
func NewServer(addr string, svc *task.Service, opts ...Option) *Server {
	server := &Server{addr: addr, tasks: svc}
	for _, opt := range opts {
		if opt != nil {
			opt(server)
		}
	}
	return server
}

// Option configures optional features on the API server.
type Option func(*Server)

// WithMetrics 启用 HTTP 请求指标采集。
func WithMetrics(enabled bool) Option {
	return func(s *Server) {
		s.metricsEnabled = enabled
	}
}

// Start 启动 HTTP 服务，直到上下文取消或出现错误。
func (s *Server) Start(ctx context.Context) error {
	mux := http.NewServeMux()
	mux.Handle("/api/v1/tasks", s.instrument("tasks", http.HandlerFunc(s.handleTasks)))
	if s.metricsEnabled {
		mux.Handle("/metrics", metrics.Handler())
	}

	server := &http.Server{
		Addr:              s.addr,
		Handler:           withContext(ctx, mux),
		ReadHeaderTimeout: 5 * time.Second,
	}

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

func (s *Server) instrument(name string, handler http.Handler) http.Handler {
	if !s.metricsEnabled {
		return handler
	}
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		sw := &statusWriter{ResponseWriter: w}
		handler.ServeHTTP(sw, r)
		metrics.ObserveHTTPRequest(name, r.Method, sw.statusCode(), time.Since(start))
	})
}

// handleCreateTask 处理创建智能体任务的请求。
func (s *Server) handleCreateTask(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "仅支持 POST", http.StatusMethodNotAllowed)
		return
	}

	var req agent.TaskRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		logger.L().Warn("任务创建请求解析失败", slog.Any("error", err))
		http.Error(w, "请求体解析失败", http.StatusBadRequest)
		return
	}

	if s.tasks == nil {
		http.Error(w, "任务服务未初始化", http.StatusServiceUnavailable)
		return
	}

	ctx := r.Context()
	taskItem, err := s.tasks.Submit(ctx, req)
	if err != nil {
		logger.L().Error("任务提交失败", slog.Any("error", err))
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusAccepted)
	_ = json.NewEncoder(w).Encode(map[string]any{
		"task_id":     taskItem.ID,
		"status":      taskItem.Status,
		"attempts":    taskItem.Attempts,
		"max_retries": taskItem.MaxRetries,
	})
	logger.L().Info("任务已受理", slog.String("task_id", taskItem.ID))
}

func (s *Server) handleListTasks(w http.ResponseWriter, r *http.Request) {
	if s.tasks == nil {
		http.Error(w, "任务服务未初始化", http.StatusServiceUnavailable)
		return
	}

	ctx := r.Context()
	if id := r.URL.Query().Get("id"); id != "" {
		taskItem, err := s.tasks.Get(ctx, id)
		if err != nil {
			if errors.Is(err, task.ErrTaskNotFound) {
				http.Error(w, "任务不存在", http.StatusNotFound)
				return
			}
			logger.L().Error("查询任务失败", slog.Any("error", err), slog.String("task_id", id))
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(taskItem)
		return
	}

	limit := 20
	if raw := r.URL.Query().Get("limit"); raw != "" {
		if parsed, err := strconv.Atoi(raw); err == nil && parsed > 0 {
			limit = parsed
		}
	}

	tasks, err := s.tasks.List(ctx, limit)
	if err != nil {
		logger.L().Error("列出任务失败", slog.Any("error", err))
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(tasks)
}

type statusWriter struct {
	http.ResponseWriter
	status int
}

func (w *statusWriter) WriteHeader(code int) {
	w.status = code
	w.ResponseWriter.WriteHeader(code)
}

func (w *statusWriter) statusCode() int {
	if w.status == 0 {
		return http.StatusOK
	}
	return w.status
}

// withContext 确保请求处理能够感知根上下文取消。
func withContext(ctx context.Context, handler http.Handler) http.Handler {
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
