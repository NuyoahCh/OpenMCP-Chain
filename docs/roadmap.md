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
* 引入可配置的静态知识库，在推理时补充领域提示与安全建议。
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
