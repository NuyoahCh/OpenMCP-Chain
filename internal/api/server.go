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
	"OpenMCP-Chain/internal/auth"
	xerrors "OpenMCP-Chain/internal/errors"
	"OpenMCP-Chain/internal/observability/metrics"
	"OpenMCP-Chain/internal/task"
	"OpenMCP-Chain/pkg/logger"
)

// Server 负责暴露 REST 接口，供外部驱动智能体执行。
type Server struct {
	addr           string
	tasks          *task.Service
	metricsEnabled bool
	auth           *auth.Service
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

// WithAuthService 设置身份认证服务。
func WithAuthService(authn *auth.Service) Option {
	return func(s *Server) {
		s.auth = authn
	}
}

// Start 启动 HTTP 服务，直到上下文取消或出现错误。
func (s *Server) Start(ctx context.Context) error {
	// 设置路由和中间件。
	mux := http.NewServeMux()
	taskHandler := http.HandlerFunc(s.handleTasks)
	// 应用身份认证中间件（如果配置了认证服务）。
	if s.auth != nil && s.auth.Mode() != auth.ModeDisabled {
		taskHandler = s.auth.Middleware(auth.MiddlewareConfig{
			RequiredPermissions: map[string][]string{
				http.MethodGet:  {"tasks.read"},
				http.MethodPost: {"tasks.write"},
			},
			AuditEvent: "tasks",
		})(taskHandler).(http.HandlerFunc)
	}
	// 应用请求日志中间件。
	mux.Handle("/api/v1/tasks", s.instrument("tasks", taskHandler))
	mux.Handle("/api/v1/auth/token", s.instrument("auth_token", http.HandlerFunc(s.handleAuthToken)))
	// 如果启用指标采集，注册指标处理器。
	if s.metricsEnabled {
		mux.Handle("/metrics", metrics.Handler())
	}

	// 启动 HTTP 服务器。
	server := &http.Server{
		Addr:              s.addr,
		Handler:           withContext(ctx, mux),
		ReadHeaderTimeout: 5 * time.Second,
	}

	// 监听关闭信号并优雅关闭服务器。
	errCh := make(chan error, 1)
	go func() {
		if err := server.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			errCh <- err
		}
		close(errCh)
	}()

	// 等待上下文取消或服务器错误。
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

// handleTasks 处理与智能体任务相关的请求。
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

// instrument 包装 HTTP 处理器以收集请求指标。
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
	// 仅允许 POST 方法。
	if r.Method != http.MethodPost {
		http.Error(w, "仅支持 POST", http.StatusMethodNotAllowed)
		return
	}

	// 解析请求体。
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

	// 提交任务。
	ctx := r.Context()
	taskItem, err := s.tasks.Submit(ctx, req)
	if err != nil {
		logger.L().Error("任务提交失败", slog.Any("error", err))
		status := statusFromError(err)
		http.Error(w, err.Error(), status)
		return
	}

	// 返回任务创建响应。
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusAccepted)
	_ = json.NewEncoder(w).Encode(map[string]any{
		"task_id":     taskItem.ID,
		"status":      taskItem.Status,
		"attempts":    taskItem.Attempts,
		"max_retries": taskItem.MaxRetries,
	})
	// 记录任务受理日志。
	logFields := []any{slog.String("task_id", taskItem.ID)}
	if subject := auth.SubjectFromContext(ctx); subject != nil {
		logFields = append(logFields, slog.String("user", subject.Username))
	}
	logger.L().Info("任务已受理", logFields...)
}

// handleAuthToken 处理身份认证令牌请求。
func (s *Server) handleAuthToken(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "仅支持 POST", http.StatusMethodNotAllowed)
		return
	}
	if s.auth == nil || s.auth.Mode() == auth.ModeDisabled {
		http.Error(w, "身份认证未启用", http.StatusServiceUnavailable)
		return
	}
	var req auth.TokenRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		logger.L().Warn("认证请求解析失败", slog.Any("error", err))
		http.Error(w, "请求体解析失败", http.StatusBadRequest)
		return
	}
	token, err := s.auth.Authenticate(r.Context(), req)
	if err != nil {
		switch {
		case errors.Is(err, auth.ErrInvalidCredentials):
			http.Error(w, "认证失败", http.StatusUnauthorized)
		case errors.Is(err, auth.ErrUnsupportedGrant):
			http.Error(w, err.Error(), http.StatusBadRequest)
		case errors.Is(err, auth.ErrSubjectRevoked):
			http.Error(w, "账号已禁用", http.StatusForbidden)
		case errors.Is(err, auth.ErrDisabled):
			http.Error(w, "身份认证未启用", http.StatusServiceUnavailable)
		default:
			logger.L().Error("令牌签发失败", slog.Any("error", err))
			http.Error(w, "服务器内部错误", http.StatusInternalServerError)
		}
		return
	}
	if token.TokenType == "" {
		token.TokenType = "Bearer"
	}
	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(token); err != nil {
		logger.L().Error("输出认证响应失败", slog.Any("error", err))
	}
	logger.L().Info("令牌签发成功", slog.String("grant_type", req.GrantType))
}

// handleListTasks 处理列出智能体任务的请求。
func (s *Server) handleListTasks(w http.ResponseWriter, r *http.Request) {
	if s.tasks == nil {
		http.Error(w, "任务服务未初始化", http.StatusServiceUnavailable)
		return
	}

	// 处理单个任务查询。
	ctx := r.Context()
	if id := r.URL.Query().Get("id"); id != "" {
		taskItem, err := s.tasks.Get(ctx, id)
		if err != nil {
			if task.IsTaskError(err, task.CodeTaskNotFound) || xerrors.CodeOf(err) == xerrors.CodeNotFound {
				http.Error(w, "任务不存在", http.StatusNotFound)
				return
			}
			logger.L().Error("查询任务失败", slog.Any("error", err), slog.String("task_id", id))
			http.Error(w, err.Error(), statusFromError(err))
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(taskItem)
		return
	}

	// 处理任务列表查询，支持 limit 参数。
	limit := 20
	if raw := r.URL.Query().Get("limit"); raw != "" {
		if parsed, err := strconv.Atoi(raw); err == nil && parsed > 0 {
			limit = parsed
		}
	}

	// 列出任务。
	tasks, err := s.tasks.List(ctx, limit)
	if err != nil {
		logger.L().Error("列出任务失败", slog.Any("error", err))
		http.Error(w, err.Error(), statusFromError(err))
		return
	}

	// 返回任务列表响应。
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(tasks)
}

// statusWriter 包装 http.ResponseWriter 以捕获响应状态码。
type statusWriter struct {
	http.ResponseWriter
	status int
}

// Write 捕获响应状态码。
func (w *statusWriter) WriteHeader(code int) {
	w.status = code
	w.ResponseWriter.WriteHeader(code)
}

// statusCode 返回捕获的状态码，默认为 200。
func (w *statusWriter) statusCode() int {
	if w.status == 0 {
		return http.StatusOK
	}
	return w.status
}

// withContext 确保请求处理能够感知根上下文取消。
func withContext(ctx context.Context, handler http.Handler) http.Handler {
	// 如果根上下文已取消，立即返回服务不可用响应。
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

// statusFromError 将错误映射为 HTTP 状态码。
func statusFromError(err error) int {
	if err == nil {
		return http.StatusInternalServerError
	}
	code := xerrors.CodeOf(err)
	switch code {
	case xerrors.CodeInvalidArgument, task.CodeTaskValidation:
		return http.StatusBadRequest
	case xerrors.CodeNotFound, task.CodeTaskNotFound:
		return http.StatusNotFound
	case xerrors.CodeConflict, task.CodeTaskConflict:
		return http.StatusConflict
	case xerrors.CodeInitializationFailure:
		return http.StatusServiceUnavailable
	case xerrors.CodeQueueFailure, task.CodeTaskPublish:
		return http.StatusBadGateway
	default:
		return http.StatusInternalServerError
	}
}
