# Stock Analyzer

## Description
全球股票综合分析工具。支持A股、港股、美股等东方财富覆盖的所有市场。根据用户输入的股票名称或代码，从东方财富网获取股票信息，进行基本面、新闻面、资金面三维分析，给出投资建议、买入价位和卖出价位。

## Commands
- **Analyze Stock** (analyze): 分析指定股票，生成投资分析报告
  - Parameters:
    - stock (required): 股票名称或代码（如：贵州茅台、600519、00700、AAPL、MU）
    - market (optional): 市场类型（sh-沪市、sz-深市、hk-港股、us-美股，默认自动识别）

## Features
- 支持A股、港股、美股等多市场分析
- 基本面、新闻面、资金面三维分析
- 生成Markdown和HTML两种格式的报告
- 提供投资建议、买入价位和卖出价位

## Security
- Basic analysis available to all users
- Advanced features restricted to authorized users defined in `USER.md`

## Configuration
- Uses Python scripts in `scripts/` directory for data fetching
- Supports both text and JSON output formats
- Generates HTML reports using templates in `assets/` directory

## Examples
- Analyze A-share: `analyze.sh --stock 600519 --market sh`
- Analyze HK stock: `analyze.sh --stock 00700 --market hk`
- Analyze US stock: `analyze.sh --stock AAPL --market us`