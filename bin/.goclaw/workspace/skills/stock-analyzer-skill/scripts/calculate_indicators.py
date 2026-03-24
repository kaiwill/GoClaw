#!/usr/bin/env python3
"""
技术指标计算脚本

功能：基于股票历史数据计算常用技术分析指标
"""

import json
import sys
import argparse
import numpy as np
import pandas as pd


def calculate_ma(data, period):
    """
    计算移动平均线

    参数:
        data: 价格序列
        period: 周期

    返回:
        移动平均线序列
    """
    return data.rolling(window=period).mean()


def calculate_ema(data, period):
    """
    计算指数移动平均线

    参数:
        data: 价格序列
        period: 周期

    返回:
        EMA序列
    """
    return data.ewm(span=period, adjust=False).mean()


def calculate_macd(close_prices, fast=12, slow=26, signal=9):
    """
    计算MACD指标

    参数:
        close_prices: 收盘价序列
        fast: 快线周期（默认12）
        slow: 慢线周期（默认26）
        signal: 信号线周期（默认9）

    返回:
        包含DIF、DEA、MACD柱的字典
    """
    ema_fast = calculate_ema(close_prices, fast)
    ema_slow = calculate_ema(close_prices, slow)

    dif = ema_fast - ema_slow
    dea = calculate_ema(dif, signal)
    macd = (dif - dea) * 2  # MACD柱

    return {
        'dif': dif.tolist(),
        'dea': dea.tolist(),
        'macd': macd.tolist()
    }


def calculate_rsi(close_prices, periods=[6, 12, 24]):
    """
    计算RSI相对强弱指标

    参数:
        close_prices: 收盘价序列
        periods: 计算周期列表

    返回:
        包含不同周期RSI的字典
    """
    rsi_values = {}

    for period in periods:
        delta = close_prices.diff()

        # 计算涨跌
        gain = delta.where(delta > 0, 0)
        loss = -delta.where(delta < 0, 0)

        # 计算平均涨跌
        avg_gain = gain.rolling(window=period).mean()
        avg_loss = loss.rolling(window=period).mean()

        # 避免除以零
        rs = avg_gain / avg_loss.where(avg_loss != 0, 1e-10)
        rsi = 100 - (100 / (1 + rs))

        rsi_values[f'rsi{period}'] = rsi.tolist()

    return rsi_values


def calculate_bollinger_bands(close_prices, period=20, std_dev=2):
    """
    计算布林带

    参数:
        close_prices: 收盘价序列
        period: 周期（默认20）
        std_dev: 标准差倍数（默认2）

    返回:
        包含上轨、中轨、下轨的字典
    """
    middle = calculate_ma(close_prices, period)
    std = close_prices.rolling(window=period).std()

    upper = middle + (std * std_dev)
    lower = middle - (std * std_dev)

    return {
        'upper': upper.tolist(),
        'middle': middle.tolist(),
        'lower': lower.tolist(),
        'band_width': ((upper - lower) / middle * 100).tolist()
    }


def calculate_volume_indicators(historical_data):
    """
    计算成交量指标

    参数:
        historical_data: 历史数据列表

    返回:
        包含成交量指标的字典
    """
    df = pd.DataFrame(historical_data)

    # 5日平均成交量
    vol_ma5 = calculate_ma(df['volume'], 5)

    # 量比 = 当日成交量 / 5日平均成交量
    volume_ratio = df['volume'] / vol_ma5.where(vol_ma5 != 0, 1e-10)

    return {
        'volume_ma5': vol_ma5.tolist(),
        'volume_ratio': volume_ratio.tolist()
    }


def main():
    parser = argparse.ArgumentParser(description='计算股票技术指标')
    parser.add_argument('--data_file', required=True, help='股票数据JSON文件路径')
    parser.add_argument('--output', default='indicators.json', help='输出文件名（默认indicators.json）')

    args = parser.parse_args()

    try:
        # 读取数据
        print(f"正在读取数据文件: {args.data_file}")
        with open(args.data_file, 'r', encoding='utf-8') as f:
            stock_data = json.load(f)

        historical_data = stock_data['historical']

        if len(historical_data) < 30:
            print(f"警告: 历史数据仅 {len(historical_data)} 条，建议至少30条以获得准确指标")

        # 转换为DataFrame
        df = pd.DataFrame(historical_data)
        df = df.sort_values('date')

        print("开始计算技术指标...")

        # 计算移动平均线
        print("  - 计算移动平均线（MA5, MA10, MA20, MA60）...")
        ma_data = {
            'ma5': calculate_ma(df['close'], 5).tolist(),
            'ma10': calculate_ma(df['close'], 10).tolist(),
            'ma20': calculate_ma(df['close'], 20).tolist(),
            'ma60': calculate_ma(df['close'], 60).tolist() if len(df) >= 60 else []
        }

        # 计算MACD
        print("  - 计算MACD指标...")
        macd_data = calculate_macd(df['close'])

        # 计算RSI
        print("  - 计算RSI指标...")
        rsi_data = calculate_rsi(df['close'])

        # 计算布林带
        print("  - 计算布林带...")
        bollinger_data = calculate_bollinger_bands(df['close'])

        # 计算成交量指标
        print("  - 计算成交量指标...")
        volume_data = calculate_volume_indicators(historical_data)

        # 提取最新指标值（用于快速参考）
        latest_date = df['date'].iloc[-1]
        latest_close = df['close'].iloc[-1]

        latest_indicators = {
            'date': latest_date,
            'price': latest_close,
            'ma5': ma_data['ma5'][-1] if ma_data['ma5'] else None,
            'ma10': ma_data['ma10'][-1] if ma_data['ma10'] else None,
            'ma20': ma_data['ma20'][-1] if ma_data['ma20'] else None,
            'ma60': ma_data['ma60'][-1] if ma_data['ma60'] else None,
            'dif': macd_data['dif'][-1] if macd_data['dif'] else None,
            'dea': macd_data['dea'][-1] if macd_data['dea'] else None,
            'macd': macd_data['macd'][-1] if macd_data['macd'] else None,
            'rsi6': rsi_data['rsi6'][-1] if rsi_data['rsi6'] else None,
            'rsi12': rsi_data['rsi12'][-1] if rsi_data['rsi12'] else None,
            'rsi24': rsi_data['rsi24'][-1] if rsi_data['rsi24'] else None,
            'bb_upper': bollinger_data['upper'][-1] if bollinger_data['upper'] else None,
            'bb_middle': bollinger_data['middle'][-1] if bollinger_data['middle'] else None,
            'bb_lower': bollinger_data['lower'][-1] if bollinger_data['lower'] else None,
            'volume_ratio': volume_data['volume_ratio'][-1] if volume_data['volume_ratio'] else None
        }

        # 组装输出数据
        output_data = {
            'symbol': stock_data['symbol'],
            'latest': latest_indicators,
            'indicators': {
                'ma': ma_data,
                'macd': macd_data,
                'rsi': rsi_data,
                'bollinger': bollinger_data,
                'volume': volume_data
            },
            'calculate_time': stock_data.get('fetch_time', '')
        }

        # 保存结果
        with open(args.output, 'w', encoding='utf-8') as f:
            json.dump(output_data, f, ensure_ascii=False, indent=2)

        print(f"\n技术指标计算完成！")
        print(f"结果已保存到 {args.output}")
        print(f"\n最新指标值（{latest_date}）:")
        print(f"  收盘价: {latest_close:.2f}")
        print(f"  MA5/MA10/MA20: {latest_indicators['ma5']:.2f} / {latest_indicators['ma10']:.2f} / {latest_indicators['ma20']:.2f}")
        print(f"  MACD: DIF={latest_indicators['dif']:.4f}, DEA={latest_indicators['dea']:.4f}, MACD={latest_indicators['macd']:.4f}")
        print(f"  RSI(6/12/24): {latest_indicators['rsi6']:.2f} / {latest_indicators['rsi12']:.2f} / {latest_indicators['rsi24']:.2f}")
        print(f"  布林带: 上={latest_indicators['bb_upper']:.2f}, 中={latest_indicators['bb_middle']:.2f}, 下={latest_indicators['bb_lower']:.2f}")
        print(f"  量比: {latest_indicators['volume_ratio']:.2f}")

        return 0

    except FileNotFoundError:
        print(f"错误: 找不到数据文件 {args.data_file}", file=sys.stderr)
        return 1
    except Exception as e:
        print(f"错误: {str(e)}", file=sys.stderr)
        import traceback
        traceback.print_exc()
        return 1


if __name__ == '__main__':
    sys.exit(main())
