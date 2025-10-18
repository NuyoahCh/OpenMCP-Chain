# 部署与配置指南

本文档描述了在生产环境中启用 MySQL 持久化、执行数据库迁移以及调整连接池参数的步骤。

## 1. 准备数据库

1. 创建用于存储任务记录的 MySQL 实例，并为 OpenMCP 创建专用库，例如 `openmcp`。
2. 为服务账号授予最低限度的权限（`CREATE`, `ALTER`, `INSERT`, `SELECT`, `UPDATE`, `DELETE`）。
3. 记录连接串（DSN），格式示例：
   ```text
   user:password@tcp(mysql.example.com:3306)/openmcp?parseTime=true&charset=utf8mb4
   ```

## 2. 迁移管理

* 所有迁移脚本位于 [`deploy/migrations`](../deploy/migrations)。
* `openmcpd` 启动时会自动创建 `schema_migrations` 元数据表，并顺序执行尚未应用的脚本。
* 迁移执行采用事务保证原子性；失败时会回滚且不会记录版本号。
* 如需手工执行迁移，可在构建了 `mysql` 标签的环境中运行：
  ```bash
  go test ./internal/storage/mysql -run TestSQLTaskRepositoryRunMigrations
  ```
  该测试会验证迁移流程是否能在当前环境下执行。

## 3. 配置说明

在 `configs/openmcp.json` 中的 `storage.task_store` 节点新增了连接池参数：

```jsonc
"storage": {
  "task_store": {
    "driver": "mysql",
    "dsn": "user:password@tcp(mysql.example.com:3306)/openmcp",
    "max_open_conns": 30,
    "max_idle_conns": 15,
    "conn_max_lifetime_seconds": 1800,
    "conn_max_idle_time_seconds": 300
  }
}
```

* `max_open_conns`：最大并发连接数，默认 20。
* `max_idle_conns`：空闲连接池大小，默认 10。
* `conn_max_lifetime_seconds`：连接生命周期上限，默认 1800 秒。
* `conn_max_idle_time_seconds`：空闲连接存活时长，默认 0（不限）。

## 4. 运行服务

1. 确保配置文件已更新为上述参数，并将 `driver` 设置为 `mysql`。
2. 通过 `-tags mysql` 构建或运行服务：
   ```bash
   go build -tags mysql ./cmd/openmcpd
   ./openmcpd
   ```
3. 服务启动后会自动执行迁移并使用配置的连接池参数。

## 5. 验证

* 通过 `mysql` 客户端检查 `tasks` 表与 `schema_migrations` 表是否创建。
* 调用 API 触发任务执行，确认记录写入 `tasks`。
* 在日志中检查是否有连接失败或迁移错误。

## 6. 可观测性与审计

OpenMCP 默认开启结构化日志、审计日志轮转及核心指标导出，相关配置集中在 `observability` 节点：

```jsonc
"observability": {
  "logging": {
    "level": "info",
    "format": "json",
    "outputs": ["stdout"]
  },
  "metrics": {
    "enabled": true,
    "address": ""
  },
  "audit": {
    "enabled": true,
    "file": "../data/audit.log",
    "max_size_mb": 100,
    "max_backups": 7,
    "max_age_days": 30
  }
}
```

### 日志

* 应用日志遵循 JSON 结构化格式，字段包含时间戳、级别、消息及动态键值，便于在 Loki、Elasticsearch 等后端中检索。
* `audit.log` 记录任务入队、执行成功或失败等关键行为，默认保存在数据目录（`runtime.data_dir`）下，可通过 `max_size_mb / max_backups / max_age_days` 控制滚动策略。
* 若需同时输出到文件与标准输出，可在 `logging.outputs` 中追加路径，如 `"outputs": ["stdout", "../logs/openmcp.log"]`。

### 指标

* 指标采用 Prometheus 文本协议暴露，默认和 API 服务共用端口（`address` 为空时），路径为 `/metrics`。
* 若希望在独立端口监听，设置 `observability.metrics.address`（例如 `":9090"`），Prometheus 可直接抓取 `http://<host>:9090/metrics`。
* 已内置的指标包括：
  * `openmcp_http_requests_total{handler,method,code}`：请求量。
  * `openmcp_http_request_errors_total{handler,method}`：5xx 错误次数。
  * `openmcp_http_request_duration_seconds`：请求延迟直方图。
* 参考 Prometheus `scrape_config` 示例：

  ```yaml
  scrape_configs:
    - job_name: openmcp
      metrics_path: /metrics
      static_configs:
        - targets: ["openmcp.example.com:8080"]
  ```

### Grafana 仪表盘

1. 在 Grafana 中添加 Prometheus 数据源，指向上述抓取地址。
2. 创建仪表盘并使用以下查询：
   * 请求速率：`rate(openmcp_http_requests_total[5m])`
   * 错误率：`rate(openmcp_http_request_errors_total[5m])`
   * P95 延迟：`histogram_quantile(0.95, sum(rate(openmcp_http_request_duration_seconds_bucket[5m])) by (le))`
3. 可结合审计日志对异常任务进行追踪，实现闭环监控。
