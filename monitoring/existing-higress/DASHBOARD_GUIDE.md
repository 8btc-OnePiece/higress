# Higress All-in-One Dashboard 说明

## 📊 问题分析

您的 Higress all-in-one Pod:
- **分配内存**: 8GB
- **实际使用**: ~2GB

但原来的 Dashboard 显示只有 ~8MB，这是因为：

### 指标来源不同

1. **Istio Agent 指标**（端口 15020）
   - 只显示 Go 运行时内存
   - 包括: heap, stack, goroutines
   - **不包括**: Envoy、其他组件、系统开销
   - 约 77 个指标

2. **容器指标**（cAdvisor）
   - 显示整个 Pod/容器的内存
   - 包括: 所有进程 + 页缓存 + 系统开销
   - **这才是实际使用的 2GB**

## 🎯 两个 Dashboard 对比

### Dashboard 1: `higress-allinone-dashboard.json`（旧版）
- **指标来源**: 仅 Istio Agent (15020 端口)
- **显示内容**: Go 进程内存（~8MB heap）
- **适用场景**: 监控 Go 程序内部
- ❌ **不准确**: 不反映实际内存使用

### Dashboard 2: `higress-allinone-dashboard-fixed.json`（推荐）✅
- **指标来源**: 混合使用 cAdvisor + Istio Agent
- **显示内容**:
  - Pod 总内存（~2GB）✅
  - Working Set, RSS, Cache
  - 内存使用率
  - Go 进程内存
- ✅ **准确**: 反映真实内存使用

## 🚀 使用修正版 Dashboard

### 步骤 1: 重新导入 Dashboard

1. 访问 Grafana: http://localhost:3000
2. 删除旧的 Dashboard（可选）
3. 点击 **"+" → "Import"**
4. 上传新文件:
   ```
   /Users/xiaodian/IdeaProjects/higress/monitoring/existing-higress/higress-allinone-dashboard-fixed.json
   ```

### 步骤 2: 验证数据

在新 Dashboard 中，第一个图应该显示：
- **Pod 总内存**: 约 2GB ✅
- **Pod RSS**: 约 1-2GB
- **Pod Cache**: 几百 MB
- **Go 进程内存**: 约 150MB

### 步骤 3: 检查 Prometheus 是否有容器指标

在 Prometheus UI (http://localhost:9090) 中执行：

```promql
# 查询容器 Working Set 内存
container_memory_working_set_bytes{
  namespace="agent",
  pod=~"higress-allinone.*"
}

# 应该返回约 2GB 的数据（2147483648 字节左右）
```

如果返回空，说明 Prometheus 没有抓取到容器指标。

## 🔧 如果容器指标为空

### 原因
Prometheus 可能没有配置 cAdvisor 抓取，或者 Higress Pod 的注解不正确。

### 解决方案 1: 添加 ServiceMonitor（如果需要）

创建 `higress-allinone-servicemonitor.yaml`:

```yaml
apiVersion: monitoring.coreos.com/v1
kind: ServiceMonitor
metadata:
  name: higress-allinone-cadvisor
  namespace: agent
  labels:
    release: prometheus
spec:
  selector:
    matchLabels:
      app: higress-allinone
  endpoints:
    - port: stats-prom
      path: /stats/prometheus
      interval: 30s
```

但这不会解决问题，因为 15020 端口没有容器指标。

### 解决方案 2: 使用 Node Exporter + Kubelet（推荐）

检查 Prometheus 是否已经在抓取 cAdvisor：

```bash
export KUBECONFIG=/Users/xiaodian/.kube/huawei-pref-kubeconfig.yaml

# 检查 Prometheus targets
kubectl port-forward -n monitoring svc/prometheus-operated 9090:9090 &
# 访问 http://localhost:9090/targets
# 搜索 "cadvisor" 或 "kubelet"
```

如果看到 cAdvisor targets，说明容器指标应该可用。

### 解决方案 3: 直接查询 Kubelet（临时方案）

```bash
# 获取 Kubelet 端口
export KUBECONFIG=/Users/xiaodian/.kube/huawei-pref-kubeconfig.yaml
kubectl get node -o jsonpath='{.items[0].status.addresses[?(@.type=="InternalIP")].address}')

# 访问 Kubelet stats（需要权限）
curl http://<node-ip>:10250/stats/summary
```

## 📈 推荐的查询方式

### 1. 查看 Pod 总内存（推荐）

```promql
# Working Set - 实际使用的内存
container_memory_working_set_bytes{
  namespace="agent",
  pod=~"higress-allinone.*",
  container=""
}

# RSS - 常驻内存
container_memory_rss{
  namespace="agent",
  pod=~"higress-allinone.*",
  container=""
}

# Cache - 页缓存
container_memory_cache{
  namespace="agent",
  pod=~"higress-allinone.*",
  container=""
}
```

### 2. 查看内存使用率

```promql
# 相对于 8GB 限制的使用率
container_memory_working_set_bytes{
  namespace="agent",
  pod=~"higress-allinone.*",
  container=""
} / container_spec_memory_limit_bytes{
  namespace="agent",
  pod=~"higress-allinone.*",
  container=""
} * 100
```

### 3. 查看 Go 进程内存（仅 Go 部分）

```promql
# Go Heap
istio_agent_go_memstats_heap_inuse_bytes{
  namespace="agent",
  pod=~"higress-allinone.*"
}

# Go Sys (总进程内存)
istio_agent_go_memstats_sys_bytes{
  namespace="agent",
  pod=~"higress-allinone.*"
}
```

## ✅ 验证清单

- [ ] 导入修正版 Dashboard (`higress-allinone-dashboard-fixed.json`)
- [ ] 第一个图显示约 2GB（而不是 8MB）
- [ ] Prometheus 中能查询到 `container_memory_working_set_bytes`
- [ ] 内存使用率显示约 25% (2GB / 8GB)

## 💡 关键理解

### 为什么有两个指标集？

1. **Istio Agent (15020)**
   - 范围: 仅 Go 控制平面
   - 大小: ~8MB heap, ~150MB sys
   - 用途: 调试 Go 程序

2. **容器 (cAdvisor)**
   - 范围: 整个 Pod
   - 大小: ~2GB
   - 用途: 资源规划、OOM 预警

### 实际内存组成（约 2GB）

- Envoy: ~1-1.5GB
- Go 控制平面: ~150MB
- Java/Console: ~200-500MB
- 其他: ~100MB
- 页缓存: ~100-500MB

## 🎯 快速修复

执行以下命令：

```bash
# 1. 更新 PodMonitor（移除过滤）
cd /Users/xiaodian/IdeaProjects/higress/monitoring/existing-higress
kubectl apply -f higress-allinone-podmonitor.yaml

# 2. 在 Grafana 中导入修正版 Dashboard
# 文件: higress-allinone-dashboard-fixed.json

# 3. 验证数据
# 访问 Grafana → 选择新 Dashboard → 查看第一个图
```

## 📞 仍然看不到 2GB？

如果修正版 Dashboard 仍然显示不正确的数据，请执行：

```bash
# 检查 Prometheus 中有哪些 Higress 相关指标
export KUBECONFIG=/Users/xiaodian/.kube/huawei-pref-kubeconfig.yaml
kubectl port-forward -n monitoring svc/prometheus-operated 9090:9090 &

# 在浏览器打开 http://localhost:9090
# 执行查询: {namespace="agent", pod=~"higress.*"}
# 查看所有可用指标
```

然后告诉我有哪些指标，我会帮您调整 Dashboard。

---

**总结**:
- ❌ 旧 Dashboard: 只显示 Go 内存（~8MB）
- ✅ 新 Dashboard: 显示 Pod 总内存（~2GB）
- 使用 `higress-allinone-dashboard-fixed.json`
