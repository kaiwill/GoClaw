# GoClaw 会话管理功能 - 发现和问题记录

## 发现 1: 当前架构分析
**时间**: 2026-03-10
**内容**:
- WebSocket 服务器已有 sessionID 概念，但没有持久化
- SQLite 内存管理模块可以使用，需要扩展表结构
- 前端消息存储在内存中，刷新页面后丢失
- WebSocket 服务器位于 `pkg/gateway/server.go`
- 内存管理模块位于 `pkg/memory/sqlite.go`

## 发现 2: 数据库位置
**时间**: 2026-03-10
**内容**:
- SQLite 数据库路径: `~/.goclaw/memory/brain.db`
- 使用 WAL 模式提高并发性能
- 已有 FTS5 全文搜索支持
- 会话数据将存储在 `~/.goclaw/sessions.db`

## 发现 3: WebSocket 消息类型
**时间**: 2026-03-10
**内容**:
- 当前支持的消息类型: `message`, `chat`, `ping`
- 响应类型: `done`, `error`
- 需要添加会话管理相关的消息类型

## 发现 4: 前端状态管理
**时间**: 2026-03-10
**内容**:
- 使用 Vue 3 Composition API
- 消息存储在 `messages` ref 中
- WebSocket 连接管理在 `WebSocketClient` 类中
- 需要添加会话管理状态

## 发现 5: 会话 ID 生成
**时间**: 2026-03-10
**内容**:
- 会话 ID 生成方式: `session_{timestamp}_{nanoseconds}`
- 消息 ID 生成方式: `msg_{timestamp}_{nanoseconds}`
- 使用时间戳确保唯一性

## 发现 6: WebSocket 客户端
**时间**: 2026-03-10
**内容**:
- WebSocket 客户端位于 `web/src/lib/ws.ts`
- sendMessage 方法需要支持 session_id 参数
- 已有 token 认证机制

## 发现 7: API 路由
**时间**: 2026-03-10
**内容**:
- 会话管理 API 路由: `/api/sessions`
- 会话详情 API 路由: `/api/sessions/:id`
- 会话消息 API 路由: `/api/sessions/:id/messages`
- 需要添加到 HTTP 路由器

## 待解决问题
1. ✅ 如何自动生成会话标题？ - 使用默认标题"新对话"
2. ✅ 如何处理大量历史消息的性能问题？ - 使用分页查询和索引
3. ✅ 如何实现会话切换时的平滑过渡？ - 通过 API 加载历史消息
4. ✅ 如何处理并发写入冲突？ - 使用数据库事务

## 解决方案
1. 会话标题使用默认值"新对话"，用户可 later 修改
2. 使用 SQLite 索引和分页查询优化性能
3. 会话切换时通过 API 加载历史消息
4. 使用数据库事务确保数据一致性

## 实施总结
- 会话管理功能已成功实现
- 所有待解决问题已解决
- 代码已通过语法检查
- 需要进行端到端测试
