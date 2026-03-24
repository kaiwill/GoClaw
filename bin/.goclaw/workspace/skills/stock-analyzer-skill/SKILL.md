# Stock Analyzer Skill

## Description

全球股票综合分析工具。支持A股、港股、美股、加密货币分析。

- **A股/港股**：使用东方财富数据源，基本面+资金面分析
- **美股/加密货币**：使用Yahoo Finance，8维度深度分析
- **投资组合管理**：创建组合、添加/删除资产、查看持仓

## Commands

### analyze - 统一分析入口

自动识别市场类型并生成投资分析报告。

**Parameters:**
- `stock` (required): 股票名称或代码
- `market` (optional): 市场类型 (sh/sz/bj/hk/us/auto)

**Usage:**
```bash
# A股
./analyze.sh 贵州茅台
./analyze.sh 600519 --market sh

# 港股
./analyze.sh 腾讯控股
./analyze.sh 00700 --market hk

# 美股 (自动识别)
./analyze.sh AAPL
./analyze.sh TSLA NVDA

# 加密货币
./analyze.sh BTC-USD
./analyze.sh ETH-USD SOL-USD
```

### analyze-us - 美股/加密货币深度分析

8维度评估：收益惊喜、基本面、分析师情绪、历史模式、市场环境、行业表现、动量、情绪分析。

**Parameters:**
- `ticker` (required): 美股代码或加密货币代码

**美股分析维度 (8个):**
1. **Earnings Surprise (30% weight)**: 实际EPS vs 预期
2. **Fundamentals (20% weight)**: P/E、利润率、营收增长、负债
3. **Analyst Sentiment (20% weight)**: 共识评级、目标价
4. **Historical Patterns (10% weight)**: 过去财报反应
5. **Market Context (10% weight)**: VIX、SPY/QQQ趋势
6. **Sector Performance (15% weight)**: 行业相对表现
7. **Momentum (15% weight)**: RSI、52周区间、成交量
8. **Sentiment (10% weight)**: 恐惧贪婪指数、做空比例、内部交易

**加密货币分析维度 (3个):**
1. **Crypto Fundamentals**: 市值、类别、BTC相关性
2. **Momentum**: RSI、价格区间
3. **Market Context**: VIX、整体市场环境

### analyze-cn - A股/港股分析

使用东方财富数据源，基本面+资金面分析。

**Parameters:**
- `stock` (required): A股/港股代码或名称
- `market` (optional): 市场类型 (sh/sz/bj/hk)

### portfolio - 投资组合管理

**Actions:**
- `create <name>`: 创建新组合
- `list`: 列出所有组合
- `show`: 显示组合详情
- `delete <name>`: 删除组合
- `add <ticker> --quantity <n> --cost <price>`: 添加资产
- `update <ticker> --quantity <n> --cost <price>`: 更新资产
- `remove <ticker>`: 移除资产

**Usage:**
```bash
# 创建组合
./analyze.sh portfolio create "我的组合"

# 添加资产
./analyze.sh portfolio add AAPL --quantity 100 --cost 150
./analyze.sh portfolio add BTC-USD --quantity 0.5 --cost 40000

# 查看组合
./analyze.sh portfolio show

# 分析组合
./analyze.sh --portfolio "我的组合" --period weekly
```

## Supported Markets

| 市场 | 代码格式 | 数据源 |
|------|---------|--------|
| A股沪市 | 6开头 (600519) | 东方财富 |
| A股深市 | 0/3开头 (000001, 300750) | 东方财富 |
| 北交所 | 8/4开头 (830799) | 东方财富 |
| 港股 | 5位数字 (00700) | 东方财富 |
| 美股 | 大写字母 (AAPL, TSLA) | Yahoo Finance |
| 加密货币 | XXX-USD (BTC-USD) | Yahoo Finance |

## Supported Cryptocurrencies (Top 20)

BTC-USD, ETH-USD, BNB-USD, SOL-USD, XRP-USD, ADA-USD, DOGE-USD, AVAX-USD, DOT-USD, MATIC-USD, LINK-USD, ATOM-USD, UNI-USD, LTC-USD, BCH-USD, XLM-USD, ALGO-USD, VET-USD, FIL-USD, NEAR-USD

## Features

- **多市场支持**: A股、港股、美股、加密货币
- **深度分析**: 8维度美股分析、3维度加密货币分析
- **风险评估**: 地缘政治风险、突发新闻检测、避险资产追踪
- **投资组合**: 创建和管理投资组合
- **定期报告**: 日/周/月/季度/年度收益

## Risk Features (美股)

- **Breaking News Alerts**: 扫描危机关键词
- **Geopolitical Risk**: 台海、中国、俄乌、中东、银行危机
- **Safe-Haven Tracking**: GLD、TLT、UUP 避险资产
- **Concentration Warnings**: 单一资产超过30%警告

## Security

- 基础分析对所有用户开放
- 高级功能需要 USER.md 中定义的授权用户

## Configuration

- A股/港股脚本: `scripts/fetch_stock.py`
- 美股脚本: `scripts/us/analyze_stock.py`
- 投资组合脚本: `scripts/us/portfolio.py`
- Python依赖: `scripts/requirements.txt`, `scripts/us/requirements.txt`

## Data Sources

- **A股/港股**: 东方财富网 (https://www.eastmoney.com)
- **美股/加密货币**: Yahoo Finance (https://finance.yahoo.com)
- **恐惧贪婪指数**: CNN Fear & Greed
- **内部交易**: SEC EDGAR

---

**Disclaimer:** This tool is for informational purposes only and does NOT constitute financial advice.
