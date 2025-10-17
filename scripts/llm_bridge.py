#!/usr/bin/env python3
"""llm_bridge.py

一个非常轻量的 Python 脚本，用于模拟大模型的推理过程。
脚本从标准输入读取 JSON 请求，并输出结构化的 JSON 结果。
"""

import json
import sys
from datetime import datetime


def build_reply(goal: str, action: str, address: str) -> tuple[str, str]:
    """根据输入生成思考过程与回复。"""
    now = datetime.utcnow().strftime("%Y-%m-%d %H:%M:%S UTC")
    thought_lines = [
        f"当前目标: {goal}",
        f"预期链上操作: {action or '未指定'}",
        f"涉及地址: {address or '未指定'}",
        f"时间戳: {now}",
    ]
    thought = "\n".join(thought_lines)

    reply = (
        f"我已经理解你的目标『{goal}』。"
        f"下一步可以按照『{action or '补充链上操作'}』在链上执行，并保持地址 {address or '待定'} 的安全。"
    )
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

    thought, reply = build_reply(goal, action, address)
    json.dump({"thought": thought, "reply": reply}, sys.stdout, ensure_ascii=False)


if __name__ == "__main__":
    main()
