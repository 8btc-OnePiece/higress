# Higress Prometheus 内存监控 - 文档索引

## 📚 文档导航

1. **[QUICKSTART.md](./QUICKSTART.md)** - 快速开始指南 ⭐ 推荐从这里开始
2. **[README.md](./README.md)** - 完整配置文档
3. **本文档** - 配置文件索引

## 📁 配置文件清单

### 核心配置文件

| 文件名 | 大小 | 说明 |
|--------|------|------|
| `setup-higress-monitoring.sh` | 1.6K | 🚀 一键部署脚本 |
| `install-prometheus-operator.sh` | 3.7K | Prometheus Operator 安装脚本 |

### 监控配置

| 文件名 | 大小 | 说明 |
|--------|------|------|
| `prometheus-operator-values.yaml` | 1.8K | Prometheus Operator Helm values |
| `higress-values-with-metrics.yaml` | 1.5K | Higress 启用监控的配置 |

### PodMonitor 配置

| 文件名 | 大小 | 说明 |
|--------|------|------|
| `higress-gateway-podmonitor.yaml` | 875B | Gateway PodMonitor 定义 |
| `higress-controller-podmonitor.yaml` | 587B | Controller PodMonitor 定义 |

### Dashboard & 告警

| 文件名 | 大小 | 说明 |
|--------|------|------|
| `higress-memory-dashboard.json` | 11K | Grafana Dashboard（内存监控） |
| `higress-memory-alerts.yaml` | 7.5K | Prometheus 告警规则 |

## 🎯 使用场景

### 场景 1: 全新安装

```bash
# 1. 一键部署（推荐）
./setup-higress-monitoring.sh

# 2. 访问监控
kubectl port-forward -n monitoring svc/grafana 3000:3000
open http://localhost:3000
```

### 场景 2: 仅配置 Higress 监控

如果已经安装了 Prometheus Operator：

```bash
# 1. 应用 PodMonitor
kubectl apply -f higress-gateway-podmonitor.yaml
kubectl apply -f higress-controller-podmonitor.yaml

# 2. 更新 Higress 配置
cd /Users/xiaodian/IdeaProjects/higress/helm/core
helm upgrade higress . \
  --namespace higress-system \
  --values ../monitoring/higress-values-with-metrics.yaml

# 3. 导入 Dashboard
# 在 Grafana UI 中导入 higress-memory-dashboard.json
```

### 场景 3: 仅添加告警规则

```bash
# 应用告警规则
kubectl apply -f higress-memory-alerts.yaml

# 验证
kubectl get prometheusrule -n higress-system
```

## 📊 监控指标概览

### Gateway 内存指标

- `container_memory_working_set_bytes` - 工作集内存（实际使用）
- `container_memory_rss` - 常驻内存
- `container_memory_cache` - 页缓存
- `go_memstats_heap_inuse_bytes` - Go 堆内存
- `go_memstats_stack_inuse_bytes` - Go 栈内存

### 告警规则

| 告警名称 | 级别 | 阈值 | 持续时间 |
|----------|------|------|----------|
| HigressGatewayHighMemoryUsage | warning | 80% | 5分钟 |
| HigressGatewayMemoryNearLimit | critical | 90% | 5分钟 |
| HigressGatewayPotentialOOM | critical | 预计1小时内OOM | 10分钟 |
| HigressGatewayMemoryGrowing | warning | 持续增长 | 2小时 |

## 🔗 相关资源

- [Prometheus 官方文档](https://prometheus.io/docs/)
- [Prometheus Operator 文档](https://prometheus-operator.dev/)
- [Grafana Dashboard 文档](https://grafana.com/docs/grafana/latest/dashboards/)
- [Higress 官方文档](https://higress.io/docs/)

## 🛠️ 维护建议

### 日常维护

1. **每日检查**
   - 查看内存使用趋势
   - 检查告警状态

2. **每周检查**
   - 分析内存峰值
   - 优化资源配置

3. **每月检查**
   - 评估是否需要扩容
   - 更新告警规则

### 容量规划

| 并发请求 | 推荐内存 | 最大内存 |
|----------|----------|----------|
| 低流量（<100 QPS） | 512Mi | 1Gi |
| 中流量（100-1000 QPS） | 1Gi | 2Gi |
| 高流量（>1000 QPS） | 2Gi | 4Gi |

## 📞 获取帮助

- 查看日志：`kubectl logs -n monitoring prometheus-kube-prometheus-prometheus-0`
- 检查配置：`kubectl describe podmonitor -n higress-system`
- 验证目标：Prometheus UI → Status → Targets

## 📝 更新日志

- **v1.0** (2024-02-27): 初始版本
  - 支持 Gateway 和 Controller 监控
  - 提供 Grafana Dashboard
  - 配置内存告警规则

---

**快速链接：**
- 📖 [完整文档](./README.md)
- 🚀 [快速开始](./QUICKSTART.md)
- 🔧 [配置文件](./)

