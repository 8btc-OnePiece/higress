---
title: AI KeyName 配额管理
keywords: [ AI网关, AI配额, KeyName限流 ]
description: 基于 KeyName 前缀的 AI 配额管理插件配置参考
---

## 功能说明

`ai-keyname-quota` 插件实现根据 API Key 的 KeyName 前缀进行配额管理。

### 核心特性

1. **动态 KeyName 查询**：通过远端 API 查询 API Key 对应的 KeyName
2. **前缀匹配配额池**：根据 KeyName 前缀匹配不同的配额池，实现多租户或不同业务线的配额隔离
3. **自动配额初始化**：首次访问时自动初始化配额池的配额值
4. **灵活的策略配置**：支持未匹配时的放行或拒绝策略
5. **基于 Token 扣减**：根据实际使用的 AI Token 数量自动扣减配额

### 工作流程

```
请求 → 提取 API Key → 远端 API 查询 KeyName → 匹配配额池 → 检查配额 → 扣减 Token 配额
```

### 插件组合

`ai-keyname-quota` 插件需要配合以下插件使用：

- **认证插件**（如 `key-auth`、`jwt-auth`）：用于获取 API Key
- **ai-statistics**：用于获取 AI Token 统计信息

## 运行属性

插件执行阶段：`认证阶段 (AUTHN)`
插件执行优先级：`290`

## 配置说明

### 全局配置

| 名称 | 数据类型 | 填写要求 | 默认值 | 描述 |
|------|---------|---------|-------|------|
| `authHeader` | string | 选填 | Authorization | 从哪个请求头获取 API Key |
| `apiKeyParamName` | string | 选填 | key | 调用 keyname 查询接口时，传递 API Key 的参数名 |
| `global_redis_key_prefix` | string | 选填 | keyname_quota: | 所有配额池的 Redis key 前缀 |
| `unmatched_action` | string | 选填 | continue | 当 keyname 不匹配任何配额池时的行为：continue(放行) 或 reject(拒绝) |

### KeyName 查询服务配置 (`keyNameService`)

| 配置项 | 类型 | 必填 | 默认值 | 说明 |
|--------|------|------|--------|------|
| `service_name` | string | 必填 | - | Higress Console 中的服务名称，例如 keyname-service.default.svc.cluster.local |
| `api_url` | string | 必填 | - | 查询 keyname 接口的完整 URL |
| `timeout` | int | 否 | 5000 | HTTP 调用超时时间，单位毫秒 |

### Redis 配置 (`redis`)

| 配置项 | 类型 | 必填 | 默认值 | 说明 |
|--------|------|------|--------|------|
| `service_name` | string | 必填 | - | Redis 服务名称，带服务类型的完整 FQDN 名称 |
| `service_port` | int | 否 | 6379 | Redis 服务端口 |
| `username` | string | 否 | - | Redis 用户名 |
| `password` | string | 否 | - | Redis 密码 |
| `timeout` | int | 否 | 1000 | Redis 连接超时时间，单位毫秒 |
| `database` | int | 否 | 0 | 使用的数据库 id |

### 配额池配置 (`quota_pools`)

| 配置项 | 类型 | 必填 | 说明 |
|--------|------|------|------|
| `key_name_prefix` | string | 必填 | 匹配的 keyname 前缀，例如 "外部培训"、"内部" |
| `quota_limit` | int | 必填 | 该配额池的 token 数量限制，例如 1000000 (100w) |
| `redis_key_prefix` | string | 选填 | 该配额池在 Redis 中的 key 后缀，默认使用 key_name_prefix |

### 远端 API 响应格式

插件支持多种响应格式：

#### 格式 1：数组格式
```json
[
  {
    "keyname": "外部培训-key-001"
  }
]
```

#### 格式 2：对象格式
```json
{
  "keyname": "内部-key-002"
}
```

#### 格式 3：带 data 字段
```json
{
  "data": [
    {
      "keyname": "外部培训-key-003"
    }
  ]
}
```

支持的字段名（优先级从高到低）：
- `keyname`
- `key_name`
- `name`

## 配置示例

### 示例 1：基础配置

```yaml
authHeader: "Authorization"
apiKeyParamName: "key"
global_redis_key_prefix: "keyname_quota:"
unmatched_action: "continue"

keyNameService:
  service_name: keyname-service.default.svc.cluster.local
  api_url: "http://keyname-service:8080/api/keyinfo"
  timeout: 5000

redis:
  service_name: redis-service.default.svc.cluster.local
  service_port: 6379
  timeout: 2000
  database: 0

quota_pools:
  - key_name_prefix: "外部培训"
    quota_limit: 1000000
    redis_key_prefix: "external_training"
  - key_name_prefix: "内部"
    quota_limit: 10000000
    redis_key_prefix: "internal"
```

### 示例 2：严格模式（不匹配则拒绝）

```yaml
authHeader: "Authorization"
apiKeyParamName: "key"
global_redis_key_prefix: "keyname_quota:"
unmatched_action: "reject"

keyNameService:
  service_name: keyname-service.default.svc.cluster.local
  api_url: "http://keyname-service:8080/api/keyinfo"
  timeout: 5000

redis:
  service_name: redis-service.default.svc.cluster.local
  service_port: 6379
  timeout: 2000
  database: 0

quota_pools:
  - key_name_prefix: "外部培训"
    quota_limit: 1000000
  - key_name_prefix: "内部"
    quota_limit: 10000000
```

## 工作原理

### 1. 提取 API Key

从请求头（默认为 `Authorization`）中提取 API Key：
```
Authorization: Bearer sk-xxxxx-xxxxx
```

提取后得到：`sk-xxxxx-xxxxx`

### 2. 查询 KeyName

调用远端 API 查询 KeyName：
```
GET http://keyname-service:8080/api/keyinfo?key=sk-xxxxx-xxxxx
```

响应：
```json
{
  "keyname": "外部培训-key-001"
}
```

### 3. 匹配配额池

根据 KeyName 前缀匹配配额池：
- `外部培训-key-001` → 匹配 `外部培训` 配额池
- 配额池限制：1000000 tokens

### 4. 检查配额

从 Redis 查询当前配额：
```
Redis Key: keyname_quota:external_training
```

- 首次访问：初始化为 1000000
- 后续访问：检查剩余配额
- 配额耗尽：拒绝请求（返回 403）

### 5. 扣减配额

根据实际使用的 Token 数量扣减配额：
```
input_tokens: 100
output_tokens: 200
total: 300

Redis 操作: DECRBY keyname_quota:external_training 300
```

## Redis 数据结构

```
keyname_quota:external_training  → 999700  (剩余配额)
keyname_quota:internal          → 9999500 (剩余配额)
```

## 使用场景

### 场景 1：企业内部外部培训配额管理

- **内部员工**：1000w tokens（不限制）
- **外部培训**：100w tokens（严格限制）

```yaml
quota_pools:
  - key_name_prefix: "内部"
    quota_limit: 10000000
  - key_name_prefix: "外部培训"
    quota_limit: 1000000
    redis_key_prefix: "external_training"
```

### 场景 2：多业务线配额隔离

- **研发线**：5000w tokens
- **市场线**：1000w tokens
- **测试线**：500w tokens

```yaml
quota_pools:
  - key_name_prefix: "研发"
    quota_limit: 50000000
    redis_key_prefix: "rd"
  - key_name_prefix: "市场"
    quota_limit: 10000000
    redis_key_prefix: "marketing"
  - key_name_prefix: "测试"
    quota_limit: 5000000
    redis_key_prefix: "qa"
```

## 错误码

| 状态码 | 错误详情 | 说明 |
|--------|---------|------|
| 403 | ai-keyname-quota.unmatched_keyname | KeyName 不匹配任何配额池 |
| 403 | ai-keyname-quota.no_quota | 配额已耗尽 |
| 503 | ai-keyname-quota.api_error | 远端 API 调用失败 |
| 503 | ai-keyname-quota.http_error | HTTP 调用失败 |

## 注意事项

1. **配额初始化**：配额池首次访问时才会初始化，初始化后配额值固定
2. **配额管理**：如需动态调整配额，建议通过 Redis 管理工具直接修改 Redis 中的配额值
3. **API 可用性**：远端 API 的可用性直接影响插件功能，建议 `unmatched_action` 根据实际需求选择
4. **Token 计算**：配额基于实际使用的 Token 数量（input + output）扣减
5. **并发控制**：配额检查和扣减在 Redis 层面保证原子性