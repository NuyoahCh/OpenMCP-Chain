package auth

import (
	"net/http"
	"time"

	loggerpkg "OpenMCP-Chain/pkg/logger"
)

// MiddlewareConfig 配置身份认证中间件的行为。
type MiddlewareConfig struct {
	// RequiredPermissions 定义每个 HTTP 方法所需的权限列表。
	RequiredPermissions map[string][]string
	// AuditEvent 指定记录审计日志时使用的事件名称。
	AuditEvent string
}

// Middleware 返回一个 HTTP 中间件，用于处理身份认证和授权。
func (s *Service) Middleware(cfg MiddlewareConfig) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		// 返回实际的中间件处理函数。
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if s == nil || s.mode == ModeDisabled {
				next.ServeHTTP(w, r)
				return
			}
			// 认证请求。
			subject, err := s.AuthenticateRequest(r.Context(), r.Header.Get("Authorization"))
			if err != nil {
				status := http.StatusUnauthorized
				switch err {
				case ErrMissingToken:
					status = http.StatusUnauthorized
				case ErrPermissionDenied, ErrSubjectRevoked:
					status = http.StatusForbidden
				default:
					if err == ErrInvalidToken {
						status = http.StatusUnauthorized
					}
				}
				http.Error(w, http.StatusText(status), status)
				logger := s.audit
				if logger == nil {
					logger = loggerpkg.Audit()
				}
				logger.Warn("access_denied",
					"path", r.URL.Path,
					"method", r.Method,
					"status", status,
					"error", err.Error(),
				)
				return
			}
			// 授权请求。
			perms := cfg.RequiredPermissions[r.Method]
			if len(perms) == 0 {
				perms = cfg.RequiredPermissions["*"]
			}
			if len(perms) > 0 {
				if err := subject.Authorize(perms...); err != nil {
					status := http.StatusForbidden
					http.Error(w, http.StatusText(status), status)
					logger := s.audit
					if logger == nil {
						logger = loggerpkg.Audit()
					}
					logger.Warn("permission_denied",
						"path", r.URL.Path,
						"method", r.Method,
						"status", status,
						"error", err.Error(),
						"user", subject.Username,
					)
					return
				}
			}
			// 记录审计日志。
			start := time.Now()
			aw := &auditWriter{ResponseWriter: w, status: http.StatusOK}
			ctx := WithSubject(r.Context(), subject)
			next.ServeHTTP(aw, r.WithContext(ctx))
			event := cfg.AuditEvent
			if event == "" {
				event = r.URL.Path
			}
			logger := s.audit
			if logger == nil {
				logger = loggerpkg.Audit()
			}
			logger.Info("api_request",
				"event", event,
				"method", r.Method,
				"path", r.URL.Path,
				"status", aw.status,
				"duration_ms", time.Since(start).Milliseconds(),
				"user", subject.Username,
			)
		})
	}
}

// auditWriter 是一个包装了 http.ResponseWriter 的结构体，用于捕获响应状态码。
type auditWriter struct {
	http.ResponseWriter
	status int
}

// WriteHeader 捕获响应状态码并调用底层的 WriteHeader 方法。
func (w *auditWriter) WriteHeader(code int) {
	w.status = code
	w.ResponseWriter.WriteHeader(code)
}
