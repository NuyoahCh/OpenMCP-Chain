import { useEffect, useState } from "react";

interface ConnectionSettingsProps {
  baseUrl: string;
  defaultBaseUrl: string;
  onUpdate: (value: string) => Promise<void> | void;
  onReset: () => Promise<void> | void;
  onTest: () => Promise<void>;
  testing: boolean;
  status: "idle" | "success" | "error";
  lastSynced: number | null;
  refreshing: boolean;
  onRefresh: () => void;
  fetchError?: string | null;
  isOnline: boolean;
  connectionType?: string | null;
}

function formatSyncTime(lastSynced: number | null): string {
  if (!lastSynced) {
    return "尚未同步";
  }
  const date = new Date(lastSynced);
  return `${date.toLocaleDateString()} ${date.toLocaleTimeString()}`;
}

export default function ConnectionSettings({
  baseUrl,
  defaultBaseUrl,
  onUpdate,
  onReset,
  onTest,
  testing,
  status,
  lastSynced,
  refreshing,
  onRefresh,
  fetchError,
  isOnline,
  connectionType
}: ConnectionSettingsProps) {
  const [value, setValue] = useState(baseUrl);
  const [saving, setSaving] = useState(false);
  const [message, setMessage] = useState<string | null>(null);
  const [messageType, setMessageType] = useState<"success" | "error" | null>(null);

  useEffect(() => {
    setValue(baseUrl);
  }, [baseUrl]);

  const handleSave = async () => {
    setSaving(true);
    setMessage(null);
    setMessageType(null);
    try {
      await onUpdate(value);
      setMessage("API 地址已更新");
      setMessageType("success");
    } catch (error) {
      setMessage(error instanceof Error ? error.message : "更新失败");
      setMessageType("error");
    } finally {
      setSaving(false);
    }
  };

  const handleReset = async () => {
    setSaving(true);
    setMessage(null);
    setMessageType(null);
    try {
      await onReset();
      setMessage("已恢复默认地址");
      setMessageType("success");
    } catch (error) {
      setMessage(error instanceof Error ? error.message : "恢复失败");
      setMessageType("error");
    } finally {
      setSaving(false);
    }
  };

  const syncStateLabel =
    status === "success" ? "连接成功" : status === "error" ? "连接异常" : "等待检测";

  return (
    <div className="card settings-card">
      <h3 style={{ marginTop: 0, marginBottom: "0.75rem" }}>运行提示 &amp; 服务配置</h3>
      <p className="helper-text" style={{ marginTop: 0 }}>
        默认连接 <code>{defaultBaseUrl}</code>，可通过 <code>VITE_API_BASE_URL</code> 或下方设置覆盖。
      </p>
      <div className="input-field" style={{ marginTop: "1rem" }}>
        <label htmlFor="api-base-url">API Base URL</label>
        <input
          id="api-base-url"
          value={value}
          onChange={(event) => {
            setValue(event.target.value);
            setMessage(null);
            setMessageType(null);
          }}
          placeholder="http://127.0.0.1:8080"
        />
        <span className="helper-text">
          该地址应指向后端服务，例如 <code>http://127.0.0.1:8080</code>。
        </span>
      </div>
      {message ? (
        <p
          className="helper-text"
          style={{
            color:
              messageType === "error"
                ? "#fda4af"
                : messageType === "success"
                  ? "#a5f3fc"
                  : "rgba(148, 163, 184, 0.75)"
          }}
        >
          {message}
        </p>
      ) : null}
      <div className="actions" style={{ marginTop: "1.25rem", flexWrap: "wrap" }}>
        <button type="button" className="primary" onClick={handleSave} disabled={saving}>
          {saving ? "保存中..." : "保存地址"}
        </button>
        <button type="button" className="secondary" onClick={handleReset} disabled={saving}>
          恢复默认
        </button>
        <button type="button" className="ghost" onClick={onTest} disabled={testing}>
          {testing ? "检测中..." : "测试连接"}
        </button>
      </div>
      <div className="meta-row" style={{ marginTop: "1.25rem" }}>
        <span>
          <strong>连接状态:</strong> {syncStateLabel}
        </span>
        <span>
          <strong>最近同步:</strong> {formatSyncTime(lastSynced)}
        </span>
        <span>
          <strong>网络:</strong> {isOnline ? `在线${connectionType ? ` · ${connectionType}` : ""}` : "离线"}
        </span>
        <button type="button" className="link" onClick={onRefresh} disabled={refreshing}>
          {refreshing ? "同步中..." : "立即刷新"}
        </button>
      </div>
      {fetchError ? (
        <p className="helper-text" style={{ color: "#fda4af", marginTop: "0.75rem" }}>
          {fetchError}
        </p>
      ) : null}
    </div>
  );
}
