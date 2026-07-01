# AI Request ID Header Plugin

## 概述

AI Request ID Header 插件用于在网关侧传播同一个 request ID：

- request headers 阶段写入 upstream `request-id`，便于 open-platform 等上游服务读取；
- response headers 阶段写入客户端响应头 `request-id`，便于客户端本地 session 记录；
- 与 `ai-token-report` 使用同一 request ID 来源，保证 token usage 上报与 header 链路一致。
- 可按 Envoy request id 追加 HMAC 签名段，生成可信 canonical request-id，避免直接透传外部伪造 ID。

## 核心行为

- 读取 request ID 来源：

```go
proxywasm.GetProperty([]string{"x_request_id"})
proxywasm.GetHttpRequestHeader("request-id")
proxywasm.GetHttpRequestHeader("x-request-id")
```

- 当配置 `signatureSecret` 后，插件会生成或校验 canonical request-id：

```text
<envoy-request-id>-<sig8>
```

其中 `sig8` 默认为 `HMAC-SHA256(rawRequestID, signatureSecret)` 的前 8 位 hex 字符。外部传入的 `request-id` 只有签名校验通过才会复用；校验失败、格式不合法或为空时，基于 Envoy `x_request_id` 重新生成。Envoy request id 也不可用时，才 fallback 生成本地 UUID 并签名。

- 请求阶段会把最终 request id 写入请求内属性，供后续插件读取：

```go
proxywasm.SetProperty([]string{"wasm.canonicalRequestID"}, []byte("<canonical request id>"))
```

- 当 request ID 非空且不为 `-`，且请求不是转发给 AI 提供商时，在 request headers 阶段写入 upstream header：

```text
request-id: <request id>
```

- 同时在 response headers 阶段写入客户端响应头（无论是否转发给 AI 提供商，响应头都返回给客户端）：

```text
request-id: <request id>
```

- 当 request ID 为空、`-` 或读取失败时，不写 header，不影响主请求链路。

## AI 提供商路由识别

为避免向 AI 提供商（如 rightcode）注入 `request-id` 导致上游返回 404，插件在 request headers 阶段先判断当前请求是否转发给 AI 提供商，是则跳过 upstream header 注入。判定顺序：

1. **主信号**：读取 `wasm.providerType` 属性。该属性由 `ai-proxy` 在请求阶段写入（见 `ai-proxy/main.go` 的 `onHttpRequestHeader`），与 `ai-token-report` 读取该属性的方式一致。属性非空即视为 AI 路由。
   - 前提：本插件的执行优先级必须低于 `ai-proxy`，否则属性尚未写入会导致漏判。
2. **兜底信号**：读取 `cluster_name` 属性，命中配置的 `skipClusterNamePatterns` 中任一子串即视为 AI 路由。覆盖绕开 `ai-proxy` 直连 AI 厂商的路由（如 `right-codes.dns`）。

response headers 阶段不进行该判断——响应头只返回给客户端，不会到达 AI 提供商。

## 配置说明

```json
{
  "skipClusterNamePatterns": ["llm-", "right-codes.dns", "right-codes-v2.dns"],
  "signatureSecret": "<secret>",
  "signatureLength": 8
}
```

| 字段 | 类型 | 默认值 | 说明 |
| --- | --- | --- | --- |
| `skipClusterNamePatterns` | `[]string` | `["llm-"]` | cluster_name 子串匹配列表，命中任一则跳过 upstream `request-id` 注入。传空数组 `[]` 可关闭 cluster_name 兜底（仅依赖 `wasm.providerType`）。 |
| `signatureSecret` | `string` | `""` | canonical request-id 签名密钥。为空时保持旧行为，不启用签名校验，不阻断主链路。 |
| `signatureLength` | `int` | `8` | 签名截断长度，允许范围 4-64；超出范围时使用默认值。 |

默认 `["llm-"]` 对应集群命名约定（`llm-RightCodes.internal.dns` 等 AI 提供商 cluster 均带此前缀）。若存在不带 `llm-` 前缀的直连 AI 路由（如 `right-codes.dns`），需在配置中追加对应 cluster 名。

推荐插件执行顺序：

```text
ai-proxy -> ai-request-id-header -> ai-token-report
```

`ai-request-id-header` 需要在 `ai-proxy` 之后读取 `wasm.providerType`，`ai-token-report` 需要在其之后读取 `wasm.canonicalRequestID`。

## 构建

```bash
go test ./...
GOOS=wasip1 GOARCH=wasm go build -buildmode=c-shared -o main.wasm ./
```

## 说明

该插件只处理 request/response headers，不读取请求体或响应体。`ai-token-report` 使用同一个 request ID 作为上报体中的 `requestId`，以保证 upstream request header、客户端 response header 和 token usage 上报链路一致。

## 迁移说明

该插件由旧名 `ai-request-id-response-header` 改名为 `ai-request-id-header`。如果已有部署使用旧 WasmPlugin 名称或镜像名，需要同步迁移到新名称。
