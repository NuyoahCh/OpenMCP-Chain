#!/usr/bin/env python3
"""task_client.py

使用标准库向 openmcpd 暴露的 REST 接口发起请求，方便在命令行触发智能体任务或查看历史记录。
"""

from __future__ import annotations

import argparse
import json
import sys
import urllib.error
import urllib.parse
import urllib.request
from typing import Any


def request_json(method: str, url: str, data: dict[str, Any] | None = None) -> Any:
    body: bytes | None = None
    headers = {"Content-Type": "application/json", "Accept": "application/json"}
    if data is not None:
        body = json.dumps(data, ensure_ascii=False).encode("utf-8")

    req = urllib.request.Request(url=url, data=body, headers=headers, method=method)
    try:
        with urllib.request.urlopen(req, timeout=10) as resp:
            charset = resp.headers.get_content_charset("utf-8")
            payload = resp.read().decode(charset)
            if not payload:
                return None
            return json.loads(payload)
    except urllib.error.HTTPError as exc:  # pragma: no cover - 直接打印错误信息
        sys.stderr.write(f"HTTP {exc.code}: {exc.read().decode('utf-8', errors='ignore')}\n")
        raise SystemExit(1) from exc
    except urllib.error.URLError as exc:  # pragma: no cover
        sys.stderr.write(f"请求失败: {exc}\n")
        raise SystemExit(1) from exc


def do_invoke(args: argparse.Namespace) -> None:
    payload = {
        "goal": args.goal,
        "chain_action": args.chain_action,
        "address": args.address,
    }
    url = urllib.parse.urljoin(args.base_url, "/api/v1/tasks")
    result = request_json("POST", url, payload)
    json.dump(result, sys.stdout, ensure_ascii=False, indent=2)
    sys.stdout.write("\n")


def do_history(args: argparse.Namespace) -> None:
    query = urllib.parse.urlencode({"limit": args.limit})
    url = urllib.parse.urljoin(args.base_url, f"/api/v1/tasks?{query}")
    result = request_json("GET", url)
    json.dump(result, sys.stdout, ensure_ascii=False, indent=2)
    sys.stdout.write("\n")


def build_parser() -> argparse.ArgumentParser:
    parser = argparse.ArgumentParser(description="OpenMCP-Chain REST API 命令行助手")
    parser.add_argument(
        "--base-url",
        default="http://127.0.0.1:8080",
        help="openmcpd HTTP 服务地址，默认 http://127.0.0.1:8080",
    )

    subparsers = parser.add_subparsers(dest="command", required=True)

    invoke = subparsers.add_parser("invoke", help="触发一次智能体任务")
    invoke.add_argument("--goal", required=True, help="任务目标描述")
    invoke.add_argument("--chain-action", default="", help="预期的链上操作，例如 eth_getBalance")
    invoke.add_argument("--address", default="", help="可能涉及的链上地址")
    invoke.set_defaults(func=do_invoke)

    history = subparsers.add_parser("history", help="查看最近的任务执行记录")
    history.add_argument("--limit", type=int, default=5, help="返回的记录条数，默认 5")
    history.set_defaults(func=do_history)

    return parser


def main(argv: list[str] | None = None) -> None:
    parser = build_parser()
    args = parser.parse_args(argv)
    args.func(args)


if __name__ == "__main__":
    main()
