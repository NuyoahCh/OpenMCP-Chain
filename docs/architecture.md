# OpenMCP-Chain Architecture

OpenMCP-Chain is designed as a modular protocol stack that connects large model
agents with blockchain infrastructure in a verifiable manner. The architecture
is split into clearly defined domains to enable independent evolution, secure
execution, and rigorous observability.

## Core Principles

1. **Agent-Centric Design** – The system revolves around autonomous agents that
   make decisions based on natural-language instructions while adhering to
   deterministic policy constraints.
2. **Security and Provenance** – Every agent action produces auditable
   artifacts, including signed inference results and on-chain transaction
   receipts.
3. **Extensibility** – Modules are designed with interfaces and plug-in points
   that accommodate new LLM providers, blockchain networks, and storage
   backends.
4. **Operational Resilience** – Configuration management, observability, and
   deployment tooling emphasize reliability across heterogeneous environments.

## High-Level Components

### Agent Runtime (`internal/agent`)

* Implements the agent state machine, including planning, tool selection, and
  execution loops.
* Integrates with the LLM module for inference and with the Web3 module for
  blockchain interactions.
* Enforces policy checks and safety constraints defined by administrators.

### Large Language Model Adapters (`internal/llm`)

* Provides a normalized interface for invoking third-party LLM providers (e.g.,
  OpenAI, local inference backends).
* Manages prompt templates, token accounting, caching hints, and streaming
  responses.
* Enables future support for decentralized inference marketplaces.

### Web3 Interaction Layer (`internal/web3`)

* Abstracts RPC interactions, transaction crafting, signing, and submission for
  supported blockchain networks.
* Maintains wallet integrations, nonce management, and event subscriptions.
* Facilitates standard workflows for smart-contract reads and writes with
  deterministic error handling.

### Proofs and Attestations (`internal/proofs`)

* Captures cryptographic signatures for agent outputs and blockchain
  transactions.
* Provides hashing utilities for anchoring metadata to on-chain or decentralized
  storage networks.
* Lays the groundwork for integrating zero-knowledge proofs or trusted execution
  environments.

### Storage Layer (`internal/storage`)

* **MySQL** repositories persist durable data such as agent sessions, task
  lifecycles, configuration profiles, and blockchain receipts。开发阶段默认启用基于文件的内存实现，生产环境可在构建时附加 `mysql` build tag 以加载真正的驱动。
* **Redis** utilities deliver low-latency caching, distributed locks, and task
  queues for asynchronous workloads。
* Common interfaces allow services to decouple from storage-specific details and
  simplify testing via mocks。

### API Gateway (`internal/api`)

* Hosts REST and gRPC services for controlling agents, retrieving logs, and
  querying audit trails。当前 REST 网关已支持任务提交与历史查询。
* Integrates authentication and authorization layers to guard access to
  sensitive capabilities.
* Emits metrics, tracing spans, and structured logs for every request.

### SDK (`pkg/sdk`)

* Offers typed client libraries for external systems to communicate with the
  OpenMCP APIs.
* Encapsulates authentication flows, retries, and error modeling for consistent
  consumer experience.

### Deployment Tooling (`deploy/` and `scripts/`)

* Dockerfiles, Helm charts, and Terraform modules streamline local development
  and production deployments.
* Automation scripts handle tasks such as database migrations, code generation,
  and environment bootstrapping.

## Runtime Flow

1. An external client submits a task via the API gateway.
2. The agent runtime validates policies, prepares context, and requests
   inference from an LLM provider.
3. The agent translates the inference result into actionable steps and may
   invoke blockchain tools via the Web3 layer.
4. Transactions are signed, submitted, and tracked; receipts and metadata are
   persisted to MySQL while Redis maintains transient state.
5. The proofs module signs the final outcome, anchoring evidence on-chain when
   required.
6. The API gateway returns structured responses, including provenance details,
   to the caller.

## Observability and Operations

* **Logging** – Structured, leveled logging with correlation IDs across
  components.
* **Metrics** – Prometheus-compatible metrics capture latency, throughput, error
  rates, and resource utilization.
* **Tracing** – OpenTelemetry instrumentation links API requests to LLM and
  blockchain operations for holistic visibility.

## Security Considerations

* Secrets management integrates with external vaults to protect API keys and
  private keys.
* Policy enforcement guards against unsafe contract interactions or budget
  overruns.
* Audit trails maintain tamper-evident records of all agent actions.

This architecture provides a foundation for iterative development while keeping
security, extensibility, and reliability at the forefront.


## 当前实现进度概览

* **Agent Runtime**：已提供最小可用实现，能够调用 Python Bridge 推理脚本并记录任务结果。
* **Web3 Interaction**：封装基础的 JSON-RPC 调用，支持查询链 ID 与区块高度，方便后续扩展交易提交能力。
* **Storage Layer**：在缺乏外部依赖的环境下以本地文件模拟 MySQL，并提供可选的 `SQLTaskRepository`，通过 `-tags mysql` 即可接入真实数据库。
* **API Gateway**：暴露 `/api/v1/tasks` REST 接口，支持触发一次完整的“推理 + 链上探测”流程，并新增历史查询能力。

## Go + Python 协同模式

当前版本通过 `scripts/llm_bridge.py` 演示如何在守护进程内部调用 Python 逻辑：

1. Go 端将任务参数序列化为 JSON，通过管道写入脚本标准输入。
2. Python 端解析参数，生成“思考过程”和“最终回复”两段文本。
3. Go 端读取 JSON 输出，并合并链上快照、任务元数据后返回给调用方。

这种模式既能利用 Go 的并发和工程化能力，也能灵活接入 Python 生态的模型推理、数据处理库。后续可逐步替换为真正的大模型接口、向量数据库检索等高级功能。
# OpenMCP-Chain 架构

OpenMCP-Chain 被设计为一个模块化的协议栈，用于以可验证的方式连接大模型智能体与区块链基础设施。架构被划分为明确的领域，以实现独立演进、安全执行和严格的可观测性。

## 核心原则

1. **以智能体为中心的设计** – 系统围绕自主智能体运行，这些智能体基于自然语言指令做出决策，同时遵守确定性策略约束。
2. **安全性与来源可追溯性** – 每个智能体操作都会生成可审计的工件，包括签名的推理结果和链上交易回执。
3. **可扩展性** – 模块通过接口和插件点设计，能够适应新的 LLM 提供商、区块链网络和存储后端。
4. **操作弹性** – 配置管理、可观测性和部署工具强调在异构环境中的可靠性。

## 高级组件

### 智能体运行时（`internal/agent`）

* 实现智能体状态机，包括规划、工具选择和执行循环。
* 集成 LLM 模块进行推理，并与 Web3 模块进行区块链交互。
* 强制执行管理员定义的策略检查和安全约束。

### 大语言模型适配器（`internal/llm`）

* 提供调用第三方 LLM 提供商（如 OpenAI、本地推理后端）的标准化接口。
* 管理提示模板、令牌计数、缓存提示和流式响应。
* 支持未来的去中心化推理市场。

### Web3 交互层（`internal/web3`）

* 抽象 RPC 交互、交易构建、签名和提交，支持的区块链网络。
* 维护钱包集成、nonce 管理和事件订阅。
* 为智能合约读写提供标准化工作流，并具有确定性错误处理。

### 证明与认证（`internal/proofs`）

* 捕获智能体输出和区块链交易的加密签名。
* 提供哈希工具，用于将元数据锚定到链上或去中心化存储网络。
* 为集成零知识证明或可信执行环境奠定基础。

### 存储层（`internal/storage`）

* **MySQL** 存储库持久化智能体会话、任务生命周期、配置文件和区块链回执等数据。
* **Redis** 工具提供低延迟缓存、分布式锁和异步工作负载的任务队列。
* 通用接口使服务与存储细节解耦，并通过模拟简化测试。

### API 网关（`internal/api`）

* 托管 REST 和 gRPC 服务，用于控制智能体、检索日志和查询审计记录。
* 集成身份验证和授权层，保护对敏感功能的访问。
* 为每个请求发出指标、跟踪跨度和结构化日志。

### SDK（`pkg/sdk`）

* 提供类型化的客户端库，供外部系统与 OpenMCP API 通信。
* 封装身份验证流程、重试和错误建模，确保一致的用户体验。

### 部署工具（`deploy/` 和 `scripts/`）

* Dockerfiles、Helm 图表和 Terraform 模块简化了本地开发和生产部署。
* 自动化脚本处理数据库迁移、代码生成和环境引导等任务。

## 运行时流程

1. 外部客户端通过 API 网关提交任务。
2. 智能体运行时验证策略、准备上下文，并向 LLM 提供商请求推理。
3. 智能体将推理结果转换为可执行步骤，并可能通过 Web3 层调用区块链工具。
4. 交易被签名、提交和跟踪；回执和元数据被持久化到 MySQL，而 Redis 维护瞬态状态。
5. 证明模块对最终结果签名，并在需要时将证据锚定到链上。
6. API 网关返回结构化响应，包括来源细节，给调用方。

## 可观测性与操作

* **日志记录** – 结构化、分级日志，跨组件使用相关 ID。
* **指标** – Prometheus 兼容的指标捕获延迟、吞吐量、错误率和资源利用率。
* **跟踪** – OpenTelemetry 仪表化将 API 请求与 LLM 和区块链操作链接起来，实现整体可见性。

## 安全性考量

* 密钥管理集成外部密钥库，保护 API 密钥和私钥。
* 策略强制执行，防止不安全的合约交互或预算超支。
* 审计记录维护所有智能体操作的防篡改记录。

此架构为迭代开发提供了基础，同时将安全性、可扩展性和可靠性置于首位。
