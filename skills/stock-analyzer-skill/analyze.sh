#!/bin/bash

# Stock Analyzer - 股票分析工具
# 支持A股、港股、美股等东方财富覆盖的所有市场

set -e

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PYTHON_SCRIPT="$SCRIPT_DIR/scripts/fetch_stock.py"

# 默认参数
STOCK=""
MARKET="auto"
OUTPUT="text"

# 解析参数
while [[ $# -gt 0 ]]; do
    case $1 in
        --stock)
            STOCK="$2"
            shift 2
            ;;
        --market)
            MARKET="$2"
            shift 2
            ;;
        --output)
            OUTPUT="$2"
            shift 2
            ;;
        *)
            STOCK="$1"
            shift
            ;;
    esac
done

# 检查股票参数
if [[ -z "$STOCK" ]]; then
    echo '{"success": false, "error": "请提供股票名称或代码"}'
    exit 1
fi

# 检查市场参数，如果为空则设置为默认值
if [[ -z "$MARKET" ]]; then
    MARKET="auto"
fi

# 检查 Python 脚本是否存在
if [[ ! -f "$PYTHON_SCRIPT" ]]; then
    echo '{"success": false, "error": "Python 脚本不存在: fetch_stock.py"}'
    exit 1
fi

# 执行 Python 脚本
PYTHON_CMD="${PYTHON_CMD:-python3}"
if ! command -v "$PYTHON_CMD" &> /dev/null; then
    echo '{"success": false, "error": "Python 未安装或不在 PATH 中"}'
    exit 1
fi

# 执行分析
cd "$SCRIPT_DIR/scripts"
"$PYTHON_CMD" "$PYTHON_SCRIPT" "$STOCK" --market "$MARKET" --output "$OUTPUT"