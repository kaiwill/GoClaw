#!/usr/bin/env python3
import subprocess
import json
from datetime import datetime
import os
import sys

# 添加 stock-analyzer-skill 到路径
stock_skill_path = "/tmp/skills/stock-analyzer-skill"
if os.path.exists(stock_skill_path):
    sys.path.insert(0, stock_skill_path)

# 获取股票分析
try:
    from handler import analyze
    result = analyze({"stock": "爱尔眼科", "market": "sz"})
    stock_data = json.dumps(result)
except Exception as e:
    stock_data = json.dumps({"error": str(e)})

# 准备邮件内容
email_body = f"""股票分析报告

股票: 爱尔眼科 (300015)
分析时间: {datetime.now().strftime('%Y-%m-%d %H:%M:%S')}

{stock_data}

---
本邮件由自动定时任务发送"""

# 准备邮件数据
email_data = json.dumps({
    "recipient": "your-email@example.com",
    "subject": "【每日股票分析】爱尔眼科 (300015)",
    "body": email_body
})

# 发送邮件 - 通过 stdin 传递 JSON
email_script = "/tmp/skills/email-sender-skill/index.js"

if os.path.exists(email_script):
    try:
        result = subprocess.run(
            ["node", email_script],
            input=email_data,
            capture_output=True,
            text=True,
            timeout=60
        )
        print(result.stdout)
        if result.stderr:
            print("Error:", result.stderr)
        if result.returncode != 0:
            sys.exit(1)
    except Exception as e:
        print(f"发送邮件失败: {str(e)}")
        sys.exit(1)
else:
    print(f"邮件脚本不存在: {email_script}")
    sys.exit(1)
