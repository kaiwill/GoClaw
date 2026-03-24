# GoClaw 会话管理功能实现计划

## 目标
实现会话持久化和历史记录管理，解决 Web 界面每次打开后聊天记录丢失的问题。

## 当前状态分析

### 已有基础设施
- ✅ WebSocket 服务器 (`pkg/gateway/websocket.go`) - 有基本的 sessionID 概念
- ✅ SQLite 内存管理 (`pkg/memory/sqlite.go`) - 可复用数据库
- ✅ 前端聊天界面 (`web/src/pages/AgentChat.vue`) - 有消息显示功能

### 已实现功能
- ✅ 会话持久化存储
- ✅ 会话历史记录查询
- ✅ 会话列表显示
- ✅ 会话切换功能
- ✅ 会话删除功能

## 实施阶段

### 阶段 1: 数据库架构设计 ✅
**状态**: completed
**任务**:
- 设计会话表结构
- 设计消息表结构
- 创建数据库迁移脚本

### 阶段 2: 后端会话管理模块 ✅
**状态**: completed
**任务**:
- 创建 `pkg/session/manager.go`
- 实现会话 CRUD 操作
- 实现消息历史存储和查询
- 添加会话搜索和过滤功能

### 阶段 3: WebSocket 服务器集成 ✅
**状态**: completed
**任务**:
- 在 WebSocket 服务器中集成会话管理
- 自动保存消息到数据库
- 提供会话列表和历史记录 API

### 阶段 4: 前端会话列表组件 ✅
**状态**: completed
**任务**:
- 创建会话列表组件
- 添加会话切换功能
- 添加会话删除功能
- 添加会话搜索功能

### 阶段 5: 前端历史记录显示 ✅
**状态**: completed
**任务**:
- 修改聊天页面以支持会话历史
- 实现会话加载功能
- 优化消息显示和滚动

## 数据库设计

### sessions 表
```sql
CREATE TABLE sessions (
    id TEXT PRIMARY KEY,
    title TEXT NOT NULL,
    user_id TEXT,
    created_at TEXT NOT NULL,
    updated_at TEXT NOT NULL,
    message_count INTEGER DEFAULT 0,
    metadata TEXT
);
```

### messages 表
```sql
CREATE TABLE messages (
    id TEXT PRIMARY KEY,
    session_id TEXT NOT NULL,
    role TEXT NOT NULL,
    content TEXT NOT NULL,
    created_at TEXT NOT NULL,
    metadata TEXT,
    FOREIGN KEY (session_id) REFERENCES sessions(id) ON DELETE CASCADE
);
```

### 索引
```sql
CREATE INDEX idx_messages_session ON messages(session_id);
CREATE INDEX idx_sessions_user ON sessions(user_id);
CREATE INDEX idx_sessions_updated ON sessions(updated_at DESC);
CREATE INDEX idx_messages_created ON messages(created_at);
```

## API 设计

### 会话管理 API
- `GET /api/sessions` - 获取会话列表
- `GET /api/sessions/:id` - 获取会话详情
- `POST /api/sessions` - 创建新会话
- `PUT /api/sessions/:id` - 更新会话标题
- `DELETE /api/sessions/:id` - 删除会话

### 消息历史 API
- `GET /api/sessions/:id/messages` - 获取会话消息历史
- `POST /api/sessions/:id/messages` - 添加消息到会话

## 技术要点

1. **会话 ID 生成**: 使用 UUID 或时间戳 + 随机数
2. **会话标题**: 自动从第一条用户消息生成，或使用 AI 生成
3. **消息存储**: 每条消息都存储到数据库
4. **性能优化**: 使用索引和分页查询
5. **并发控制**: 使用互斥锁保护数据库访问

## 错误处理

- 数据库连接失败
- 会话不存在
- 消息存储失败
- 并发写入冲突

## 测试计划

- 单元测试：会话管理器
- 集成测试：WebSocket + 会话管理
- 端到端测试：前端 + 后端

## 预计工作量
2-3 天

## 实际工作量
1 天

## 进度跟踪 ✅
- 阶段 1: 100%
- 阶段 2: 100%
- 阶段 3: 100%
- 阶段 4: 100%
- 阶段 5: 100%

## 完成情况 ✅
- 总体进度: 100%
- 会话管理功能已成功实现
- 会话持久化存储已实现
- 会话历史记录管理已实现
- 会话列表显示和搜索已实现
- 会话切换功能已实现
- 会话删除功能已实现
- 自动保存聊天消息到数据库已实现
- 前端会话列表组件已实现
- WebSocket 自动保存消息已实现
- 会话管理器初始化已实现
- 主程序集成已实现

## 实施日期
2026-03-10

## 实施者
AI Assistant

## 文件清单
- `pkg/session/manager.go` - 会话管理模块
- `web/src/components/SessionList.vue` - 会话列表组件
- `web/src/pages/AgentChat.vue` - 聊天页面（已集成会话历史）
- `web/src/lib/ws.ts` - WebSocket 客户端（已支持 session_id）
- `main.go` - 主程序（已集成会话管理器初始化）
- `pkg/gateway/server.go` - WebSocket 服务器（已集成会话管理）

## 下一步
- 运行并测试会话管理功能
- 根据测试结果进行优化
- 更新 TASK_PLAN.md

## 编译状态 ✅
- 编译成功
- 无语法错误
- 无类型错误

## 使用方法
1. 启动网关服务器: `./goclaw gateway`
2. 访问 Web 界面: http://localhost:4096
3. 在聊天页面中，可以创建新会话、切换会话、删除会话
4. 所有聊天消息会自动保存到数据库
5. 刷新页面后，会话历史仍然保留

## 数据库
- 会话数据库路径: `~/.goclaw/sessions.db`
- 自动创建数据库和表结构
- 使用 WAL 模式提高并发性能

## API 端点
- `GET /api/sessions` - 获取会话列表
- `POST /api/sessions` - 创建新会话
- `GET /api/sessions/:id` - 获取会话详情
- `PUT /api/sessions/:id` - 更新会话标题
- `DELETE /api/sessions/:id` - 删除会话
- `GET /api/sessions/:id/messages` - 获取会话消息历史
- `POST /api/sessions/:id/messages` - 添加消息到会话

## 代码重构计划

### 当前问题
- `pkg/gateway/server.go` 文件太大（2276 行）
- 多个 API 处理函数混杂在一起
- 不利于维护和扩展

### 重构方案
将 server.go 拆分为多个文件，每个文件负责一类 API：

1. **pkg/gateway/server.go** - 主服务器逻辑
   - Server 结构体定义
   - WebSocket 处理
   - HTTP 路由
   - 启动/停止逻辑

2. **pkg/gateway/api.go** - API 处理函数
   - handleSessions
   - handleSessionDetail
   - handleSessionMessages
   - handleMemoryAPI
   - handleCronAPI

3. **pkg/gateway/admin_api.go** - 管理员 API
   - handleAdminLogin
   - handleAdminUsers
   - handleAdminApproveUser
   - handleAdminPasswordChange

4. **pkg/gateway/user_api.go** - 用户 API
   - handleUserInfo
   - handleUserUpdate

5. **pkg/gateway/wechat_api.go** - 微信 API
   - handleWechatLogin
   - handleWechatCallback
   - handleWechatUserInfo

### 重构步骤
1. ✅ 分析 server.go 的结构
2. ✅ 修复 scheduler not available 问题
3. ⏳ 创建拆分后的 API 文件
4. ⏳ 重构主 server.go 文件
5. ⏳ 编译验证重构结果

### 重构优先级
- 高: 修复 scheduler 问题
- 中: 代码重构
- 低: 文档更新

### 重构时间
预计 2-3 小时
