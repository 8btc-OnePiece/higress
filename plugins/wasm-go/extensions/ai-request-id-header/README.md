# AI Request ID Header Plugin

## 概述

AI Request ID Header 插件用于在网关侧传播同一个 request ID：

- request headers 阶段写入 upstream `request-id`，便于 open-platform 等上游服务读取；
- response headers 阶段写入客户端响应头 `request-id`，便于客户端本地 session 记录；
- 与 `ai-token-report` 使用同一 request ID 来源，保证 token usage 上报与 header 链路一致。

## 核心行为

- 读取 request ID 来源：

```go
proxywasm.GetProperty([]string{"x_request_id"})
proxywasm.GetHttpRequestHeader("request-id")
proxywasm.GetHttpRequestHeader("x-request-id")
```

- 当 request ID 非空且不为 `-` 时，在 request headers 阶段写入 upstream header：

```text
request-id: <request id>
```

- 同时在 response headers 阶段写入客户端响应头：

```text
request-id: <request id>
```

- 当 request ID 为空、`-` 或读取失败时，不写 header，不影响主请求链路。

## 配置说明

插件无需配置。

```json
{}
```

## 构建

```bash
go test ./...
GOOS=wasip1 GOARCH=wasm go build -buildmode=c-shared -o main.wasm ./
```

## 说明

该插件只处理 request/response headers，不读取请求体或响应体。`ai-token-report` 使用同一个 request ID 作为上报体中的 `requestId`，以保证 upstream request header、客户端 response header 和 token usage 上报链路一致。

## 迁移说明

该插件由旧名 `ai-request-id-response-header` 改名为 `ai-request-id-header`。如果已有部署使用旧 WasmPlugin 名称或镜像名，需要同步迁移到新名称。
