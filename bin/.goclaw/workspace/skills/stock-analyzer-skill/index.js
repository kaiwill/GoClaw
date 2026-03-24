#!/usr/bin/env node

/**
 * Stock Analyzer Skill - 统一入口
 * 支持：
 * - A股/港股/美股 (东方财富数据源)
 * - 美股深度分析 (Yahoo Finance, 8维度)
 * - 加密货币分析 (Top 20)
 * - 投资组合管理
 */

const { exec, spawn } = require('child_process');
const path = require('path');

const SCRIPTS_DIR = path.join(__dirname, 'scripts');
const CN_SCRIPT = path.join(SCRIPTS_DIR, 'fetch_stock.py');
const US_SCRIPT = path.join(SCRIPTS_DIR, 'us', 'analyze_stock.py');
const US_PORTFOLIO_SCRIPT = path.join(SCRIPTS_DIR, 'us', 'portfolio.py');

// ============================================================================
// 工具函数
// ============================================================================

function formatNumber(num) {
  if (num === null || num === undefined || isNaN(num)) {
    return '-';
  }
  return num.toFixed(2);
}

// ============================================================================
// Python 执行工具
// ============================================================================

async function executePython(scriptPath, args, options = {}) {
    return new Promise((resolve, reject) => {
        const pythonCmd = process.env.PYTHON_CMD || 'python3';
        const cwd = options.cwd || path.dirname(scriptPath);
        
        const cmd = exec(
            `${pythonCmd} "${scriptPath}" ${args.join(' ')}`,
            {
                cwd: cwd,
                maxBuffer: 20 * 1024 * 1024, // 20MB
                timeout: options.timeout || 60000, // 60s timeout
            },
            (error, stdout, stderr) => {
                if (error) {
                    reject({
                        success: false,
                        error: error.message,
                        stderr: stderr
                    });
                } else {
                    try {
                        const result = JSON.parse(stdout);
                        resolve(result);
                    } catch (e) {
                        // 如果输出不是JSON，返回原始文本
                        resolve({
                            success: true,
                            output: stdout.trim(),
                            stderr: stderr
                        });
                    }
                }
            }
        );
    });
}

// ============================================================================
// 市场识别
// ============================================================================

// 判断是否为美股代码
function isUSStock(ticker) {
    if (!ticker) return false;
    const upper = ticker.toUpperCase();
    
    // 美股代码特征：纯大写字母，1-5个字符
    if (/^[A-Z]{1,5}$/.test(upper)) {
        // 排除可能的A股代码混淆
        const cnPrefixes = ['SH', 'SZ', 'BJ', 'HK'];
        if (!cnPrefixes.includes(upper)) {
            return true;
        }
    }
    
    // 加密货币
    if (upper.endsWith('-USD')) {
        return true;
    }
    
    return false;
}

// 判断是否为加密货币
function isCrypto(ticker) {
    if (!ticker) return false;
    return ticker.toUpperCase().endsWith('-USD');
}

// 规范化市场参数
function normalizeMarket(market) {
    if (!market || market === '') {
        return 'auto';
    }
    
    const m = market.toLowerCase().trim();
    
    // 支持的格式：sh, sh-沪市, 沪市, 上海, sha, shanghai 等
    if (m.startsWith('sh') || m.includes('沪') || m === '上海' || m === 'sha' || m === 'shanghai') {
        return 'sh';
    }
    if (m.startsWith('sz') || m.includes('深') || m === '深圳' || m === 'sze' || m === 'shenzhen') {
        return 'sz';
    }
    if (m.startsWith('bj') || m.includes('京') || m === '北京' || m === 'bse') {
        return 'bj';
    }
    if (m.startsWith('hk') || m.includes('港') || m === '香港' || m === 'hongkong') {
        return 'hk';
    }
    if (m.startsWith('us') || m.includes('美') || m === '美国' || m === 'usa') {
        return 'us';
    }
    
    // 如果是有效的值，直接返回
    const validMarkets = ['sh', 'sz', 'bj', 'hk', 'us', 'auto'];
    if (validMarkets.includes(m)) {
        return m;
    }
    
    // 默认自动识别
    return 'auto';
}

// ============================================================================
// A股/港股分析 (东方财富)
// ============================================================================

async function analyzeCNStock(stock, market = 'auto') {
    const normalizedMarket = normalizeMarket(market);
    console.log(`[INFO] 开始分析A股/港股: ${stock} (市场: ${normalizedMarket})`);
    
    try {
        // 第一步：获取股票数据
        const args = [stock, '--market', normalizedMarket, '--output', 'json'];
        console.log(`[DEBUG] 执行命令: ${CN_SCRIPT} ${args.join(' ')}`);
        const result = await executePython(CN_SCRIPT, args);
        console.log(`[DEBUG] 股票数据获取结果: ${JSON.stringify(result)}`);
        
        if (!result.success) {
            console.log(`[DEBUG] 获取股票数据失败: ${JSON.stringify(result)}`);
            return {
                success: false,
                stock: stock,
                market: normalizedMarket,
                error: result.error || '获取数据失败',
                stderr: result.stderr
            };
        }
        
        const data = result.data || {};
        const name = data.name || stock;
        const price = data.price || '-';
        const change = data.change || '-';
        const changePercent = data.change_percent || '-';
        
        console.log(`[INFO] 成功获取数据: ${name}`);
        console.log(`[INFO] 价格: ${price}, 涨跌: ${change} (${changePercent})`);
        
        // 第二步：获取历史数据用于技术指标计算
        // 首先需要将股票名称转换为代码
        let stockCode = stock;
        if (!/^\d+$/.test(stock)) {
            // 如果是名称而不是代码，需要先搜索
            const searchArgs = [stock, '--market', normalizedMarket, '--output', 'json'];
            const searchResult = await executePython(CN_SCRIPT, searchArgs);
            if (searchResult.success && searchResult.data) {
                stockCode = searchResult.data.code;
            }
        }
        
        const fetchArgs = ['--symbol', stockCode, '--days', '30', '--output', 'stock_data.json'];
        const fetchScriptPath = path.join(__dirname, 'scripts', 'fetch_stock_data.py');
        console.log(`[DEBUG] 执行命令: ${fetchScriptPath} ${fetchArgs.join(' ')}`);
        const fetchResult = await executePython(fetchScriptPath, fetchArgs);
        console.log(`[DEBUG] 历史数据获取结果: ${JSON.stringify(fetchResult)}`);
        
        if (fetchResult.success) {
            // 第三步：计算技术指标
            const indicatorsArgs = ['--data_file', 'stock_data.json', '--output', 'indicators.json'];
            const indicatorsScriptPath = path.join(__dirname, 'scripts', 'calculate_indicators.py');
            console.log(`[DEBUG] 执行命令: ${indicatorsScriptPath} ${indicatorsArgs.join(' ')}`);
            const indicatorsResult = await executePython(indicatorsScriptPath, indicatorsArgs, { cwd: SCRIPTS_DIR });
            console.log(`[DEBUG] 技术指标计算结果: ${JSON.stringify(indicatorsResult)}`);
            
            // 检查生成的文件
            const fs = require('fs');
            if (fs.existsSync(path.join(SCRIPTS_DIR, 'indicators.json'))) {
                console.log(`[DEBUG] 成功生成 indicators.json 文件`);
                try {
                    const indicatorsData = JSON.parse(fs.readFileSync(path.join(SCRIPTS_DIR, 'indicators.json'), 'utf8'));
                    console.log(`[DEBUG] 指标数据结构: ${JSON.stringify(Object.keys(indicatorsData))}`);
                } catch (e) {
                    console.log(`[DEBUG] JSON 解析错误: ${e.message}`);
                    // 尝试修复 NaN 值
                    const content = fs.readFileSync(path.join(SCRIPTS_DIR, 'indicators.json'), 'utf8');
                    const fixedContent = content.replace(/NaN/g, 'null');
                    fs.writeFileSync(path.join(SCRIPTS_DIR, 'indicators_fixed.json'), fixedContent);
                    console.log(`[DEBUG] 已修复 NaN 值并保存到 indicators_fixed.json`);
                }
            } else {
                console.log(`[DEBUG] 未找到 indicators.json 文件`);
            }
            
            if (indicatorsResult.success) {
                // 第四步：生成包含技术指标的综合报告
                // 读取修复后的指标数据
                const fs = require('fs');
                let indicatorsData = indicatorsResult.data;
                const indicatorsFile = path.join(SCRIPTS_DIR, 'indicators_fixed.json');
                if (fs.existsSync(indicatorsFile)) {
                    try {
                        const indicatorsContent = fs.readFileSync(indicatorsFile, 'utf8');
                        // 修复 NaN 值
                        const fixedContent = indicatorsContent.replace(/NaN/g, 'null');
                        indicatorsData = JSON.parse(fixedContent);
                    } catch (e) {
                        console.error('[ERROR] 读取指标数据失败:', e.message);
                    }
                }
                const report = generateCNReportWithIndicators(data, indicatorsData);
                
                // 清理临时文件
                try {
                    require('fs').unlinkSync(path.join(SCRIPTS_DIR, 'stock_data.json'));
                    require('fs').unlinkSync(path.join(SCRIPTS_DIR, 'indicators.json'));
                } catch (e) {
                    // 忽略清理错误
                }
                
                return {
                    success: true,
                    stock: stock,
                    market: normalizedMarket,
                    source: '东方财富',
                    data: data,
                    indicators: indicatorsData,
                    report: report,
                    urls: data.urls || {}
                };
            } else {
                console.log(`[DEBUG] 计算技术指标失败: ${JSON.stringify(indicatorsResult)}`);
            }
        } else {
            console.log(`[DEBUG] 获取历史数据失败: ${JSON.stringify(fetchResult)}`);
        }
        
        // 如果技术指标计算失败，生成基本报告
        const report = generateCNReport(data);
        
        return {
            success: true,
            stock: stock,
            market: normalizedMarket,
            source: '东方财富',
            data: data,
            report: report,
            urls: data.urls || {}
        };
    } catch (error) {
        console.error(`[ERROR] 分析A股/港股失败: ${error.message}`);
        console.error(`[ERROR] 错误堆栈: ${error.stack}`);
        return {
            success: false,
            stock: stock,
            market: normalizedMarket,
            error: error.message
        };
    }
}

function generateCNReport(data) {
    const name = data.name || '未知';
    const code = data.code || '-';
    const price = data.price || '-';
    const change = data.change || '-';
    const changePercent = data.change_percent || '-';
    const pe = data.pe || '-';
    const pb = data.pb || '-';
    const marketCap = data.market_cap || '-';
    const amount = data.amount || '-';
    
    // 判断涨跌
    let trend = '➡️';
    if (change !== '-' && parseFloat(change) > 0) {
        trend = '📈';
    } else if (change !== '-' && parseFloat(change) < 0) {
        trend = '📉';
    }
    
    // 基本面评分
    let fundamentalScore = 5;
    let fundamentalComment = '基本面中性';
    
    if (pe !== '-' && parseFloat(pe) < 20) {
        fundamentalScore += 2;
        fundamentalComment = 'PE合理，估值较低';
    } else if (pe !== '-' && parseFloat(pe) > 50) {
        fundamentalScore -= 1;
        fundamentalComment = 'PE较高，估值偏高';
    }
    
    // 资金面评分
    let fundScore = 5;
    let fundComment = '资金面中性';
    
    if (data.main_ratio) {
        const ratio = parseFloat(data.main_ratio);
        if (ratio > 10) {
            fundScore += 3;
            fundComment = '主力强势介入';
        } else if (ratio > 5) {
            fundScore += 2;
            fundComment = '主力温和流入';
        } else if (ratio < -5) {
            fundScore -= 2;
            fundComment = '主力明显流出';
        }
    }
    
    // 综合评分
    const totalScore = fundamentalScore + fundScore;
    let recommendation = '观望';
    let recommendationEmoji = '⚠️';
    
    if (totalScore >= 10) {
        recommendation = '推荐买入';
        recommendationEmoji = '✅';
    } else if (totalScore >= 8) {
        recommendation = '可以关注';
        recommendationEmoji = '👍';
    } else if (totalScore <= 4) {
        recommendation = '谨慎操作';
        recommendationEmoji = '❌';
    }
    
    // 买卖价位建议
    let buyPrice = '-';
    let targetPrice = '-';
    let stopPrice = '-';
    
    if (price !== '-') {
        const currentPrice = parseFloat(price);
        buyPrice = (currentPrice * 0.97).toFixed(2);
        targetPrice = (currentPrice * 1.1).toFixed(2);
        stopPrice = (currentPrice * 0.92).toFixed(2);
    }
    
    return `
═══════════════════════════════════════════════════════════════
  ${trend} ${name} (${code}) - A股/港股分析
═════════════════════════════════════════════════════════════════

💰 当前价格: ${price}
${trend} 涨跌额:   ${change}
${trend} 涨跌幅:   ${changePercent}

────────────────────────────────────────────────────────────────────

📊 基本信息
  今开: ${data.open || '-'}
  昨收: ${data.prev_close || '-'}
  最高: ${data.high || '-'}
  最低: ${data.low || '-'}
  涨停: ${data.limit_up || '-'}
  跌停: ${data.limit_down || '-'}

────────────────────────────────────────────────────────────────────

💼 市场数据
  成交额: ${amount}
  总市值: ${marketCap}
  流通市值: ${data.float_cap || '-'}
  换手率: ${data.turnover || '-'}
  量比: ${data.volume_ratio || '-'}

────────────────────────────────────────────────────────────────────

📈 估值指标
  市盈率(PE): ${pe}
  市净率(PB): ${pb}

────────────────────────────────────────────────────────────────────

💡 综合分析
  基本面评分: ${fundamentalScore}/10
  ${fundamentalComment}
  
  资金面评分: ${fundScore}/10
  ${fundComment}
  
  综合评分: ${totalScore}/20
  ${recommendationEmoji} 投资建议: ${recommendation}

────────────────────────────────────────────────────────────────────

🎯 买卖价位建议
  建仓价位: ${buyPrice}
  目标价位: ${targetPrice}
  止损价位: ${stopPrice}

═════════════════════════════════════════════════════════════════
`;
}

function generateCNReportWithIndicators(data, indicators) {
    const name = data.name || '未知';
    const code = data.code || '-';
    const price = data.price || '-';
    const change = data.change || '-';
    const changePercent = data.change_percent || '-';
    const pe = data.pe || '-';
    const pb = data.pb || '-';
    const marketCap = data.market_cap || '-';
    const amount = data.amount || '-';
    
    // 判断涨跌
    let trend = '➡️';
    if (change !== '-' && parseFloat(change) > 0) {
        trend = '📈';
    } else if (change !== '-' && parseFloat(change) < 0) {
        trend = '📉';
    }
    
    // 基本面评分
    let fundamentalScore = 5;
    let fundamentalComment = '基本面中性';
    
    if (pe !== '-' && parseFloat(pe) < 20) {
        fundamentalScore += 2;
        fundamentalComment = 'PE合理，估值较低';
    } else if (pe !== '-' && parseFloat(pe) > 50) {
        fundamentalScore -= 1;
        fundamentalComment = 'PE较高，估值偏高';
    }
    
    // 资金面评分
    let fundScore = 5;
    let fundComment = '资金面中性';
    
    if (data.main_ratio) {
        const ratio = parseFloat(data.main_ratio);
        if (ratio > 10) {
            fundScore += 3;
            fundComment = '主力强势介入';
        } else if (ratio > 5) {
            fundScore += 2;
            fundComment = '主力温和流入';
        } else if (ratio < -5) {
            fundScore -= 2;
            fundComment = '主力明显流出';
        }
    }
    
    // 技术面评分
    let technicalScore = 5;
    let technicalComment = '技术面中性';
    
    if (indicators && indicators.latest) {
        const latest = indicators.latest;
        
        // 移动平均线分析
        if (latest.ma5 && latest.ma10 && latest.ma20 && latest.ma60) {
            const ma5 = parseFloat(latest.ma5);
            const ma10 = parseFloat(latest.ma10);
            const ma20 = parseFloat(latest.ma20);
            const ma60 = parseFloat(latest.ma60);
            const close = parseFloat(price);
            
            // 简单的均线系统分析
            if (close > ma5 && ma5 > ma10 && ma10 > ma20 && ma20 > ma60) {
                technicalScore += 2;
                technicalComment = '多头排列，趋势向上';
            } else if (close < ma5 && ma5 < ma10 && ma10 < ma20 && ma20 < ma60) {
                technicalScore -= 2;
                technicalComment = '空头排列，趋势向下';
            }
        }
        
        // MACD分析
        if (latest.dif && latest.dea && latest.macd) {
            const dif = parseFloat(latest.dif);
            const dea = parseFloat(latest.dea);
            const macd = parseFloat(latest.macd);
            
            if (dif > dea && macd > 0) {
                technicalScore += 1;
                technicalComment += ', MACD金叉';
            } else if (dif < dea && macd < 0) {
                technicalScore -= 1;
                technicalComment += ', MACD死叉';
            }
        }
        
        // RSI分析
        if (latest.rsi6) {
            const rsi = parseFloat(latest.rsi6);
            if (rsi > 70) {
                technicalScore -= 1;
                technicalComment += ', RSI超买';
            } else if (rsi < 30) {
                technicalScore += 1;
                technicalComment += ', RSI超卖';
            }
        }
    }
    
    // 综合评分
    const totalScore = fundamentalScore + fundScore + technicalScore;
    let recommendation = '观望';
    let recommendationEmoji = '⚠️';
    
    if (totalScore >= 15) {
        recommendation = '推荐买入';
        recommendationEmoji = '✅';
    } else if (totalScore >= 10) {
        recommendation = '可以关注';
        recommendationEmoji = '👍';
    } else if (totalScore <= 5) {
        recommendation = '谨慎操作';
        recommendationEmoji = '❌';
    }
    
    // 买卖价位建议
    let buyPrice = '-';
    let targetPrice = '-';
    let stopPrice = '-';
    
    if (price !== '-') {
        const currentPrice = parseFloat(price);
        buyPrice = (currentPrice * 0.97).toFixed(2);
        targetPrice = (currentPrice * 1.1).toFixed(2);
        stopPrice = (currentPrice * 0.92).toFixed(2);
    }
    
    // 从指标数据中提取最新值
    let ma5 = '-', ma10 = '-', ma20 = '-', ma60 = '-';
    let dif = '-', dea = '-', macd = '-';
    let rsi6 = '-', rsi12 = '-', rsi24 = '-';
    let bb_upper = '-', bb_middle = '-', bb_lower = '-';
    let volume_ratio = '-';
    
    if (indicators && indicators.latest) {
        const latest = indicators.latest;
        ma5 = (latest.ma5 !== null && !isNaN(latest.ma5)) ? latest.ma5 : '-';
        ma10 = (latest.ma10 !== null && !isNaN(latest.ma10)) ? latest.ma10 : '-';
        ma20 = (latest.ma20 !== null && !isNaN(latest.ma20)) ? latest.ma20 : '-';
        ma60 = (latest.ma60 !== null && !isNaN(latest.ma60)) ? latest.ma60 : '-';
        dif = (latest.dif !== null && !isNaN(latest.dif)) ? latest.dif : '-';
        dea = (latest.dea !== null && !isNaN(latest.dea)) ? latest.dea : '-';
        macd = (latest.macd !== null && !isNaN(latest.macd)) ? latest.macd : '-';
        rsi6 = (latest.rsi6 !== null && !isNaN(latest.rsi6)) ? latest.rsi6 : '-';
        rsi12 = (latest.rsi12 !== null && !isNaN(latest.rsi12)) ? latest.rsi12 : '-';
        rsi24 = (latest.rsi24 !== null && !isNaN(latest.rsi24)) ? latest.rsi24 : '-';
        bb_upper = (latest.bb_upper !== null && !isNaN(latest.bb_upper)) ? latest.bb_upper : '-';
        bb_middle = (latest.bb_middle !== null && !isNaN(latest.bb_middle)) ? latest.bb_middle : '-';
        bb_lower = (latest.bb_lower !== null && !isNaN(latest.bb_lower)) ? latest.bb_lower : '-';
        volume_ratio = (latest.volume_ratio !== null && !isNaN(latest.volume_ratio)) ? latest.volume_ratio : '-';
    } else if (indicators && indicators.indicators && indicators.indicators.ma) {
        // 如果没有 latest 字段，尝试从 indicators.ma 中提取
        const ma = indicators.indicators.ma;
        if (ma.ma5 && ma.ma5.length > 0) {
            const lastVal = ma.ma5[ma.ma5.length - 1];
            ma5 = (lastVal !== null && !isNaN(lastVal)) ? lastVal : '-';
        }
        if (ma.ma10 && ma.ma10.length > 0) {
            const lastVal = ma.ma10[ma.ma10.length - 1];
            ma10 = (lastVal !== null && !isNaN(lastVal)) ? lastVal : '-';
        }
        if (ma.ma20 && ma.ma20.length > 0) {
            const lastVal = ma.ma20[ma.ma20.length - 1];
            ma20 = (lastVal !== null && !isNaN(lastVal)) ? lastVal : '-';
        }
        if (ma.ma60 && ma.ma60.length > 0) {
            const lastVal = ma.ma60[ma.ma60.length - 1];
            ma60 = (lastVal !== null && !isNaN(lastVal)) ? lastVal : '-';
        }
    }
    
    return `
═══════════════════════════════════════════════════════════════
  ${trend} ${name} (${code}) - A股/港股综合分析
═════════════════════════════════════════════════════════════════

💰 当前价格: ${price}
${trend} 涨跌额:   ${change}
${trend} 涨跌幅:   ${changePercent}

────────────────────────────────────────────────────────────────────

📊 基本信息
  今开: ${data.open || '-'}
  昨收: ${data.prev_close || '-'}
  最高: ${data.high || '-'}
  最低: ${data.low || '-'}
  涨停: ${data.limit_up || '-'}
  跌停: ${data.limit_down || '-'}

────────────────────────────────────────────────────────────────────

💼 市场数据
  成交额: ${amount}
  总市值: ${marketCap}
  流通市值: ${data.float_cap || '-'}
  换手率: ${data.turnover || '-'}
  量比: ${data.volume_ratio || '-'}

────────────────────────────────────────────────────────────────────

📈 估值指标
  市盈率(PE): ${pe}
  市净率(PB): ${pb}

────────────────────────────────────────────────────────────────────

📊 技术指标摘要
  移动平均线: MA5=${formatNumber(ma5)}, MA10=${formatNumber(ma10)}, MA20=${formatNumber(ma20)}, MA60=${formatNumber(ma60)}
  MACD: DIF=${formatNumber(dif)}, DEA=${formatNumber(dea)}, MACD=${formatNumber(macd)}
  RSI(6/12/24): ${formatNumber(rsi6)}/${formatNumber(rsi12)}/${formatNumber(rsi24)}
  布林带: 上轨=${formatNumber(bb_upper)}, 中轨=${formatNumber(bb_middle)}, 下轨=${formatNumber(bb_lower)}
  量比: ${formatNumber(volume_ratio)}

────────────────────────────────────────────────────────────────────

💡 综合分析
  基本面评分: ${fundamentalScore}/10
  ${fundamentalComment}
  
  资金面评分: ${fundScore}/10
  ${fundComment}
  
  技术面评分: ${technicalScore}/10
  ${technicalComment}
  
  综合评分: ${totalScore}/30
  ${recommendationEmoji} 投资建议: ${recommendation}

────────────────────────────────────────────────────────────────────

🎯 买卖价位建议
  建仓价位: ${buyPrice}
  目标价位: ${targetPrice}
  止损价位: ${stopPrice}

═════════════════════════════════════════════════════════════════
`;
}

// ============================================================================
// 美股/加密货币分析 (Yahoo Finance, 8维度)
// ============================================================================

async function analyzeUSStock(ticker, options = {}) {
    console.log(`[INFO] 开始分析美股/加密货币: ${ticker}`);
    
    try {
        const args = [ticker];
        if (options.output) {
            args.push('--output', options.output);
        }
        if (options.verbose) {
            args.push('--verbose');
        }
        
        const result = await executePython(US_SCRIPT, args, { timeout: 120000 });
        
        if (result.success !== false) {
            console.log(`[INFO] 美股分析完成: ${ticker}`);
            return {
                success: true,
                ticker: ticker,
                source: 'Yahoo Finance (8维度分析)',
                data: result,
                report: result.output || result
            };
        } else {
            return {
                success: false,
                ticker: ticker,
                error: result.error || '分析失败',
                stderr: result.stderr
            };
        }
    } catch (error) {
        console.error(`[ERROR] 分析美股失败: ${error.message}`);
        return {
            success: false,
            ticker: ticker,
            error: error.message
        };
    }
}

// ============================================================================
// 投资组合管理
// ============================================================================

async function managePortfolio(action, options = {}) {
    console.log(`[INFO] 投资组合操作: ${action}`);
    
    try {
        const args = [action];
        
        switch (action) {
            case 'create':
                if (!options.name) throw new Error('需要指定组合名称');
                args.push(options.name);
                break;
            case 'list':
                break;
            case 'show':
                if (options.portfolio) {
                    args.push('--portfolio', options.portfolio);
                }
                break;
            case 'delete':
                if (!options.name) throw new Error('需要指定组合名称');
                args.push(options.name);
                break;
            case 'add':
                if (!options.ticker) throw new Error('需要指定股票代码');
                args.push(options.ticker);
                if (options.quantity) args.push('--quantity', options.quantity);
                if (options.cost) args.push('--cost', options.cost);
                if (options.portfolio) args.push('--portfolio', options.portfolio);
                break;
            case 'update':
                if (!options.ticker) throw new Error('需要指定股票代码');
                args.push(options.ticker);
                if (options.quantity) args.push('--quantity', options.quantity);
                if (options.cost) args.push('--cost', options.cost);
                if (options.portfolio) args.push('--portfolio', options.portfolio);
                break;
            case 'remove':
                if (!options.ticker) throw new Error('需要指定股票代码');
                args.push(options.ticker);
                if (options.portfolio) args.push('--portfolio', options.portfolio);
                break;
            default:
                throw new Error(`未知操作: ${action}`);
        }
        
        const result = await executePython(US_PORTFOLIO_SCRIPT, args);
        
        return {
            success: true,
            action: action,
            result: result
        };
    } catch (error) {
        console.error(`[ERROR] 投资组合操作失败: ${error.message}`);
        return {
            success: false,
            action: action,
            error: error.message
        };
    }
}

// ============================================================================
// 统一分析入口
// ============================================================================

async function analyzeStock(stock, market = 'auto') {
    const normalizedMarket = normalizeMarket(market);
    
    // 判断分析类型
    if (normalizedMarket === 'us' || (normalizedMarket === 'auto' && isUSStock(stock))) {
        // 美股或加密货币
        return analyzeUSStock(stock);
    } else {
        // A股或港股
        return analyzeCNStock(stock, normalizedMarket);
    }
}

// ============================================================================
// 主程序入口
// ============================================================================

if (require.main === module) {
    const args = process.argv.slice(2);
    let command = 'analyze';
    let stock = '';
    let market = 'auto';
    let portfolioOptions = {};
    let usOptions = {};
    
    // 解析命令行参数
    for (let i = 0; i < args.length; i++) {
        if (args[i] === '--command' && args[i + 1]) {
            command = args[i + 1];
            i++;
        } else if (args[i] === '--stock' && args[i + 1]) {
            stock = args[i + 1];
            i++;
        } else if (args[i] === '--market' && args[i + 1]) {
            market = args[i + 1];
            i++;
        } else if (args[i] === '--ticker' && args[i + 1]) {
            stock = args[i + 1];
            i++;
        } else if (args[i] === '--portfolio' && args[i + 1]) {
            portfolioOptions.portfolio = args[i + 1];
            i++;
        } else if (args[i] === '--name' && args[i + 1]) {
            portfolioOptions.name = args[i + 1];
            i++;
        } else if (args[i] === '--quantity' && args[i + 1]) {
            portfolioOptions.quantity = args[i + 1];
            i++;
        } else if (args[i] === '--cost' && args[i + 1]) {
            portfolioOptions.cost = args[i + 1];
            i++;
        } else if (args[i] === '--output' && args[i + 1]) {
            usOptions.output = args[i + 1];
            i++;
        } else if (args[i] === '--verbose') {
            usOptions.verbose = true;
        } else if (!args[i].startsWith('-') && !stock) {
            // 第一个非选项参数作为股票代码
            stock = args[i];
        }
    }
    
    // 执行命令
    async function run() {
        let result;
        
        switch (command) {
            case 'analyze':
            case 'analyse':
                if (!stock) {
                    console.error('[ERROR] 缺少必要参数: stock/ticker');
                    process.exit(1);
                }
                result = await analyzeStock(stock, market);
                break;
                
            case 'analyze-us':
            case 'us':
                if (!stock) {
                    console.error('[ERROR] 缺少必要参数: ticker');
                    process.exit(1);
                }
                result = await analyzeUSStock(stock, usOptions);
                break;
                
            case 'analyze-cn':
            case 'cn':
                if (!stock) {
                    console.error('[ERROR] 缺少必要参数: stock');
                    process.exit(1);
                }
                result = await analyzeCNStock(stock, market);
                break;
                
            case 'portfolio':
            case 'port':
                const action = args[1] || 'list';
                if (action !== 'list' && !portfolioOptions.name && !stock) {
                    console.error('[ERROR] 需要指定组合名称或股票代码');
                    process.exit(1);
                }
                if (stock && !portfolioOptions.ticker) {
                    portfolioOptions.ticker = stock;
                }
                result = await managePortfolio(action, portfolioOptions);
                break;
                
            default:
                console.error(`[ERROR] 未知命令: ${command}`);
                console.error('可用命令: analyze, analyze-us, analyze-cn, portfolio');
                process.exit(1);
        }
        
        console.log(JSON.stringify(result, null, 2));
        process.exit(result.success ? 0 : 1);
    }
    
    // 如果没有指定stock，从stdin读取
    if (!stock && command.startsWith('analyze')) {
        process.stdin.setEncoding('utf8');
        process.stdin.on('readable', () => {
            let chunk;
            while ((chunk = process.stdin.read()) !== null) {
                try {
                    const data = JSON.parse(chunk);
                    if (data.stock) stock = data.stock;
                    if (data.market) market = data.market;
                    if (data.ticker) stock = data.ticker;
                } catch (e) {
                    console.error(`[ERROR] JSON解析失败: ${e.message}`);
                }
            }
        });
        
        process.stdin.on('end', async () => {
            if (!stock) {
                console.error('[ERROR] 缺少必要参数: stock/ticker');
                console.error('Usage: node index.js --stock "股票代码" [--market "市场类型"]');
                process.exit(1);
            }
            await run();
        });
    } else {
        run().catch(error => {
            console.error(`[ERROR] ${error.message}`);
            process.exit(1);
        });
    }
}

module.exports = {
    analyzeStock,
    analyzeCNStock,
    analyzeUSStock,
    managePortfolio,
    isUSStock,
    isCrypto,
    normalizeMarket
};