# 任务接口详解

## 创建任务 `POST /api/v1/tasks`

用于触发一次“智能体推理 + 链上观测”流程。请求体字段如下：

| 字段 | 类型 | 是否必填 | 描述 |
| --- | --- | --- | --- |
| `goal` | string | 是 | 任务目标，描述希望 Agent 完成的操作。 |
| `chain_action` | string | 否 | 指定需要调用的 JSON-RPC 方法，如 `eth_getBalance`。若缺省则仅返回推理结果。 |
| `address` | string | 否 | 与链上操作关联的地址，部分模板（如 `eth_getBalance`）需要。 |
| `metadata` | object | 否 | 自定义键值对，将原样写入任务记录，便于业务关联。 |

```bash
curl -X POST http://127.0.0.1:8080/api/v1/tasks \
  -H "Content-Type: application/json" \
  -d '{
        "goal": "查询账户余额",
        "chain_action": "eth_getBalance",
        "address": "0x0000000000000000000000000000000000000000"
      }'
```

### 成功响应

```json
{
  "task_id": "20240501-0001",
  "goal": "查询账户余额",
  "chain_action": "eth_getBalance",
  "address": "0x0000000000000000000000000000000000000000",
  "thought": "当前目标: 查询账户余额\n预期链上操作: eth_getBalance\n...",
  "reply": "该地址余额为 ...，建议在后续交易前...",
  "chain_id": "0x1",
  "block_number": "0xabcdef",
  "observations": "eth_getBalance 返回: 0x0234c8a3397aab58",
  "created_at": 1714564800
}
```

字段含义：

- `task_id`：内部生成的任务唯一标识，可用于后续检索。
- `thought` / `reply`：来自 Python Bridge 的思考过程与最终回复。
- `chain_id` / `block_number`：Web3 模块在调用成功时返回的网络标识与区块高度。
- `observations`：附加提示，包括链上调用结果、知识库命中项、异常信息等。
- `created_at`：UNIX 时间戳，单位秒。

### 失败响应

| HTTP 状态码 | 场景 | 响应示例 |
| --- | --- | --- |
| `400` | 请求字段缺失或格式错误 | `{ "code": "INVALID_ARGUMENT", "message": "goal is required" }` |
| `502` | Python Bridge 启动失败或无响应 | `{ "code": "LLM_BRIDGE_ERROR", "message": "bridge exited with status 1" }` |
| `503` | Web3 节点不可用 | `{ "code": "WEB3_UNAVAILABLE", "message": "rpc dial timeout" }` |

> 失败时系统仍会记录任务上下文，便于排查问题。

## 查询任务列表 `GET /api/v1/tasks`

按创建时间倒序返回历史任务记录，默认限制为 10 条。

| 查询参数 | 描述 |
| --- | --- |
| `limit` | 可选，限制返回条数。建议范围 1–50。 |

```bash
curl "http://127.0.0.1:8080/api/v1/tasks?limit=5"
```

典型响应：

```json
[
  {
    "task_id": "20240501-0001",
    "goal": "查询账户余额",
    "chain_action": "eth_getBalance",
    "address": "0x0000000000000000000000000000000000000000",
    "thought": "...",
    "reply": "...",
    "chain_id": "0x1",
    "block_number": "0xabcdef",
    "observations": "eth_getBalance 返回: 0x0234c8a3397aab58",
    "created_at": 1714564800
  }
]
```

响应中的字段与创建任务时保持一致，便于在调试工具或审计平台中直接复用。

## 审计与追踪建议

- 将响应中的 `task_id` 作为主索引，关联日志、数据库记录以及外部分析平台。
- 若启用了 MySQL 存储，可根据 `task_id` 与 `created_at` 创建复合索引，以加速查询。
- 建议对敏感链上操作（如资产转移）扩展额外的审批流程，并在 `metadata` 字段写入审核人信息。
