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
