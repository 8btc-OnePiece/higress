# Higress 内存监控 - 快速开始

## 🚀 一键部署（推荐）

```bash
cd /Users/xiaodian/IdeaProjects/higress/monitoring
./setup-higress-monitoring.sh
```

执行后即可完成所有配置！

## 📊 查看监控

### 1. 启动端口转发

```bash
# Grafana
kubectl port-forward -n monitoring svc/grafana 3000:3000

# Prometheus
kubectl port-forward -n monitoring svc/prometheus-operated 9090:9090
```

### 2. 访问 Dashboard

- **Grafana**: http://localhost:3000 (admin/admin)
- 搜索 "Higress Memory Monitoring" dashboard
- **Prometheus**: http://localhost:9090

## 📈 关键内存指标查询

### 在 Prometheus 或 Grafana 中执行：

**Gateway 内存使用：**
```promql
container_memory_working_set_bytes{
  namespace="higress-system",
  pod=~"higress-gateway.*"
}
```

**内存使用率（%）:**
```promql
rate(container_memory_working_set_bytes{
  namespace="higress-system",
  pod=~"higress-gateway.*"
}[5m]) / container_spec_memory_limit_bytes{
  namespace="higress-system",
  pod=~"higress-gateway.*"
} * 100
```

**Go Heap 内存：**
```promql
go_memstats_heap_inuse_bytes{
  namespace="higress-system"
}
```

## ⚠️ 内存告警

### 应用告警规则：

```bash
kubectl apply -f higress-memory-alerts.yaml
```

### 告警级别：

- **警告（Warning）**: 内存使用率 > 80%
- **严重（Critical）**: 内存使用率 > 90%
- **OOM 预警**: 预计1小时内发生 OOM

## 🔍 验证配置

### 检查 PodMonitor：

```bash
kubectl get podmonitor -n higress-system
```

### 检查 Prometheus Targets：

访问 Prometheus → Status → Targets，确认 `higress-gateway` 和 `higress-controller` 都是 UP 状态。

### 查询所有 Higress 指标：

```bash
curl http://localhost:9090/api/v1/label/__name__/values | jq '.data[] | select(contains("higress") or contains("container_memory"))'
```

## 📁 文件说明

| 文件 | 说明 |
|------|------|
| `setup-higress-monitoring.sh` | 一键部署脚本 |
| `prometheus-operator-values.yaml` | Prometheus Operator 配置 |
| `higress-values-with-metrics.yaml` | Higress 监控配置 |
| `higress-gateway-podmonitor.yaml` | Gateway PodMonitor |
| `higress-controller-podmonitor.yaml` | Controller PodMonitor |
| `higress-memory-dashboard.json` | Grafana Dashboard |
| `higress-memory-alerts.yaml` | 内存告警规则 |
| `README.md` | 完整文档 |

## 💡 下一步

1. 访问 Grafana Dashboard 查看实时内存数据
2. 根据实际情况调整告警阈值
3. 配置告警通知（Email/Slack/钉钉）
4. 定期检查内存使用趋势

## 🆘 常见问题

**Q: 没有看到数据？**
- 等待 2-3 分钟让 Prometheus 采集数据
- 检查 PodMonitor 是否正确应用
- 查看 Prometheus Targets 状态

**Q: Dashboard 导入失败？**
- 确认 Grafana 已安装并可访问
- 手动导入：Grafana UI → Import → Upload JSON

**Q: 告警没有触发？**
- 检查 PrometheusRule 状态：`kubectl get prometheusrule -n higress-system`
- 查看 Prometheus 日志：`kubectl logs -n monitoring prometheus-kube-prometheus-prometheus-0`
- 确认告警规则表达式正确

更多详细信息请查看 [README.md](./README.md)
