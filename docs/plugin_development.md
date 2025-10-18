# 插件开发指南

本文档介绍了 OpenMCP-Chain 的插件体系结构、生命周期钩子以及如何编写、构建与部署第三方插件。

## 架构概览

`pkg/plugin` 模块提供统一的插件接口、生命周期管理器与隔离策略：

- **Plugin 接口**：插件必须实现 `Info()`, `Configure()`, `Init()`, `Start()` 与 `Stop()` 方法。
- **ExecutionContext**：生命周期方法会收到运行上下文，包含 `context.Context`、合并后的配置以及宿主暴露的共享资源。
- **Manager**：负责插件注册、动态加载（基于 Go `plugin` 标准库）、生命周期调度与安全策略校验。
- **IsolationPolicy**：允许宿主定义可用能力（Capabilities），对插件访问敏感资源进行限制。

> 若插件声明了能力（例如 `filesystem`、`network`、`execution`），必须在配置中为其提供隔离策略，否则管理器会拒绝加载。

## 生命周期

1. **Configure**：在插件注册时触发，用于读取并补全配置。
2. **Init**：插件初始化，只执行一次。
3. **Start**：插件进入运行态，可启动 goroutine、订阅消息等。
4. **Stop**：宿主关闭或重载插件时调用，用于释放资源。

生命周期上下文会复用宿主注册的共享资源，例如日志组件、消息总线或数据通道。

## 动态加载与配置

插件管理配置位于 `configs/plugins.yaml`，示例：

```yaml
pluginDir: ./plugins
defaults:
  allowedCapabilities:
    - filesystem
plugins:
  memory:
    enabled: true
    path: memory-datasource.so
    config:
      records:
        - {"id": 1, "value": "hello"}
  logger:
    enabled: true
    path: logging-processor.so
    policy:
      allowedCapabilities:
        - execution
```

- `pluginDir`：相对或绝对路径，指向 `.so` 文件所在目录。
- `defaults`：全局隔离策略，插件可继承并根据需要覆盖。
- `plugins`：逐个插件的启用状态、二进制路径及特定配置。

宿主程序可通过 `plugin.LoadManagerConfig` 读取配置并构建管理器：

```go
cfg, err := plugin.LoadManagerConfig("configs/plugins.yaml")
manager, err := plugin.NewManager(cfg,
    plugin.WithResource("datasource:sink", emitFunc),
    plugin.WithResource("processor:input", inputChan),
)
if err != nil {
    panic(err)
}
if err := manager.StartAll(context.Background()); err != nil {
    panic(err)
}
```

## 开发示例

仓库在 `examples/plugins` 下提供两个基础插件：

- `datasource/memory`：从配置中读取静态数据并推送至 `datasource:sink`。
- `processor/logger`：消费 `processor:input` 通道并输出结构化日志。

### 步骤 1：实现 `plugin.Plugin`

```go
package main

import "OpenMCP-Chain/pkg/plugin"

type customPlugin struct{}

func (c *customPlugin) Info() plugin.Info { /* ... */ }
func (c *customPlugin) Configure(cfg map[string]any) error { return nil }
func (c *customPlugin) Init(ctx *plugin.ExecutionContext) error { return nil }
func (c *customPlugin) Start(ctx *plugin.ExecutionContext) error { return nil }
func (c *customPlugin) Stop(ctx *plugin.ExecutionContext) error { return nil }

var Plugin plugin.Plugin = &customPlugin{}
```

> 注意：必须导出名为 `Plugin` 的变量（或工厂函数），供管理器在动态加载时检索。

### 步骤 2：构建共享对象

在插件目录执行：

```bash
go build -buildmode=plugin -o my-plugin.so
```

生成的 `.so` 文件应放置在 `configs/plugins.yaml` 中 `pluginDir` 指定的目录下。

### 步骤 3：配置隔离策略

根据插件所需能力设置 `allowedCapabilities` 或 `deniedCapabilities`：

```yaml
policy:
  allowedCapabilities:
    - filesystem
```

若插件声明了能力但未配置策略，管理器会返回错误。

### 常见问题

- **跨平台**：Go 插件机制目前仅在 Linux、macOS 等 Unix 平台生效，Windows 不支持。
- **接口变更**：宿主升级 `pkg/plugin` 接口后，第三方插件需要重新编译。
- **资源注入**：使用 `plugin.WithResource` 将宿主依赖注入给插件（例如数据库连接、通道、回调）。

## 安全建议

1. 为插件分配最小必要能力并启用隔离策略。
2. 尽量运行不受信任插件于独立容器或进程，在 `IsolationStrategy` 中扩展真实沙箱逻辑。
3. 审核第三方插件的依赖与许可证。

## 进一步扩展

- 实现自定义 `IsolationStrategy`，例如基于 cgroup、seccomp 或 WebAssembly Sandbox。
- 提供插件市场，结合签名验证或哈希校验机制。
- 在宿主中加入插件热更新与观测指标。

欢迎社区贡献更多插件示例与最佳实践。
