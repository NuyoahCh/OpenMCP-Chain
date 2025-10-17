# 部署资源

`deploy/` 目录汇总了用于在不同环境中操作 OpenMCP-Chain 的基础设施即代码（Infrastructure-as-Code）和打包资源。

## 目录结构

* `docker/` – 守护进程及支持服务的容器构建定义。
* `helm/` – Kubernetes 的 Helm 图表，用于集群部署，支持配置副本数、入口（Ingress）和密钥管理。
* `terraform/` – 用于配置云资源（如数据库、消息队列和密钥管理系统）的基础设施模块。

每个子目录都包含特定环境的覆盖配置和文档。