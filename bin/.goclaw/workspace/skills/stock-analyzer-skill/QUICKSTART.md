# Stock Analyzer Skill 快速使用指南

## 概述

`stock-analyzer-skill` 是一个全球股票综合分析工具，已成功整合美股8维度分析、加密货币分析和投资组合管理功能。

## 支持的市场

| 市场 | 代码示例 | 数据源 | 分析类型 |
|------|---------|--------|---------|
| A股沪市 | 600519 | 东方财富 | 基本面+资金面 |
| A股深市 | 300750 | 东方财富 | 基本面+资金面 |
| 北交所 | 830799 | 东方财富 | 基本面+资金面 |
| 港股 | 00700 | 东方财富 | 基本面+资金面 |
| 美股 | AAPL, TSLA | Yahoo Finance | 8维度深度分析 |
| 加密货币 | BTC-USD | Yahoo Finance | 3维度分析 |

## 快速开始

### 1. 安装依赖

```bash
# A股/港股依赖
cd /Users/haha/.goclaw/workspace/skills/stock-analyzer-skill/scripts
pip install -r requirements.txt

# 美股/加密货币依赖
cd us
pip install -r requirements.txt
```

### 2. 分析A股

```bash
./analyze.sh 贵州茅台
./analyze.sh 600519 --market sh
```

### 3. 分析港股

```bash
./analyze.sh 腾讯控股
./analyze.sh 00700 --market hk
```

### 4. 分析美股

```bash
# 单只股票
./analyze.sh AAPL
./analyze.sh TSLA

# 多只股票对比
./analyze.sh AAPL MSFT GOOGL
```

### 5. 分析加密货币

```bash
./analyze.sh BTC-USD
./analyze.sh ETH-USD SOL-USD
```

### 6. 投资组合管理

```bash
# 创建组合
./analyze.sh portfolio create "我的组合"

# 添加资产
./analyze.sh portfolio add AAPL --quantity 100 --cost 150
./analyze.sh portfolio add BTC-USD --quantity 0.5 --cost 40000

# 查看组合
./analyze.sh portfolio show

# 分析组合
node index.js --command analyze --stock AAPL --portfolio "我的组合"
```

## 美股8维度分析说明

| 维度 | 权重 | 说明 |
|------|------|------|
| Earnings Surprise | 30% | EPS超预期/不及预期 |
| Fundamentals | 20% | P/E、利润率、营收增长、负债 |
| Analyst Sentiment | 20% | 分析师评级、目标价 |
| Historical Patterns | 10% | 过去财报反应 |
| Market Context | 10% | VIX、SPY/QQQ趋势 |
| Sector Performance | 15% | 行业相对表现 |
| Momentum | 15% | RSI、52周区间、成交量 |
| Sentiment | 10% | 恐惧贪婪指数、做空比例、内部交易 |

## 风险评估功能

### 地缘政治风险检测

自动检测以下风险事件并调整置信度：
- 台海局势 → 半导体股影响
- 中美关系 → 科技/消费股影响
- 俄乌冲突 → 能源/材料股影响
- 中东局势 → 石油/军工股影响
- 银行危机 → 金融股影响

### 避险资产追踪

当 GLD(黄金)、TLT(国债)、UUP(美元) 同时上涨时，自动触发风险规避模式。

### 突发新闻预警

扫描Google News RSS获取24小时内的危机关键词。

## 在GoClaw中使用

### 通过HTTP API

```bash
# 启动GoClaw
./bin/goclaw daemon

# 分析A股
curl -X POST http://localhost:4096/agent \
  -H "Content-Type: application/json" \
  -d '{"message": "分析贵州茅台"}'

# 分析美股
curl -X POST http://localhost:4096/agent \
  -H "Content-Type: application/json" \
  -d '{"message": "分析苹果AAPL"}'

# 分析加密货币
curl -X POST http://localhost:4096/agent \
  -H "Content-Type: application/json" \
  -d '{"message": "分析比特币BTC-USD"}'
```

## 目录结构

```
stock-analyzer-skill/
├── index.js              # Node.js 统一入口
├── analyze.sh            # Shell 入口脚本
├── scripts/
│   ├── fetch_stock.py    # A股/港股数据获取
│   ├── requirements.txt  # A股/港股依赖
│   └── us/
│       ├── analyze_stock.py  # 美股/加密货币分析 (8维度)
│       ├── portfolio.py      # 投资组合管理
│       └── requirements.txt  # 美股依赖
└── ...
```

## 测试命令

```bash
# 测试A股
node index.js --stock 600519 --market sh

# 测试港股
node index.js --stock 00700 --market hk

# 测试美股
node index.js --stock AAPL --market us

# 测试加密货币
node index.js --stock BTC-USD
```

## 注意事项

1. **数据延迟**: 行情数据可能有15-20分钟延迟
2. **请求限制**: 避免短时间内频繁查询
3. **交易时间**: 仅在交易时间内能获取实时数据
4. **Python版本**: 需要Python 3.10+

---

**免责声明**: 本工具仅供信息参考，不构成任何投资建议。股市有风险，投资需谨慎。