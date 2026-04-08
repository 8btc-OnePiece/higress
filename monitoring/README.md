# Higress Prometheus 内存监控配置指南

## 📋 概述

本指南将帮助你在 Prometheus 中配置 Higress 的内存监控。包括安装 Prometheus Operator、配置 PodMonitor、以及设置 Grafana Dashboard。

## 🚀 快速开始

### 一键部署

```bash
cd /Users/xiaodian/IdeaProjects/higress/monitoring
./setup-higress-monitoring.sh
```

这个脚本会自动完成以下步骤：
1. 安装 Prometheus Operator
2. 应用 Higress PodMonitor 配置
3. 更新 Higress 配置启用监控
4. 配置 Grafana Dashboard

## 📦 手动部署步骤

### 步骤 1: 安装 Prometheus Operator

```bash
# 添加 Helm repo
helm repo add prometheus-community https://prometheus-community.github.io/helm-charts
helm repo update

# 创建命名空间
kubectl create namespace monitoring

# 安装 Prometheus Operator
helm install prometheus prometheus-community/kube-prometheus-stack \
  --namespace monitoring \
  --values prometheus-operator-values.yaml
```

### 步骤 2: 应用 PodMonitor 配置

```bash
# 应用 Gateway PodMonitor
kubectl apply -f higress-gateway-podmonitor.yaml

# 应用 Controller PodMonitor
kubectl apply -f higress-controller-podmonitor.yaml
```

### 步骤 3: 更新 Higress 配置

```bash
cd /Users/xiaodian/IdeaProjects/higress/helm/core

# 使用包含监控配置的 values
helm upgrade --install higress . \
  --namespace higress-system \
  --values /Users/xiaodian/IdeaProjects/higress/monitoring/higress-values-with-metrics.yaml
```

### 步骤 4: 导入 Grafana Dashboard

#### 方法 1: 通过 UI 导入

1. 访问 Grafana: `http://localhost:3000`
2. 登录（admin/admin）
3. 点击 "+" → "Import"
4. 上传 `higress-memory-dashboard.json`

#### 方法 2: 通过 API 导入

```bash
# 启动 port-forward
kubectl port-forward -n monitoring svc/grafana 3000:3000 &

# 导入 dashboard
curl -X POST http://localhost:3000/api/dashboards/db \
  -u admin:admin \
  -H "Content-Type: application/json" \
  -d '{"dashboard":'$(cat higress-memory-dashboard.json)',"overwrite":true}'
```

## 📊 监控指标说明

### 关键内存指标

| 指标 | 说明 | PromQL 查询 |
|------|------|-------------|
| **Working Set** | 实际使用的内存（不包括inactive的page cache） | `container_memory_working_set_bytes{namespace="higress-system"}` |
| **RSS** | 常驻内存大小 | `container_memory_rss{namespace="higress-system"}` |
| **Cache** | 页缓存大小 | `container_memory_cache{namespace="higress-system"}` |
| **Go Heap** | Go 堆内存使用 | `go_memstats_heap_inuse_bytes{namespace="higress-system"}` |
| **Go Stack** | Go 栈内存使用 | `go_memstats_stack_inuse_bytes{namespace="higress-system"}` |
| **内存使用率** | 内存使用百分比 | `rate(container_memory_working_set_bytes{namespace="higress-system"}[5m]) / container_spec_memory_limit_bytes * 100` |

## 🔍 访问监控界面

### Grafana

```bash
# 端口转发
kubectl port-forward -n monitoring svc/grafana 3000:3000

# 访问
open http://localhost:3000
# 默认用户名/密码: admin/admin
```

### Prometheus

```bash
# 端口转发
kubectl port-forward -n monitoring svc/prometheus-operated 9090:9090

# 访问
open http://localhost:9090
```

## 📈 常用 PromQL 查询

### 1. Gateway 内存使用趋势

```promql
container_memory_working_set_bytes{
  namespace="higress-system",
  pod=~"higress-gateway.*"
}
```

### 2. 内存使用率（超过80%告警）

```promql
rate(container_memory_working_set_bytes{
  namespace="higress-system",
  pod=~"higress-gateway.*"
}[5m]) / container_spec_memory_limit_bytes{
  namespace="higress-system",
  pod=~"higress-gateway.*"
} * 100 > 80
```

### 3. Go 内存分配详情

```promql
go_memstats_heap_inuse_bytes{
  namespace="higress-system",
  pod=~"higress-gateway.*"
}
```

### 4. 内存峰值统计

```promql
max_over_time(
  container_memory_working_set_bytes{
    namespace="higress-system",
    pod=~"higress-gateway.*"
  }[1h]
)
```

### 5. 所有 Higress Pod 内存对比

```promql
container_memory_working_set_bytes{
  namespace="higress-system"
} by (pod)
```

## 🔔 配置告警

### 内存告警规则示例

创建 `higress-memory-alerts.yaml`:

```yaml
apiVersion: monitoring.coreos.com/v1
kind: PrometheusRule
metadata:
  name: higress-memory-alerts
  namespace: higress-system
  labels:
    release: prometheus
spec:
  groups:
    - name: higress-memory
      interval: 30s
      rules:
        # Gateway 内存使用率告警
        - alert: HigressGatewayHighMemoryUsage
          expr: |
            rate(container_memory_working_set_bytes{
              namespace="higress-system",
              pod=~"higress-gateway.*"
            }[5m]) / container_spec_memory_limit_bytes{
              namespace="higress-system",
              pod=~"higress-gateway.*"
            } * 100 > 80
          for: 5m
          labels:
            severity: warning
          annotations:
            summary: "Higress Gateway 内存使用率过高"
            description: "Pod {{ $labels.pod }} 内存使用率为 {{ $value }}%"

        # Gateway 内存接近限制告警
        - alert: HigressGatewayMemoryNearLimit
          expr: |
            container_memory_working_set_bytes{
              namespace="higress-system",
              pod=~"higress-gateway.*"
            } / container_spec_memory_limit_bytes{
              namespace="higress-system",
              pod=~"higress-gateway.*"
            } > 0.9
          for: 5m
          labels:
            severity: critical
          annotations:
            summary: "Higress Gateway 内存接近限制"
            description: "Pod {{ $labels.pod }} 内存使用超过90%"

        # OOM 预警
        - alert: HigressGatewayPotentialOOM
          expr: |
            predict_linear(
              container_memory_working_set_bytes{
                namespace="higress-system",
                pod=~"higress-gateway.*"
              }[30m],
              3600
            ) > container_spec_memory_limit_bytes{
              namespace="higress-system",
              pod=~"higress-gateway.*"
            } * 0.95
          for: 10m
          labels:
            severity: critical
          annotations:
            summary: "Higress Gateway 可能发生 OOM"
            description: "Pod {{ $labels.pod }} 预计在1小时内发生 OOM"
```

应用告警规则：

```bash
kubectl apply -f higress-memory-alerts.yaml
```

## 🧪 验证监控

### 1. 检查 PodMonitor 状态

```bash
kubectl get podmonitor -n higress-system
```

预期输出：
```
NAME                    AGE
higress-gateway         5m
higress-controller      5m
```

### 2. 检查 Prometheus Targets

访问 Prometheus UI → Status → Targets，应该看到：
- `higress-gateway` (UP)
- `higress-controller` (UP)

### 3. 查询指标验证

在 Prometheus UI 中执行：

```promql
up{namespace="higress-system"}
```

应该返回值为 1（表示 target 正常）。

### 4. 测试内存压力

```bash
# 进入 Gateway Pod
kubectl exec -it -n higress-system \
  $(kubectl get pod -n higress-system -l app=higress-gateway -o jsonpath='{.items[0].metadata.name}') \
  -- sh

# 使用 stress 工具测试
apk add --no-cache stress
stress -m 1 --vm-bytes 500M --vm-keep
```

观察 Grafana Dashboard 中的内存变化。

## 📁 文件结构

```
monitoring/
├── install-prometheus-operator.sh      # Prometheus Operator 安装脚本
├── setup-higress-monitoring.sh         # 一键部署脚本
├── prometheus-operator-values.yaml     # Prometheus Operator 配置
├── higress-values-with-metrics.yaml    # Higress 监控配置
├── higress-gateway-podmonitor.yaml     # Gateway PodMonitor
├── higress-controller-podmonitor.yaml  # Controller PodMonitor
├── higress-memory-dashboard.json       # Grafana Dashboard
└── README.md                           # 本文档
```

## 🎯 最佳实践

### 1. 资源限制配置

建议在 `higress-values-with-metrics.yaml` 中配置资源限制：

```yaml
resources:
  requests:
    cpu: 500m
    memory: 512Mi
  limits:
    cpu: 2000m
    memory: 2Gi
```

### 2. 监控数据保留

在 `prometheus-operator-values.yaml` 中配置：

```yaml
prometheus:
  prometheusSpec:
    retention: 30d  # 保留30天数据
```

### 3. 抓取频率

根据需求调整：
- **开发环境**: 60s 间隔
- **生产环境**: 15-30s 间隔
- **高频监控**: 5-10s 间隔

### 4. Dashboard 刷新

Grafana Dashboard 刷新间隔建议：
- 实时监控: 10s
- 趋势分析: 1-5min
- 长期统计: 15-30min

## 🔧 故障排查

### 问题 1: PodMonitor 无法发现 Targets

**检查：**
```bash
kubectl get podmonitor -n higress-system
kubectl describe podmonitor higress-gateway -n higress-system
```

**解决：**
- 确认 Pod labels 与 PodMonitor selector 匹配
- 检查 Prometheus 是否有正确的 selector

### 问题 2: 指标数据为空

**检查：**
```bash
# 检查 Pod 是否暴露 metrics 端口
kubectl port-forward -n higress-system \
  $(kubectl get pod -n higress-system -l app=higress-gateway -o jsonpath='{.items[0].metadata.name}') \
  15020:15020

# 访问 metrics
curl http://localhost:15020/stats/prometheus
```

**解决：**
- 确认 metrics 端口正确（15020 或 15021）
- 检查 `/stats/prometheus` 路径是否可访问

### 问题 3: Grafana Dashboard 无数据

**检查：**
1. Prometheus 中是否有数据
2. Dashboard 的数据源是否配置正确
3. 查询的时间范围是否正确

**解决：**
- 检查 Grafana 数据源连接
- 调整 Dashboard 时间范围
- 验证 PromQL 查询语法

## 📚 参考资源

- [Prometheus Operator 文档](https://prometheus-operator.dev/)
- [Higress 官方文档](https://higress.io/)
- [cAdvisor 指标说明](https://github.com/google/cadvisor)
- [Go Runtime Metrics](https://github.com/prometheus/client_golang)

## 🆘 获取帮助

如遇问题，请：
1. 查看日志: `kubectl logs -n monitoring prometheus-kube-prometheus-prometheus-0`
2. 检查配置: `kubectl describe podmonitor -n higress-system`
3. 查看 Prometheus Targets 状态

---

**文档版本:** 1.0
**更新时间:** 2024-02-24
**作者:** Higress Monitoring Team
