# AI Request ID Response Header Plugin

## 概述

AI Request ID Response Header 插件用于把 Envoy request ID 透出到客户端响应头，便于客户端本地 session 与服务端 token usage 上报、账单记录通过同一个 `request_id` 对齐。

## 核心行为

- 在 response headers 阶段读取 Envoy request ID：

```go
proxywasm.GetProperty([]string{"request", "id"})
```

- 当 request ID 非空且不为 `-` 时，写入响应头：

```text
request_id: <envoy request id>
```

- 当 request ID 为空、`-` 或读取失败时，不写响应头，不影响主请求链路。

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

该插件只处理响应头，不读取请求体或响应体。`ai-token-report` 使用同一个 Envoy request ID 作为上报体中的 `requestId`，以保证客户端响应头和 token usage 上报链路的 request id 一致。
