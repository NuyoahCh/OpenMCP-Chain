# OpenMCP-Chain

OpenMCP-Chain 是一个将区块链基础设施与大模型智能体深度融合的开源协议栈。项目提供可验证的代理执行环境，让 AI Agent 能够安全地调用 Web3 能力，并保留完整的审计与溯源数据。

## 目录

- [核心特性](#核心特性)
- [架构与文档](#架构与文档)
- [快速入门](#快速入门)
- [前端控制台](#前端控制台)
- [API 速览](#api-速览)
- [示例脚本](#示例脚本)
- [常见问题](#常见问题)
- [贡献指南](#贡献指南)
- [许可协议](#许可协议)

## 核心特性

- **Golang + Python 协同**：通过内置的 Python Bridge 触发推理脚本，便于在 Go 服务中复用模型能力。
- **Web3 快速接入**：内置 JSON-RPC 客户端，可查询链 ID、最新区块高度等指标，并支持 `eth_getBalance`、`eth_getTransactionCount` 等读操作。
- **可追踪任务日志**：默认使用本地文件模拟 MySQL 持久化，同时提供真实 MySQL 仓库实现，满足生产环境的数据一致性需求。
- **上下文记忆驱动的推理**：Agent 在推理前自动装载最近的任务历史，把经验注入 Prompt，提升回答连续性。
- **知识库增强**：通过静态知识卡片在推理时补充领域经验，让回复同时参考链上最佳实践与安全提示。
- **可扩展架构**：配置、存储、Agent、API、Web3 等模块均采用接口抽象，支持按需替换实现。

## 架构与文档

- 系统设计与流程图参见 [`docs/architecture.md`](docs/architecture.md)。
- API 契约说明位于 [`docs/api/`](docs/api/)。
- 部署建议与演进规划参见 [`docs/deployment.md`](docs/deployment.md) 与 [`docs/roadmap.md`](docs/roadmap.md)。
- 安全测试与渗透测试流程见 [`docs/security/SECURITY_TESTING.md`](docs/security/SECURITY_TESTING.md)。
- 数据指标、仓库建模与报表方案见 [`docs/analytics/data_warehouse_and_reporting.md`](docs/analytics/data_warehouse_and_reporting.md)。
- 文档校对流程请查阅 [`docs/documentation-review.md`](docs/documentation-review.md)。

## 快速入门

### 环境准备

1. 安装 **Go 1.22+** 与 **Python 3.9+**。
2. （可选）准备可访问的以太坊 JSON-RPC 节点。

### 构建与运行

```bash
# 拉取依赖并编译
go build ./...

# 启动守护进程（默认配置位于 configs/openmcp.json）
OPENMCP_CONFIG=$(pwd)/configs/openmcp.json go run ./cmd/openmcpd
```

服务启动后将监听 `http://127.0.0.1:8080`，并在 `data/tasks.log` 写入执行摘要。

### 触发一次任务

可直接使用 `curl`，或运行示例脚本：

```bash
# 使用示例脚本提交任务
python examples/task_quickstart.py invoke \
  --goal "查询账户余额" \
  --chain-action eth_getBalance \
  --address 0x0000000000000000000000000000000000000000

# 查看最近的 5 条任务历史
python examples/task_quickstart.py history --limit 5
```

若未配置真实 RPC 节点，链上数据字段会为空，错误信息会写入 `observations`。

## API 速览

当前版本实现以下 REST 接口，详见 [`docs/api/tasks.md`](docs/api/tasks.md)：

| Method | Path | 描述 |
| --- | --- | --- |
| `POST` | `/api/v1/tasks` | 提交一次智能体任务，返回推理输出、链上快照与审计信息。 |
| `GET` | `/api/v1/tasks` | 查询最近的任务执行记录，支持 `limit` 参数。 |

## 示例脚本

- `examples/task_quickstart.py`：命令行客户端，演示如何调用 REST API 并解析响应。
- `examples/README.md`：记录示例依赖与运行方式，便于扩展更多脚本或 Notebook。

欢迎补充新的示例，并在 PR 中引用相关文档章节。

## 前端控制台

项目提供一个基于 React + Vite 的轻量级前端，帮助快速连调 `/api/v1/tasks` 接口。源码位于 [`web/`](web/README.md)，支持：

- 图形化填写任务目标、链上操作与链上地址；
- 实时轮询任务状态，并展示模型思考、回复与链上观测结果；
- 可通过 `VITE_API_BASE_URL` 环境变量自定义后端地址。

启动方式：

```bash
cd web
npm install
npm run dev
```

默认会连接本地的 `http://127.0.0.1:8080` 服务，可结合 `OPENMCP_CONFIG=$(pwd)/configs/openmcp.json go run ./cmd/openmcpd` 快速体验端到端流程。

## 常见问题

**Q: 需要同时安装 Go 和 Python 吗？**<br>
是的。守护进程由 Go 实现，推理逻辑通过 Python Bridge 执行。若缺少 Python，`POST /api/v1/tasks` 将返回桥接失败错误。

**Q: 未连接真实 RPC 节点会怎样？**<br>
系统仍会完成推理，但 `chain_id`、`block_number` 等字段为空，并在 `observations` 中提示 RPC 错误。可通过 `configs/openmcp.json` 修改 RPC 端点。

**Q: 如何接入 MySQL？**<br>
在配置中将 `storage.task_store.driver` 设置为 `mysql` 并提供 `dsn`，随后以 `-tags mysql` 运行 `openmcpd`。示例配置见 `configs/openmcp.mysql.json`。

**Q: 文档如何保持最新？**<br>
请参考 [`docs/documentation-review.md`](docs/documentation-review.md) 中的校对流程，在 PR 中勾选受影响的文档并安排同伴审阅。

## 贡献指南

欢迎提交 Issue 或 Pull Request，共同完善区块链与大模型融合的最佳实践。贡献代码时请确保：

- 通过 `go fmt`、`go test` 等基础校验；
- 为新增模块补充文档或注释；
- 遵循[文档校对流程](docs/documentation-review.md)，在 PR 中注明已更新的章节。

## 许可协议

项目遵循 MIT License，详情请参阅 [`LICENSE`](LICENSE) 文件。
