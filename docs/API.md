# GoClaw API 文档

## 1. 概述

GoClaw 提供了丰富的 API 接口，支持与外部系统集成。本文档详细介绍了 GoClaw 的 API 接口、请求格式和响应格式。

## 2. 基础信息

### 2.1 服务地址

默认情况下，GoClaw API 服务运行在 `http://localhost:4096`。

### 2.2 认证

目前 GoClaw API 暂时不需要认证，直接访问即可。未来版本将添加 API 密钥认证。

### 2.3 内容类型

所有 API 请求和响应均使用 JSON 格式，Content-Type 为 `application/json`。

## 3. 核心 API

### 3.1 代理接口

#### 3.1.1 发送消息

**请求**：
```http
POST /agent
Content-Type: application/json

{
  "message": "你好",
  "conversation_id": "optional-conversation-id",
  "context": {}
}
```

**响应**：
```json
{
  "id": "response-id",
  "conversation_id": "conversation-id",
  "message": "你好！我是 GoClaw AI 助手，有什么可以帮助你的吗？",
  "created_at": "2026-03-05T12:00:00Z",
  "tool_calls": []
}
```

#### 3.1.2 流式响应

**请求**：
```http
POST /agent/stream
Content-Type: application/json

{
  "message": "你好",
  "conversation_id": "optional-conversation-id"
}
```

**响应**：
流式 SSE 响应，每次返回部分消息内容。

### 3.2 内存管理接口

#### 3.2.1 添加记忆

**请求**：
```http
POST /api/memory
Content-Type: application/json

{
  "key": "user_preference",
  "content": "我喜欢技术类股票",
  "category": "preference"
}
```

**响应**：
```json
{
  "id": "user_preference",
  "key": "user_preference",
  "content": "我喜欢技术类股票",
  "category": "preference",
  "created_at": "2026-03-05T12:00:00Z",
  "updated_at": "2026-03-05T12:00:00Z"
}
```

#### 3.2.2 获取记忆

**请求**：
```http
GET /api/memory
```

**响应**：
```json
{
  "count": 2,
  "entries": [
    {
      "id": "my_email",
      "key": "my_email",
      "content": "email:270901361@qq.com",
      "category": "context",
      "created_at": "2026-03-05T12:00:00Z",
      "updated_at": "2026-03-05T12:00:00Z"
    },
    {
      "id": "user_preference",
      "key": "user_preference",
      "content": "我喜欢技术类股票",
      "category": "preference",
      "created_at": "2026-03-05T12:00:00Z",
      "updated_at": "2026-03-05T12:00:00Z"
    }
  ]
}
```

#### 3.2.3 获取特定记忆

**请求**：
```http
GET /api/memory/{key}
```

**响应**：
```json
{
  "id": "my_email",
  "key": "my_email",
  "content": "email:270901361@qq.com",
  "category": "context",
  "created_at": "2026-03-05T12:00:00Z",
  "updated_at": "2026-03-05T12:00:00Z"
}
```

#### 3.2.4 删除记忆

**请求**：
```http
DELETE /api/memory/{key}
```

**响应**：
```json
{
  "status": "success",
  "message": "Memory deleted successfully"
}
```

### 3.3 工具接口

#### 3.3.1 获取工具列表

**请求**：
```http
GET /api/tools
```

**响应**：
```json
{
  "count": 27,
  "tools": [
    {
      "name": "shell",
      "description": "执行 shell 命令",
      "parameters": {
        "command": {
          "type": "string",
          "description": "要执行的命令",
          "required": true
        }
      }
    },
    {
      "name": "file_read",
      "description": "读取文件内容",
      "parameters": {
        "path": {
          "type": "string",
          "description": "文件路径",
          "required": true
        }
      }
    }
    // 更多工具...
  ]
}
```

### 3.4 系统接口

#### 3.4.1 系统状态

**请求**：
```http
GET /api/status
```

**响应**：
```json
{
  "status": "running",
  "version": "1.0.0",
  "uptime": "2h 30m 45s",
  "memory_usage": "128MB",
  "cpu_usage": "5%"
}
```

#### 3.4.2 健康检查

**请求**：
```http
GET /api/health
```

**响应**：
```json
{
  "status": "healthy",
  "components": {
    "agent": "healthy",
    "memory": "healthy",
    "providers": "healthy",
    "channels": "healthy"
  }
}
```

## 4. WebSocket 接口

### 4.1 连接

```
ws://localhost:4096/ws
```

### 4.2 消息格式

#### 客户端发送

```json
{
  "type": "message",
  "data": {
    "message": "你好",
    "conversation_id": "optional-conversation-id"
  }
}
```

#### 服务器响应

```json
{
  "type": "message",
  "data": {
    "id": "response-id",
    "conversation_id": "conversation-id",
    "message": "你好！我是 GoClaw AI 助手，有什么可以帮助你的吗？",
    "created_at": "2026-03-05T12:00:00Z"
  }
}
```

#### 流式响应

```json
{
  "type": "stream",
  "data": {
    "id": "response-id",
    "conversation_id": "conversation-id",
    "message": "你好",
    "created_at": "2026-03-05T12:00:00Z",
    "is_finished": false
  }
}
```

## 5. OpenAI 兼容接口

### 5.1 聊天完成

**请求**：
```http
POST /v1/chat/completions
Content-Type: application/json

{
  "model": "gpt-4",
  "messages": [
    {
      "role": "system",
      "content": "你是一个 helpful 的助手"
    },
    {
      "role": "user",
      "content": "你好"
    }
  ],
  "stream": false
}
```

**响应**：
```json
{
  "id": "chatcmpl-123",
  "object": "chat.completion",
  "created": 1677858242,
  "model": "gpt-4",
  "choices": [
    {
      "index": 0,
      "message": {
        "role": "assistant",
        "content": "你好！我是 GoClaw AI 助手，有什么可以帮助你的吗？"
      },
      "finish_reason": "stop"
    }
  ],
  "usage": {
    "prompt_tokens": 13,
    "completion_tokens": 17,
    "total_tokens": 30
  }
}
```

### 5.2 流式聊天完成

**请求**：
```http
POST /v1/chat/completions
Content-Type: application/json

{
  "model": "gpt-4",
  "messages": [
    {
      "role": "user",
      "content": "你好"
    }
  ],
  "stream": true
}
```

**响应**：
流式 SSE 响应，每次返回部分消息内容。

## 6. 错误处理

### 6.1 错误响应格式

```json
{
  "error": {
    "code": "INVALID_REQUEST",
    "message": "Invalid request parameters",
    "details": {
      "parameter": "message",
      "reason": "Message is required"
    }
  }
}
```

### 6.2 常见错误码

| 错误码 | 描述 | HTTP 状态码 |
|--------|------|------------|
| INVALID_REQUEST | 请求参数无效 | 400 |
| NOT_FOUND | 资源不存在 | 404 |
| INTERNAL_ERROR | 内部服务器错误 | 500 |
| SERVICE_UNAVAILABLE | 服务不可用 | 503 |
| RATE_LIMITED | 请求速率限制 | 429 |

## 7. 速率限制

为了保护系统，GoClaw API 实施了速率限制：

- 普通请求：每分钟 60 个请求
- 流式请求：每分钟 30 个请求
- 工具调用：每分钟 100 个请求

## 8. 最佳实践

### 8.1 性能优化

- 使用流式响应获取较长的回复
- 合理使用记忆系统存储用户偏好
- 批量处理工具调用以减少 API 请求

### 8.2 错误处理

- 实现重试机制处理临时错误
- 监控 API 响应时间和错误率
- 合理处理速率限制

### 8.3 安全考虑

- 不要在请求中包含敏感信息
- 限制工具调用的权限
- 验证所有用户输入

## 9. 示例代码

### 9.1 Python

```python
import requests

# 发送消息
response = requests.post(
    "http://localhost:4096/agent",
    json={"message": "你好"},
    headers={"Content-Type": "application/json"}
)
print(response.json())

# 添加记忆
response = requests.post(
    "http://localhost:4096/api/memory",
    json={
        "key": "my_email",
        "content": "email:user@example.com",
        "category": "context"
    },
    headers={"Content-Type": "application/json"}
)
print(response.json())

# 获取工具列表
response = requests.get("http://localhost:4096/api/tools")
print(response.json())
```

### 9.2 JavaScript

```javascript
// 发送消息
fetch('http://localhost:4096/agent', {
  method: 'POST',
  headers: {
    'Content-Type': 'application/json'
  },
  body: JSON.stringify({ message: '你好' })
})
.then(response => response.json())
.then(data => console.log(data));

// WebSocket 连接
const ws = new WebSocket('ws://localhost:4096/ws');

ws.onopen = () => {
  ws.send(JSON.stringify({
    type: 'message',
    data: { message: '你好' }
  }));
};

ws.onmessage = (event) => {
  const data = JSON.parse(event.data);
  console.log(data);
};
```

## 10. 版本兼容性

### 10.1 API 版本

当前 API 版本为 v1，未来可能会引入新的版本。

### 10.2 向后兼容性

GoClaw 承诺保持 API 的向后兼容性，不会在 minor 版本中破坏现有接口。

---

*最后更新：2026-03-05*