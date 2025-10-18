import { useMemo, useState } from "react";
import type { CreateTaskRequest } from "../types";

interface TaskFormProps {
  onSubmit: (payload: CreateTaskRequest) => Promise<void>;
  loading?: boolean;
}

const CHAIN_TEMPLATES: Array<{ label: string; value: string }> = [
  { label: "仅推理（不触发链上操作）", value: "" },
  { label: "查询账户余额 (eth_getBalance)", value: "eth_getBalance" },
  { label: "查询最新区块 (eth_getBlockByNumber)", value: "eth_getBlockByNumber" }
];

export default function TaskForm({ onSubmit, loading }: TaskFormProps) {
  const [goal, setGoal] = useState("查询账户余额");
  const [chainAction, setChainAction] = useState("");
  const [address, setAddress] = useState("0x0000000000000000000000000000000000000000");
  const [errors, setErrors] = useState<string | null>(null);

  const exampleHint = useMemo(() => {
    const suggestions = [
      "同步分析最新区块变化，并总结潜在风险",
      "根据账户余额生成资产概览报告",
      "对比两个地址最近 10 笔交易的 Gas 情况"
    ];
    return suggestions[Math.floor(Math.random() * suggestions.length)];
  }, []);
  const [metadata, setMetadata] = useState(
    () => JSON.stringify({ project: "demo" }, null, 2)
  );
  const [errors, setErrors] = useState<string | null>(null);

  const metadataPreview = useMemo(() => {
    if (!metadata.trim()) {
      return "此字段可选，用于写入审计或上下文信息";
    }
    try {
      const parsed = JSON.parse(metadata);
      return JSON.stringify(parsed, null, 2);
    } catch (error) {
      return "⚠️ JSON 格式错误，请检查后再提交";
    }
  }, [metadata]);

  const submitDisabled = loading || !goal.trim();

  const handleSubmit = async (event: React.FormEvent) => {
    event.preventDefault();
    if (!goal.trim()) {
      setErrors("请填写任务目标");
      return;
    }
    const payload: CreateTaskRequest = {
      goal: goal.trim(),
      chain_action: chainAction || undefined,
      address: address.trim() || undefined
    };
    if (metadata.trim()) {
      try {
        payload.metadata = JSON.parse(metadata);
      } catch (error) {
        setErrors("Metadata 需为合法 JSON 格式");
        return;
      }
    }
    setErrors(null);
    await onSubmit(payload);
  };

  return (
    <form className="card" onSubmit={handleSubmit}>
      <h2 className="section-title">发起一次智能体任务</h2>
      <p className="helper-text" style={{ marginTop: "-0.35rem", marginBottom: "1.35rem" }}>
        描述你的目标，可选择需要的链上操作，提交后系统会自动排队执行。
      </p>
      <div className="field-grid">
        <div className="input-field" style={{ gridColumn: "1 / -1" }}>
          <label htmlFor="goal">任务目标</label>
          <textarea
            id="goal"
            value={goal}
            onChange={(event) => {
              setGoal(event.target.value);
              setErrors(null);
            }}
            onChange={(event) => setGoal(event.target.value)}
            rows={3}
            placeholder="描述你希望 Agent 完成的操作"
            required
          />
          <span className="helper-text" style={{ display: "block", marginTop: "0.35rem" }}>
            示例：{exampleHint}
          </span>
        </div>
        <div className="input-field">
          <label htmlFor="chain_action">链上操作模板</label>
          <select
            id="chain_action"
            value={chainAction}
            onChange={(event) => {
              setChainAction(event.target.value);
              setErrors(null);
            }}
            onChange={(event) => setChainAction(event.target.value)}
          >
            {CHAIN_TEMPLATES.map((option) => (
              <option key={option.value || "none"} value={option.value}>
                {option.label}
              </option>
            ))}
          </select>
          <span className="helper-text">选择需要的 JSON-RPC 方法，可在后续版本扩展。</span>
        </div>
        <div className="input-field">
          <label htmlFor="address">相关地址（可选）</label>
          <input
            id="address"
            value={address}
            onChange={(event) => {
              setAddress(event.target.value);
              setErrors(null);
            }}
            onChange={(event) => setAddress(event.target.value)}
            placeholder="0x..."
          />
          <span className="helper-text">某些链上操作需要提供地址或合约。</span>
        </div>
        <div className="input-field" style={{ gridColumn: "1 / -1" }}>
          <label htmlFor="metadata">附加 Metadata（可选，JSON 格式）</label>
          <textarea
            id="metadata"
            value={metadata}
            onChange={(event) => setMetadata(event.target.value)}
            rows={4}
            placeholder='{"project": "demo", "owner": "alice"}'
          />
          <span className="helper-text">{metadataPreview}</span>
        </div>
      </div>
      {errors ? (
        <p className="helper-text" style={{ color: "#fca5a5" }}>
          {errors}
        </p>
      ) : null}
      <div className="actions" style={{ marginTop: "1.5rem" }}>
        <button type="submit" className="primary" disabled={submitDisabled}>
          {loading ? "提交中..." : "提交任务"}
        </button>
      </div>
    </form>
  );
}
