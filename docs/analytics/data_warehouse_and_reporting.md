# OpenMCP-Chain 数据指标与仓库设计

本文档描述了在 OpenMCP-Chain 协议栈中搭建分析体系的整体方案，涵盖指标定义、数据仓库结构、聚合任务、可视化展现以及导出/分享与访问控制策略。

## 1. 指标体系与报表

| 指标类别 | 指标名称 | 描述 | 计算口径 | 数据来源 | 更新频率 | 主要使用方 |
| --- | --- | --- | --- | --- | --- | --- |
| 任务执行 | 任务吞吐量 | 单位时间内成功执行的智能体任务数 | `COUNT(task_id)` 过滤 `status = 'success'` | `task_runs` 事实表 | 批量：日；实时：1 分钟 | 运维、产品 |
| 任务执行 | 失败率 | 失败任务占比 | `FAILED / TOTAL` | `task_runs` 事实表 | 批量：日；实时：1 分钟 | 运维、SRE |
| 任务执行 | 平均响应时长 | 从提交到完成的平均时长 | `AVG(completed_at - created_at)` | `task_runs` 事实表 | 批量：日；实时：5 分钟 | SRE、产品 |
| 模型推理 | Python Bridge 调用耗时 | 桥接脚本运行的平均/分位时延 | `AVG/QUANTILE(latency)` | `bridge_invocations` 事实表 | 批量：日；实时：5 分钟 | AI 工程 |
| 链上交互 | RPC 成功率 | RPC 请求中成功返回的比例 | `SUCCESS / TOTAL` | `chain_requests` 事实表 | 批量：日；实时：1 分钟 | Web3 团队 |
| 链上交互 | 区块确认延迟 | 交易从提交到确认的平均延迟 | `AVG(confirmed_at - submitted_at)` | `chain_requests` 事实表 | 批量：日 | Web3 团队 |
| 审计与合规 | 审计事件数量 | 记录的审计事件总数 | `COUNT(event_id)` | `audit_events` 事实表 | 批量：日 | 合规、审计 |
| 知识库质量 | 知识卡片引用率 | 带有知识卡片引用的任务占比 | `COUNT(task_id with knowledge) / TOTAL` | `task_enrichments` 维度 | 批量：周 | 产品、知识管理 |
| 用户行为 | 顶层 API 调用量 | REST API 调用次数 | `COUNT(request_id)` | `api_requests` 事实表 | 批量：日；实时：1 分钟 | 运营、产品 |
| 用户行为 | 活跃租户数 | 指定周期内发起任务的租户数 | `COUNT(DISTINCT tenant_id)` | `task_runs` 事实表 | 批量：日/周/月 | 商业运营 |

推荐输出以下核心报表：

- **运维运行态报表（实时）**：包含任务吞吐量、失败率、RPC 成功率等关键健康指标，支持 1 分钟刷新。
- **产品增长报表（日报/周报）**：统计租户活跃度、API 调用量、任务成功率、知识卡片引用率。
- **合规审计报表（月报）**：聚焦审计事件数量、涉及链上交互的任务明细以及关键追踪字段。

## 2. 数据仓库架构设计

采用 **分层数据仓库** 设计，包含 Raw、Staging、Mart 三层。

1. **Raw Layer（原始层）**
   - 写入来源：Go 服务产生的事务型数据（任务日志、RPC 记录）、Prometheus 指标快照、外部链上数据。
   - 存储形式：对象存储（Parquet/JSON）或 Kafka 主题，保持原始结构以便回放。
   - 分区策略：按事件时间（`event_time`）与租户（`tenant_id`）进行分区，便于批处理作业切分。

2. **Staging Layer（清洗层）**
   - 处理内容：字段规范化（时间戳、枚举值）、脏数据过滤、与租户/Agent/链 ID 的维表关联。
   - 技术选型：利用 DBT/Trino 或 Spark SQL 在对象存储上执行转换，也可选择 ClickHouse 物化视图。
   - 输出表：`stg_task_runs`、`stg_chain_requests`、`stg_audit_events` 等半结构化表。

3. **Mart Layer（数据集市层）**
   - 建模方式：以任务为中心的星型模型。
   - 事实表：
     - `fact_task_runs(task_id, tenant_id, agent_id, status, created_at, completed_at, total_latency_ms, chain_tx_count, knowledge_card_count, error_code)`
     - `fact_chain_requests(request_id, task_id, rpc_method, status, latency_ms, block_number, gas_used, submitted_at, confirmed_at)`
     - `fact_bridge_invocations(invocation_id, task_id, python_module, latency_ms, exit_code, retries)`
     - `fact_api_requests(request_id, tenant_id, route, method, status_code, latency_ms, source_ip)`
   - 维度表：`dim_time`, `dim_tenant`, `dim_agent`, `dim_chain`, `dim_knowledge_card`, `dim_error`。
   - 衍生指标：在 Mart 层构建宽表（如 `mart_task_daily`），支撑仪表盘与报表导出。

仓库可以部署在 ClickHouse、BigQuery 或 Snowflake 等列式引擎上，保证大规模查询性能；对实时需求较高的指标可引入 Apache Pinot/Druid 构建实时集市。

## 3. 数据聚合任务

### 3.1 批处理任务

- **调度工具**：推荐使用 Apache Airflow 或 Prefect 管理批处理 DAG。
- **作业划分**：
  1. `ingest_raw_logs`: 每 5 分钟将 `data/tasks.log`、`observability` 指标推送到对象存储或 Kafka。
  2. `build_staging_tables`: 每小时运行，读取 Raw 数据，执行数据清洗与格式化。
  3. `compute_daily_marts`: 每日 00:15 运行，生成 `mart_task_daily`、`mart_chain_daily` 等宽表。
  4. `publish_reports`: 每日 00:30 将聚合结果写入报表库或推送到 BI 工具（Metabase/Looker）。
- **质量保障**：在 Airflow 中配置数据校验（行数、空值、分布），并结合现有的审计日志保证追踪。

### 3.2 实时流处理

- **数据通道**：
  - 使用 Kafka 作为实时事件总线，主题划分为 `tasks.events`, `rpc.events`, `audit.events`。
  - Go 服务通过异步协程把关键事件写入 Kafka（可复用 `internal/observability/metrics` 模块暴露的指标）。
- **计算引擎**：Flink/ksqlDB/Materialize 处理实时流，输出到 ClickHouse/Pinot 供仪表盘查询。
- **实时指标**：任务吞吐量、失败率、RPC 成功率、API 请求分布等，窗口粒度 1 分钟或 5 分钟。
- **回填策略**：实时流结果按小时落地到 Raw 层，与批处理结果对齐，确保一致性。

## 4. 指标展示与仪表盘

- **仪表盘选择**：优先选择开源方案，如 Grafana（对 Prometheus/ClickHouse 支持良好）或 Apache Superset（适合多数据源联动）。
- **模块集成方式**：
  - 若 OpenMCP-Chain 前端独立：在前端应用中嵌入 iframe 或使用 Grafana React 组件加载仪表盘。
  - 若仅提供管理后台：通过反向代理（如 Nginx）将 `/analytics` 路径代理到仪表盘服务，并加上统一登录。
- **关键看板**：
  1. **Runtime Health**：展示任务吞吐量、失败率、HTTP 请求延时直方图、链上 RPC 成功率。
  2. **Tenant Growth**：关注活跃租户、API 调用趋势、知识卡片引用率等增长指标。
  3. **Compliance Audit**：提供审计事件列表、链上交易详情、可疑错误码分布。
- **交互能力**：仪表盘需支持按租户、Agent、链 ID、时间区间筛选，并允许钻取到任务级明细。

## 5. 导出、分享与访问控制

- **导出能力**：
  - 在仪表盘中启用 CSV/Excel 导出；对于批量报表，可在 `publish_reports` 作业中生成标准化文件并存储到受控的对象存储路径。
  - 提供 API `GET /api/v1/reports/:id/export` 从数据集市读取结果，后台生成临时文件，并通过预签名 URL 或消息通知用户。
- **分享流程**：
  - 支持仪表盘链接分享，但必须绑定权限校验；可通过生成短期令牌（JWT）或邀请制访问控制列表（ACL）。
  - 对外分享采用脱敏视图，仅包含聚合指标，过滤掉 `tenant_id`、`address` 等敏感字段。
- **访问控制**：
  - 复用现有认证模块 `internal/auth`，实现角色（Admin/Ops/Product/Audit）与资源（仪表盘、报表、导出 API）之间的 RBAC 绑定。
  - 结合租户隔离：查询时自动加上 `tenant_id` 过滤，避免跨租户访问。
  - 对导出文件设置到期时间与水印，防止长期泄露。

## 6. 落地路线图

1. **里程碑 1：原始数据接入（1-2 周）**
   - 扩展 Go 服务日志与 Kafka 生产者，采集任务、RPC、审计事件。
   - 部署对象存储或 Kafka 集群，完成 Raw 层数据堆积。

2. **里程碑 2：批处理与数据集市（2-3 周）**
   - 建立 Airflow 管道与 DBT 模型，生成 Staging 与 Mart 表。
   - 构建首批日常报表，并验证数据准确性。

3. **里程碑 3：实时指标与仪表盘（2 周）**
   - 搭建 Flink/ksqlDB 流任务，填充实时指标。
   - 在 Grafana/Superset 中搭建核心看板，并接入统一登录。

4. **里程碑 4：导出分享与权限体系（1 周）**
   - 实现报表导出 API、预签名链接及水印机制。
   - 配置 RBAC 与租户隔离，完成安全审计。

该路线图可根据资源投入与优先级灵活调整，优先保障运维健康指标与基础权限体系上线。
