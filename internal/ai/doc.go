// Package ai 汇总了系统中与大模型推理相关的组件说明。
//
// 历史上 OpenMCP 通过 scripts/llm_bridge.py 提供的 Python 脚本来模拟大模型，
// 由 internal/llm/pythonbridge 客户端调用脚本并返回 Thought/Reply 结构。该方案
// 适用于离线演示，但无法访问真实的大语言模型服务。
//
// 为了支持真实的 API 调用，本目录同时记录了接口规范（参见 internal/llm 包中
// 的 Request/Response 定义）以及外部推理服务（例如 OpenAI）的客户端实现在
// 哪些包中。新的实现位于 internal/llm/openai，通过 HTTP API 将 Agent 的上下文
// 转换成对话消息并解析模型的结构化输出。
package ai
