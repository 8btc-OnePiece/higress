# Higress All-in-One 内存监控 - 快速开始

## 🚀 一键部署（推荐）

```bash
cd /Users/xiaodian/IdeaProjects/higress/monitoring/existing-higress
chmod +x deploy-monitoring.sh
./deploy-monitoring.sh
```

## 📊 查看监控

### 启动端口转发

```bash
# 设置 KUBECONFIG
export KUBECONFIG=/Users/xiaodian/.kube/huawei-pref-kubeconfig.yaml

# Grafana
kubectl port-forward -n monitoring svc/grafana 3000:3000

# Prometheus
kubectl port-forward -n monitoring svc/prometheus-operated 9090:9090
```

### 访问 Dashboard

- **Grafana**: http://localhost:3000 (admin/admin)
  - 导入 `higress-allinone-dashboard.json`
  - 或搜索 "Higress All-in-One"

- **Prometheus**: http://localhost:9090

## 📈 关键查询

### Higress 内存使用

```promql
# 已分配内存
istio_agent_go_memstats_alloc_bytes{namespace="agent"}

# Heap 内存
istio_agent_go_memstats_heap_inuse_bytes{namespace="agent"}

# 内存分配速率
rate(istio_agent_go_memstats_alloc_bytes_total{namespace="agent"}[5m])

# Goroutines 数量
istio_agent_go_goroutines{namespace="agent"}
```

## ⚠️ 告警规则

| 级别 | 告警 | 阈值 |
|------|------|------|
| WARNING | 内存使用率过高 | > 80% |
| CRITICAL | 内存接近限制 | > 90% |
| CRITICAL | 预计 OOM | 1小时内 |
| WARNING | 内存持续增长 | 2小时 |

## ✅ 验证

```bash
# 检查 Metrics Service
kubectl get svc -n agent higress-allinone-metrics

# 检查 PodMonitor
kubectl get podmonitor -n agent

# 测试 metrics 端点
kubectl exec -n agent \
  $(kubectl get pod -n agent -l app=higress-allinone -o jsonpath='{.items[0].metadata.name}') \
  -- curl -s http://localhost:15020/stats/prometheus | head -5
```

## 🔧 常见问题

**Q: 没有看到数据？**
- 等待 2-3 分钟让 Prometheus 采集数据
- 检查 Prometheus Targets 状态

**Q: Dashboard 导入失败？**
- 确认 Grafana 可访问
- 手动导入：Grafana UI → Import → Upload JSON

**Q: 如何修改 KUBECONFIG？**
- 编辑 `deploy-monitoring.sh`
- 修改 `KUBECONFIG` 变量为您需要的路径

更多详细信息请查看 [README.md](./README.md)
