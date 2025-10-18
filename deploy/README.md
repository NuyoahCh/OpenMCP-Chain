# Deployment Assets

`deploy/` 目录汇总了用于在不同环境中运行 OpenMCP-Chain 的基础设施即代码（Infrastructure-as-Code）与打包资源。

## 目录结构

- `docker/` – Dockerfile、Compose 模板以及 Secrets 协调脚本。
- `k8s/` – Kubernetes Helm Chart，支持自定义副本数、Ingress 与密钥管理。
- `migrations/` – 数据库迁移脚本样例。

## Docker Compose

### 开发环境

用于快速验证链上交互，可直接启用本地依赖：

```bash
cd deploy/docker
docker compose -f docker-compose.dev.yml up -d
```

组合服务包括：

- **MySQL 8.0**：预置 `openmcp/openmcp` 账号，并自动初始化业务数据表。
- **Foundry Anvil**：启动以太坊兼容节点，监听 `8545` 端口。

停用服务：

```bash
docker compose -f docker-compose.dev.yml down
```

### 生产部署模板

`docker-compose.prod.yml` 提供了一套可与真实基础设施对接的样例：

1. 复制示例环境变量：
   ```bash
   cp deploy/docker/.env.example deploy/docker/.env
   ```
2. 在 `deploy/docker/secrets/` 目录内创建密钥文件：
   - `openai-api-key`：OpenAI 兼容 API Token。
   - `mysql-password`：MySQL 用户密码。
3. 启动服务：
   ```bash
   docker compose -f deploy/docker/docker-compose.prod.yml up -d
   ```

Compose 会通过下列方式整合配置：

- `.env` 注入通用环境变量（日志级别、模型名称等）。
- `configs/openmcp.mysql.json.tmpl` 作为模板挂载，由容器入口脚本基于环境变量与 Secret 渲染最终 `openmcp.json`。
- Docker Secrets 将 `OPENAI_API_KEY`、`MYSQL_PASSWORD` 等敏感信息以文件形式挂载，入口脚本负责导出环境变量。

## Kubernetes（Helm Chart）

`deploy/k8s` 目录提供了开箱即用的 Helm Chart：

```bash
helm upgrade --install openmcp-chain ./deploy/k8s \
  --namespace openmcp --create-namespace \
  --set secrets.openai.value="$OPENAI_API_KEY" \
  --set secrets.mysql.value="$MYSQL_PASSWORD"
```

Chart 主要能力：

- `ConfigMap`：下发配置模板并在容器启动时渲染为 `openmcp.json`。
- `Secret`：可选择内联密钥或复用集群既有 Secret。
- `PersistentVolumeClaim`：默认启用，用于持久化审计日志与任务数据。
- 可选 `Ingress`、`ServiceMonitor`，满足暴露 API 与 Prometheus 采集需求。

所有可配置项详见 `deploy/k8s/values.yaml` 与 `deploy/k8s/README.md`。

## 配置管理要点

- `OPENMCP_CONFIG_TEMPLATE` 指向 JSON 模板，容器启动时将其渲染到 `OPENMCP_CONFIG` 指定路径。
- `OPENAI_API_KEY_FILE`、`MYSQL_PASSWORD_FILE` 允许以文件方式注入敏感信息，避免出现在命令历史中。
- 通过环境变量可覆盖默认链路（如 `MYSQL_HOST`、`OPENAI_MODEL`、`LOG_LEVEL`）。

## CI/CD Pipeline 示例

`.github/workflows/deploy.yml` 演示了从源码到集群的自动化流程：

1. **build**：运行 `go test ./...`，使用 Buildx 构建镜像并推送到 GHCR。
2. **deploy**：读取 `KUBE_CONFIG`、`OPENAI_API_KEY`、`MYSQL_PASSWORD` 等机密，使用 Helm 将最新镜像发布至 Kubernetes。

该 Workflow 需在仓库 Secrets 中预先配置：

- `REGISTRY_TOKEN` – 推送镜像所需的访问凭据。
- `KUBE_CONFIG` – Base64 编码后的 kubeconfig。
- `OPENAI_API_KEY`、`MYSQL_PASSWORD` – 运行时依赖的外部服务密钥。

根据实际环境可进一步扩展审批、灰度发布等步骤。
