# OpenMCP-Chain

OpenMCP-Chain 是一个将区块链基础设施与大模型智能体深度融合的开源协议栈。项目提供可验证的代理执行环境，让 AI Agent 能够安全地调用 Web3 能力，并保留完整的审计与溯源数据。

## 核心特性

- **Golang + Python 协同**：通过内置的 Python Bridge 触发推理脚本，便于在 Go 服务中复用社区的模型能力。
- **Web3 快速接入**：内置 JSON-RPC 客户端，可直接查询链 ID、最新区块高度等关键指标，并支持执行常见读操作（如 `eth_getBalance`、`eth_getTransactionCount`）。
- **可追踪任务日志**：默认使用本地文件模拟 MySQL 持久化，同时提供真实 MySQL 仓库实现，满足生产环境的数据一致性需求。
- **任务历史查询**：REST API 除了支持触发智能体执行外，还可以拉取最近的推理记录，便于联调与审计。
- **上下文记忆驱动的推理**：Agent 会在推理前自动装载最近的任务历史，把经验注入 Prompt，提升回答的连续性与可解释性。
- **Web3 快速接入**：内置 JSON-RPC 客户端，可直接查询链 ID、最新区块高度等关键指标。
- **可追踪任务日志**：默认使用本地文件模拟 MySQL 持久化，同时提供真实 MySQL 仓库实现，满足生产环境的数据一致性需求。
- **任务历史查询**：REST API 除了支持触发智能体执行外，还可以拉取最近的推理记录，便于联调与审计。
- **可扩展架构**：配置、存储、Agent、API、Web3 等模块均采用接口抽象，支持按需替换实现。

## 代码结构

```
cmd/openmcpd/        # 守护进程入口，负责初始化所有子系统
configs/             # 默认配置文件示例
internal/
  agent/             # 智能体编排逻辑
  api/               # REST API 服务
  config/            # 配置解析与默认值填充
  llm/               # 大模型接口与 Python Bridge 实现
  storage/mysql/     # 任务落库接口及内存实现
  web3/ethereum/     # 基于 JSON-RPC 的以太坊客户端
scripts/             # Python 推理脚本、自动化工具
```

更多架构细节可在 `docs/` 目录中查看。

## 快速体验

1. 安装 Go 1.22 以及 Python 3 环境。
2. 执行 `go build ./...` 确认依赖齐全。
3. 运行守护进程：
   ```bash
   OPENMCP_CONFIG=$(pwd)/configs/openmcp.json go run ./cmd/openmcpd
   ```
4. 另开一个终端，通过 REST API 提交任务：
   ```bash
   curl -X POST http://127.0.0.1:8080/api/v1/tasks \
     -H 'Content-Type: application/json' \
     -d '{"goal":"查询账户余额","chain_action":"eth_getBalance","address":"0x0000000000000000000000000000000000000000"}'
   ```
   服务会调用 Python 脚本生成思考与回复，并尝试从配置的以太坊节点获取链上快照。

默认配置会在 `data/tasks.log` 中记录每次任务执行的摘要，便于调试和后续迁移至 MySQL。

5. 查询最新的任务执行历史：
   ```bash
   curl "http://127.0.0.1:8080/api/v1/tasks?limit=5"
   ```

   或使用内置的 Python 客户端脚本：

   ```bash
   python scripts/task_client.py invoke --goal "查询账户余额" --chain-action eth_getBalance --address 0x0000000000000000000000000000000000000000
   python scripts/task_client.py history --limit 5
   ```

### 自定义智能体记忆深度

`agent.memory_depth` 用于控制在调用 Python Bridge 前装载多少条历史任务。默认值为 5，可根据业务需求调整：

```json
{
  "agent": {
    "memory_depth": 10
  }
}
```

当历史记录不足时，Agent 会自动忽略缺失项；如果加载历史失败，相应提示会出现在任务的 `observations` 字段中。

### 启用 MySQL 持久化

当你准备好连接真实的 MySQL 时，可在配置文件中将 `storage.task_store.driver` 设置为 `mysql`，并补充 `dsn`。随后以 `-tags mysql` 构建或运行 `openmcpd`，即可启用内置的 SQL 仓库：

```bash
OPENMCP_CONFIG=$(pwd)/configs/openmcp.mysql.json go run -tags mysql ./cmd/openmcpd
```

`configs/openmcp.mysql.json` 为示例配置，包含了最小可用的 MySQL 连接参数。守护进程会在启动时自动初始化 `tasks` 数据表，并在写入失败时返回清晰的错误信息。

> 提示：构建时需要从官方源下载 `github.com/go-sql-driver/mysql`，请确保环境可以访问外网或提前在内部镜像中缓存依赖。

## 贡献指南

欢迎提交 Issue 或 Pull Request，共同完善区块链与大模型融合的最佳实践。贡献代码时请确保：

- 保持 Go 代码通过 `go fmt`。
- 为新增模块补充文档或注释，最好采用中文描述业务意图。
- 如果引入外部依赖，请在文档中说明用途与替代方案。

## 许可协议

项目遵循 MIT License，详情请参阅 `LICENSE` 文件。
