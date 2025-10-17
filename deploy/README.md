# Deployment Assets

The `deploy/` directory aggregates infrastructure-as-code and packaging assets
used to operate OpenMCP-Chain across environments.

## Structure

* `docker/` – Container build definitions for the daemon and supporting
  services.
* `helm/` – Kubernetes Helm charts for clustered deployments with configurable
  replicas, ingress, and secret management.
* `terraform/` – Infrastructure modules for provisioning cloud resources such as
  databases, message queues, and key management systems.

Each subdirectory includes environment-specific overrides and documentation.

## 快速启动开发依赖

若需要本地验证 MySQL 落库与区块链交互，可直接使用 `docker-compose`：

```bash
cd deploy/docker
docker compose -f docker-compose.dev.yml up -d
```

该组合会启动：

- **MySQL 8.0**：预置 `openmcp` 数据库及 `openmcp/openmcp` 用户，可直接与 `configs/openmcp.mysql.json` 搭配。首次启动时会自动初始化数据表。
- **Anvil**（Foundry）节点：提供以太坊兼容的本地链，监听 `8545` 端口。默认从公共 RPC fork 主网，可根据网络情况改为 `--fork-url none`。

关闭服务：

```bash
docker compose -f docker-compose.dev.yml down
```
