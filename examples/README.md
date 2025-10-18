# 示例

`examples/` 目录收录可以直接运行的脚本，帮助开发者快速理解 OpenMCP-Chain 的接口与任务流转方式。

## `task_quickstart.py`

- 作用：通过 REST API 提交任务、读取历史记录。
- 依赖：Python 3.9+ 与 `requests` 库（`pip install requests`）。
- 用法：

```bash
# 提交一次任务
python examples/task_quickstart.py invoke \
  --goal "查询账户余额" \
  --chain-action eth_getBalance \
  --address 0x0000000000000000000000000000000000000000

# 获取最近的 5 条历史记录
python examples/task_quickstart.py history --limit 5
```

> 提示：`--metadata key=value` 可在任务中附加自定义信息，例如 `--metadata project=demo owner=alice`。

欢迎在此目录新增更多示例（如 Jupyter Notebook、Go/TypeScript SDK 样例），并在 README 中进行登记。
