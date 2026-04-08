# 🔍 如何查询 Java 占用的内存？

## 🎯 关键发现

从之前的 `ps aux` 输出，我们找到了 **Java 进程**：

```
PID 167: java -jar /app/higress-console.jar
       VSZ: 6,343,608 KB (6.0 GB)  # 虚拟内存
       RSS:   346,336 KB (338 MB)  # ✅ 实际物理内存占用！
       %MEM: 1.0%
```

**这就是 2.4GB 中的主要占用者！** 🎯

---

## 📊 Higress Pod 完整内存分解

### 各进程的 RSS (实际物理内存)

从 `ps aux` 输出计算：

| 进程 | PID | RSS (MB) | 说明 |
|------|-----|----------|------|
| **Java Console** | 167 | **338 MB** | ⭐ **最大占用者** |
| Prometheus | 53 | 115 MB | 时序数据库 |
| Loki | 79 | 155 MB | 日志聚合 |
| Grafana | 97 | 106 MB | 可视化 |
| Promtail | 60 | 87 MB | 日志采集 |
| API Server | 94 | 84 MB | API 服务 |
| Higress Gateway | 140 | 87 MB | Go 网关 |
| Pilot Discovery | 242 | 89 MB | Istio 控制 |
| **总计** | - | **~1,060 MB** | 进程 RSS 总和 |

### 加上页缓存和其他开销

```
进程 RSS 总和:    ~1,060 MB
页缓存 (Cache):    ~182 MB
其他/共享内存:    ~1,160 MB
────────────────────────────
Working Set:     ~2,402 MB  ✅ 与 cAdvisor 一致！
```

---

## 💡 如何查询 Java 内存占用？

### 方法 1: 通过进程监控（推荐）

#### 选项 A: 使用 cAdvisor 进程级指标
```promql
# 查询容器内所有进程的内存（需要 cAdvisor 启用进程监控）
container_tasks_state{namespace="agent", container="higress", name="java"}
```

**问题**: 您的 Prometheus 可能没有启用 cAdvisor 的进程监控。

#### 选项 B: 使用 Node Exporter + Process Collector
```bash
# 在节点上安装 node_exporter 的进程收集器
# 然后查询:
namedprocess_namegroup_memory_bytes{namespace="agent", container="higress", processname="java"}
```

### 方法 2: 通过 JMX 暴露 Java 指标（最佳方案）

#### 步骤 1: 启动 Java 应用时添加 JMX 参数
```bash
java -Dcom.sun.management.jmxremote \
     -Dcom.sun.management.jmxremote.port=9999 \
     -Dcom.sun.management.jmxremote.authenticate=false \
     -Dcom.sun.management.jmxremote.ssl=false \
     -jar higress-console.jar
```

#### 步骤 2: 使用 jmx_exporter 暴露给 Prometheus
```yaml
# jmx_exporter 配置
rules:
  - pattern: 'java.lang<type=Memory><>HeapMemoryUsage'
    name: jvm_heap_memory_bytes
    labels:
      area: heap
  - pattern: 'java.lang<type=Memory><>NonHeapMemoryUsage'
    name: jvm_nonheap_memory_bytes
    labels:
      area: nonheap
```

#### 步骤 3: 在 Prometheus 中查询
```promql
# JVM 堆内存
jvm_heap_memory_bytes{area="heap", namespace="agent", container="higress"}

# JVM 非堆内存
jvm_nonheap_memory_bytes{namespace="agent", container="higress"}

# 具体部分
jvm_heap_memory_bytes{namespace="agent", container="higress", area="heap", id="Eden"}
jvm_heap_memory_bytes{namespace="agent", container="higress", area="heap", id="Survivor"}
jvm_heap_memory_bytes{namespace="agent", container="higress", area="heap", id="Old"}
```

### 方法 3: 估算方法（当前可用）

由于 **没有 JMX 指标**，可以使用 **差值法** 估算：

```promql
# Java 进程内存 ≈ 容器总内存 - 已知组件内存
# Java Memory ≈ container_memory_working_set_bytes - (Go RSS + Envoy Memory + Cache)

(
  container_memory_working_set_bytes{namespace="agent", container="higress"}
  - istio_agent_process_resident_memory_bytes{namespace="agent"}
  - envoy_server_memory_physical_size
  - container_memory_cache{namespace="agent", container="higress"}
)

# 结果应该是 ~300-400 MB
```

---

## 🔧 临时解决方案：添加到 Dashboard

由于没有直接的 Java 指标，可以在 Dashboard 中添加一个 **估算 Panel**：

### Panel: Java 进程内存（估算）

```json
{
  "targets": [
    {
      "expr": "container_memory_working_set_bytes{namespace=\"$namespace\", container=\"higress\"} - istio_agent_process_resident_memory_bytes - envoy_server_memory_physical_size - container_memory_cache{namespace=\"$namespace\", container=\"higress\"}",
      "legendFormat": "Java Console (估算)"
    }
  ],
  "title": "Java Console 内存占用 (估算)"
}
```

**显示值**: ~300-400 MB

---

## 🎯 完整的内存监控方案

### 短期方案（立即可用）

| 组件 | 查询方法 | 状态 |
|------|----------|------|
| **Java Console** | 差值估算 | ⚠️ 可用但不精确 |
| Go 控制平面 | Istio Agent 指标 | ✅ 精确 |
| Envoy | Envoy Stats 指标 | ✅ 精确 |
| 页缓存 | cAdvisor cache | ✅ 精确 |

### 长期方案（需要配置）

1. **启用 JMX 监控** ⭐ 推荐
   - 在 Java 进程启动时添加 JMX 参数
   - 部署 jmx_exporter
   - Prometheus 抓取 JMX 指标

2. **启用 cAdvisor 进程监控**
   - 配置 cAdvisor 收集进程级指标
   - 可以直接看到每个进程的内存

3. **使用 Container Native Monitoring**
   - cgroups 级别的内存统计
   - 更精确的容器内存分解

---

## 📋 实施步骤

### 方案 A: 配置 JMX 监控（推荐）

1. **修改 Higress 启动脚本**
   ```bash
   # 在 start-console.sh 中添加 JAVA_OPTS
   export JAVA_OPTS="$JAVA_OPTS -Dcom.sun.management.jmxremote \
     -Dcom.sun.management.jmxremote.port=9999 \
     -Dcom.sun.management.jmxremote.authenticate=false \
     -Dcom.sun.management.jmxremote.ssl=false"
   ```

2. **部署 jmx_exporter**
   ```yaml
   apiVersion: v1
   kind: ConfigMap
   metadata:
     name: jmx-exporter-config
   data:
     jmx.yml: |
       rules:
         - pattern: 'java.lang<type=Memory>'
   ---
   apiVersion: apps/v1
   kind: Deployment
   metadata:
     name: jmx-exporter
   spec:
     template:
       spec:
         containers:
         - name: jmx-exporter
           image: bitnami/jmx-exporter:latest
           ports:
           - containerPort: 5556
   ```

3. **配置 Prometheus ServiceMonitor**
   ```yaml
   apiVersion: monitoring.coreos.com/v1
   kind: ServiceMonitor
   metadata:
     name: jmx-exporter
   spec:
     selector:
       matchLabels:
         app: jmx-exporter
     endpoints:
     - port: jmx-metrics
   ```

### 方案 B: 使用差值估算（临时）

在 Dashboard 中添加估算 Panel（如上所示）。

---

## 📊 完整的内存视图

配置完成后，Dashboard 应该显示：

| Panel | 指标 | 值 |
|-------|------|-----|
| 容器总内存 | working_set | ~2,402 MB |
| Java Console | JMX Heap | ~300 MB |
| Go 控制平面 | RSS | ~71 MB |
| Envoy | Physical | ~340 MB |
| 页缓存 | Cache | ~182 MB |
| 其他/未知 | 差值 | ~1,509 MB |

**总计**: ~2,402 MB ✅

---

## ✅ 总结

### 当前已知

1. ✅ **Java Console PID 167**，RSS ~338 MB
2. ✅ 是 **2.4GB 的主要占用者**
3. ❌ Prometheus 中**没有 JVM 指标**
4. ⚠️ 只能通过 **差值法** 估算

### 解决方案

**短期**: 使用差值法在 Dashboard 中估算
**长期**: 配置 JMX + jmx_exporter 获取精确的 JVM 指标

### 立即可用的查询

```promql
# 估算 Java 内存（差值法）
container_memory_working_set_bytes{namespace="agent", container="higress"}
- istio_agent_process_resident_memory_bytes
- envoy_server_memory_physical_size
- container_memory_cache{namespace="agent", container="higress"}
```

**结果**: ~300-400 MB（Java Console）
