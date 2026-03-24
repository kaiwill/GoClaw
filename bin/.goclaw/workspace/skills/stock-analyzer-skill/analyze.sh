#!/bin/bash

# Stock Analyzer - 统一股票分析工具
# 支持：
# - A股/港股/美股 (东方财富数据源)
# - 美股深度分析 (Yahoo Finance, 8维度)
# - 加密货币分析 (Top 20)
# - 投资组合管理

set -e

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
CN_SCRIPT="$SCRIPT_DIR/scripts/fetch_stock.py"
US_SCRIPT="$SCRIPT_DIR/scripts/us/analyze_stock.py"
US_PORTFOLIO_SCRIPT="$SCRIPT_DIR/scripts/us/portfolio.py"

# 默认参数
COMMAND="analyze"
STOCK=""
MARKET="auto"
OUTPUT="text"
PORTFOLIO_NAME=""
QUANTITY=""
COST=""

# 显示帮助
show_help() {
    echo "Stock Analyzer - 统一股票分析工具"
    echo ""
    echo "用法:"
    echo "  $0 [股票代码] [选项]"
    echo ""
    echo "命令:"
    echo "  analyze (默认)   分析股票"
    echo "  portfolio        投资组合管理"
    echo ""
    echo "选项:"
    echo "  --stock, -s      股票代码或名称"
    echo "  --market, -m     市场类型 (sh/sz/bj/hk/us/auto)"
    echo "  --output, -o     输出格式 (text/json)"
    echo "  --portfolio, -p  投资组合名称"
    echo "  --quantity, -q   数量 (用于 portfolio add/update)"
    echo "  --cost, -c       成本价 (用于 portfolio add/update)"
    echo "  --help, -h       显示帮助"
    echo ""
    echo "示例:"
    echo "  # 分析A股"
    echo "  $0 600519 --market sh"
    echo "  $0 贵州茅台"
    echo ""
    echo "  # 分析港股"
    echo "  $0 00700 --market hk"
    echo "  $0 腾讯控股"
    echo ""
    echo "  # 分析美股 (自动识别)"
    echo "  $0 AAPL"
    echo "  $0 TSLA NVDA"
    echo ""
    echo "  # 分析加密货币"
    echo "  $0 BTC-USD"
    echo "  $0 ETH-USD"
    echo ""
    echo "  # 投资组合管理"
    echo "  $0 portfolio create '我的组合'"
    echo "  $0 portfolio add AAPL --quantity 100 --cost 150"
    echo "  $0 portfolio show"
    echo "  $0 portfolio list"
}

# 判断是否为美股代码
is_us_stock() {
    local ticker="$1"
    # 美股代码特征：纯大写字母，1-5个字符，或以-USD结尾
    if [[ "$ticker" =~ ^[A-Z]{1,5}$ ]] || [[ "$ticker" =~ ^[A-Z]+-USD$ ]]; then
        return 0
    fi
    return 1
}

# 解析参数
while [[ $# -gt 0 ]]; do
    case $1 in
        --help|-h)
            show_help
            exit 0
            ;;
        --command)
            COMMAND="$2"
            shift 2
            ;;
        --stock|-s)
            STOCK="$2"
            shift 2
            ;;
        --market|-m)
            MARKET="$2"
            shift 2
            ;;
        --output|-o)
            OUTPUT="$2"
            shift 2
            ;;
        --portfolio|-p)
            PORTFOLIO_NAME="$2"
            shift 2
            ;;
        --quantity|-q)
            QUANTITY="$2"
            shift 2
            ;;
        --cost|-c)
            COST="$2"
            shift 2
            ;;
        portfolio)
            COMMAND="portfolio"
            shift
            ;;
        create|list|show|delete|add|update|remove)
            if [[ "$COMMAND" == "portfolio" ]]; then
                PORTFOLIO_ACTION="$1"
                shift
            else
                STOCK="$1"
                shift
            fi
            ;;
        -*)
            echo "错误: 未知选项 $1"
            show_help
            exit 1
            ;;
        *)
            if [[ -z "$STOCK" ]]; then
                STOCK="$1"
            fi
            shift
            ;;
    esac
done

# 检查 Python 命令
PYTHON_CMD="${PYTHON_CMD:-python3}"
if ! command -v "$PYTHON_CMD" &> /dev/null; then
    echo '{"success": false, "error": "Python 未安装或不在 PATH 中"}'
    exit 1
fi

# 执行分析
case $COMMAND in
    analyze|analyse)
        # 检查股票参数
        if [[ -z "$STOCK" ]]; then
            echo '{"success": false, "error": "请提供股票名称或代码"}'
            exit 1
        fi
        
        # 规范化市场参数
        MARKET=$(echo "$MARKET" | tr '[:upper:]' '[:lower:]')
        
        # 自动识别美股
        if [[ "$MARKET" == "auto" ]] && is_us_stock "$STOCK"; then
            MARKET="us"
        fi
        
        # 选择分析脚本
        if [[ "$MARKET" == "us" ]] || is_us_stock "$STOCK"; then
            # 美股/加密货币分析
            if [[ ! -f "$US_SCRIPT" ]]; then
                echo '{"success": false, "error": "美股分析脚本不存在: scripts/us/analyze_stock.py"}'
                exit 1
            fi
            cd "$SCRIPT_DIR/scripts/us"
            "$PYTHON_CMD" "$US_SCRIPT" "$STOCK" --output "$OUTPUT"
        else
            # A股/港股分析
            if [[ ! -f "$SCRIPT_DIR/index.js" ]]; then
                echo '{"success": false, "error": "Node.js 分析脚本不存在: index.js"}'
                exit 1
            fi
            cd "$SCRIPT_DIR"
            node index.js --stock "$STOCK" --market "$MARKET"
        fi
        ;;
        
    portfolio)
        # 投资组合管理
        if [[ ! -f "$US_PORTFOLIO_SCRIPT" ]]; then
            echo '{"success": false, "error": "投资组合脚本不存在: scripts/us/portfolio.py"}'
            exit 1
        fi
        
        PORTFOLIO_ARGS=()
        
        case "${PORTFOLIO_ACTION:-list}" in
            create)
                if [[ -z "$STOCK" ]]; then
                    echo '{"success": false, "error": "请提供组合名称"}'
                    exit 1
                fi
                PORTFOLIO_ARGS+=("create" "$STOCK")
                ;;
            list)
                PORTFOLIO_ARGS+=("list")
                ;;
            show)
                PORTFOLIO_ARGS+=("show")
                if [[ -n "$PORTFOLIO_NAME" ]]; then
                    PORTFOLIO_ARGS+=("--portfolio" "$PORTFOLIO_NAME")
                fi
                ;;
            delete)
                if [[ -z "$STOCK" ]]; then
                    echo '{"success": false, "error": "请提供组合名称"}'
                    exit 1
                fi
                PORTFOLIO_ARGS+=("delete" "$STOCK")
                ;;
            add)
                if [[ -z "$STOCK" ]]; then
                    echo '{"success": false, "error": "请提供股票代码"}'
                    exit 1
                fi
                if [[ -z "$QUANTITY" ]] || [[ -z "$COST" ]]; then
                    echo '{"success": false, "error": "请提供数量和成本价"}'
                    exit 1
                fi
                PORTFOLIO_ARGS+=("add" "$STOCK" "--quantity" "$QUANTITY" "--cost" "$COST")
                if [[ -n "$PORTFOLIO_NAME" ]]; then
                    PORTFOLIO_ARGS+=("--portfolio" "$PORTFOLIO_NAME")
                fi
                ;;
            update)
                if [[ -z "$STOCK" ]]; then
                    echo '{"success": false, "error": "请提供股票代码"}'
                    exit 1
                fi
                PORTFOLIO_ARGS+=("update" "$STOCK")
                if [[ -n "$QUANTITY" ]]; then
                    PORTFOLIO_ARGS+=("--quantity" "$QUANTITY")
                fi
                if [[ -n "$COST" ]]; then
                    PORTFOLIO_ARGS+=("--cost" "$COST")
                fi
                if [[ -n "$PORTFOLIO_NAME" ]]; then
                    PORTFOLIO_ARGS+=("--portfolio" "$PORTFOLIO_NAME")
                fi
                ;;
            remove)
                if [[ -z "$STOCK" ]]; then
                    echo '{"success": false, "error": "请提供股票代码"}'
                    exit 1
                fi
                PORTFOLIO_ARGS+=("remove" "$STOCK")
                if [[ -n "$PORTFOLIO_NAME" ]]; then
                    PORTFOLIO_ARGS+=("--portfolio" "$PORTFOLIO_NAME")
                fi
                ;;
            *)
                echo '{"success": false, "error": "未知操作: '"$PORTFOLIO_ACTION"'"}'
                exit 1
                ;;
        esac
        
        cd "$SCRIPT_DIR/scripts/us"
        "$PYTHON_CMD" "$US_PORTFOLIO_SCRIPT" "${PORTFOLIO_ARGS[@]}"
        ;;
        
    *)
        echo '{"success": false, "error": "未知命令: '"$COMMAND"'"}'
        exit 1
        ;;
esac
