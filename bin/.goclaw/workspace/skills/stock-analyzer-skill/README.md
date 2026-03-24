# Stock Analyzer Skill

全球股票综合分析工具，支持A股、港股、美股、加密货币分析和投资组合管理。

## 功能特性

### 多市场支持

| 市场 | 代码示例 | 数据源 | 分析维度 |
|------|---------|--------|---------|
| A股沪市 | 600519 | 东方财富 | 基本面+资金面 |
| A股深市 | 300750 | 东方财富 | 基本面+资金面 |
| 北交所 | 830799 | 东方财富 | 基本面+资金面 |
| 港股 | 00700 | 东方财富 | 基本面+资金面 |
| 美股 | AAPL, TSLA | Yahoo Finance | 8维度深度分析 |
| 加密货币 | BTC-USD | Yahoo Finance | 3维度分析 |

### 美股8维度分析

1. **Earnings Surprise** - 收益惊喜 (30%)
2. **Fundamentals** - 基本面 (20%)
3. **Analyst Sentiment** - 分析师情绪 (20%)
4. **Historical Patterns** - 历史模式 (10%)
5. **Market Context** - 市场环境 (10%)
6. **Sector Performance** - 行业表现 (15%)
7. **Momentum** - 动量 (15%)
8. **Sentiment** - 情绪分析 (10%)

### 风险评估功能

- **突发新闻预警**: 扫描战争、衰退、制裁等关键词
- **地缘政治风险**: 台海、中国、俄乌、中东、银行危机
- **避险资产追踪**: GLD(黄金)、TLT(国债)、UUP(美元)
- **集中度警告**: 单一资产超过30%自动提醒

### 投资组合管理

- 创建多个投资组合
- 添加股票/加密货币
- 实时计算盈亏
- 定期收益报告

## 快速开始

### 分析A股

```bash
./analyze.sh 贵州茅台
./analyze.sh 600519 --market sh
```

### 分析港股

```bash
./analyze.sh 腾讯控股
./analyze.sh 00700 --market hk
```

### 分析美股

```bash
./analyze.sh AAPL
./analyze.sh TSLA NVDA
```

### 分析加密货币

```bash
./analyze.sh BTC-USD
./analyze.sh ETH-USD SOL-USD
```

### 投资组合管理

```bash
# 创建组合
./analyze.sh portfolio create "我的组合"

# 添加资产
./analyze.sh portfolio add AAPL --quantity 100 --cost 150
./analyze.sh portfolio add BTC-USD --quantity 0.5 --cost 40000

# 查看持仓
./analyze.sh portfolio show
```

## 输出示例

### A股分析

```
═══════════════════════════════════════════════════════════════
  📉 贵州茅台 (600519) - A股/港股分析
═════════════════════════════════════════════════════════════════

💰 当前价格: 1440.11
📉 涨跌额:   -14.91
📉 涨跌幅:   -1.02%

────────────────────────────────────────────────────────────────────

📊 基本信息
  今开: 1450.00
  昨收: 1455.02
  最高: 1457.00
  最低: 1436.66
  涨停: 1600.52
  跌停: 1309.52

────────────────────────────────────────────────────────────────────

💡 综合分析
  基本面评分: 5/10
  资金面评分: 5/10
  综合评分: 10/20
  ✅ 投资建议: 推荐买入

────────────────────────────────────────────────────────────────────

🎯 买卖价位建议
  建仓价位: 1396.91
  目标价位: 1584.12
  止损价位: 1324.90
```

### 美股分析

```
═══════════════════════════════════════════════════════════════
  AAPL - Apple Inc.
═════════════════════════════════════════════════════════════════

📊 8维度分析结果

1. Earnings Surprise: +8.2% beat (Score: +0.7)
2. Fundamentals: P/E 28.5, Margin 26% (Score: +0.3)
3. Analyst Sentiment: BUY, 15% upside (Score: +0.7)
4. Historical Patterns: 3/4 beats (Score: +0.5)
5. Market Context: VIX 18, Bull market (Score: +0.2)
6. Sector Performance: Outperforming (Score: +0.3)
7. Momentum: RSI 55, Near 52w high (Score: 0.0)
8. Sentiment: Fear & Greed 62 (Score: -0.1)

────────────────────────────────────────────────────────────────────

📈 综合评分: 2.6/5.0
✅ Recommendation: BUY (Confidence: 75%)

────────────────────────────────────────────────────────────────────

⚠️ Caveats:
- Earnings in 12 days - high volatility expected
- RSI near overbought territory (55)
```

## 目录结构

```
stock-analyzer-skill/
├── index.js              # Node.js 统一入口
├── analyze.sh            # Shell 入口脚本
├── package.json          # Node.js 配置
├── skill.json            # 技能定义
├── SKILL.md              # 技能文档
├── README.md             # 使用指南
├── scripts/
│   ├── fetch_stock.py    # A股/港股数据获取
│   ├── requirements.txt  # A股/港股依赖
│   └── us/
│       ├── analyze_stock.py  # 美股/加密货币分析 (8维度)
│       ├── portfolio.py      # 投资组合管理
│       └── requirements.txt  # 美股依赖
├── assets/
│   ├── report_template.html  # HTML报告模板
│   └── report_template.md    # Markdown报告模板
└── references/
    └── eastmoney_guide.md    # 东方财富使用指南
```

## 安装依赖

### A股/港股依赖

```bash
cd scripts
pip install -r requirements.txt
```

### 美股/加密货币依赖

```bash
cd scripts/us
pip install -r requirements.txt
```

## 数据来源

| 数据类型 | 来源 |
|---------|------|
| A股/港股行情 | 东方财富网 |
| 美股/加密货币行情 | Yahoo Finance |
| 恐惧贪婪指数 | CNN |
| 内部交易 | SEC EDGAR |
| 突发新闻 | Google News RSS |

## 注意事项

1. **数据延迟**: 行情数据可能有15-20分钟延迟
2. **请求限制**: 避免短时间内频繁查询
3. **交易时间**: 仅在交易时间内能获取实时数据
4. **免责声明**: 所有数据仅供参考，不构成投资建议

## 版本历史

- **v2.0.0** - 合并美股8维度分析、加密货币分析、投资组合管理
- **v1.0.0** - 初始版本，支持A股/港股分析

---

**免责声明**: 本工具仅供信息参考，不构成任何投资建议。股市有风险，投资需谨慎。