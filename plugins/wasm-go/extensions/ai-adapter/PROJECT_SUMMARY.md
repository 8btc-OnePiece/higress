# AI Adapter Plugin 项目总结

## 项目概述

AI Adapter 插件是一个为 Higress AI 网关设计的插件，用于解决不同 AI 服务渠道之间的协议差异问题。该插件支持模型与渠道的映射、请求/响应转换、URL 重写、参数映射等多种功能。

## 项目结构

```
ai-adapter/
├── main.go                      # 插件主逻辑
├── main_test.go                 # 单元测试
├── go.mod                       # Go 模块定义
├── go.sum                       # Go 依赖版本锁定
├── VERSION                      # 版本号
├── Dockerfile                   # Docker 镜像构建
├── Makefile                     # Make 构建文件
├── build.sh                     # Shell 构建脚本
├── plugin.yaml                  # 插件元数据配置
├── .gitignore                   # Git 忽略配置
├── README.md                    # 中文文档
├── README_EN.md                 # 英文文档
├── config-example.json          # 完整配置示例
├── test-config-azure.json       # Azure 渠道配置示例
├── test-config-rightcode.json   # RightCode 渠道配置示例
├── test-config-qiniu.json       # Qiniu 渠道配置示例
└── deployment-example.yaml      # Kubernetes 部署示例
```

## 核心功能

### 1. 模型与渠道映射
- 支持精确匹配（如 `gpt-image-2`）
- 支持通配符匹配（如 `gpt-*`）
- 支持默认渠道配置

### 2. 请求转换
- **URL 重写**：支持直接路径设置和模式替换
- **参数转换**：支持参数重命名、删除和添加
- **格式转换**：支持 JSON 到 multipart/form-data 转换
- **头转换**：支持头的添加、删除和重命名
- **Body 转换**：支持 JSON 路径操作

### 3. 响应转换
- 支持与请求转换相同的转换类型
- 独立启用/禁用控制

### 4. 内存安全保证
- 配置复用：配置在插件启动时加载并复用
- 及时清理：请求处理完成后，上下文中的临时数据自动清理
- 大小限制：请求和响应 body 大小限制为 100MB
- 内存重建：每处理 1000 个请求或内存超过 200MB 后重建配置

## 配置示例

### Azure 渠道 - Multipart 转换

```json
{
  "modelProviderMappings": [
    {
      "model": "gpt-image-2",
      "provider": "Azure",
      "requestTransform": {
        "type": "format_transform",
        "formatTransform": {
          "targetFormat": "multipart",
          "multipartConfig": {
            "fieldMapping": {
              "image": "file"
            }
          }
        }
      }
    }
  ],
  "enableRequestTransform": true
}
```

### RightCode 渠道 - URL 重写

```json
{
  "modelProviderMappings": [
    {
      "model": "gpt-image-2",
      "provider": "RightCode",
      "requestTransform": {
        "type": "url_rewrite",
        "urlRewrite": {
          "fromPattern": "/v1/images/edits",
          "toPattern": "/v1/images/generations"
        }
      }
    }
  ],
  "enableRequestTransform": true
}
```

### Qiniu 渠道 - 不转换

```json
{
  "modelProviderMappings": [
    {
      "model": "gpt-image-2",
      "provider": "Qiniu",
      "requestTransform": {
        "type": "none"
      }
    }
  ],
  "enableRequestTransform": false
}
```

## 构建和部署

### 构建

```bash
# 使用 Go 1.24 原生编译
GOOS=wasip1 GOARCH=wasm go build -buildmode=c-shared -o main.wasm

# 使用构建脚本
./build.sh

# 使用 Makefile
make build
```

### Docker 镜像

```bash
# 构建镜像
docker build -t ai-adapter:1.0.0 .

# 推送到镜像仓库
docker push your-registry/ai-adapter:1.0.0
```

### Kubernetes 部署

参考 `deployment-example.yaml` 文件中的配置。

## 插件执行顺序

```
请求 → key-auth (鉴权) → model-router (模型路由) → ai-proxy (Body重写) → ai-adapter (请求转换) → 上游服务
                                                                                ↓
响应 ← ai-adapter (响应转换) ← 上游响应
```

插件配置：
- **阶段**: AUTHN
- **优先级**: 500 (确保在 ai-proxy 之后执行)

## 内存管理

### 内存限制配置

```go
wrapper.WithRebuildAfterRequests[PluginConfig](1000)        // 1000次请求后重建
wrapper.WithRebuildMaxMemBytes[PluginConfig](200*1024*1024) // 200MB内存限制
```

### Body 大小限制

```go
const defaultMaxBodyBytes uint32 = 100 * 1024 * 1024  // 100MB
```

### 内存释放机制

1. **上下文清理**：每次请求完成后，上下文中的临时数据自动清理
2. **配置复用**：配置对象在插件启动时创建，后续只读
3. **及时释放**：临时变量在函数作用域内自动回收
4. **周期重建**：定期重建插件配置，释放累积的内存

## 测试

```bash
# 运行单元测试
go test -v ./...

# 使用构建脚本测试
./build.sh
```

## 依赖项

- `github.com/higress-group/proxy-wasm-go-sdk` v0.0.0-20251103120604-77e9cce339d2
- `github.com/higress-group/wasm-go` v1.0.10-0.20260120033417-1c84f010156d
- `github.com/tidwall/gjson` v1.18.0
- `github.com/tidwall/sjson` v1.2.5

## 注意事项

1. **Provider 识别**：插件通过请求头 `X-Provider` 或 Envoy 属性 `wasm.ai_provider` 来识别渠道
2. **Base64 图片处理**：支持将 Base64 编码的图片 data URI 自动转换为 multipart file 格式
3. **查询参数保留**：URL 重写时会保留原始请求的查询参数
4. **错误处理**：转换失败时会记录错误日志，但不会阻塞请求继续处理
5. **性能考虑**：复杂的格式转换（如 multipart）会有一定的性能开销

## 版本历史

### v1.0.0 (2025-05-15)
- 初始版本发布
- 支持模型与渠道映射
- 支持多种请求/响应转换类型
- 支持 multipart 格式转换
- 内存安全保证
- 完整的文档和示例

## 许可证

Apache License 2.0

## 贡献

欢迎提交 Issue 和 Pull Request。