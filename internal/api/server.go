package api

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"strconv"
	"strings"
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
	var taskHandler http.Handler = http.HandlerFunc(s.handleTasks)
	var taskDetailHandler http.Handler = http.HandlerFunc(s.handleTaskDetail)
	// 应用身份认证中间件（如果配置了认证服务）。
	if s.auth != nil && s.auth.Mode() != auth.ModeDisabled {
		cfg := auth.MiddlewareConfig{
			RequiredPermissions: map[string][]string{
				http.MethodGet:  {"tasks.read"},
				http.MethodPost: {"tasks.write"},
			},
			AuditEvent: "tasks",
		}
		taskHandler = s.auth.Middleware(cfg)(taskHandler)
		detailCfg := cfg
		detailCfg.AuditEvent = "task_detail"
		taskDetailHandler = s.auth.Middleware(detailCfg)(taskDetailHandler)
	}
	// 应用请求日志中间件。
	mux.Handle("/api/v1/tasks", s.instrument("tasks", taskHandler))
	mux.Handle("/api/v1/tasks/", s.instrument("task_detail", taskDetailHandler))
	mux.Handle("/api/v1/tasks/stats", s.instrument("task_stats", http.HandlerFunc(s.handleTaskStats)))
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
		writeJSONError(w, http.StatusMethodNotAllowed, "METHOD_NOT_ALLOWED", "仅支持 GET/POST")
	}
}

// handleTaskDetail 处理单个任务查询请求。
func (s *Server) handleTaskDetail(w http.ResponseWriter, r *http.Request) {
	id := strings.TrimSpace(strings.TrimPrefix(r.URL.Path, "/api/v1/tasks/"))
	id = strings.Trim(id, "/")
	if id == "" {
		// 与历史行为保持一致：当路径以 `/api/v1/tasks/` 结尾时，回退到原有任务处理逻辑，
		// 使得包含查询参数的请求（如 `/api/v1/tasks/?status=...`）和创建请求都能继续使用。
		s.handleTasks(w, r)
		return
	}

	if r.Method != http.MethodGet {
		writeJSONError(w, http.StatusMethodNotAllowed, "METHOD_NOT_ALLOWED", "仅支持 GET")
		return
	}
	if s.tasks == nil {
		writeJSONError(w, http.StatusServiceUnavailable, string(xerrors.CodeInitializationFailure), "任务服务未初始化")
		return
	}

	id := strings.TrimSpace(strings.TrimPrefix(r.URL.Path, "/api/v1/tasks/"))
	id = strings.Trim(id, "/")
	if id == "" {
		// 与历史行为保持一致：当路径以 `/api/v1/tasks/` 结尾时，回退到列表处理逻辑，
		// 使得包含查询参数的请求（如 `/api/v1/tasks/?status=...`）仍然生效。
		s.handleListTasks(w, r)
		writeJSONError(w, http.StatusBadRequest, "INVALID_TASK_ID", "任务 ID 不能为空")
		return
	}
	if strings.Contains(id, "/") {
		writeJSONError(w, http.StatusNotFound, string(task.CodeTaskNotFound), "任务不存在")
		return
	}

	ctx := r.Context()
	taskItem, err := s.tasks.Get(ctx, id)
	if err != nil {
		if task.IsTaskError(err, task.CodeTaskNotFound) || xerrors.CodeOf(err) == xerrors.CodeNotFound {
			writeJSONError(w, http.StatusNotFound, string(task.CodeTaskNotFound), "任务不存在")
			return
		}
		logger.L().Error("查询任务失败", slog.Any("error", err), slog.String("task_id", id))
		status := statusFromError(err)
		code := string(xerrors.CodeOf(err))
		if code == "" {
			code = "TASK_QUERY_FAILED"
		}
		writeJSONError(w, status, code, err.Error())
		return
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(taskItem)
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
		writeJSONError(w, http.StatusMethodNotAllowed, "METHOD_NOT_ALLOWED", "仅支持 POST")
		return
	}

	// 解析请求体。
	var req agent.TaskRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		logger.L().Warn("任务创建请求解析失败", slog.Any("error", err))
		writeJSONError(w, http.StatusBadRequest, string(xerrors.CodeInvalidArgument), "请求体解析失败")
		return
	}

	if s.tasks == nil {
		writeJSONError(w, http.StatusServiceUnavailable, string(xerrors.CodeInitializationFailure), "任务服务未初始化")
		return
	}

	// 提交任务。
	ctx := r.Context()
	taskItem, err := s.tasks.Submit(ctx, req)
	if err != nil {
		logger.L().Error("任务提交失败", slog.Any("error", err))
		status := statusFromError(err)
		code := string(xerrors.CodeOf(err))
		if code == "" {
			code = "TASK_SUBMIT_FAILED"
		}
		writeJSONError(w, status, code, err.Error())
		return
	}

	// 返回任务创建响应。
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusAccepted)
	_ = json.NewEncoder(w).Encode(map[string]any{
		"task_id":      taskItem.ID,
		"status":       taskItem.Status,
		"attempts":     taskItem.Attempts,
		"max_retries":  taskItem.MaxRetries,
		"goal":         taskItem.Goal,
		"chain_action": taskItem.ChainAction,
		"address":      taskItem.Address,
		"metadata":     taskItem.Metadata,
		"created_at":   taskItem.CreatedAt,
		"updated_at":   taskItem.UpdatedAt,
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
		writeJSONError(w, http.StatusMethodNotAllowed, "METHOD_NOT_ALLOWED", "仅支持 POST")
		return
	}
	if s.auth == nil || s.auth.Mode() == auth.ModeDisabled {
		writeJSONError(w, http.StatusServiceUnavailable, "AUTH_DISABLED", "身份认证未启用")
		return
	}
	var req auth.TokenRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		logger.L().Warn("认证请求解析失败", slog.Any("error", err))
		writeJSONError(w, http.StatusBadRequest, "INVALID_REQUEST", "请求体解析失败")
		return
	}
	token, err := s.auth.Authenticate(r.Context(), req)
	if err != nil {
		switch {
		case errors.Is(err, auth.ErrInvalidCredentials):
			writeJSONError(w, http.StatusUnauthorized, "INVALID_CREDENTIALS", "认证失败")
		case errors.Is(err, auth.ErrUnsupportedGrant):
			writeJSONError(w, http.StatusBadRequest, "UNSUPPORTED_GRANT", err.Error())
		case errors.Is(err, auth.ErrSubjectRevoked):
			writeJSONError(w, http.StatusForbidden, "ACCOUNT_DISABLED", "账号已禁用")
		case errors.Is(err, auth.ErrDisabled):
			writeJSONError(w, http.StatusServiceUnavailable, "AUTH_DISABLED", "身份认证未启用")
		default:
			logger.L().Error("令牌签发失败", slog.Any("error", err))
			writeJSONError(w, http.StatusInternalServerError, "TOKEN_ISSUE_FAILED", "服务器内部错误")
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
		writeJSONError(w, http.StatusServiceUnavailable, string(xerrors.CodeInitializationFailure), "任务服务未初始化")
		return
	}

	// 处理单个任务查询。
	ctx := r.Context()
	if id := r.URL.Query().Get("id"); id != "" {
		taskItem, err := s.tasks.Get(ctx, id)
		if err != nil {
			if task.IsTaskError(err, task.CodeTaskNotFound) || xerrors.CodeOf(err) == xerrors.CodeNotFound {
				writeJSONError(w, http.StatusNotFound, string(task.CodeTaskNotFound), "任务不存在")
				return
			}
			logger.L().Error("查询任务失败", slog.Any("error", err), slog.String("task_id", id))
			status := statusFromError(err)
			code := string(xerrors.CodeOf(err))
			if code == "" {
				code = "TASK_QUERY_FAILED"
			}
			writeJSONError(w, status, code, err.Error())
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(taskItem)
		return
	}

	// 处理任务列表查询，支持分页与过滤参数。
	query := r.URL.Query()

	limit := task.DefaultListLimit
	if raw := query.Get("limit"); raw != "" {
		parsed, err := strconv.Atoi(raw)
		if err != nil || parsed <= 0 {
			writeJSONError(w, http.StatusBadRequest, "INVALID_LIMIT", "limit 参数必须为正整数")
			return
		}
		if parsed > task.MaxListLimit {
			parsed = task.MaxListLimit
		}
		limit = parsed
	}

	filters, reqErr := parseFilterOptions(query)
	if reqErr != nil {
		writeJSONError(w, reqErr.status, reqErr.code, reqErr.message)
		return
	}

	opts := make([]task.ListOption, 0, len(filters.options)+2)
	opts = append(opts, filters.options...)
	opts = append(opts, task.WithOffset(filters.offset))
	opts = append(opts, task.WithLimit(limit))

	// 列出任务。
	tasks, err := s.tasks.List(ctx, opts...)
	if err != nil {
		logger.L().Error("列出任务失败", slog.Any("error", err))
		status := statusFromError(err)
		code := string(xerrors.CodeOf(err))
		if code == "" {
			code = "TASK_LIST_FAILED"
		}
		writeJSONError(w, status, code, err.Error())
		return
	}

	total := filters.offset + len(tasks)
	hasMore := false
	statsAvailable := false

	if s.tasks != nil {
		if stats, statsErr := s.tasks.Stats(ctx, filters.options...); statsErr != nil {
			logger.L().Warn("统计任务总数失败", slog.Any("error", statsErr))
		} else {
			statsAvailable = true
			if stats.Total > total {
				total = stats.Total
			}
			hasMore = filters.offset+len(tasks) < stats.Total
		}
	}

	if total < filters.offset+len(tasks) {
		total = filters.offset + len(tasks)
	}

	if !statsAvailable && len(tasks) == limit {
		hasMore = true
	}

	var nextOffset *int
	if hasMore {
		next := filters.offset + len(tasks)
		nextOffset = &next
	}

	// 返回任务列表响应。
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(listResponse{
		Tasks:      tasks,
		Total:      total,
		HasMore:    hasMore,
		NextOffset: nextOffset,
	})
}

// handleTaskStats 处理任务统计查询请求。
func (s *Server) handleTaskStats(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeJSONError(w, http.StatusMethodNotAllowed, "METHOD_NOT_ALLOWED", "仅支持 GET")
		return
	}
	if s.tasks == nil {
		writeJSONError(w, http.StatusServiceUnavailable, string(xerrors.CodeInitializationFailure), "任务服务未初始化")
		return
	}

	filters, reqErr := parseFilterOptions(r.URL.Query())
	if reqErr != nil {
		writeJSONError(w, reqErr.status, reqErr.code, reqErr.message)
		return
	}

	stats, err := s.tasks.Stats(r.Context(), filters.options...)
	if err != nil {
		logger.L().Error("统计任务失败", slog.Any("error", err))
		status := statusFromError(err)
		code := string(xerrors.CodeOf(err))
		if code == "" {
			code = "TASK_STATS_FAILED"
		}
		writeJSONError(w, status, code, err.Error())
		return
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(stats)
}

// handleTaskStats 处理任务统计查询请求。
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

type errorResponse struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

func writeJSONError(w http.ResponseWriter, status int, code, message string) {
	if code == "" {
		code = "UNKNOWN_ERROR"
	}
	if message == "" {
		message = http.StatusText(status)
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(errorResponse{Code: code, Message: message}); err != nil {
		logger.L().Error("输出错误响应失败", slog.Any("error", err))
	}
}

type requestError struct {
	status  int
	code    string
	message string
}

type listResponse struct {
	Tasks      []*task.Task `json:"tasks"`
	Total      int          `json:"total"`
	HasMore    bool         `json:"has_more"`
	NextOffset *int         `json:"next_offset,omitempty"`
}

type filterOptions struct {
	options []task.ListOption
	offset  int
}

func parseFilterOptions(query map[string][]string) (filterOptions, *requestError) {
	filters := filterOptions{options: make([]task.ListOption, 0, 4)}

	if rawStatuses, ok := query["status"]; ok {
		statuses, err := parseStatusFilters(rawStatuses)
		if err != nil {
			return filterOptions{}, &requestError{status: http.StatusBadRequest, code: "INVALID_STATUS", message: err.Error()}
		}
		if len(statuses) > 0 {
			filters.options = append(filters.options, task.WithStatuses(statuses...))
		}
	}

	if raw := firstValue(query, "since"); raw != "" {
		ts, err := time.Parse(time.RFC3339, raw)
		if err != nil {
			return filterOptions{}, &requestError{status: http.StatusBadRequest, code: "INVALID_SINCE", message: "since 参数需为 RFC3339 时间格式"}
		}
		filters.options = append(filters.options, task.WithUpdatedSince(ts))
	}

	if raw := firstValue(query, "until"); raw != "" {
		ts, err := time.Parse(time.RFC3339, raw)
		if err != nil {
			return filterOptions{}, &requestError{status: http.StatusBadRequest, code: "INVALID_UNTIL", message: "until 参数需为 RFC3339 时间格式"}
		}
		filters.options = append(filters.options, task.WithUpdatedUntil(ts))
	}

	if raw := firstValue(query, "has_result"); raw != "" {
		parsed, err := strconv.ParseBool(raw)
		if err != nil {
			return filterOptions{}, &requestError{status: http.StatusBadRequest, code: "INVALID_HAS_RESULT", message: "has_result 参数需为布尔值"}
		}
		filters.options = append(filters.options, task.WithResultPresence(parsed))
	}

	if raw := strings.ToLower(firstValue(query, "order")); raw != "" {
		switch raw {
		case "asc":
			filters.options = append(filters.options, task.WithSortOrder(task.SortByUpdatedAsc))
		case "desc":
			filters.options = append(filters.options, task.WithSortOrder(task.SortByUpdatedDesc))
		default:
			return filterOptions{}, &requestError{status: http.StatusBadRequest, code: "INVALID_ORDER", message: "order 参数仅支持 asc/desc"}
		}
	}

	if raw := firstValue(query, "offset"); raw != "" {
		parsed, err := strconv.Atoi(raw)
		if err != nil || parsed < 0 {
			return filterOptions{}, &requestError{status: http.StatusBadRequest, code: "INVALID_OFFSET", message: "offset 参数必须为非负整数"}
		}
		filters.offset = parsed
	}

	if raw := strings.TrimSpace(firstValue(query, "q")); raw != "" {
		filters.options = append(filters.options, task.WithQuery(raw))
	}

	return filters, nil
}

func firstValue(values map[string][]string, key string) string {
	items, ok := values[key]
	if !ok || len(items) == 0 {
		return ""
	}
	return items[0]
}

func parseStatusFilters(values []string) ([]task.Status, error) {
	if len(values) == 0 {
		return nil, nil
	}
	seen := make(map[task.Status]struct{}, len(values))
	statuses := make([]task.Status, 0, len(values))
	for _, raw := range values {
		for _, token := range strings.Split(raw, ",") {
			trimmed := strings.TrimSpace(token)
			if trimmed == "" {
				continue
			}
			status := task.Status(trimmed)
			if !task.IsValidStatus(status) {
				return nil, fmt.Errorf("status 参数包含未知的任务状态: %s", trimmed)
			}
			if _, ok := seen[status]; ok {
				continue
			}
			seen[status] = struct{}{}
			statuses = append(statuses, status)
		}
	}
	if len(statuses) == 0 {
		return nil, nil
	}
	return statuses, nil
}
