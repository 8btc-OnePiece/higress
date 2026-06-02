# Model Router Plugin

Higress WASM 插件，用于基于规则自动路由到不同的模型。

## 功能特性

1. **模型路由**: 根据请求体中的 `model` 字段或 `*` 通配符路由到指定的模型
2. **前缀匹配**: 支持按前缀匹配模型名称（如 `openai/*` 路由到 `gpt-4`）
3. **Header 映射**: 将模型信息映射到请求头（`x-higress-llm-model`）
4. **Provider 拆分**: 自动从模型标识（如 `openai/gpt-4`）中提取 provider 并添加到请求头
5. **自动路由**: 支持基于正则表达式的自动路由规则，根据用户消息内容自动选择模型
6. **路径后缀过滤**: 只对特定路径后缀的请求进行处理（如 `/completions`, `/embeddings` 等）

## 快速开始

### 构建插件

```bash
# 使用 Go 1.24 原生编译（默认）
./build.sh

# 使用 TinyGo 编译（生成更小的 WASM 文件）
./build.sh tinygo

# 或者使用 Makefile
make build-go
```

### 构建并推送镜像

```bash
# 一键构建和推送
./build-and-push.sh

# 或者使用 Makefile
make all
```

## 配置说明

### 基础配置

```json
{
  "modelKey": "model",
  "modelMapping": {
    "gpt-3.5": "gpt-4",
    "gpt-4": "gpt-4-32k",
    "openai/*": "gpt-4",
    "claude/*": "claude-3-sonnet",
    "*": "gpt-4"
  },
  "defaultModel": "gpt-4",
  "enableOnPathSuffix": [
    "/completions",
    "/embeddings",
    "/audio/speech",
    "/fine_tuning/jobs",
    "/moderations",
    "/image-synthesis",
    "/video-synthesis",
    "/rerank",
    "/messages"
  ],
  "addProviderHeader": "x-higress-llm-provider",
  "modelToHeader": "x-higress-llm-model"
}
```

### 自动路由配置

```json
{
  "autoRouting": {
    "enable": true,
    "defaultModel": "gpt-4",
    "rules": [
      {
        "pattern": ".*图片.*",
        "model": "gpt-4-vision"
      },
      {
        "pattern": ".*代码.*|.*编程.*",
        "model": "claude-3-sonnet"
      },
      {
        "pattern": ".*翻译.*",
        "model": "gpt-4"
      }
    ]
  }
}
```

## 在 Higress 中使用

```yaml
apiVersion: extensions.higress.io/v1alpha1
kind: WasmPlugin
metadata:
  name: model-router
spec:
  priority: 200
  image: docker.swr.cn-east-3.myhuaweicloud.com/btc8_public/higress-model-router:1.0.0
  config: |
    {
      "modelKey": "model",
      "modelMapping": {
        "*": "gpt-4"
      }
    }
```

## 使用示例

### 示例 1: 简单模型映射

请求:
```json
{
  "model": "gpt-3.5"
}
```

处理结果:
- `model` 字段被替换为 `gpt-4`
- 添加请求头 `x-higress-llm-model: gpt-4`

### 示例 2: Provider 拆分

请求:
```json
{
  "model": "openai/gpt-4"
}
```

配置:
```json
{
  "addProviderHeader": "x-higress-llm-provider"
}
```

处理结果:
- 添加请求头 `x-higress-llm-provider: openai`
- 添加请求头 `x-higress-llm-model: gpt-4`

### 示例 3: 前缀匹配

配置:
```json
{
  "modelMapping": {
    "claude/*": "claude-3-sonnet"
  }
}
```

请求:
```json
{
  "model": "claude-3-opus"
}
```

处理结果:
- `model` 字段被替换为 `claude-3-sonnet`

### 示例 4: 自动路由

Chat API 请求:
```json
{
  "messages": [
    {"role": "assistant", "content": "你好"},
    {"role": "user", "content": "请帮我处理这张图片"}
  ],
  "model": "higress/auto"
}
```

配置:
```json
{
  "autoRouting": {
    "enable": true,
    "rules": [
      {"pattern": ".*图片.*", "model": "gpt-4-vision"}
    ]
  }
}
```

处理结果:
- 用户消息匹配正则 `.*图片.*`
- `model` 字段被替换为 `gpt-4-vision`
- 添加请求头 `x-higress-llm-model: gpt-4-vision`

## 文件说明

- `main.go`: 插件主程序
- `Makefile`: Make 构建配置
- `Dockerfile`: Docker 镜像构建文件
- `VERSION`: 版本号
- `build.sh`: 构建脚本（支持 Go 原生编译和 TinyGo）
- `build-and-push.sh`: 一键构建并推送镜像
- `README.md`: 中文使用文档

## 版本

当前版本: 1.0.0
