#!/usr/bin/env python3
"""
股票数据获取脚本

功能：从公开API获取股票实时行情和历史K线数据
"""

import json
import sys
import argparse
from datetime import datetime, timedelta
from urllib.request import urlopen
import ssl


def fetch_realtime_quote(symbol):
    """
    获取股票实时行情

    参数:
        symbol: 股票代码（如 000001, 600519）

    返回:
        实时行情数据字典
    """
    # 使用东方财富API - 实时行情
    if symbol.startswith('6'):
        market_code = '1'
    elif symbol.startswith('0') or symbol.startswith('3'):
        market_code = '0'
    else:
        market_code = '1'

    secid = f"{market_code}.{symbol}"
    url = f"http://push2.eastmoney.com/api/qt/stock/get?secid={secid}&fields=f43,f44,f45,f46,f47,f48,f58,f60,f107,f116,f117,f162,f168,f169,f170,f171"

    try:
        context = ssl._create_unverified_context()
        with urlopen(url, context=context, timeout=10) as response:
            data = json.loads(response.read().decode('utf-8'))

        if data.get('rc') != 0 or not data.get('data'):
            raise Exception(f"未找到股票代码: {symbol}")

        stock_info = data['data']
        realtime_data = {
            'symbol': symbol,
            'name': stock_info.get('f58', ''),
            'open': stock_info.get('f46', 0) / 100,
            'pre_close': stock_info.get('f60', 0) / 100,
            'current': stock_info.get('f43', 0) / 100,
            'high': stock_info.get('f44', 0) / 100,
            'low': stock_info.get('f45', 0) / 100,
            'volume': stock_info.get('f47', 0),
            'amount': stock_info.get('f48', 0),
            'change': stock_info.get('f169', 0) / 100,
            'change_percent': stock_info.get('f170', 0) / 100,
            'date': stock_info.get('f116', ''),
            'time': stock_info.get('f117', '')
        }

        if realtime_data['change'] == 0 and realtime_data['pre_close'] > 0:
            realtime_data['change'] = realtime_data['current'] - realtime_data['pre_close']
            realtime_data['change_percent'] = (realtime_data['change'] / realtime_data['pre_close']) * 100

        return realtime_data

    except Exception as e:
        raise Exception(f"获取实时行情失败: {str(e)}")


def fetch_historical_data(symbol, days=30):
    """
    获取股票历史K线数据

    参数:
        symbol: 股票代码
        days: 获取天数

    返回:
        历史K线数据列表
    """
    # 使用新浪财经历史数据接口（通过代理方式）
    # 格式：https://money.finance.sina.com.cn/quotes_service/api/json_v2.php/CN_MarketData.getKLineData
    # 参数：symbol=sh600519, scale=240（日线）, ma=no, datalen=天数

    if symbol.startswith('6'):
        symbol_with_prefix = f'sh{symbol}'
    elif symbol.startswith('0') or symbol.startswith('3'):
        symbol_with_prefix = f'sz{symbol}'
    else:
        symbol_with_prefix = f'sh{symbol}'

    url = f"https://money.finance.sina.com.cn/quotes_service/api/json_v2.php/CN_MarketData.getKLineData?symbol={symbol_with_prefix}&scale=240&ma=no&datalen={days}"

    try:
        context = ssl._create_unverified_context()
        with urlopen(url, context=context, timeout=30) as response:
            response_text = response.read().decode('gbk')

        if not response_text or response_text == 'null' or response_text.strip() == '':
            raise Exception(f"未找到股票历史数据: {symbol}")

        historical_data = json.loads(response_text)

        if not historical_data:
            raise Exception(f"未找到股票历史数据: {symbol}")

        # 数据格式化处理
        formatted_data = []
        for item in historical_data:
            formatted_item = {
                'date': item.get('day', ''),
                'open': float(item.get('open', 0)),
                'high': float(item.get('high', 0)),
                'low': float(item.get('low', 0)),
                'close': float(item.get('close', 0)),
                'volume': int(item.get('volume', 0))
            }
            formatted_data.append(formatted_item)

        return formatted_data

    except Exception as e:
        raise Exception(f"获取历史数据失败: {str(e)}")


def main():
    parser = argparse.ArgumentParser(description='获取股票数据')
    parser.add_argument('--symbol', required=True, help='股票代码（如000001, 600519）')
    parser.add_argument('--days', type=int, default=30, help='获取历史数据天数（默认30）')
    parser.add_argument('--output', default='stock_data.json', help='输出文件名（默认stock_data.json）')

    args = parser.parse_args()

    try:
        print(f"正在获取股票 {args.symbol} 的数据...")

        # 获取实时行情
        print("获取实时行情...")
        realtime_data = fetch_realtime_quote(args.symbol)
        print(f"当前价格: {realtime_data['current']} 涨跌幅: {realtime_data['change_percent']:.2f}%")

        # 获取历史数据
        print(f"获取最近 {args.days} 天的历史数据...")
        historical_data = fetch_historical_data(args.symbol, args.days)
        print(f"获取到 {len(historical_data)} 条K线数据")

        # 合并数据并保存
        output_data = {
            'symbol': args.symbol,
            'name': realtime_data['name'],
            'realtime': realtime_data,
            'historical': historical_data,
            'fetch_time': datetime.now().strftime('%Y-%m-%d %H:%M:%S')
        }

        with open(args.output, 'w', encoding='utf-8') as f:
            json.dump(output_data, f, ensure_ascii=False, indent=2)

        print(f"数据已保存到 {args.output}")
        print(f"数据包含:")
        print(f"  - 实时行情: 价格、涨跌、成交额等")
        print(f"  - 历史K线: {len(historical_data)} 天的开高低收数据")

        return 0

    except Exception as e:
        print(f"错误: {str(e)}", file=sys.stderr)
        return 1


if __name__ == '__main__':
    sys.exit(main())
