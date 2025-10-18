# OpenMCP-Chain Helm Chart

该 Helm Chart 将 OpenMCP-Chain 以容器方式部署到 Kubernetes 集群，并整合 ConfigMap 与 Secret 来管理配置与敏感信息。

## 快速开始

```bash
helm repo add openmcp-chain https://example.com/charts # 示例
helm upgrade --install openmcp-chain deploy/k8s \
  --set secrets.openai.value="$OPENAI_API_KEY" \
  --set secrets.mysql.value="$MYSQL_PASSWORD" \
  --namespace openmcp --create-namespace
```

> 如果已经在集群中托管了 Secret，可通过 `secrets.openai.existingSecret`、`secrets.mysql.existingSecret` 指定现有资源并省略 `value`。

## 主要参数

| 参数 | 描述 | 默认值 |
| --- | --- | --- |
| `replicaCount` | Pod 副本数 | `1` |
| `image.repository` | 镜像仓库 | `ghcr.io/openmcp/openmcp-chain` |
| `config.mysql.host` | MySQL 服务地址 | `mysql.default.svc.cluster.local` |
| `config.mysql.database` | MySQL 数据库名 | `openmcp` |
| `config.openai.baseURL` | OpenAI 兼容服务地址 | `https://api.openai.com/v1` |
| `config.dataDir` | 数据持久化路径 | `/var/lib/openmcp` |
| `persistence.enabled` | 是否启用 PVC | `true` |
| `ingress.enabled` | 是否暴露 Ingress | `false` |

更多参数可参考 `values.yaml`。
