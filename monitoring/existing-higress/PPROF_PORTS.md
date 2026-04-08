# ⚠️ Higress pprof 端口说明

## 🔍 问题分析

### 发现的问题

您的 Higress all-in-one Pod 中：
- ✅ **Pilot Discovery (15014)** - **支持 pprof**
- ❌ **Higress Gateway (8080)** - **不支持 pprof**（这是 Envoy，不是 Go 程序）

### 端口映射

| 端口 | 组件 | 类型 | pprof 支持 |
|------|------|------|-----------|
| 8080 | Higress Gateway | **Envoy** (C++) | ❌ 不支持 |
| 8443 | Higress Gateway HTTPS | **Envoy** (C++) | ❌ 不支持 |
| 15014 | Pilot Discovery | **Go** (istio) | ✅ 支持 |
| 15000 | Envoy admin | Envoy | ❌ 不支持 |

### 原因

端口 8080 虽然是 Higress Gateway 的端口，但实际监听的是 **Envoy 代理**（C++ 编写），不是 Go 程序，所以不支持 Go 的 pprof。

---

## ✅ 解决方案

### 使用 Pilot Discovery pprof（推荐）

**为什么选择 Pilot Discovery？**
1. ✅ 是 Go 程序，支持 pprof
2. ✅ 包含 Istio 控制平面逻辑
3. ✅ 可以分析 xDS 推送、配置处理等
4. ✅ 有 796 个 goroutines，内存使用 ~89MB

### 运行脚本

```bash
cd /Users/xiaodian/IdeaProjects/higress/monitoring/existing-higress

# 默认采样 30 秒
./pprof-pilot.sh

# 或自定义采样时长（例如 60 秒）
./pprof-pilot.sh 60
```

### 输出文件

```
/tmp/higress-pprof-20250228_105000/
├── pilot-discovery-cpu.prof         # CPU profile
├── pilot-discovery-cpu.svg          # ⭐ CPU 火焰图
├── pilot-discovery-heap.prof        # Heap profile
├── pilot-discovery-heap.svg         # ⭐ 内存火焰图
└── pilot-discovery-goroutine.prof   # Goroutine profile
```

---

## 📊 Pilot Discovery 可以分析什么？

### 1. CPU 性能分析

**热点函数可能包括**：
- xDS 配置处理（CDS/EDS/LDS/RDS）
- gRPC 调用处理
- 配置更新逻辑
- 证书管理
- 服务发现

**示例场景**：
- 配置更新慢
- CPU 使用率高
- xDS 推送延迟

### 2. 内存分析

**可以查看**：
- 堆内存分配
- 内存泄漏
- Goroutine 数量
- 对象分配热点

### 3. Goroutine 分析

**可以查看**：
- Goroutine 泄漏
- 阻塞的 Goroutine
- 死锁检测

---

## 🚀 使用方法

### **方法 1: 一键采集**

```bash
cd /Users/xiaodian/IdeaProjects/higress/monitoring/existing-higress
./pprof-pilot.sh 30
```

### **方法 2: 手动采集**

```bash
export KUBECONFIG="/Users/xiaodian/.kube/href-kubeconfig.yaml"

# 1. 端口转发
kubectl port-forward -n agent pod/higress-allinone-5cc7d4c7d-kgnk9 15014:15014 &

# 2. 采集 CPU profile (30秒)
curl "http://localhost:15014/debug/pprof/profile?seconds=30" -o cpu.prof

# 3. 生成火焰图
go tool pprof -raw cpu.prof | /tmp/FlameGraph/flamegraph.pl > cpu.svg

# 4. 打开查看
open cpu.svg
```

### **方法 3: 交互式分析**

```bash
cd /tmp/higress-pprof-*

# Web UI
go tool pprof -http=:9999 pilot-discovery-cpu.prof
open http://localhost:9999

# 命令行
go tool pprof pilot-discovery-cpu.prof
(pprof) top
(pprof) list functionName
(pprof) web
```

---

## 💡 如果需要分析 Higress Gateway

### 选项 1: 使用 Envoy admin 接口

```bash
# 端口转发到 15000 (Envoy admin)
kubectl port-forward -n agent pod/higress-allinone-5cc7d4c7d-kgnk9 15000:15000

# 查看统计
curl http://localhost:15000/stats

# 启用 profiling
curl http://localhost:15000/server_info
```

### 选项 2: 使用 Envoy 的 profiling

Envoy 支持有限的 profiling（通过 stats 端点），但不是标准 pprof 格式。

### 选项 3: 监控 Higress Gateway 的 Envoy 内存

使用我们之前配置的 Dashboard：
- `envoy_server_memory_allocated`
- `envoy_server_memory_heap_size`
- `envoy_server_memory_physical_size`

---

## 📊 实际可用性

### ✅ 可以分析

| 组件 | 方法 | 覆盖范围 |
|------|------|----------|
| **Pilot Discovery** | pprof | 100% - CPU、内存、Goroutine |
| **Higress Gateway** | Envoy stats | ~60% - 只有内存统计 |
| **Java Console** | JMX (需要配置) | 100% - 如果配置了 JMX |

---

## 🎯 推荐做法

### **日常性能分析**
1. 使用 `pprof-pilot.sh` 采集 Pilot Discovery
2. 查看火焰图找出热点
3. 使用 Dashboard 监控 Envoy 内存

### **深入分析 Gateway**
1. 查看日志找出慢请求
2. 使用 Envoy stats 端点
3. 分析配置复杂度

### **全链路分析**
1. 采集 Pilot Discovery profile
2. 查看 Dashboard 的 Envoy 内存
3. 结合日志分析请求路径

---

## ✅ 总结

### **重要发现**
- Higress Gateway (8080) 是 **Envoy**，不支持 pprof
- **Pilot Discovery (15014)** 是 Go 程序，**完全支持 pprof**

### **立即可用**
```bash
cd /Users/xiaodian/IdeaProjects/higress/monitoring/existing-higress
./pprof-pilot.sh
```

### **下一步**
1. 运行 `pprof-pilot.sh`
2. 打开生成的 SVG 文件
3. 分析 Pilot Discovery 性能
4. 使用 Dashboard 查看 Envoy 指标

**Pilot Discovery 的 pprof 足以分析控制平面性能！** 🚀
