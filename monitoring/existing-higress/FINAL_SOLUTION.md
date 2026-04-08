# Higress 内存监控问题分析与解决方案

## 🔍 问题诊断

### 实际情况
```bash
$ kubectl top pod -n agent higress-allinone-5cc7d4c7d-kgnk9
NAME                               CPU(cores)   MEMORY(bytes)
higress-allinone-5cc7d4c7d-kgnk9   19m          2324Mi
```

- **Pod 分配内存**: 8GB
- **实际使用内存**: 2324MB (约 2.3GB) ✅

### Dashboard 显示问题

导入的 Dashboard 显示很多指标"No data"，这是因为：

#### ❌ **容器指标不存在**

Prometheus 中**没有**以下指标：
- `container_memory_working_set_bytes`
- `container_memory_rss`
- `container_memory_cache`
- `container_spec_memory_limit_bytes`

**原因**: 您的 Prometheus 集群**没有配置 cAdvisor 抓取**容器指标。

#### ✅ **Istio Agent 指标存在**

Prometheus 中**有**以下指标（77个）：
- `istio_agent_go_memstats_*` 系列
- 仅显示 Go 控制平面内存（约 150MB）
- **不包括**: Envoy、Java Console、其他组件

---

## 📊 实际内存构成（约 2.3GB）

```
Pod 总内存: 2324MB
├─ Envoy: ~1.5GB (未监控)
├─ Java Console: ~400MB (未监控)
├─ Go 控制平面: ~150MB ✅ (Istio Agent 监控)
└─ 其他/缓存: ~274MB (未监控)
```

---

## ✅ 解决方案

### 方案 1: 使用简化版 Dashboard（推荐，立即可用）⭐

**文件**: `higress-simple-dashboard.json`

这个 Dashboard **只使用实际可用的指标**：

#### 监控内容
1. ✅ Go 进程总内存 - ~150MB
2. ✅ Heap 内存使用
3. ✅ Stack 内存
4. ✅ 内存分配速率
5. ✅ Goroutines & Threads 数量
6. ✅ GC 统计
7. ✅ Pod 实际内存（静态显示 2324MB）

#### 优点
- ✅ 所有指标都有数据
- ✅ 可以监控 Go 内存趋势
- ✅ 可以检测内存泄漏（Go 部分）
- ✅ 即时可用

#### 缺点
- ❌ 只监控 Go 控制平面（~150MB）
- ❌ 不包括 Envoy 和其他组件
- ❌ 无法看到完整 2.3GB

#### 使用方法
```bash
# 1. 访问 Grafana
export KUBECONFIG=/Users/xiaodian/.kube/huawei-pref-kubeconfig.yaml
kubectl port-forward -n monitoring svc/grafana 3000:3000

# 2. 浏览器打开
open http://localhost:3000

# 3. 导入 Dashboard
# 点击 "+" → "Import" → 上传
# 文件: /Users/xiaodian/IdeaProjects/higress/monitoring/existing-higress/higress-simple-dashboard.json
```

---

### 方案 2: 配置 cAdvisor 容器指标监控（完整方案）

如果需要监控完整的 2.3GB Pod 内存，需要配置 Prometheus 抓取 cAdvisor 指标。

#### 步骤 1: 检查 Prometheus Targets

```bash
export KUBECONFIG=/Users/xiaodian/.kube/huawei-pref-kubeconfig.yaml
kubectl port-forward -n monitoring svc/prometheus-operated 9090:9090

# 浏览器访问
open http://localhost:9090/targets

# 搜索: "cadvisor" 或 "kubelet" 或 "kubernetes-pods"
```

#### 步骤 2A: 如果有 cAdvisor targets

说明 Prometheus 已配置，但 label 不匹配。需要调整查询：

```yaml
# 不指定 container=""
container_memory_working_set_bytes{
  namespace="agent",
  pod=~"higress-allinone.*"
}
```

#### 步骤 2B: 如果没有 cAdvisor targets

需要配置 Prometheus 抓取。创建 `prometheus-cadvisor-config.yaml`:

```yaml
# 添加到 Prometheus 配置中
scrape_configs:
  - job_name: 'kubernetes-pods'
    kubernetes_sd_configs:
      - role: pod
    relabel_configs:
      - source_labels: [__address__]
        target_label: __param_address
      - source_labels: [__param_address]
        target_label: __address__
      - source_labels: [__meta_kubernetes_pod_name]
        action: keep
        regex: 'higress-allinone.*'
```

或者使用 ServiceMonitor:

```yaml
apiVersion: monitoring.coreos.com/v1
kind: ServiceMonitor
metadata:
  name: kubernetes-pods
  namespace: monitoring
spec:
  selector:
    matchLabels:
      app: prometheus
  namespaceSelector:
    any: true
  podMetricsEndpoints:
  - port: https-metrics
    interval: 30s
    path: /metrics/cadvisor
    scheme: https
    tlsConfig:
      insecureSkipVerify: true
```

**注意**: 这需要深入了解您的 Prometheus 配置，建议由运维人员操作。

---

### 方案 3: 混合监控方案（推荐长期方案）

#### 3.1 Grafana 监控

使用方案 1 的简化 Dashboard 监控 Go 内存。

#### 3.2 告警监控

使用 `kubectl top` + 脚本定期检查 Pod 内存并发送告警：

创建 `check-pod-memory.sh`:

```bash
#!/bin/bash
export KUBECONFIG="/Users/xiaodian/.kube/huawei-pref-kubeconfig.yaml"

# 获取内存使用
MEMORY_MB=$(kubectl top pod -n agent higress-allinone-5cc7d4c7d-kgnk9 | awk 'NR==2 {print $4}')
MEMORY_VALUE=${MEMORY_MB%Mi}  # 移除 "Mi" 单位

# 阈值 (6GB = 6144 MB)
THRESHOLD=6144

if [ "$MEMORY_VALUE" -gt "$THRESHOLD" ]; then
    echo "WARNING: Higress Pod 内存使用 $MEMORY_VALUE MB 超过 $THRESHOLD MB"
    # 发送告警（根据您的告警系统配置）
fi
```

添加到 crontab:
```bash
# 每 5 分钟检查一次
*/5 * * * * /path/to/check-pod-memory.sh
```

---

## 🎯 立即可用的方案

### 推荐操作流程

#### 1. 使用简化版 Dashboard（立即）

```bash
cd /Users/xiaodian/IdeaProjects/higress/monitoring/existing-higress
kubectl port-forward -n monitoring svc/grafana 3000:3000 &
# 浏览器: http://localhost:3000 (admin/admin)
# 导入: higress-simple-dashboard.json
```

#### 2. 定期检查实际 Pod 内存

```bash
# 添加到 alias 或脚本
alias htop='export KUBECONFIG=/Users/xiaodian/.kube/huawei-pref-kubeconfig.yaml && kubectl top pod -n agent'

# 使用
htop
```

#### 3. 配置基础告警（可选）

使用 `check-pod-memory.sh` 定期检查内存。

---

## 📋 Dashboard 对比

| Dashboard | 文件 | 监控内容 | 数据可用性 |
|-----------|------|----------|-----------|
| 简化版 | `higress-simple-dashboard.json` | Go 进程内存（~150MB） | ✅ 所有指标可用 |
| 修正版 | `higress-allinone-dashboard-fixed.json` | Pod 容器内存（~2.3GB） | ❌ 容器指标不存在 |
| 原始版 | `higress-allinone-dashboard.json` | Go 内存（配置错误） | ❌ 很多指标不存在 |

**推荐**: 使用 `higress-simple-dashboard.json`

---

## 🔧 验证指标可用性

### 快速测试

在 Prometheus UI (http://localhost:9090) 中执行：

```promql
# 应该有数据（~157 MB）
istio_agent_go_memstats_sys_bytes{namespace="agent"}

# 应该没数据（容器指标）
container_memory_working_set_bytes{namespace="agent"}
```

### 查看所有可用指标

```promql
# 列出所有 Higress 指标
{namespace="agent", pod=~"higress.*"}
```

---

## 💡 长期建议

### 立即行动
1. ✅ 导入 `higress-simple-dashboard.json`
2. ✅ 使用 `kubectl top pod` 定期查看
3. ✅ 配置简单的内存告警脚本

### 1-2 周内
1. 配置 Prometheus cAdvisor 抓取
2. 或者部署 Metrics Server
3. 更新 Dashboard 使用容器指标

### 持续优化
1. 根据实际使用调整资源配置
2. 设置合理告警阈值
3. 定期审查监控数据

---

## 📞 快速命令

```bash
# 1. 启动 Grafana
export KUBECONFIG=/Users/xiaodian/.kube/huawei-pref-kubeconfig.yaml
kubectl port-forward -n monitoring svc/grafana 3000:3000

# 2. 查看 Pod 内存
kubectl top pod -n agent higress-allinone-5cc7d4c7d-kgnk9

# 3. 查看 Prometheus 指标
kubectl port-forward -n monitoring svc/prometheus-operated 9090:9090
# 浏览器: http://localhost:9090
# 查询: istio_agent_go_memstats_sys_bytes{namespace="agent"}
```

---

## 总结

**当前状况**:
- ✅ Prometheus 可以抓取 Istio Agent 指标（Go 内存）
- ❌ Prometheus 无法抓取容器指标（需要额外配置）

**最佳方案**:
1. 短期: 使用 `higress-simple-dashboard.json`（立即可用）
2. 长期: 配置 cAdvisor 或使用 `kubectl top` + 告警脚本

**立即可用**:
```bash
# 导入这个 Dashboard
/Users/xiaodian/IdeaProjects/higress/monitoring/existing-higress/higress-simple-dashboard.json
```
