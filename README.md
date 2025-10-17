# OpenMCP-Chain

OpenMCP-Chain 是一个将区块链基础设施与大模型智能体深度融合的开源协议栈。项目提供可验证的代理执行环境，让 AI Agent 能够安全地调用 Web3 能力，并保留完整的审计与溯源数据。

## 核心特性

- **Golang + Python 协同**：通过内置的 Python Bridge 触发推理脚本，便于在 Go 服务中复用社区的模型能力。
- **Web3 快速接入**：内置 JSON-RPC 客户端，可直接查询链 ID、最新区块高度等关键指标。
- **可追踪任务日志**：默认使用本地文件模拟 MySQL 持久化，便于后续平滑切换至真实数据库。
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

## 贡献指南

欢迎提交 Issue 或 Pull Request，共同完善区块链与大模型融合的最佳实践。贡献代码时请确保：

- 保持 Go 代码通过 `go fmt`。
- 为新增模块补充文档或注释，最好采用中文描述业务意图。
- 如果引入外部依赖，请在文档中说明用途与替代方案。

## 许可协议

项目遵循 MIT License，详情请参阅 `LICENSE` 文件。
