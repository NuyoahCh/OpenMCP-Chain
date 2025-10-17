# API Overview

当前版本提供一个最小可用的 REST 接口，便于验证“大模型推理 + 区块链观测”的联动流程。后续会逐步扩展为完整的任务编排、审计追踪等能力。

## 已实现的 REST 接口

| Method | Path | 描述 |
| --- | --- | --- |
| POST | `/api/v1/tasks` | 提交一次智能体任务，返回包含大模型思考、回复以及链上快照的结果。 |
| GET | `/api/v1/tasks` | 查询最近的任务执行记录，可通过 `limit` 参数控制返回数量。 |

### 请求示例

```http
POST /api/v1/tasks HTTP/1.1
Host: localhost:8080
Content-Type: application/json

{
  "goal": "查询账户余额",
  "chain_action": "eth_getBalance",
  "address": "0x0000000000000000000000000000000000000000"
}
```

### 响应示例

```json
{
  "goal": "查询账户余额",
  "chain_action": "eth_getBalance",
  "address": "0x0000000000000000000000000000000000000000",
  "thought": "当前目标: 查询账户余额\n预期链上操作: eth_getBalance\n涉及地址: 0x0000000000000000000000000000000000000000\n时间戳: 2024-05-01 12:00:00 UTC",
  "reply": "我已经理解你的目标『查询账户余额』。下一步可以按照『eth_getBalance』在链上执行，并保持地址 0x0000000000000000000000000000000000000000 的安全。",
  "chain_id": "0x1",
  "block_number": "0xabcdef",
  "observations": "",
  "created_at": 1714564800
}
```

### 历史查询示例

```http
GET /api/v1/tasks?limit=10 HTTP/1.1
Host: localhost:8080
Accept: application/json
```

响应为任务数组，按时间倒序排列：

```json
[
  {
    "goal": "查询账户余额",
    "chain_action": "eth_getBalance",
    "address": "0x0000000000000000000000000000000000000000",
    "thought": "...",
    "reply": "...",
    "chain_id": "0x1",
    "block_number": "0xabcdef",
    "observations": "",
    "created_at": 1714564800
  }
]
```

> 说明：链上信息依赖配置的 RPC 节点。当节点不可达时，`chain_id`、`block_number` 会为空，并在 `observations` 字段给出错误提示。

## 规划中的扩展

* 任务状态轮询、事件流订阅。
* Agent 多实例调度与配额控制。
* 审计日志与可验证证据的导出接口。

完整的 API 契约（OpenAPI/Protobuf）将在功能稳定后发布。
