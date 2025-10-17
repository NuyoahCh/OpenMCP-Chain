# OpenMCP-Chain Roadmap

The roadmap outlines incremental milestones for building a production-ready
platform. Each phase can be split into smaller GitHub issues to streamline
execution.

## Phase 0 – Foundation

* Bootstrap Go module, CI pipeline, and linting/testing workflows.
* Implement configuration loader with environment file support.
* Provide local development environment via Docker Compose (MySQL, Redis, test
  blockchain).
* Draft contribution guidelines and coding standards.

## Phase 1 – Minimum Viable Agent

* Implement core agent loop capable of:
  * receiving tasks via REST endpoint,
  * invoking a single LLM provider for reasoning,
  * executing a predefined smart-contract interaction (read/write).
* Persist agent session data, prompts, and blockchain receipts to MySQL.
* 提供任务历史查询接口与命令行辅助工具，方便联调与验收。
* Add Redis-backed cache for LLM responses and rate limiting.
* Produce structured logs and basic Prometheus metrics.

## Phase 2 – Multi-Chain & Governance

* Abstract wallet management and transaction signing (hardware wallet, custodial
  integrations).
* Support multiple EVM networks with chain-specific configuration.
* Introduce policy engine for transaction approvals (value thresholds, contract
  allowlists, gas limits).
* Expand SDK with strongly typed clients and command-line tooling.

## Phase 3 – Observability & UX

* Implement OpenTelemetry tracing across API, agent, and Web3 calls.
* Build web dashboard for monitoring agent activity and reviewing audit trails.
* Automate database migrations and provide blue/green deployment scripts.
* Harden security with role-based access control (RBAC) and secrets management
  integrations.

## Phase 4 – Verifiable Inference

* Integrate trusted execution environments (TEE) or zero-knowledge proofs for
  inference provenance.
* Support decentralized inference providers (e.g., Bittensor, Gensyn) via pluggable
  adapters.
* Enable workflow orchestration with DAG-style task definitions and multi-agent
  collaboration.

## Phase 5 – Ecosystem Expansion

* Launch plugin marketplace for specialized Web3 tooling (DEX trading, NFT
  operations, oracle data).
* Release comprehensive documentation, tutorials, and sample projects.
* Foster community contributions through issue triage, public roadmaps, and
  regular release notes.

This roadmap will evolve based on community feedback and technological
advancements. Each phase should be accompanied by measurable success criteria
and automated testing to ensure regressions are avoided.
# OpenMCP-Chain 路线图

该路线图概述了构建生产就绪平台的渐进式里程碑。每个阶段可以进一步拆分为更小的 GitHub 问题，以简化执行。

## 阶段 0 – 基础

* 初始化 Go 模块、CI 流水线以及代码检查/测试工作流。
* 实现支持环境文件的配置加载器。
* 提供基于 Docker Compose 的本地开发环境（包括 MySQL、Redis、测试区块链）。
* 起草贡献指南和编码标准。

## 阶段 1 – 最小可行智能体

* 实现核心智能体循环，具备以下功能：
  * 通过 REST 接口接收任务，
  * 调用单一 LLM 提供商进行推理，
  * 执行预定义的智能合约交互（读/写）。
* 将智能体会话数据、提示和区块链回执持久化到 MySQL。
* 添加基于 Redis 的缓存，用于存储 LLM 响应和速率限制。
* 生成结构化日志和基本的 Prometheus 指标。

## 阶段 2 – 多链支持与治理

* 抽象钱包管理和交易签名（支持硬件钱包和托管集成）。
* 支持多个 EVM 网络，并提供链特定的配置。
* 引入策略引擎，用于交易审批（如价值阈值、合约白名单、Gas 限制）。
* 扩展 SDK，提供强类型客户端和命令行工具。

## 阶段 3 – 可观测性与用户体验

* 在 API、智能体和 Web3 调用中实现 OpenTelemetry 跟踪。
* 构建 Web 仪表盘，用于监控智能体活动和审计记录。
* 自动化数据库迁移，并提供蓝/绿部署脚本。
* 加强安全性，集成基于角色的访问控制（RBAC）和密钥管理。

## 阶段 4 – 可验证推理

* 集成可信执行环境（TEE）或零知识证明，用于推理来源验证。
* 通过可插拔适配器支持去中心化推理提供商（如 Bittensor、Gensyn）。
* 支持使用 DAG 风格的任务定义和多智能体协作的工作流编排。

## 阶段 5 – 生态系统扩展

* 推出用于专用 Web3 工具（如 DEX 交易、NFT 操作、预言机数据）的插件市场。
* 发布全面的文档、教程和示例项目。
* 通过问题分类、公开路线图和定期发布说明促进社区贡献。

该路线图将根据社区反馈和技术进步不断演进。每个阶段都应伴随可衡量的成功标准和自动化测试，以确保避免回归。
