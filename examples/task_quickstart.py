"""示例脚本：通过 REST API 提交任务并查询历史。

用法：
    python task_quickstart.py invoke --goal "查询账户余额" --chain-action eth_getBalance \
        --address 0x0000000000000000000000000000000000000000

    python task_quickstart.py history --limit 5

在运行前请确保已启动 openmcpd 守护进程，并根据需要设置 --host 与 --port。
"""

from __future__ import annotations

import argparse
import json
import sys
from typing import Any, Dict

import requests


def _build_client(host: str, port: int) -> str:
    return f"http://{host}:{port}/api/v1/tasks"


def invoke_task(endpoint: str, goal: str, chain_action: str | None, address: str | None, metadata: Dict[str, Any]) -> None:
    payload: Dict[str, Any] = {"goal": goal}
    if chain_action:
        payload["chain_action"] = chain_action
    if address:
        payload["address"] = address
    if metadata:
        payload["metadata"] = metadata

    response = requests.post(endpoint, json=payload, timeout=30)
    response.raise_for_status()
    print(json.dumps(response.json(), indent=2, ensure_ascii=False))


def fetch_history(endpoint: str, limit: int) -> None:
    response = requests.get(endpoint, params={"limit": limit}, timeout=10)
    response.raise_for_status()
    print(json.dumps(response.json(), indent=2, ensure_ascii=False))


def parse_metadata(pairs: list[str]) -> Dict[str, Any]:
    result: Dict[str, Any] = {}
    for item in pairs:
        if "=" not in item:
            raise argparse.ArgumentTypeError("metadata 项需使用 key=value 形式")
        key, value = item.split("=", 1)
        result[key] = value
    return result


def main() -> None:
    parser = argparse.ArgumentParser(description="OpenMCP-Chain REST API 示例客户端")
    parser.add_argument("action", choices=["invoke", "history"], help="要执行的操作")
    parser.add_argument("--host", default="127.0.0.1", help="API 服务主机名")
    parser.add_argument("--port", type=int, default=8080, help="API 服务端口")

    parser.add_argument("--goal", help="任务目标描述")
    parser.add_argument("--chain-action", dest="chain_action", help="链上操作 (如 eth_getBalance)")
    parser.add_argument("--address", help="与链上操作相关的地址")
    parser.add_argument("--metadata", nargs="*", default=[], help="附加元数据，使用 key=value 格式")
    parser.add_argument("--limit", type=int, default=10, help="history 模式下返回的任务数量")

    args = parser.parse_args()
    endpoint = _build_client(args.host, args.port)

    try:
        if args.action == "invoke":
            if not args.goal:
                parser.error("invoke 模式需要指定 --goal")
            metadata = parse_metadata(args.metadata)
            invoke_task(endpoint, args.goal, args.chain_action, args.address, metadata)
        elif args.action == "history":
            fetch_history(endpoint, args.limit)
        else:
            parser.error(f"未知操作: {args.action}")
    except requests.HTTPError as exc:  # pragma: no cover - 示例脚本仅做演示
        print(f"请求失败: {exc.response.status_code} {exc.response.text}", file=sys.stderr)
        sys.exit(1)
    except requests.RequestException as exc:  # pragma: no cover - 示例脚本仅做演示
        print(f"网络错误: {exc}", file=sys.stderr)
        sys.exit(1)


if __name__ == "__main__":
    main()
