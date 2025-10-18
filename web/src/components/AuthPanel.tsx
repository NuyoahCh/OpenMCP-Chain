import { useEffect, useState } from "react";
import type { AuthState } from "../api";
import type { AuthCredentials } from "../hooks/useAuth";

interface AuthPanelProps {
  auth: AuthState | null;
  isExpired: boolean;
  requiresAuth: boolean;
  onLogin: (credentials: AuthCredentials) => Promise<void>;
  onLogout: () => void;
  onSessionRestored?: () => void;
}

function formatRemaining(expiresAt?: number): string {
  if (!expiresAt) {
    return "-";
  }
  const delta = expiresAt - Date.now();
  if (delta <= 0) {
    return "已过期";
  }
  const minutes = Math.floor(delta / 60000);
  const seconds = Math.floor((delta % 60000) / 1000);
  if (minutes > 120) {
    const hours = Math.floor(minutes / 60);
    return `${hours} 小时`;
  }
  if (minutes > 0) {
    return `${minutes} 分 ${seconds.toString().padStart(2, "0")} 秒`;
  }
  return `${seconds} 秒`;
}

export default function AuthPanel({
  auth,
  isExpired,
  requiresAuth,
  onLogin,
  onLogout,
  onSessionRestored
}: AuthPanelProps) {
  const [username, setUsername] = useState("");
  const [password, setPassword] = useState("");
  const [submitting, setSubmitting] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [info, setInfo] = useState<string | null>(null);
  const [remaining, setRemaining] = useState(() => formatRemaining(auth?.expiresAt));

  useEffect(() => {
    setRemaining(formatRemaining(auth?.expiresAt));
    if (!auth?.expiresAt) {
      return;
    }
    const timer = setInterval(() => {
      setRemaining(formatRemaining(auth?.expiresAt));
    }, 1000);
    return () => clearInterval(timer);
  }, [auth?.expiresAt]);

  const handleSubmit = async (event: React.FormEvent) => {
    event.preventDefault();
    if (!username.trim() || !password) {
      setError("请输入用户名和密码");
      return;
    }
    setSubmitting(true);
    setError(null);
    setInfo(null);
    try {
      await onLogin({ username: username.trim(), password });
      setPassword("");
      setInfo("登录成功，令牌已缓存");
      onSessionRestored?.();
    } catch (loginError) {
      setError(loginError instanceof Error ? loginError.message : "登录失败");
    } finally {
      setSubmitting(false);
    }
  };

  const handleLogout = () => {
    onLogout();
    setInfo("已退出登录");
  };

  const canCopy = typeof navigator !== "undefined" && Boolean(navigator.clipboard?.writeText);

  const copyToken = async () => {
    if (!auth?.accessToken || !canCopy) {
      return;
    }
    try {
      await navigator.clipboard.writeText(auth.accessToken);
      setInfo("访问令牌已复制到剪贴板");
    } catch (copyError) {
      setError(copyError instanceof Error ? copyError.message : "复制失败");
    }
  };

  return (
    <div className="card auth-card">
      <h3 style={{ marginTop: 0, marginBottom: "0.75rem" }}>身份认证</h3>
      <p className="helper-text" style={{ marginTop: 0 }}>
        若后端启用了 JWT/OAuth 认证，请先登录以获取访问令牌。默认配置关闭认证，可直接访问。
      </p>
      {requiresAuth && !auth ? (
        <p className="helper-text" style={{ color: "#fda4af", marginBottom: "0.75rem" }}>
          后端返回 401 未授权，请使用拥有权限的账号登录。
        </p>
      ) : null}
      {auth ? (
        <div className="auth-state">
          <div className="meta-row" style={{ marginBottom: "0.5rem" }}>
            <span>
              <strong>当前用户:</strong> {auth.username || "已认证"}
            </span>
            <span>
              <strong>剩余有效期:</strong> {isExpired ? "已过期" : remaining}
            </span>
          </div>
          {auth.scope ? (
            <p className="helper-text" style={{ marginBottom: "0.75rem" }}>
              权限范围: {auth.scope.join(", ")}
            </p>
          ) : null}
          <div className="actions" style={{ marginTop: "0.5rem", flexWrap: "wrap" }}>
            <button type="button" className="secondary" onClick={handleLogout}>
              退出登录
            </button>
            <button type="button" className="ghost" onClick={copyToken} disabled={!canCopy}>
              复制访问令牌
            </button>
            <button
              type="button"
              className="ghost"
              onClick={() => {
                if (!auth) {
                  return;
                }
                navigator?.clipboard
                  ?.writeText(JSON.stringify(auth, null, 2))
                  .then(() => {
                    setError(null);
                    setInfo("认证详情已复制");
                  })
                  .catch((error) =>
                    setError(error instanceof Error ? error.message : "复制失败")
                  );
              }}
              disabled={!canCopy}
            >
              复制会话详情
            </button>
          </div>
          {isExpired ? (
            <p className="helper-text" style={{ color: "#f97316", marginTop: "0.75rem" }}>
              会话已过期，请重新登录以继续操作。
            </p>
          ) : null}
        </div>
      ) : (
        <form className="auth-form" onSubmit={handleSubmit}>
          <div className="input-field">
            <label htmlFor="auth-username">用户名</label>
            <input
              id="auth-username"
              value={username}
              onChange={(event) => {
                setUsername(event.target.value);
                setError(null);
                setInfo(null);
              }}
              placeholder="admin"
              autoComplete="username"
            />
          </div>
          <div className="input-field">
            <label htmlFor="auth-password">密码</label>
            <input
              id="auth-password"
              value={password}
              onChange={(event) => {
                setPassword(event.target.value);
                setError(null);
                setInfo(null);
              }}
              type="password"
              placeholder="••••••"
              autoComplete="current-password"
            />
          </div>
          {error ? (
            <p className="helper-text" style={{ color: "#f87171" }}>{error}</p>
          ) : null}
          {info ? (
            <p className="helper-text" style={{ color: "#a5f3fc" }}>{info}</p>
          ) : null}
          <button type="submit" className="primary" disabled={submitting}>
            {submitting ? "登录中..." : "获取访问令牌"}
          </button>
        </form>
      )}
    </div>
  );
}
