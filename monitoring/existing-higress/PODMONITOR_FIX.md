# Higress PodMonitor 修复指南

## 问题描述

**PodMonitor 已创建，但 Prometheus 没有抓取 `istio_agent_go_*` 指标**

---

## 问题诊断

### 原始配置

```yaml
apiVersion: monitoring.coreos.com/v1
kind: PodMonitor
metadata:
  name: higress-allinone
  namespace: agent
spec:
  podMetricsEndpoints:
  - interval: 30s
    path: /stats/prometheus
    port: stats-prom        # ❌ 问题：Pod 没有这个端口名
    scheme: http
    scrapeTimeout: 10s
  selector:
    matchLabels:
      app: higress-allinone
```

### 问题原因

1. **Pod 容器端口**:
   ```json
   {
     "containerPort": 8001,
     "name": "ui"
   },
   {
     "containerPort": 8443,
     "name": "https"
   },
   {
     "containerPort": 8080,
     "name": "http"
   }
   ```
   **没有名为 `stats-prom` 的端口！**

2. **Prometheus 行为**:
   - PodMonitor 默认通过 **Pod IP** 抓取
   - 使用 `port` 字段查找 Pod 的**容器端口名**
   - 找不到 `stats-prom` 端口名 → 抓取失败

3. **为什么 metrics 端点实际可用？**
   - Istio sidecar 监听在 **15020 端口**
   - Service `higress-allinone-metrics` 暴露了这个端口
   - 但 Pod 容器定义中没有声明这个端口

---

## 解决方案

### ✅ 使用 `targetPort` 直接指定端口号

```yaml
apiVersion: monitoring.coreos.com/v1
kind: PodMonitor
metadata:
  name: higress-allinone
  namespace: agent
  labels:
    app: higress-allinone
    release: prometheus
spec:
  podMetricsEndpoints:
  - interval: 30s
    path: /stats/prometheus
    targetPort: 15020        # ✅ 直接使用端口号
    scheme: http
    scrapeTimeout: 10s
    relabelings:
    - replacement: higress-allinone
      targetLabel: instance
    - sourceLabels: [__meta_kubernetes_pod_name]
      targetLabel: pod
    - sourceLabels: [__meta_kubernetes_namespace]
      targetLabel: namespace
  selector:
    matchLabels:
      app: higress-allinone
```

### 关键差异

| 字段 | 原配置 | 新配置 | 说明 |
|------|--------|--------|------|
| `port` | `stats-prom` | - | 需要容器端口名 |
| `targetPort` | - | `15020` | 直接使用端口号，不依赖容器定义 |

---

## 验证步骤

### 1. 应用新配置

```bash
export KUBECONFIG="/Users/xiaodian/Desktop/supernode-prod-kubeconfig.yaml"

kubectl apply -f higress-allinone-podmonitor-v2.yaml
```

### 2. 等待 Prometheus 重新加载配置

```bash
# Prometheus Operator 会自动检测 PodMonitor 变化
# 通常需要 10-30 秒生效
```

### 3. 验证 Prometheus 配置

```bash
# 检查 Prometheus 是否加载了新配置
PROM_POD=$(kubectl get pods -n monitoring -l app.kubernetes.io/name=prometheus -o jsonpath='{.items[0].metadata.name}')

kubectl exec $PROM_POD -n monitoring -- cat /etc/prometheus/config_out/prometheus.env.yaml | grep -A 10 "higress-allinone"
```

**期望输出**:
```yaml
- job_name: podMonitor/agent/higress-allinone/0
  ...
  - action: keep
    source_labels:
    - __meta_kubernetes_pod_container_port_number
    regex: 15020        # ✅ 使用端口号匹配
```

### 4. 检查 Prometheus targets

访问 Prometheus UI: `http://<prometheus>:9090/targets`

搜索: `higress-allinone`

**期望状态**:
- State: **UP** ✅
- Endpoint: `http://10.x.x.x:15020/stats/prometheus`
- Labels: `instance="higress-allinone"`, `pod="higress-allinone-xxx"`

### 5. 查询指标验证

在 Prometheus UI 或 Grafana 中查询:

```promql
# Go 内存指标
istio_agent_go_memstats_alloc_bytes{pod=~"higress-allinone-.*"}

# Go Goroutines
istio_agent_go_goroutines{pod=~"higress-allinone-.*"}

# Go GC 信息
istio_agent_go_gc_duration_seconds{pod=~"higress-allinone-.*"}
```

**期望**: 有数据返回

---

## 为什么这样修复有效？

### PodMonitor 的工作原理

1. **通过 Pod IP 抓取**: 默认使用 `status.podIP`
2. **端口查找**:
   - 使用 `port`: 查找 Pod 的**容器端口名**
   - 使用 `targetPort`: 直接使用端口号
3. **Istio Sidecar**: 15020 端口在 Pod 内监听，但不需要在容器定义中声明

### targetPort vs port

| 字段 | 值类型 | 说明 | 需要容器定义 |
|------|--------|------|--------------|
| `port` | 端口名（字符串） | 查找 `containerPorts[*].name` | ✅ 需要 |
| `targetPort` | 端口号（数字） | 直接使用端口号 | ❌ 不需要 |

---

## 其他可选方案

### 方案 2: 在 Pod 中声明端口（不推荐）

需要修改 Higress Deployment，在容器中添加:

```yaml
ports:
- name: stats-prom
  containerPort: 15020
  protocol: TCP
```

**缺点**:
- 需要修改 Higress 部署配置
- 可能需要重启 Pod
- 不如使用 `targetPort` 简单

### 方案 3: 使用 ServiceMonitor（复杂）

```yaml
apiVersion: monitoring.coreos.com/v1
kind: ServiceMonitor
metadata:
  name: higress-allinone
spec:
  selector:
    matchLabels:
      app: higress-allinone
  namespaceSelector:
    matchNames:
    - agent
  endpoints:
  - port: stats-prom
    path: /stats/prometheus
    interval: 30s
```

**缺点**:
- 需要额外的 ServiceMonitor 资源
- PodMonitor 更简单直接

---

## 验证脚本

创建验证脚本 `verify-podmonitor.sh`:

```bash
#!/bin/bash

export KUBECONFIG="/Users/xiaodian/Desktop/supernode-prod-kubeconfig.yaml"

echo "1. 检查 PodMonitor 配置"
kubectl get podmonitor higress-allinone -n agent -o yaml | grep -A 5 "targetPort"

echo ""
echo "2. 等待 Prometheus 重新加载（30秒）"
sleep 30

echo ""
echo "3. 检查 Prometheus 抓取状态"
PROM_POD=$(kubectl get pods -n monitoring -l app.kubernetes.io/name=prometheus -o jsonpath='{.items[0].metadata.name}')
kubectl exec $PROM_POD -n monitoring -- wget -qO- 'localhost:9090/api/v1/targets' | python3 -c "import sys, json; data=json.load(sys.stdin); [print(f\"{t['labels']['job']}: {t['health']}\") for t in data['data']['activeTargets'] if 'higress' in t['labels']['job']]"

echo ""
echo "4. 查询指标测试"
kubectl exec $PROM_POD -n monitoring -- wget -qO- 'localhost:9090/api/v1/query?query=istio_agent_go_goroutines' | python3 -c "import sys, json; data=json.load(sys.stdin); print(f\"指标数量: {len(data['data']['result'])}\")"
```

---

## 常见问题

### Q1: PodMonitor 已生效，但仍然看不到数据？

**检查**:
1. Prometheus 是否重新加载配置（可能需要 1-2 分钟）
2. Pod 的 15020 端口是否可访问
3. `/stats/prometheus` 端点是否返回数据

**调试**:
```bash
# 从 Pod 内部测试
kubectl exec higress-allinone-xxx -n agent -- curl -s http://localhost:15020/stats/prometheus | head

# 从 Prometheus Pod 测试
PROM_POD=$(kubectl get pods -n monitoring -l app.kubernetes.io/name=prometheus -o jsonpath='{.items[0].metadata.name}')
POD_IP=$(kubectl get pod higress-allinone-xxx -n agent -o jsonpath='{.status.podIP}')
kubectl exec $PROM_POD -n monitoring -- curl -s http://$POD_IP:15020/stats/prometheus | head
```

### Q2: 指标有数据，但 Grafana Dashboard 没有显示？

**检查**:
1. Dashboard 的数据源是否正确
2. Dashboard 查询的 label selector 是否匹配
3. 时间范围是否正确

**验证查询**:
```promql
# 检查 label 是否正确
istio_agent_go_memstats_alloc_bytes{namespace="agent", pod=~"higress-allinone-.*"}
```

### Q3: 多个 Higress Pod，只抓取到一个？

**正常**: PodMonitor 的 `selector` 会匹配所有 Pod，Prometheus 会为每个 Pod 创建一个 target。

**检查**:
```bash
kubectl get pods -n agent -l app=higress-allinone
```

应该看到多个 Pod，Prometheus UI 也应该显示多个 targets。

---

## 总结

### ✅ 修复完成

1. **修改**: 使用 `targetPort: 15020` 代替 `port: stats-prom`
2. **应用**: `kubectl apply -f higress-allinone-podmonitor-v2.yaml`
3. **等待**: Prometheus 自动重新加载配置（10-30秒）
4. **验证**: Prometheus UI 或查询指标确认

### 📊 可用的指标

现在可以查询以下指标:

- `istio_agent_go_memstats_alloc_bytes` - Go 堆内存分配
- `istio_agent_go_goroutines` - Goroutine 数量
- `istio_agent_go_gc_duration_seconds` - GC 耗时
- `istio_agent_process_resident_memory_bytes` - RSS 内存
- `istio_agent_process_virtual_memory_bytes` - VMS 内存

### 🎯 下一步

1. 更新 Grafana Dashboard 使用这些新指标
2. 配置告警规则（如内存泄漏、Goroutine 泄漏）
3. 定期检查 Prometheus targets 状态

---

**文件位置**: `/Users/xiaodian/IdeaProjects/higress/monitoring/existing-higress/higress-allinone-podmonitor-v2.yaml`
