# Consumer Group Mapping Plugin

## 功能说明

消费者分组映射插件，通过调用外部接口动态获取消费者分组信息，实现多个消费者共享认证凭证。

## 核心特性

1. **动态分组查询**：调用外部接口获取消费者所属分组
2. **自动凭证替换**：鉴权前替换为分组凭证，鉴权后还原为原始凭证
3. **非侵入式设计**：上游服务感知不到分组的存在
4. **多种服务发现**：支持 Kubernetes、Nacos、静态 IP、DNS 等服务来源

## 工作流程

```
客户端请求: Authorization: Bearer consumer-key-123
    ↓
[前置] 提取 consumer key
    ↓
[前置] 调用 GET /apiKey/groupInfo?apiKey=consumer-key-123
    ↓
[前置] 获取分组 key: group-key-789
    ↓
[前置] 保存原始 key 到 X-Original-Api-Key
    ↓
[前置] 替换 Authorization: Bearer group-key-789
    ↓
[Key Auth] 使用分组凭证认证
    ↓
[后置] 还原 Authorization: Bearer consumer-key-123
    ↓
发送到上游服务
```

## 快速开始

### 1. 部署分组信息服务

首先需要部署一个提供分组信息查询的 API 服务：

```go
// 示例 Go 服务
package main

import (
    "net/http"
    "encoding/json"
)

type GroupInfoResponse struct {
    Code    int         `json:"code"`
    Message string      `json:"message"`
    Data    GroupData   `json:"data"`
}

type GroupData struct {
    GroupKey  string `json:"groupKey"`
    GroupName string `json:"groupName"`
}

func groupInfoHandler(w http.ResponseWriter, r *http.Request) {
    apiKey := r.URL.Query().Get("apiKey")

    // 根据业务逻辑查询该 apiKey 所属的分组
    groupKey, groupName := getGroupByApiKey(apiKey)

    response := GroupInfoResponse{
        Code:    200,
        Message: "success",
        Data: GroupData{
            GroupKey:  groupKey,
            GroupName: groupName,
        },
    }

    json.NewEncoder(w).Encode(response)
}

func getGroupByApiKey(apiKey string) (string, string) {
    // 实现你的业务逻辑
    // 例如：从数据库查询、缓存查询等
    return "group-shared-key-789", "production-group"
}

func main() {
    http.HandleFunc("/apiKey/groupInfo", groupInfoHandler)
    http.ListenAndServe(":8080", nil)
}
```

### 2. 配置插件

```yaml
global:
  consumer-group-mapping:
    authHeader: Authorization
    groupInfoApi: /apiKey/groupInfo
    apiKeyParamName: apiKey

    # Kubernetes 服务配置
    serviceSource: k8s
    serviceName: api-group-service
    servicePort: 8080
    namespace: default
```

### 3. 配置 Key Auth 插件

```yaml
global:
  key-auth:
    consumers:
      - name: production-group
        credential: group-shared-key-789
      - name: staging-group
        credential: group-shared-key-staging

    keys:
      - Authorization
    in_header: true
    global_auth: true
```

### 4. 测试

```bash
# 使用消费者自己的 API Key 发送请求
curl -H "Authorization: Bearer consumer-key-123" \
     http://gateway/api/test

# 插件会自动：
# 1. 查询到该 key 属于 production-group
# 2. 替换为 group-shared-key-789 进行认证
# 3. 认证后还原为 consumer-key-123
# 4. 上游服务看到的是原始的 consumer-key-123
```

## 配置参数

| 参数 | 类型 | 必填 | 默认值 | 说明 |
|------|------|------|--------|------|
| `authHeader` | string | 否 | `Authorization` | 从哪个请求头获取 API Key |
| `groupInfoApi` | string | 否 | `/apiKey/groupInfo` | 分组信息查询接口路径 |
| `apiKeyParamName` | string | 否 | `apiKey` | 传递给接口的参数名 |
| `serviceSource` | string | 是 | - | 服务来源：k8s、nacos、ip、dns |
| `serviceName` | string | 是 | - | 服务名称或 IP 地址 |
| `servicePort` | number | 是 | - | 服务端口 |
| `namespace` | string | 条件 | - | K8s/Nacos 命名空间 |
| `domain` | string | 条件 | - | DNS 域名 |

## 外部接口规范

### 请求

```http
GET /apiKey/groupInfo?apiKey={consumerApiKey}
```

### 响应

```json
{
  "code": 200,
  "message": "success",
  "data": {
    "groupKey": "group-shared-key-789",
    "groupName": "production-group"
  }
}
```

### 字段说明

- `code`: 响应码，200 表示成功
- `message`: 响应消息
- `data.groupKey`: **必填**，分组的 API Key
- `data.groupName`: 可选，分组名称

## 使用场景

### 场景 1：多租户共享认证

多个租户使用同一个分组凭证认证，但需要分别限流：

```yaml
# 配置
consumer-group-mapping:
  serviceSource: k8s
  serviceName: api-group-service
  servicePort: 8080
  namespace: default

# 不同租户的 API Key 会映射到同一个分组
# 租户 A: consumer-a-123 → production-group
# 租户 B: consumer-b-456 → production-group
# 租户 C: consumer-c-789 → staging-group
```

### 场景 2：环境隔离

同一服务在不同环境使用不同凭证：

```bash
# 生产环境
curl -H "Authorization: Bearer prod-consumer-123" \
     http://gateway/api/test
# → 映射到 production-group

# 预发环境
curl -H "Authorization: Bearer staging-consumer-456" \
     http://gateway/api/test
# → 映射到 staging-group
```

### 场景 3：微服务聚合

多个微服务调用同一个后端 API：

```yaml
# 所有服务都映射到 api-gateway-group
service-a-key → api-gateway-group
service-b-key → api-gateway-group
service-c-key → api-gateway-group
```

## 服务发现配置

### Kubernetes Service

```yaml
serviceSource: k8s
serviceName: api-group-service
servicePort: 8080
namespace: default
```

### Nacos Service

```yaml
serviceSource: nacos
serviceName: api-group-service
servicePort: 8080
namespace: public
```

### Static IP

```yaml
serviceSource: ip
serviceName: 192.168.1.100
servicePort: 8080
```

### DNS

```yaml
serviceSource: dns
serviceName: api-group-service
servicePort: 8080
domain: api.example.com
```

## 故障处理

### 接口调用失败

- **行为**：记录错误日志，使用原始 API Key 继续处理
- **日志**：`group info API returned error status: 503`
- **影响**：不影响请求继续，但可能认证失败

### 返回分组 Key 为空

- **行为**：记录警告日志，使用原始 API Key
- **日志**：`group key is empty in response`
- **建议**：检查分组信息服务的实现

### 服务不可达

- **行为**：HTTP 调用超时，记录错误
- **日志**：`failed to dispatch HTTP call`
- **建议**：检查服务配置和网络连通性

## 与其他插件配合

### 与 Key Auth 配合

```yaml
plugins:
  - name: consumer-group-mapping
    priority: 100  # 先执行

  - name: key-auth
    priority: 321  # 后执行
```

### 与 AI Token 限流配合

```yaml
ai-token-ratelimit:
  ruleItems:
    - limitType: custom
      customKey: "X-Original-Api-Key"  # 使用原始 Key 限流
      configItems:
        - count: 1000
          timeWindow: 60
```

## 监控指标

建议监控以下指标：

1. **接口调用成功率**：分组信息接口的成功率
2. **接口响应时间**：分组信息查询的延迟
3. **分组命中率**：成功获取到分组 Key 的比例
4. **认证成功率**：使用分组凭证认证的成功率

## 性能考虑

1. **缓存建议**：分组信息接口应该实现缓存（如 Redis）
2. **超时设置**：默认 5 秒超时，可根据需要调整
3. **连接池**：使用 HTTP 连接池避免频繁建连
4. **异步调用**：插件使用异步调用，不阻塞主请求

## 最佳实践

1. **高可用**：分组信息服务应该部署多副本
2. **缓存策略**：建议缓存分组映射关系（TTL: 5-10分钟）
3. **降级方案**：接口失败时的降级策略
4. **日志记录**：记录足够的日志用于问题排查
5. **监控告警**：配置接口调用的监控和告警

## 构建和部署

本插件支持两种构建和部署方式：

### 方式一：Docker 镜像（自定义插件模式 - 推荐）

这是最简单的方式，适合快速开发和测试。

```bash
# 1. 编译 wasm 文件
./build.sh

# 2. 构建 Docker 镜像
docker build -t consumer-group-mapping:1.0.0 .

# 3. 推送到镜像仓库
docker push your-registry/consumer-group-mapping:1.0.0
```

在 Higress Console 中使用：
- 镜像地址：`your-registry/consumer-group-mapping:1.0.0`

### 方式二：OCI 镜像（符合 OCI 规范）

符合 Higress Wasm 插件 OCI 镜像规范的完整方式，包含 spec.yaml 元数据。

```bash
# 1. 编译 wasm 文件
./build.sh

# 2. 使用 ORAS 推送到 OCI 仓库
IMAGE_REGISTRY_SERVICE=docker.io \
IMAGE_REPOSITORY=plugins/consumer-group-mapping \
VERSION=1.0.0 \
./push-oci.sh
```

在 Higress Console 中使用：
- 镜像地址：`oci://docker.io/plugins/consumer-group-mapping:1.0.0`

**前置要求**：
- 安装 ORAS CLI：`brew install oras`（macOS）或访问 https://oras.land/cli/

详细使用说明请参考 [USAGE.md](USAGE.md) 文档。

## 许可证

Apache License 2.0
