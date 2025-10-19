# OpenMCP Chain Web Console

一个基于 React + Vite 构建的轻量控制台，用于演示如何通过现有的 OpenMCP-Chain 后端提交与观察智能体任务。

## 功能亮点

- ✅ 图形化提交任务：填写目标、链上操作、可选地址与 Metadata，一键触发 `/api/v1/tasks` 接口。
- ✅ 实时轮询：在任务完成前自动轮询状态，并展示最新的模型输出、链上观测与错误信息。
- ✅ 任务联动视图：内置状态概览、快速刷新与详情抽屉，便于在大屏监控或运维排障中使用。
- ✅ 服务可配置：支持通过 UI 或 `VITE_API_BASE_URL` 配置后端地址，连接成功与失败均有提示。
- ✅ 身份认证卡片：可使用账号密码换取访问令牌并自动缓存、失效后提醒续期。
- ✅ 离线保护：自动检测网络状态，离线时暂停轮询与提交并提示恢复时间。
- ✅ 排查友好：按状态筛选任务，并可一键导出 JSON 或复制单条详情，便于做审计留存。

## 快速开始

```bash
cd web
npm install
npm run dev
```

默认会启动在 <http://localhost:5173>，并尝试访问后端 `http://127.0.0.1:8080`。若后端部署在其他地址，可在 UI 右上角“运行提示 & 服务配置”卡片中修改并保存，或在启动前配置：

```bash
VITE_API_BASE_URL="http://your-openmcp-host:8080" npm run dev
```

## 构建生产版本

```bash
npm run build
```

生成的静态资源位于 `dist/` 目录，可直接托管，也可在后端服务中增加静态文件路由，将 `dist` 目录挂载到 `/console` 等路径下。

## 目录结构

```
web/
├── src/
│   ├── App.tsx                 # 主界面逻辑与轮询流程
│   ├── api.ts                  # 与后端交互的封装函数、认证状态管理
│   ├── hooks/
│   │   ├── useApiBaseUrl.ts    # 本地持久化的后端地址配置
│   │   ├── useAuth.ts          # 登录状态与令牌缓存 Hook
│   │   └── useNetworkStatus.ts # 网络状态检测
│   ├── components/
│   │   ├── ConnectionSettings.tsx # API 地址、连通性测试与刷新控制
│   │   ├── AuthPanel.tsx          # 登录/登出入口与令牌管理
│   │   ├── StatusSummary.tsx      # 任务状态概览卡片
│   │   ├── TaskDetails.tsx        # 结果详情展示与复制
│   │   ├── TaskForm.tsx           # 任务创建表单（含快捷模板与 Metadata）
│   │   └── TaskList.tsx           # 历史任务列表、筛选与导出
│   └── index.css               # 自定义玻璃拟态风格样式
└── vite.config.ts             # Vite 开发/构建配置
```

## 与后端连调小贴士

- 确保后端已通过 `OPENMCP_CONFIG=$(pwd)/configs/openmcp.json go run ./cmd/openmcpd` 启动。
- 开发模式下如遇浏览器跨域限制，可通过 `npm run dev -- --host 127.0.0.1` 并在同一主机访问，或在后端补充 CORS 设置。
- 由于任务创建接口返回 202 状态码，前端会自动轮询直至 `succeeded`/`failed`。默认最多轮询 40 次，可按需在 `App.tsx` 中调整。
- 如果后端开启了认证（`auth.mode != "disabled"`），请使用拥有 `tasks.read`/`tasks.write` 权限的账号在“身份认证”卡片中登录。令牌过期后页面会提示重新登录。

欢迎根据业务需求扩展页面组件或替换成更复杂的 UI 框架。
