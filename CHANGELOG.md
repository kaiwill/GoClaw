# 变更日志

## 2026-03-06

### 修复

1. **配置问题修复**
   - 修复了 `daemon` 命令中缺少设置 `wechat_enabled` 配置的问题
   - 文件: `main.go`

2. **WebSocket认证支持**
   - 后端WebSocket处理代码添加了token验证逻辑
   - 支持用户token和管理员token验证
   - 文件: `pkg/gateway/server.go`

3. **前端状态管理优化**
   - 添加了 `updateAuthState` 函数统一更新认证状态
   - 简化了 `pair` 和 `logout` 函数的状态更新逻辑
   - 文件: `web/src/hooks/useAuth.ts`

4. **WebSocket连接URL修复**
   - 修改了WebSocket连接逻辑，直接连接到后端服务器
   - 解决了Vite代理可能导致的WebSocket连接问题
   - 文件: `web/src/lib/ws.ts`

5. **WebSocket认证策略调整**
   - 允许匿名WebSocket连接，但记录警告
   - 解决了严格的token验证导致匿名连接被拒绝的问题
   - 文件: `pkg/gateway/server.go`

6. **WebSocket连接触发条件修复**
   - 修改了watch依赖，只监听loading状态
   - 添加了页面挂载时的WebSocket连接尝试
   - 文件: `web/src/pages/AgentChat.vue`

7. **WebSocket子协议问题修复**
   - 修改了前端代码，将token放在查询参数中而不是子协议中
   - 修改了后端代码，从查询参数中提取并验证token
   - 解决了WebSocket子协议中包含特殊字符的问题
   - 文件: `web/src/lib/ws.ts` 和 `pkg/gateway/server.go`

### 功能

- 支持WebSocket连接
- 支持匿名WebSocket连接
- 支持token验证
- 支持微信登录
- 支持管理员登录

### 测试

- WebSocket连接测试
- 微信登录测试
- 管理员登录测试
- 聊天功能测试