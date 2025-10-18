# OpenMCP Chain Web Console

一个基于 React + Vite 构建的极简控制台，用于演示如何通过现有的 OpenMCP-Chain 后端提交与观察智能体任务。

## 功能亮点

- ✅ 图形化提交任务：填写目标、链上操作与可选地址字段，一键触发后端的 `/api/v1/tasks` 接口。
- ✅ 实时轮询：在任务完成前自动轮询状态，并展示最新的模型输出、链上观测与错误信息。
- ✅ 历史追踪：自动刷新最近的任务列表，支持手动点选并重新拉取任务详情。
- ✅ 易于部署：Vite 构建产物为纯静态文件，可由任意静态服务器托管，亦可通过 Nginx 反向代理挂载在同域。

## 快速开始

```bash
cd web
npm install
npm run dev
```

默认会启动在 <http://localhost:5173>，并尝试访问后端 `http://127.0.0.1:8080`。若后端部署在其他地址，可在启动前配置：

```bash
VITE_API_BASE_URL="http://your-openmcp-host:8080" npm run dev
```

## 构建生产版本

```bash
npm run build
```

生成的静态资源位于 `dist/` 目录，可直接托管。也可以在后端服务中增加静态文件路由，将 `dist` 目录挂载到 `/console` 等路径下。

## 目录结构

```
web/
├── src/
│   ├── App.tsx            # 主界面逻辑与轮询流程
│   ├── api.ts             # 与后端交互的封装函数
│   ├── components/
│   │   ├── TaskForm.tsx   # 任务创建表单
│   │   └── TaskList.tsx   # 历史任务列表
│   └── index.css          # 自定义玻璃拟态风格样式
└── vite.config.ts         # Vite 开发/构建配置
```

## 与后端连调小贴士

- 确保后端已通过 `OPENMCP_CONFIG=$(pwd)/configs/openmcp.json go run ./cmd/openmcpd` 启动。
- 开发模式下如遇浏览器跨域限制，可通过 `npm run dev -- --host 127.0.0.1` 并在同一主机访问，或在后端补充 CORS 设置。
- 由于任务创建接口返回 202 状态码，前端会自动轮询直至 `succeeded`/`failed`。默认最多轮询 40 次，可按需在 `App.tsx` 中调整。

欢迎根据业务需求扩展页面组件、接入身份认证或替换成更复杂的 UI 框架。
