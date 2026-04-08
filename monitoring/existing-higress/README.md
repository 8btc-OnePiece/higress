# Higress All-in-One 内存监控配置指南

## 📋 概述

本指南适用于已经通过 **all-in-one** 模式部署 Higress 到 Kubernetes 集群的用户，帮助您快速配置 Prometheus 监控 Higress 的内存使用情况。

## 🎯 前置条件

- 已通过 all-in-one 模式部署 Higress 到 K8s 集群
- Kubeconfig 文件: `/Users/xiaodian/.kube/huawei-pref-kubeconfig.yaml`
- Higress 部署在 `agent` 命名空间
- 有 kubectl 和 helm 访问权限

## 🚀 快速开始

### 一键部署（推荐）

```bash
cd /Users/xiaodian/IdeaProjects/higress/monitoring/existing-higress
chmod +x deploy-monitoring.sh
./deploy-monitoring.sh
```

这个脚本会自动：
1. ✅ 检查 Higress Pod 状态和 metrics 端点
2. ✅ 安装 Prometheus Operator（如果未安装）
3. ✅ 创建 Higress Metrics Service
4. ✅ 配置 PodMonitor
5. ✅ 应用内存告警规则
6. ✅ 验证配置

## 📊 配置文件说明

### 1. higress-metrics-service.yaml
暴露 Higress Pod 的 metrics 端口（15020）供 Prometheus 采集

### 2. higress-allinone-podmonitor.yaml
Prometheus Operator 的 PodMonitor 配置，定义如何抓取 Higress 指标

### 3. deploy-monitoring.sh
一键部署脚本，自动完成所有配置

### 4. higress-allinone-dashboard.json
Grafana Dashboard，展示 Higress 内存使用情况

## 🔍 访问监控

### Grafana Dashboard

```bash
# 1. 端口转发
export KUBECONFIG=/Users/xiaodian/.kube/huawei-pref-kubeconfig.yaml
kubectl port-forward -n monitoring svc/grafana 3000:3000

# 2. 浏览器访问
open http://localhost:3000
# 用户名: admin
# 密码: admin

# 3. 导入 Dashboard
# 点击 "+" → "Import" → Upload JSON
# 选择 higress-allinone-dashboard.json
```

### Prometheus

```bash
# 端口转发
kubectl port-forward -n monitoring svc/prometheus-operated 9090:9090

# 浏览器访问
open http://localhost:9090
```

## 📈 关键内存指标

### Istio Agent 指标（Higress All-in-One）

| 指标 | 说明 | PromQL 查询 |
|------|------|-------------|
| **已分配内存** | Go 程序实际分配的内存 | `istio_agent_go_memstats_alloc_bytes{namespace="agent"}` |
| **Heap 内存** | Go 堆内存使用 | `istio_agent_go_memstats_heap_inuse_bytes{namespace="agent"}` |
| **Stack 内存** | Go 栈内存使用 | `istio_agent_go_memstats_stack_inuse_bytes{namespace="agent"}` |
| **内存分配速率** | 每秒分配的内存字节数 | `rate(istio_agent_go_memstats_alloc_bytes_total{namespace="agent"}[5m])` |
| **Goroutines 数量** | 当前 goroutines 数量 | `istio_agent_go_goroutines{namespace="agent"}` |
| **GC 耗时** | GC 暂停时间 | `istio_agent_go_gc_duration_seconds{namespace="agent"}` |

### 常用查询示例

```promql
# 1. 当前内存使用量
istio_agent_go_memstats_alloc_bytes{namespace="agent"}

# 2. 内存分配速率（每秒）
rate(istio_agent_go_memstats_alloc_bytes_total{namespace="agent"}[5m])

# 3. Heap 内存趋势
istio_agent_go_memstats_heap_inuse_bytes{namespace="agent"}

# 4. GC 频率（每秒 GC 次数）
rate(istio_agent_go_gc_duration_seconds_count{namespace="agent"}[5m])

# 5. Goroutines 数量趋势
istio_agent_go_goroutines{namespace="agent"}

# 6. 内存使用峰值（最近1小时）
max_over_time(
  istio_agent_go_memstats_alloc_bytes{namespace="agent"}[1h]
)
```

## ✅ 验证步骤

### 1. 检查 Metrics Service

```bash
export KUBECONFIG=/Users/xiaodian/.kube/huawei-pref-kubeconfig.yaml
kubectl get svc -n agent higress-allinone-metrics
```

预期输出：
```
NAME                        TYPE        CLUSTER-IP       PORT(S)     AGE
higress-allinone-metrics    ClusterIP   10.102.123.45    15020/TCP   5m
```

### 2. 检查 PodMonitor

```bash
kubectl get podmonitor -n agent
```

预期输出：
```
NAME                   AGE
higress-allinone       5m
```

### 3. 验证 Prometheus Targets

访问 Prometheus UI → Status → Targets，应该看到：
- `higress-allinone` (UP)

### 4. 查询指标验证

在 Prometheus UI 中执行：

```promql
up{namespace="agent"}
```

应该返回值为 1。

### 5. 测试 metrics 端点

```bash
# 直接从 Pod 获取 metrics
kubectl exec -n agent \
  $(kubectl get pod -n agent -l app=higress-allinone -o jsonpath='{.items[0].metadata.name}') \
  -- curl -s http://localhost:15020/stats/prometheus | head -20
```

## ⚠️ 告警规则

已配置以下告警（自动适配 all-in-one 环境）：

| 告警名称 | 级别 | 触发条件 | 说明 |
|----------|------|----------|------|
| HigressGatewayHighMemoryUsage | warning | 内存使用率 > 80% | 持续5分钟 |
| HigressGatewayMemoryNearLimit | critical | 内存使用率 > 90% | 持续5分钟 |
| HigressGatewayPotentialOOM | critical | 预计1小时内OOM | 持续10分钟 |
| HigressGatewayMemoryGrowing | warning | 内存持续增长2小时 | 可能内存泄漏 |
| HigressMemorySpike | warning | 15分钟内增长>20% | 突然内存增长 |
| HigressHighGoHeapUsage | warning | Go Heap > 80% | 持续10分钟 |
| HigressHighGCFrequency | warning | GC 频率 > 10次/秒 | 持续10分钟 |

## 🔧 手动部署步骤

如果您想手动执行每一步：

### 步骤 1: 安装 Prometheus Operator

```bash
export KUBECONFIG=/Users/xiaodian/.kube/huawei-pref-kubeconfig.yaml

helm repo add prometheus-community https://prometheus-community.github.io/helm-charts
helm repo update

kubectl create namespace monitoring

helm install prometheus prometheus-community/kube-prometheus-stack \
  --namespace monitoring \
  --set prometheus.prometheusSpec.serviceMonitorSelectorNilUsesHelmValues=false \
  --set prometheus.prometheusSpec.podMonitorSelectorNilUsesHelmValues=false \
  --set grafana.enabled=true \
  --set grafana.adminPassword=admin
```

### 步骤 2: 创建 Metrics Service

```bash
kubectl apply -f higress-metrics-service.yaml
```

### 步骤 3: 创建 PodMonitor

```bash
kubectl apply -f higress-allinone-podmonitor.yaml
```

### 步骤 4: 应用告警规则

```bash
# 修改命名空间并应用
sed 's/namespace: higress-system/namespace: agent/g' \
  ../higress-memory-alerts.yaml | \
sed 's/pod=~"higress-gateway.*"/pod=~"higress-allinone.*"/g' | \
kubectl apply -f -
```

### 步骤 5: 导入 Grafana Dashboard

访问 Grafana → Import → 上传 `higress-allinone-dashboard.json`

## 🔍 故障排查

### 问题 1: Metrics 端点无法访问

**检查：**
```bash
kubectl exec -n agent \
  $(kubectl get pod -n agent -l app=higress-allinone -o jsonpath='{.items[0].metadata.name}') \
  -- netstat -tlnp | grep 15020
```

**应该看到：**
```
tcp        0      0 0.0.0.0:15020          0.0.0.0:*               LISTEN      XXX/pilot-agent
```

### 问题 2: PodMonitor 无数据

**检查：**
```bash
kubectl describe podmonitor higress-allinone -n agent
```

**确认：**
- Pod label 是否匹配
- Prometheus selector 是否正确

### 问题 3: Prometheus Targets 显示 Down

**检查：**
1. Metrics Service 是否正常：
```bash
kubectl get svc -n agent higress-allinone-metrics
```

2. 从 Prometheus Pod 测试连接：
```bash
kubectl exec -n monitoring \
  $(kubectl get pod -n monitoring -l app.kubernetes.io/name=prometheus -o jsonpath='{.items[0].metadata.name}') \
  -- curl -s http://higress-allinone-metrics.agent.svc.cluster.local:15020/stats/prometheus | head -5
```

### 问题 4: Grafana Dashboard 无数据

**检查：**
1. Prometheus 是否有数据
2. Dashboard 的 PromQL 查询是否正确
3. 时间范围是否合适（默认最近1小时）

## 📚 相关资源

- [Higress 官方文档](https://higress.io/docs/)
- [Prometheus Operator 文档](https://prometheus-operator.dev/)
- [Istio 监控指标](https://istio.io/latest/docs/reference/config/metrics/)
- [Go Runtime Metrics](https://github.com/prometheus/client_golang)

## 🆘 获取帮助

### 查看日志

```bash
# Higress Pod 日志
kubectl logs -n agent \
  $(kubectl get pod -n agent -l app=higress-allinone -o jsonpath='{.items[0].metadata.name}')

# Prometheus 日志
kubectl logs -n monitoring prometheus-kube-prometheus-prometheus-0

# PodMonitor 状态
kubectl describe podmonitor -n agent
```

### 配置检查

```bash
# 检查 Pod labels
kubectl get pod -n agent -l app=higress-allinone --show-labels

# 检查 Service endpoints
kubectl get endpoints -n agent higress-allinone-metrics

# 检查 PodMonitor
kubectl get podmonitor -n agent -o yaml
```

---

**文档版本:** 1.0
**更新时间:** 2024-02-27
**适用环境:** Higress All-in-One on Kubernetes
