# WASM 插件中 HTTP 客户端的重要限制

## 问题

在 WASM 插件环境中，**不能直接使用标准 Go 的 `net/http.Client`**。

## 原因

WASM 插件运行在 Envoy 的沙箱环境中，受到以下限制：

1. **没有直接网络访问**：WASM 无法直接创建 TCP 连接
2. **必须通过 Envoy**：所有 HTTP 请求必须通过 Envoy 代理
3. **只能使用 DispatchHttpCall**：`proxywasm.DispatchHttpCall` 是唯一的 HTTP 调用接口

## proxywasm.DispatchHttpCall

签名：
```go
func DispatchHttpCall(
    cluster string,           // Envoy cluster 名称（必需！）
    headers [][2]string,     // HTTP headers
    body []byte,             // 请求 body
    trailers [][2]string,    // Trailers
    timeout uint32,          // 超时时间（毫秒）
    callback func(...)       // 回调函数
) error
```

**关键问题**：需要提供 `cluster` 参数，这个 cluster 必须在 Envoy 中预先配置！

## 现状分析

### 方案对比

| 方案 | Cluster 类型 | 问题 | 结果 |
|------|------------|------|------|
| DnsCluster | outbound\|80\|\|xxx.dns | 外部域名不支持 | bad argument |
| FQDNCluster | outbound\|443\|\|xxx | 可能需要预配置 | bad argument |
| RouteCluster | 当前请求的 cluster | 复用现有路由 | 404 路由错误 |

### 根本问题

**Envoy 的 cluster 机制是为内部服务设计的**，对于外部公网 API：
- 需要在 Envoy 配置中预定义 cluster
- 或者使用特殊的 cluster 类型（如 strict_dns, logical_dns）
- WASM 插件无法动态创建新的 cluster

## 可能的解决方案

### 1. 使用预配置的 Envoy Cluster（推荐）

在 Higress/Envoy 配置中预先配置外部服务 cluster：

```yaml
apiVersion: networking.istio.io/v1alpha3
kind: ServiceEntry
metadata:
  name: external-api
spec:
  hosts:
  - pref-gate.wujieai.com
  ports:
  - number: 443
    name: https
    protocol: HTTPS
  resolution: DNS
  location: MESH_EXTERNAL
```

然后在插件中使用 FQDNCluster：

```go
config.client = wrapper.NewClusterClient(wrapper.FQDNCluster{
    FQDN: "pref-gate.wujieai.com",
    Host: "pref-gate.wujieai.com",
    Port: 443,
})
```

### 2. 通过当前请求的 upstream 转发

如果外部 API 已在 Higress 中配置为服务，可以通过 upstream 访问：

```go
config.client = wrapper.NewClusterClient(wrapper.RouteCluster{
    Host: "external-service.default.svc.cluster.local",
})
```

### 3. 使用 ext-auth 模式（架构调整）

将外部 API 调用移到 ext-auth 服务，插件通过 ext-auth 获取结果。

## 当前建议

1. **最简单**：在 Higress 中配置 ServiceEntry，使用 FQDNCluster
2. **最灵活**：通过独立的后端服务调用外部 API
3. **临时方案**：继续使用 RouteCluster，但检查 404 的具体原因

## 关于"使用标准 HTTP 客户端"

**不可行**，因为：
- WASM 没有直接网络访问
- 必须通过 Envoy 的 cluster 机制
- proxywasm 只提供了 DispatchHttpCall 接口

即使我们尝试使用 `http.Client`，编译时也会失败或运行时报错，因为 WASM 环境不支持。
