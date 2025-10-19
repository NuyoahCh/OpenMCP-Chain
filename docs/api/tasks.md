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
  "status": "pending",
  "attempts": 0,
  "max_retries": 3,
  "goal": "查询账户余额",
  "chain_action": "eth_getBalance",
  "address": "0x0000000000000000000000000000000000000000",
  "metadata": { "project": "demo" },
  "created_at": 1714564800,
  "updated_at": 1714564800
}
```

字段含义：

- `task_id`：内部生成的任务唯一标识，可用于后续检索。
- `status` / `attempts` / `max_retries`：队列状态与剩余重试次数，便于前端轮询。
- `goal` / `chain_action` / `address` / `metadata`：原始请求信息的回显。
- `created_at` / `updated_at`：UNIX 时间戳（秒），表示任务创建与最近更新的时间。

> 智能体执行是异步的，`thought`/`reply` 等详细结果可通过轮询 `GET /api/v1/tasks?id=...` 获取。

### 失败响应

| HTTP 状态码 | 场景 | 响应示例 |
| --- | --- | --- |
| `400` | 请求字段缺失或格式错误 | `{ "code": "INVALID_ARGUMENT", "message": "请求体解析失败" }` |
| `401` | 身份认证失败 | `{ "code": "INVALID_CREDENTIALS", "message": "认证失败" }` |
| `502` | 推理桥接或队列推送失败 | `{ "code": "TASK_PUBLISH_FAILED", "message": "发布任务到队列失败" }` |
| `503` | 服务初始化失败 | `{ "code": "TASK_SERVICE_UNINITIALIZED", "message": "任务服务未初始化" }` |

> 失败时系统仍会记录任务上下文，便于排查问题。

## 查询任务列表 `GET /api/v1/tasks`

按更新时间倒序返回历史任务记录，默认限制为 20 条。支持以下查询参数：

| 查询参数 | 描述 |
| --- | --- |
| `limit` | 可选，限制返回条数，范围 1–100。 |
| `status` | 可选，过滤指定状态，可多次传入或使用逗号分隔，例如 `status=pending,failed`。 |
| `since` / `until` | 可选，RFC3339 时间戳，过滤在指定时间区间内更新的任务。 |
| `has_result` | 可选，布尔值，是否仅返回已有执行结果的任务。 |
| `order` | 可选，`asc` 或 `desc`，控制更新时间排序方向，默认 `desc`。 |

```bash
curl "http://127.0.0.1:8080/api/v1/tasks?limit=5&status=succeeded&has_result=true"
```

典型响应：

```json
[
  {
    "id": "20240501-0001",
    "goal": "查询账户余额",
    "chain_action": "eth_getBalance",
    "address": "0x0000000000000000000000000000000000000000",
    "status": "succeeded",
    "attempts": 1,
    "max_retries": 3,
    "result": {
      "thought": "当前目标: 查询账户余额",
      "reply": "该地址余额为 ...",
      "chain_id": "0x1",
      "block_number": "0xabcdef",
      "observations": "eth_getBalance 返回: 0x0234c8a3397aab58"
    },
    "created_at": 1714564800,
    "updated_at": 1714564820
  }
]
```

响应中的字段与任务实体保持一致，便于在调试工具或审计平台中直接复用。若需要查询单条记录，可使用 `GET /api/v1/tasks?id=<task_id>`。

## 审计与追踪建议

- 将响应中的 `task_id` 作为主索引，关联日志、数据库记录以及外部分析平台。
- 若启用了 MySQL 存储，可根据 `task_id` 与 `created_at` 创建复合索引，以加速查询。
- 建议对敏感链上操作（如资产转移）扩展额外的审批流程，并在 `metadata` 字段写入审核人信息。
