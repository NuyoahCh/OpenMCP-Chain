#!/usr/bin/env python3
"""llm_bridge.py

一个非常轻量的 Python 脚本，用于模拟大模型的推理过程。
脚本从标准输入读取 JSON 请求，并输出结构化的 JSON 结果。
"""

import json
import sys
from datetime import datetime
from typing import Any, Iterable


def _format_history_lines(history: Iterable[dict[str, Any]]) -> list[str]:
    lines: list[str] = ["历史参考任务："]
    for idx, item in enumerate(history, start=1):
        goal = str(item.get("goal", "未知目标"))
        reply = str(item.get("reply", ""))
        observations = str(item.get("observations", ""))
        created_at = item.get("created_at")
        if isinstance(created_at, (int, float)):
            ts = datetime.utcfromtimestamp(int(created_at)).strftime("%Y-%m-%d %H:%M:%S UTC")
        else:
            ts = str(created_at or "未知时间")

        summary = reply or observations
        if len(summary) > 60:
            summary = summary[:57] + "..."
        lines.append(f"- [{idx}] {ts} 目标: {goal} | 结果: {summary or '无摘要'}")
        if idx >= 3:
            break
    return lines


def _format_knowledge_lines(knowledge: Iterable[dict[str, Any]]) -> list[str]:
    lines: list[str] = ["知识库参考："]
    for idx, item in enumerate(knowledge, start=1):
        title = str(item.get("title", "知识点")) or "知识点"
        content = str(item.get("content", ""))
        if len(content) > 80:
            content = content[:77] + "..."
        lines.append(f"- [{idx}] {title}: {content or '暂无详细内容'}")
        if idx >= 3:
            break
    return lines


def build_reply(
    goal: str,
    action: str,
    address: str,
    history: list[dict[str, Any]],
    knowledge: list[dict[str, Any]],
) -> tuple[str, str]:
    """根据输入生成思考过程与回复。"""
    now = datetime.utcnow().strftime("%Y-%m-%d %H:%M:%S UTC")
    thought_lines = [
        f"当前目标: {goal}",
        f"预期链上操作: {action or '未指定'}",
        f"涉及地址: {address or '未指定'}",
        f"时间戳: {now}",
    ]
    if history:
        thought_lines.extend(_format_history_lines(history))
    if knowledge:
        thought_lines.extend(_format_knowledge_lines(knowledge))
    thought = "\n".join(thought_lines)

    reply = (
        f"我已经理解你的目标『{goal}』。"
        f"下一步可以按照『{action or '补充链上操作'}』在链上执行，并保持地址 {address or '待定'} 的安全。"
    )
    if history:
        latest_goal = str(history[0].get("goal", "近期任务")).strip()
        if latest_goal:
            reply += f" 同时我参考了历史任务『{latest_goal}』的经验，以保证策略保持一致。"
        else:
            reply += " 我也结合了最近的任务经验，帮助你更快迭代。"
    if knowledge:
        key_title = str(knowledge[0].get("title", "知识库建议")).strip()
        if key_title:
            reply += f" 我额外查阅了知识库中的『{key_title}』，为你的下一步提供理论依据。"
        else:
            reply += " 我也参考了知识库中的相关条目，以提升方案可靠性。"
    return thought, reply


def main() -> None:
    try:
        payload = json.load(sys.stdin)
    except json.JSONDecodeError as exc:  # pragma: no cover - 直接写入错误即可
        json.dump({"error": f"输入不是有效的 JSON: {exc}"}, sys.stdout, ensure_ascii=False)
        return

    goal = str(payload.get("goal", "")).strip()
    action = str(payload.get("chain_action", "")).strip()
    address = str(payload.get("address", "")).strip()

    if not goal:
        json.dump({"error": "goal 字段不能为空"}, sys.stdout, ensure_ascii=False)
        return

    history = payload.get("history")
    if not isinstance(history, list):
        history = []

    normalized_history: list[dict[str, Any]] = []
    for item in history:
        if isinstance(item, dict):
            normalized_history.append(item)

    knowledge = payload.get("knowledge")
    if not isinstance(knowledge, list):
        knowledge = []

    normalized_knowledge: list[dict[str, Any]] = []
    for item in knowledge:
        if isinstance(item, dict):
            normalized_knowledge.append(item)

    thought, reply = build_reply(goal, action, address, normalized_history, normalized_knowledge)
    json.dump({"thought": thought, "reply": reply}, sys.stdout, ensure_ascii=False)


if __name__ == "__main__":
    main()
