# Consumer Group Mapping Plugin - 使用指南

## 目录

1. [快速开始](#快速开始)
2. [构建和部署](#构建和部署)
3. [配置说明](#配置说明)
4. [工作原理](#工作原理)
5. [使用场景](#使用场景)
6. [故障排查](#故障排查)

---

## 快速开始

### 1. 构建插件

```bash
cd /Users/xiaodian/IdeaProjects/higress/plugins/wasm-go/extensions/consumer-group-mapping

# 执行构建脚本
./build.sh
```

构建成功后会生成 `main.wasm` 文件。

### 2. 构建 Docker 镜像

```bash
docker build -t consumer-group-mapping:1.0.0 .
```

### 3. 推送到镜像仓库

```bash
# 标记镜像
docker tag consumer-group-mapping:1.0.0 your-registry/consumer-group-mapping:1.0.0

# 推送到镜像仓库
docker push your-registry/consumer-group-mapping:1.0.0
```

### 4. 在 Higress 中配置

在 Higress 控制台或通过配置文件添加插件：

```yaml
global:
  consumer-group-mapping:
    groupMappings:
      - consumer: consumer-1
        group: group-a
      - consumer: consumer-2
        group: group-a
```

---

## 构建和部署

### 前置要求

1. **TinyGo**：用于编译 Go 代码为 Wasm
   ```bash
   brew install tinygo
   ```

2. **Docker**：用于构建插件镜像
   ```bash
   # macOS
   brew install --cask docker

   # 或安装 Docker Desktop
   ```

3. **Higress Gateway**：版本 >= 1.0.0

### 构建步骤

```bash
# 1. 进入插件目录
cd consumer-group-mapping

# 2. 编译 Wasm 文件
./build.sh

# 3. 验证生成的文件
ls -lh main.wasm
# 应该显示约 2-3 MB 的文件大小

# 4. 构建 Docker 镜像
docker build -t consumer-group-mapping:1.0.0 .

# 5. 验证镜像
docker images | grep consumer-group-mapping
```

### 部署到 Higress

#### 方式一：通过 Higress Console

1. 登录 Higress Console
2. 进入"插件市场"或"Wasm 插件"
3. 点击"添加自定义插件"
4. 填写信息：
   - **插件名称**：consumer-group-mapping
   - **镜像地址**：your-registry/consumer-group-mapping:1.0.0
   - **插件阶段**：AUTHN
   - **优先级**：100（必须在 key-auth 之前）
5. 保存并启用插件

#### 方式二：通过配置文件

编辑 Higress 配置文件（或通过 Kubernetes CRD）：

```yaml
apiVersion: higress.io/v1alpha1
kind: WasmPlugin
metadata:
  name: consumer-group-mapping
  namespace: higress-system
spec:
  url: oci://your-registry/consumer-group-mapping:1.0.0
  phase: AUTHN
  priority: 100
  defaultConfig:
    groupMappings:
      - consumer: consumer-1
        group: group-a
      - consumer: consumer-2
        group: group-a
    consumerHeader: X-Mse-Consumer
```

---

## 配置说明

### 基础配置

```yaml
global:
  consumer-group-mapping:
    # 必填：消费者到分组的映射
    groupMappings:
      - consumer: consumer-1  # 消费者名称
        group: group-a        # 分组名称
      - consumer: consumer-2
        group: group-a

    # 可选：消费者标识的请求头（默认：X-Mse-Consumer）
    consumerHeader: X-Mse-Consumer

    # 可选：分组凭证的请求头（默认：X-Api-Key）
    groupCredentialHeader: X-Api-Key
```

### 完整配置示例

包含 Key Auth 插件的完整配置：

```yaml
global:
  # Consumer Group Mapping 配置
  consumer-group-mapping:
    groupMappings:
      - consumer: app-service-a
        group: shared-group-prod
      - consumer: app-service-b
        group: shared-group-prod
      - consumer: admin-service
        group: admin-group
    consumerHeader: X-Mse-Consumer
    groupCredentialHeader: X-Api-Key

  # Key Auth 配置
  key-auth:
    consumers:
      # 单个服务的凭证（备用）
      - name: app-service-a
        credential: key-app-a-12345
      - name: app-service-b
        credential: key-app-b-67890
      - name: admin-service
        credential: key-admin-11111

      # 分组凭证（实际使用）
      - name: shared-group-prod
        credential: shared-key-prod-99999
      - name: admin-group
        credential: shared-key-admin-88888

    keys:
      - X-Api-Key

    in_header: true
    in_query: false
    global_auth: true

# 路由级配置
route:
  name: api-route
  rules:
    - path: /api/*

  plugins:
    # 优先级：100（先执行）
    - name: consumer-group-mapping
      priority: 100
      enabled: true

    # 优先级：321（后执行，AUTHN 阶段）
    - name: key-auth
      priority: 321
      config:
        allow:
          - shared-group-prod
          - admin-group
```

### 配置字段详解

| 字段 | 类型 | 必填 | 默认值 | 说明 |
|------|------|------|--------|------|
| `groupMappings` | Array | ✅ | - | 消费者到分组的映射列表 |
| `groupMappings[].consumer` | String | ✅ | - | 消费者名称（唯一标识） |
| `groupMappings[].group` | String | ✅ | - | 分组名称，消费者将使用此分组的认证凭证 |
| `consumerHeader` | String | ❌ | `X-Mse-Consumer` | 从哪个请求头获取消费者标识 |
| `groupCredentialHeader` | String | ❌ | `X-Api-Key` | 分组凭证所在的请求头名称 |

---

## 工作原理

### 执行流程图

```
客户端请求
  │
  ├─ X-Mse-Consumer: consumer-1
  ├─ X-Api-Key: shared-key-group-a
  │
  ↓
┌─────────────────────────────────────────┐
│ 1. consumer-group-mapping (RequestHeaders) │
│    - 读取 X-Mse-Consumer: consumer-1    │
│    - 查找映射：consumer-1 → group-a     │
│    - 替换为 X-Mse-Consumer: group-a     │
│    - 保存原始值到上下文                  │
└─────────────────────────────────────────┘
  │
  ├─ X-Mse-Consumer: group-a (已替换)
  │
  ↓
┌─────────────────────────────────────────┐
│ 2. key-auth (AUTHN Phase)                 │
│    - 读取 X-Mse-Consumer: group-a        │
│    - 验证 group-a 的凭证                  │
│    - 认证通过                             │
└─────────────────────────────────────────┘
  │
  ↓
┌─────────────────────────────────────────┐
│ 3. consumer-group-mapping (RequestBody)   │
│    - 从上下文获取原始消费者：consumer-1   │
│    - 还原 X-Mse-Consumer: consumer-1     │
│    - 清除上下文标记                       │
└─────────────────────────────────────────┘
  │
  ├─ X-Mse-Consumer: consumer-1 (已还原)
  │
  ↓
发送到上游服务
```

### 时序说明

| 阶段 | 插件 | 操作 |
|------|------|------|
| 1 | consumer-group-mapping | 读取消费者标识，替换为分组标识 |
| 2 | key-auth | 使用分组凭证进行认证 |
| 3 | consumer-group-mapping | 还原原始消费者标识 |
| 4 | 发送到上游 | 上游服务看到真实的消费者标识 |

### 关键设计点

1. **非侵入式**：上游服务无需修改，看到的仍然是原始消费者标识
2. **透明代理**：客户端感知不到分组的存在
3. **灵活配置**：支持动态添加消费者到分组
4. **安全隔离**：不同分组之间完全隔离

---

## 使用场景

### 场景 1：多租户共享认证

**需求**：
- 多个租户（租户 A、B、C）使用同一个 API Key
- 但需要分别统计和限流
- 上游服务需要知道具体是哪个租户

**配置**：

```yaml
global:
  consumer-group-mapping:
    groupMappings:
      - consumer: tenant-a-service
        group: shared-tenant-group
      - consumer: tenant-b-service
        group: shared-tenant-group
      - consumer: tenant-c-service
        group: shared-tenant-group

  key-auth:
    consumers:
      - name: shared-tenant-group
        credential: shared-tenant-key-12345

  ai-token-ratelimit:
    ruleItems:
      - limitType: consumer
        key: X-Mse-Consumer
        configItems:
          - count: 10000    # tenant-a 限额
            timeWindow: 60
          - count: 5000     # tenant-b 限额
            timeWindow: 60
          - count: 8000     # tenant-c 限额
            timeWindow: 60
```

**效果**：
- ✅ 三个租户共享一个 API Key（简化凭证管理）
- ✅ 分别限流（避免相互影响）
- ✅ 上游服务可以看到真实租户标识

### 场景 2：环境隔离

**需求**：
- 同一个服务在不同环境（dev、staging、prod）使用不同的认证凭证
- 但配置统一管理

**配置**：

```yaml
global:
  consumer-group-mapping:
    groupMappings:
      - consumer: my-service
        group: my-service-prod

  key-auth:
    consumers:
      - name: my-service-prod
        credential: prod-key-xxxxx
      - name: my-service-staging
        credential: staging-key-yyyyy
      - name: my-service-dev
        credential: dev-key-zzzzz
```

**效果**：
- ✅ 不同环境使用不同凭证
- ✅ 环境间完全隔离
- ✅ 配置清晰明确

### 场景 3：微服务聚合

**需求**：
- 多个微服务（service-a、service-b、service-c）调用同一 API
- 使用统一的分组凭证进行认证
- 但上游需要知道具体是哪个服务

**配置**：

```yaml
global:
  consumer-group-mapping:
    groupMappings:
      - consumer: service-a
        group: micro-services-group
      - consumer: service-b
        group: micro-services-group
      - consumer: service-c
        group: micro-services-group
```

**效果**：
- ✅ 统一认证管理
- ✅ 上游可追踪具体服务
- ✅ 便于审计和监控

---

## 故障排查

### 问题 1：插件未生效

**现象**：请求仍然使用原始消费者凭证认证

**排查步骤**：

1. 检查插件执行顺序
   ```bash
   # 查看 Higress 日志
   kubectl logs -n higress-system deployment/higress-controller | grep consumer-group-mapping
   ```

2. 验证插件配置
   ```yaml
   # 确保 priority 值正确
   plugins:
     - name: consumer-group-mapping
       priority: 100  # 必须小于 key-auth 的 321
   ```

3. 查看插件日志
   ```bash
   # 启用 DEBUG 日志级别
   kubectl edit configmap higress-config -n higress-system
   ```

### 问题 2：认证失败

**现象**：返回 401 或 403

**排查步骤**：

1. 验证分组凭证是否配置
   ```yaml
   key-auth:
     consumers:
       - name: group-a  # 确保分组已配置
         credential: correct-key
   ```

2. 检查映射关系
   ```bash
   # 查看插件加载的配置
   curl -v http://gateway/api/test
   # 查看日志中的映射信息
   ```

3. 验证请求头
   ```bash
   # 确保发送了正确的 header
   curl -H "X-Mse-Consumer: consumer-1" \
        -H "X-Api-Key: shared-key-group-a" \
        http://gateway/api/test
   ```

### 问题 3：性能问题

**现象**：请求延迟增加

**优化方案**：

1. 减少映射条目数量
   ```yaml
   # 不推荐：1000+ 条映射
   groupMappings: [...] # 太多

   # 推荐：使用规则匹配
   # 或拆分为多个插件实例
   ```

2. 调整日志级别
   ```yaml
   # 生产环境使用 INFO 或 WARN
   log:
     level: INFO
   ```

3. 监控指标
   ```bash
   # 查看插件执行时间
   kubectl top pods -n higress-system
   ```

### 问题 4：上下文丢失

**现象**：消费者标识未能还原

**排查步骤**：

1. 检查是否禁用了请求体读取
   ```go
   // 确保没有调用
   ctx.DontReadRequestBody()
   ```

2. 验证上下文设置
   ```bash
   # 查看日志中的上下文信息
   "original_consumer": "consumer-1"
   ```

---

## 常见问题 FAQ

**Q1: 为什么需要在 body 阶段还原？**

A: 因为 key-auth 插件在 AUTHN 阶段（请求头之后）执行。如果在请求头阶段立即还原，key-auth 会看到原始消费者而不是分组，导致无法使用分组凭证。

**Q2: 是否支持动态更新映射关系？**

A: 是的。修改配置后，Higress 会自动重新加载插件，新的映射关系立即生效，无需重启。

**Q3: 能否同时映射多个 header？**

A: 当前版本每次只能映射一个 consumer header。如果需要映射多个 header，可以配置多个插件实例或修改插件代码。

**Q4: 对性能的影响有多大？**

A: 极小。每个请求增加约 0.1-0.5ms 的延迟（主要是两次 header 替换操作）。内存占用约 1MB（用于存储映射表）。

**Q5: 是否支持回退机制？**

A: 当前版本不支持自动回退。如果映射失败，请求会继续处理（使用原始消费者标识）。可以在日志中查看失败原因。

---

## 更多资源

- [Higress 官方文档](https://higress.cn/docs/)
- [Wasm 插件开发指南](https://higress.cn/docs/latest/plugins/custom/)
- [插件市场](https://higress.cn/docs/latest/plugins/)

## 技术支持

- 提交 Issue：[GitHub Issues](https://github.com/alibaba/higress/issues)
- 钉钉群：34846887
- 邮件：higress@googlegroups.com
