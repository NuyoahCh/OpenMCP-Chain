package auth

import (
	"net/http"
	"time"

	loggerpkg "OpenMCP-Chain/pkg/logger"
)

// MiddlewareConfig controls how the authentication middleware behaves.
type MiddlewareConfig struct {
	// RequiredPermissions maps HTTP methods to the permissions that must be
	// present on the authenticated subject. Use "*" for method-agnostic
	// requirements.
	RequiredPermissions map[string][]string
	// AuditEvent overrides the audit event name. Defaults to the request path.
	AuditEvent string
}

// Middleware enforces authentication for the given handler and records audit
// events through the configured audit logger.
func (s *Service) Middleware(cfg MiddlewareConfig) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if s == nil || s.mode == ModeDisabled {
				next.ServeHTTP(w, r)
				return
			}
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

type auditWriter struct {
	http.ResponseWriter
	status int
}

func (w *auditWriter) WriteHeader(code int) {
	w.status = code
	w.ResponseWriter.WriteHeader(code)
}
