# AI Proxy Plugin - 构建和部署指南

## 概述

AI Proxy 插件是一个多协议 AI 服务网关，支持以下功能：

- ✅ **多协议支持**：OpenAI、Claude、Gemini、Azure、DeepSeek 等 20+ AI 提供商
- ✅ **协议转换**：自动转换不同 AI 提供商之间的协议格式
- ✅ **流式响应支持**：支持 SSE 流式和普通 HTTP 响应
- ✅ **多 Provider 管理**：支持多个 AI 提供商配置和切换
- ✅ **Token 计费**：自动计算输入/输出/总 Token 数量
- ✅ **错误处理和重试**：内置重试机制和错误处理

## 快速开始

### 1. 构建插件

#### 使用 Go 1.24 原生编译器（推荐）

```bash
cd /Users/xiaodian/IdeaProjects/higress/plugins/wasm-go/extensions/ai-proxy
./build.sh
```

#### 使用 TinyGo 构建

```bash
./build.sh tinygo
```

### 2. 构建 Docker 镜像

```bash
# 构建本地镜像
docker build -t ai-proxy:1.0.0 .

# 或使用版本文件中的版本
VERSION=$(cat VERSION)
docker build -t ai-proxy:$VERSION .
```

### 3. 推送到镜像仓库

```bash
# 标签镜像
docker tag ai-proxy:1.0.0 your-registry/ai-proxy:1.0.0

# 推送镜像
docker push your-registry/ai-proxy:1.0.0
```

### 4. 在 Higress 中配置

#### 方式一：使用 WasmPlugin 资源（推荐）

```yaml
apiVersion: extensions.higress.io/v1alpha1
kind: WasmPlugin
metadata:
  name: ai-proxy
  namespace: agent
spec:
  defaultConfig:
    disable: false
    providers:
      - id: "openai"
        type: "openai"
        apiTokens:
          - "sk-xxxxx"
        endpoint: "https://api.openai.com"
        timeout: 30000
        maxRetries: 2
  url: oci://your-registry/ai-proxy:1.0.0:latest
  phase: DONE
```

#### 方式二：应用到路由

```yaml
apiVersion: networking.k8s.io/v1beta1
kind: Ingress
metadata:
  name: ai-ingress
  namespace: agent
spec:
  rules:
    - host: ai.example.com
      http:
        paths:
          - path: /v1/chat/completions
            backend:
              service:
                name: ai-service
            extensions:
              - name: ai-proxy
                config:
                  disable: false
                  providers:
                    - id: "openai"
                      type: "openai"
                      apiTokens:
                        - "sk-xxxxx"
                      endpoint: "https://api.openai.com"
                      timeout: 30000
                      maxRetries: 2
```

## 配置说明

### 核心配置项

| 配置项 | 类型 | 说明 | 默认值 |
|---------|------|------|---------|
| `disable` | boolean | 是否禁用插件 | `false` |
| `providers` | array | AI 提供商配置数组 | `[]` |
| `activeProviderId` | string | 默认激活的提供商 ID | - |

### Provider 配置项

| 配置项 | 类型 | 说明 | 默认值 |
|---------|------|------|---------|
| `id` | string | Provider 唯一标识 | - |
| `type` | string | Provider 类型 | - |
| `apiTokens` | array | API Token 列表 | `[]` |
| `endpoint` | string | 自定义端点 URL | - |
| `timeout` | integer | 请求超时时间（毫秒） | `30000` |
| `maxRetries` | integer | 最大重试次数 | `2` |
| `enableContextCache` | boolean | 是否启用上下文缓存 | `true` |
| `contextCacheMaxSize` | integer | 上下文缓存最大大小 | `1000` |

### 支持的 Provider 类型

| 类型 | 说明 | 示例 |
|------|------|------|
| `openai` | OpenAI 兼容接口 | OpenAI、Moonshot、DeepSeek 等 |
| `claude` | Claude API | Anthropic Claude |
| `azure` | Azure OpenAI | Microsoft Azure OpenAI |
| `gemini` | Google Gemini | Google Gemini API |
| `vertex` | Google Vertex AI | Google Vertex AI |
| `minimax` | MiniMax | MiniMax API |
| `doubao` | 字节豆包 | 字节跳动 AI |
| `stepfun` | 阶跃 | 阶跃 AI |
| `generic` | 通用 Provider | 自定义 AI 服务 |

### 完整配置示例

```json
{
  "disable": false,
  "providers": [
    {
      "id": "openai",
      "type": "openai",
      "apiTokens": [
        "sk-proj-xxxx",
        "sk-proj-yyyy"
      ],
      "endpoint": "https://api.openai.com",
      "timeout": 30000,
      "maxRetries": 2,
      "enableContextCache": true,
      "contextCacheMaxSize": 1000
    },
    {
      "id": "claude",
      "type": "claude",
      "apiTokens": [
        "sk-ant-xxxx"
      ],
      "timeout": 60000,
      "maxRetries": 3
    },
    {
      "id": "gemini",
      "type": "gemini",
      "apiTokens": [
        "AIzaSyxxxx"
      ]
    }
  ],
  "activeProviderId": "openai"
}
```

## 高级配置

### Token 统计

插件自动计算和记录以下 Token 指标：

- **Input Tokens**：请求中的输入 Token 数量
- **Output Tokens**：响应中的输出 Token 数量
- **Total Tokens**：总 Token 数量（Input + Output）

### 上下文缓存

启用后可以缓存对话上下文，提高性能：

```json
{
  "providers": [
    {
      "id": "openai",
      "type": "openai",
      "enableContextCache": true,
      "contextCacheMaxSize": 1000
    }
  ]
}
```

### 自定义 Header

插件会自动添加以下 Header：

- `Authorization`: `Bearer {apiToken}`
- `Content-Type`: `application/json`
- `User-Agent`: `Higress-AI-Proxy/1.0.0`

## 监控和日志

### 关键指标

| 指标名称 | 说明 |
|----------|------|
| `ai_proxy_request_total` | 总请求数 |
| `ai_proxy_request_duration_seconds` | 请求耗时 |
| `ai_proxy_token_input_total` | 输入 Token 总数 |
| `ai_proxy_token_output_total` | 输出 Token 总数 |
| `ai_proxy_provider_switch_count` | Provider 切换次数 |

### 日志级别

- **DEBUG**: 详细的处理流程
- **INFO**: 关键操作（配置加载、请求处理等）
- **WARN**: 警告信息（配置缺失、重试等）
- **ERROR**: 错误信息（请求失败等）

## 性能优化

### 推荐配置

```json
{
  "providers": [
    {
      "id": "openai",
      "type": "openai",
      "enableContextCache": true,
      "contextCacheMaxSize": 1000,
      "timeout": 30000,
      "maxRetries": 2
    }
  ]
}
```

### 优化建议

1. **启用上下文缓存**：减少重复请求的处理时间
2. **合理设置超时**：避免长时间等待
3. **配置多 Token**：实现自动故障转移
4. **监控内存使用**：定期检查 WASM 插件内存

## 故障排查

### 问题 1：请求失败

**症状**：日志显示 "Failed to call provider API"

**可能原因**：
- API Token 无效或过期
- 网络连接问题
- API 端点不可用

**解决方案**：
1. 检查 `apiTokens` 配置是否正确
2. 验证网络连接到 API 端点
3. 检查 API 服务状态

### 问题 2：Token 计算错误

**症状**：Token 统计不准确

**可能原因**：
- AI 服务响应格式不匹配
- Token 计算算法版本不匹配

**解决方案**：
1. 检查 AI 服务响应格式
2. 更新插件到最新版本
3. 禁用 Token 自动计算，手动提供

### 问题 3：内存增长

**症状**：插件内存持续增长

**可能原因**：
- 上下文缓存无限增长
- 请求上下文未正确清理

**解决方案**：
1. 限制 `contextCacheMaxSize`
2. 检查缓存清理逻辑
3. 定期重启 Pod

### 问题 4：Provider 切换失败

**症状**：自动切换到备用 Provider 失败

**可能原因**：
- 所有 Provider 的 Token 都无效
- 网络问题导致所有端点不可达

**解决方案**：
1. 验证所有 Provider 的 API Token
2. 检查网络连接
3. 配置健康检查端点

## 版本历史

| 版本 | 日期 | 说明 |
|------|------|------|
| v1.0.0 | 2024-04-08 | 初始版本，支持多协议和 Provider 管理 |

## 技术支持

如有问题，请提交 Issue 到 Higress 项目：

- **GitHub**: https://github.com/alibaba/higress
- **官方文档**: https://higress.io/
- **社区论坛**: https://github.com/alibaba/higress/discussions

## 许可证

Apache License 2.0

## 贡献指南

欢迎提交 Pull Request 改进此插件！

1. Fork 本项目
2. 创建特性分支
3. 提交更改
4. 推送到分支
5. 创建 Pull Request

## 相关链接

- [Higress 官方文档](https://higress.io/docs/latest/)
- [AI Proxy 插件源码](./main.go)
- [构建脚本](./build.sh)
- [Dockerfile](./Dockerfile)
