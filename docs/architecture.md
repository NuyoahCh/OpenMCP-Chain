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
  lifecycles, configuration profiles, and blockchain receipts.
* **Redis** utilities deliver low-latency caching, distributed locks, and task
  queues for asynchronous workloads.
* Common interfaces allow services to decouple from storage-specific details and
  simplify testing via mocks.

### API Gateway (`internal/api`)

* Hosts REST and gRPC services for controlling agents, retrieving logs, and
  querying audit trails.
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
* **Storage Layer**：在缺乏外部依赖的环境下以本地文件模拟 MySQL，接口保持一致，方便切换到真实数据库。
* **API Gateway**：暴露 `/api/v1/tasks` REST 接口，用于触发一次完整的“推理 + 链上探测”流程。

## Go + Python 协同模式

当前版本通过 `scripts/llm_bridge.py` 演示如何在守护进程内部调用 Python 逻辑：

1. Go 端将任务参数序列化为 JSON，通过管道写入脚本标准输入。
2. Python 端解析参数，生成“思考过程”和“最终回复”两段文本。
3. Go 端读取 JSON 输出，并合并链上快照、任务元数据后返回给调用方。

这种模式既能利用 Go 的并发和工程化能力，也能灵活接入 Python 生态的模型推理、数据处理库。后续可逐步替换为真正的大模型接口、向量数据库检索等高级功能。
