# AI Adapter 插件快速入门指南

## 1. 快速构建

```bash
cd /Users/xiaodian/IdeaProjects/higress/plugins/wasm-go/extensions/ai-adapter

# 使用构建脚本
./build.sh

# 或使用 Go 直接编译
GOOS=wasip1 GOARCH=wasm go build -buildmode=c-shared -o main.wasm

# 编译成功后会生成 main.wasm 文件 (约 5.6MB)
```

## 2. 快速配置

### 基本配置结构

```json
{
  "modelProviderMappings": [
    {
      "model": "模型名称",
      "provider": "渠道名称",
      "requestTransform": {
        "type": "转换类型",
        "...": "具体配置"
      }
    }
  ],
  "defaultProvider": "默认渠道",
  "enableRequestTransform": true,
  "enableResponseTransform": false
}
```

### 三种典型场景

#### 场景1：Azure 渠道 - 图片转换为 multipart

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

#### 场景2：RightCode 渠道 - URL 重写

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

#### 场景3：Qiniu 渠道 - 不转换

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

## 3. 快速部署

### 构建 Docker 镜像

```bash
# 构建镜像
docker build -t ai-adapter:1.0.0 .

# 推送到镜像仓库
docker tag ai-adapter:1.0.0 your-registry/ai-adapter:1.0.0
docker push your-registry/ai-adapter:1.0.0
```

### 在 Higress 中配置

```yaml
apiVersion: extensions.higress.io/v1alpha1
kind: WasmPlugin
metadata:
  name: ai-adapter
  namespace: higress-system
spec:
  image: your-registry/ai-adapter:1.0.0
  priority: 500
  phase: AUTHN
  defaultConfig: |
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

## 4. 支持的转换类型

| 类型 | 说明 | 配置字段 |
|------|------|----------|
| `url_rewrite` | URL 路径重写 | `urlRewrite` |
| `param_transform` | JSON 参数转换 | `paramTransform` |
| `format_transform` | 格式转换（JSON↔multipart） | `formatTransform` |
| `header_transform` | HTTP 头转换 | `headerTransform` |
| `body_transform` | JSON Body 路径操作 | `bodyTransform` |
| `none` | 不进行转换 | - |

## 5. 使用示例

### 在请求中指定 Provider

插件通过以下方式识别 Provider：

1. **请求头**：`X-Provider: Azure`
2. **Envoy 属性**：`wasm.ai_provider`

### 请求示例

```bash
# 指定 Azure 渠道
curl -X POST http://your-gateway/v1/images/generations \
  -H "X-Provider: Azure" \
  -H "Content-Type: application/json" \
  -d '{
    "model": "gpt-image-2",
    "image": "data:image/png;base64,iVBORw0KGgo..."
  }'

# 插件会自动将 image 参数转换为 multipart file 格式
```

## 6. 故障排查

### 检查日志

```bash
# 查看插件日志
kubectl logs -n higress-system <gateway-pod> | grep ai-adapter
```

### 常见问题

**Q: 插件没有生效？**
A: 检查优先级设置，确保在 key-auth、model-router、ai-proxy 之后执行。

**Q: Provider 无法识别？**
A: 确保请求头中设置了 `X-Provider` 或之前的插件设置了 `wasm.ai_provider` 属性。

**Q: 转换失败但请求继续？**
A: 这是预期行为，转换失败时只记录错误日志，不会阻塞请求。

## 7. 更多信息

- 完整文档：README.md
- 配置示例：config-example.json
- 部署示例：deployment-example.yaml
- 项目总结：PROJECT_SUMMARY.md