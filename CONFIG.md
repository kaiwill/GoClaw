# GoClaw 配置文件说明

## 📁 配置文件位置

配置文件位于：`~/.goclaw/config.toml`

首次运行时会自动创建默认配置文件。

## 🔧 配置文件结构

### [agent] - Agent 配置

Agent 行为和执行参数配置。

| 配置项 | 类型 | 默认值 | 说明 |
|--------|------|--------|------|
| `max_tool_iterations` | int | 15 | 最大工具调用迭代次数，用于防止死循环。多步骤任务建议设置为 15-20 |
| `max_history_messages` | int | 20 | 最大历史消息数量，超过后会进行压缩 |
| `parallel_tools` | bool | false | 是否并行执行工具 |
| `tool_dispatcher` | string | "auto" | 工具调度器类型：auto, simple, advanced |
| `compact_context` | bool | true | 是否压缩上下文以节省token |
| `min_relevance_score` | float64 | 0.1 | 记忆体检索的最小相关性分数（0.0-1.0） |
| `default_model` | string | - | 默认使用的模型名称 |
| `default_temperature` | float64 | 0.7 | 默认温度参数（0.0-2.0） |

**示例：**
```toml
[agent]
max_tool_iterations = 15
max_history_messages = 20
parallel_tools = false
tool_dispatcher = "auto"
compact_context = true
min_relevance_score = 0.1
default_temperature = 0.7
```

### [provider] - AI 模型提供商配置

AI 模型提供商配置。

| 配置项 | 类型 | 默认值 | 说明 |
|--------|------|--------|------|
| `name` | string | "openai" | 提供商名称：openai, bailian, gitee, custom:URL |
| `model` | string | "gpt-4" | 使用的模型名称 |
| `api_key` | string | - | API 密钥（建议使用环境变量） |
| `base_url` | string | - | 自定义 API 基础 URL |

**示例：**

**使用阿里云百炼（推荐）：**
```toml
[provider]
name = "bailian"
model = "qwen-plus"
api_key = "your-bailian-api-key"
```

**使用 GiteeAI（免费模型）：**
```toml
[provider]
name = "gitee"
model = "GLM-4.7-Flash"
url = "custom:https://ai.gitee.com/v1"
api_key = "your-gitee-ai-api-key"
```

**使用 OpenAI：**
```toml
[provider]
name = "openai"
model = "gpt-4"
api_key = "your-openai-api-key"
```

### [memory] - 记忆体配置

记忆体存储和检索配置。

| 配置项 | 类型 | 默认值 | 说明 |
|--------|------|--------|------|
| `backend` | string | "sqlite" | 记忆体后端：none, sqlite, qdrant |
| `auto_save` | bool | true | 是否自动保存对话到记忆体 |
| `hygiene_enabled` | bool | true | 是否启用记忆体清理 |
| `archive_after_days` | int | 7 | 归档天数，超过此天数的对话会被归档 |
| `purge_after_days` | int | 30 | 清理天数，超过此天数的对话会被删除 |
| `conversation_retention_days` | int | 30 | 对话保留天数 |
| `embedding_provider` | string | "none" | 嵌入向量提供商 |
| `embedding_model` | string | "text-embedding-3-small" | 嵌入模型 |
| `embedding_dimensions` | int | 1536 | 嵌入向量维度 |
| `vector_weight` | float64 | 0.7 | 向量搜索权重（0.0-1.0） |
| `keyword_weight` | float64 | 0.3 | 关键词搜索权重（0.0-1.0） |
| `min_relevance_score` | float64 | 0.4 | 最小相关性分数（0.0-1.0） |
| `embedding_cache_size` | int | 10000 | 嵌入缓存大小 |
| `chunk_max_tokens` | int | 512 | 文本分块最大 token 数 |
| `response_cache_enabled` | bool | false | 是否启用响应缓存 |
| `response_cache_ttl_minutes` | int | 60 | 响应缓存 TTL（分钟） |
| `response_cache_max_entries` | int | 5000 | 响应缓存最大条目数 |
| `snapshot_enabled` | bool | false | 是否启用快照 |
| `snapshot_on_hygiene` | bool | false | 清理时是否创建快照 |
| `auto_hydrate` | bool | true | 是否自动清理过期数据 |

**示例：**
```toml
[memory]
backend = "sqlite"
auto_save = true
hygiene_enabled = true
archive_after_days = 7
purge_after_days = 30
vector_weight = 0.7
keyword_weight = 0.3
min_relevance_score = 0.4
```

### [gateway] - 网关配置

Web 网关服务器配置。

| 配置项 | 类型 | 默认值 | 说明 |
|--------|------|--------|------|
| `port` | int | 4096 | 网关服务器端口 |
| `host` | string | "0.0.0.0" | 网关服务器监听地址 |
| `static_dir` | string | - | 静态文件目录 |
| `locale` | string | "zh-CN" | 界面语言 |
| `require_pairing` | bool | false | 是否需要配对码 |
| `allow_public_bind` | bool | false | 是否允许公网绑定 |
| `paired_tokens` | array | [] | 已配对的 token 列表 |
| `pair_rate_limit_per_minute` | int | 10 | 配对速率限制（每分钟） |
| `webhook_rate_limit_per_minute` | int | 60 | Webhook 速率限制（每分钟） |
| `trust_forwarded_headers` | bool | false | 是否信任转发的请求头 |
| `rate_limit_max_keys` | int | 10000 | 速率限制最大键数 |
| `idempotency_ttl_secs` | int | 300 | 幂等性 TTL（秒） |
| `idempotency_max_keys` | int | 10000 | 幂等性最大键数 |
| `enable_wechat_login` | bool | false | 是否启用微信登录 |
| `wechat_app_id` | string | - | 微信 App ID |
| `wechat_app_secret` | string | - | 微信 App Secret |

**示例：**
```toml
[gateway]
port = 4096
host = "0.0.0.0"
locale = "zh-CN"
require_pairing = false
enable_wechat_login = true
wechat_app_id = "your-wechat-app-id"
wechat_app_secret = "your-wechat-app-secret"
```

### [skills] - 技能配置

技能系统配置。

| 配置项 | 类型 | 默认值 | 说明 |
|--------|------|--------|------|
| `open_skills_enabled` | bool | false | 是否启用开放技能 |
| `prompt_injection_mode` | string | "full" | 提示注入模式：none, partial, full |

**示例：**
```toml
[skills]
open_skills_enabled = false
prompt_injection_mode = "full"
```

### [channels_config] - 通知渠道配置

各种通知渠道的配置。

#### [channels_config.cli] - CLI 渠道

| 配置项 | 类型 | 默认值 | 说明 |
|--------|------|--------|------|
| `enabled` | bool | true | 是否启用 CLI 渠道 |

#### [channels_config.email] - 邮件渠道

| 配置项 | 类型 | 默认值 | 说明 |
|--------|------|--------|------|
| `enabled` | bool | false | 是否启用邮件渠道 |
| `imap_host` | string | - | IMAP 服务器地址 |
| `imap_port` | int | 993 | IMAP 端口 |
| `imap_folder` | string | "INBOX" | IMAP 文件夹 |
| `smtp_host` | string | - | SMTP 服务器地址 |
| `smtp_port` | int | 465 | SMTP 端口 |
| `smtp_tls` | bool | true | 是否使用 TLS |
| `username` | string | - | 邮箱用户名 |
| `password` | string | - | 邮箱密码 |
| `from_address` | string | - | 发件人地址 |
| `idle_timeout_secs` | int | 30 | 空闲超时（秒） |
| `disable_idle` | bool | true | 是否禁用空闲检查 |
| `allowed_senders` | array | [] | 允许的发送者列表 |

**示例：**
```toml
[channels_config.email]
enabled = true
imap_host = "imap.qq.com"
imap_port = 993
imap_folder = "INBOX"
smtp_host = "smtp.qq.com"
smtp_port = 465
smtp_tls = true
username = "your-email@qq.com"
password = "your-password"
from_address = "your-email@qq.com"
```

#### [channels_config.dingtalk] - 钉钉渠道

| 配置项 | 类型 | 默认值 | 说明 |
|--------|------|--------|------|
| `client_id` | string | - | 钉钉 Client ID |
| `client_secret` | string | - | 钉钉 Client Secret |
| `allowed_users` | array | [] | 允许的用户列表 |

**示例：**
```toml
[channels_config.dingtalk]
client_id = "your-client-id"
client_secret = "your-client-secret"
allowed_users = ["*"]
```

### [scheduler] - 调度器配置

任务调度器配置。

| 配置项 | 类型 | 默认值 | 说明 |
|--------|------|--------|------|
| `enabled` | bool | true | 是否启用调度器 |
| `max_tasks` | int | 64 | 最大任务数 |
| `max_concurrent` | int | 4 | 最大并发任务数 |

**示例：**
```toml
[scheduler]
enabled = true
max_tasks = 64
max_concurrent = 4
```

### [cron] - 定时任务配置

定时任务系统配置。

| 配置项 | 类型 | 默认值 | 说明 |
|--------|------|--------|------|
| `enabled` | bool | true | 是否启用定时任务 |
| `max_run_history` | int | 50 | 最大运行历史记录数 |

**示例：**
```toml
[cron]
enabled = true
max_run_history = 50
```

### [reliability] - 可靠性配置

系统可靠性和容错配置。

| 配置项 | 类型 | 默认值 | 说明 |
|--------|------|--------|------|
| `provider_retries` | int | 1 | 提供商重试次数 |
| `provider_backoff_ms` | int | 200 | 提供商退避时间（毫秒） |
| `fallback_providers` | array | [] | 备用提供商列表 |
| `api_keys` | array | [] | API 密钥列表（用于轮询） |
| `channel_initial_backoff_secs` | int | 1 | 渠道初始退避时间（秒） |
| `channel_max_backoff_secs` | int | 10 | 渠道最大退避时间（秒） |
| `scheduler_poll_secs` | int | 2 | 调度器轮询间隔（秒） |
| `scheduler_retries` | int | 1 | 调度器重试次数 |

**示例：**
```toml
[reliability]
provider_retries = 1
provider_backoff_ms = 200
fallback_providers = []
channel_initial_backoff_secs = 1
channel_max_backoff_secs = 10
```

### [cost] - 成本控制配置

API 调用成本控制。

| 配置项 | 类型 | 默认值 | 说明 |
|--------|------|--------|------|
| `enabled` | bool | false | 是否启用成本控制 |
| `daily_limit_usd` | float64 | 10.0 | 每日成本限制（美元） |
| `monthly_limit_usd` | float64 | 100.0 | 每月成本限制（美元） |
| `warn_at_percent` | int | 80 | 警告阈值（百分比） |
| `allow_override` | bool | false | 是否允许覆盖限制 |

**示例：**
```toml
[cost]
enabled = false
daily_limit_usd = 10.0
monthly_limit_usd = 100.0
warn_at_percent = 80
```

### [web_search] - 网络搜索配置

网络搜索功能配置。

| 配置项 | 类型 | 默认值 | 说明 |
|--------|------|--------|------|
| `enabled` | bool | false | 是否启用网络搜索 |
| `provider` | string | "duckduckgo" | 搜索提供商 |
| `max_results` | int | 5 | 最大结果数 |
| `timeout_secs` | int | 15 | 超时时间（秒） |

**示例：**
```toml
[web_search]
enabled = false
provider = "duckduckgo"
max_results = 5
timeout_secs = 15
```

### [web_fetch] - 网页获取配置

网页内容获取配置。

| 配置项 | 类型 | 默认值 | 说明 |
|--------|------|--------|------|
| `enabled` | bool | false | 是否启用网页获取 |
| `allowed_domains` | array | ["*"] | 允许的域名列表 |
| `blocked_domains` | array | [] | 阻止的域名列表 |
| `max_response_size` | int | 500000 | 最大响应大小（字节） |
| `timeout_secs` | int | 30 | 超时时间（秒） |

**示例：**
```toml
[web_fetch]
enabled = false
allowed_domains = ["*"]
blocked_domains = []
max_response_size = 500000
timeout_secs = 30
```

### [browser] - 浏览器配置

浏览器功能配置。

| 配置项 | 类型 | 默认值 | 说明 |
|--------|------|--------|------|
| `enabled` | bool | false | 是否启用浏览器 |
| `allowed_domains` | array | [] | 允许的域名列表 |
| `backend` | string | "agent_browser" | 浏览器后端 |
| `native_headless` | bool | true | 是否使用无头模式 |
| `native_webdriver_url` | string | - | 原生 WebDriver URL |

**示例：**
```toml
[browser]
enabled = false
allowed_domains = []
backend = "agent_browser"
native_headless = true
```

### [http_request] - HTTP 请求配置

HTTP 请求功能配置。

| 配置项 | 类型 | 默认值 | 说明 |
|--------|------|--------|------|
| `enabled` | bool | true | 是否启用 HTTP 请求 |
| `allowed_domains` | array | [...] | 允许的域名列表 |
| `max_response_size` | int | 1000000 | 最大响应大小（字节） |
| `timeout_secs` | int | 30 | 超时时间（秒） |

**示例：**
```toml
[http_request]
enabled = true
allowed_domains = ["oapi.dingtalk.com", "api.tianqiapi.com"]
max_response_size = 1000000
timeout_secs = 30
```

## 🎯 常见配置场景

### 场景 1：多步骤任务（股票分析 + 邮件发送）

对于需要多次工具调用的复杂任务，建议增加最大迭代次数：

```toml
[agent]
max_tool_iterations = 20
```

### 场景 2：优化记忆体检索

提高记忆体检索准确率：

```toml
[memory]
vector_weight = 0.7
keyword_weight = 0.3
min_relevance_score = 0.3
```

### 场景 3：启用邮件通知

配置邮件渠道：

```toml
[channels_config.email]
enabled = true
smtp_host = "smtp.qq.com"
smtp_port = 465
smtp_tls = true
username = "your-email@qq.com"
password = "your-password"
from_address = "your-email@qq.com"
```

### 场景 4：成本控制

启用 API 成本控制：

```toml
[cost]
enabled = true
daily_limit_usd = 10.0
monthly_limit_usd = 100.0
warn_at_percent = 80
```

## 🔐 环境变量

除了配置文件，也可以使用环境变量：

| 环境变量 | 说明 |
|----------|------|
| `BAILIAN_API_KEY` | 阿里云百炼 API 密钥 |
| `OPENAI_API_KEY` | OpenAI API 密钥 |
| `GITEE_AI_API_KEY` | GiteeAI API 密钥 |

## 📝 配置文件示例

### 完整配置示例

```toml
# GoClaw Configuration

[provider]
name = "gitee"
model = "GLM-4.7-Flash"
url = "custom:https://ai.gitee.com/v1"
api_key = "your-api-key"

[agent]
max_tool_iterations = 15
max_history_messages = 20
parallel_tools = false
tool_dispatcher = "auto"
compact_context = true
min_relevance_score = 0.1
default_temperature = 0.7

[memory]
backend = "sqlite"
auto_save = true
hygiene_enabled = true
archive_after_days = 7
purge_after_days = 30
vector_weight = 0.7
keyword_weight = 0.3
min_relevance_score = 0.4

[gateway]
port = 4096
host = "0.0.0.0"
locale = "zh-CN"
require_pairing = false
enable_wechat_login = true
wechat_app_id = "your-wechat-app-id"
wechat_app_secret = "your-wechat-app-secret"

[skills]
open_skills_enabled = false
prompt_injection_mode = "full"

[scheduler]
enabled = true
max_tasks = 64
max_concurrent = 4

[cron]
enabled = true
max_run_history = 50

[reliability]
provider_retries = 1
provider_backoff_ms = 200
channel_initial_backoff_secs = 1
channel_max_backoff_secs = 10

[cost]
enabled = false
daily_limit_usd = 10.0
monthly_limit_usd = 100.0
warn_at_percent = 80

[web_search]
enabled = false
provider = "duckduckgo"
max_results = 5
timeout_secs = 15

[web_fetch]
enabled = false
allowed_domains = ["*"]
blocked_domains = []
max_response_size = 500000
timeout_secs = 30

[browser]
enabled = false
allowed_domains = []
backend = "agent_browser"
native_headless = true

[http_request]
enabled = true
allowed_domains = ["oapi.dingtalk.com", "api.tianqiapi.com"]
max_response_size = 1000000
timeout_secs = 30
```

## 🔄 配置重载

修改配置文件后，需要重启服务才能生效：

```bash
# 停止服务
lsof -ti:4096 | xargs kill -9

# 启动服务
go run main.go gateway
```

## 📚 相关文档

- [README.md](README.md) - 项目说明
- [README.md](#配置-ai-模型提供商) - AI 模型提供商配置
- [README.md](#使用方法) - 使用方法

## 💡 配置建议

1. **安全性**：不要在配置文件中存储敏感信息，使用环境变量
2. **性能**：根据任务复杂度调整 `max_tool_iterations`
3. **成本**：启用成本控制以避免意外费用
4. **可靠性**：配置备用提供商和重试策略
5. **记忆体**：调整向量权重和相关性分数以优化检索效果

## 🆘 问题排查

### 配置不生效

1. 检查配置文件路径：`~/.goclaw/config.toml`
2. 确认配置文件格式正确（TOML 语法）
3. 重启服务以加载新配置
4. 查看服务启动日志确认配置已加载

### 工具调用次数不足

1. 增加 `max_tool_iterations` 值
2. 检查是否有死循环（重复的工具调用）
3. 优化 Agent 提示词以减少不必要的工具调用

### 记忆体检索不准确

1. 调整 `vector_weight` 和 `keyword_weight` 比例
2. 降低 `min_relevance_score` 以包含更多结果
3. 检查记忆体内容是否包含相关关键词
