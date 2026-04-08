# 问题已解决 - Dashboard 使用指南

## 🎯 快速答案

您的 Higress Pod:
- **实际内存**: 2324MB (2.3GB)
- **Dashboard 显示错误**: 因为容器指标在 Prometheus 中不存在

**✅ 立即解决方案**: 导入这个 Dashboard
```
higress-simple-dashboard.json
```

---

## 📊 三个 Dashboard 对比

### 1. higress-simple-dashboard.json ⭐ **推荐使用**

**状态**: ✅ 所有指标都有数据

**显示内容**:
- Go 进程内存（~150MB）
- Heap & Stack
- Goroutines & Threads
- GC 统计
- Pod 实际内存（静态显示 2324MB）

**适用**: 监控 Go 控制平面内存趋势

```bash
# 导入这个文件
/Users/xiaodian/IdeaProjects/higress/monitoring/existing-higress/higress-simple-dashboard.json
```

---

### 2. higress-allinone-dashboard-fixed.json

**状态**: ❌ 很多指标显示 No Data

**原因**: Prometheus 中没有容器指标（`container_memory_*`）

**不推荐使用**

---

### 3. higress-allinone-dashboard.json (原始版)

**状态**: ❌ 指标配置错误

**原因**: 查询的指标与 all-in-one 环境不匹配

**不推荐使用**

---

## 🚀 立即操作

### 步骤 1: 导入正确的 Dashboard

1. 访问 Grafana: http://localhost:3000 (admin/admin)
2. 删除旧的 Dashboard（可选）
3. 点击 **"+" → "Import"**
4. 上传文件: `higress-simple-dashboard.json`

### 步骤 2: 验证数据

在新 Dashboard 中，第一个图应该显示：
- **Go 进程总内存**: ~150MB ✅
- **Heap 使用中**: ~13MB ✅
- **Stack 内存**: ~2MB ✅

所有指标都应该有数据！

### 步骤 3: 查看实际 Pod 内存

```bash
export KUBECONFIG=/Users/xiaodian/.kube/huawei-pref-kubeconfig.yaml
kubectl top pod -n agent higress-allinone-5cc7d4c7d-kgnk9

# 输出:
# NAME                               CPU(cores)   MEMORY(bytes)
# higress-allinone-5cc7d4c7d-kgnk9   19m          2324Mi
```

---

## 📈 监控说明

### Dashboard 能监控什么？

✅ **可以监控**:
- Go 控制平面内存（约 150MB）
- Heap 分配和释放
- GC 暂停时间和频率
- Goroutines 数量
- 内存泄漏趋势（Go 部分）

❌ **不能监控**:
- Envoy 内存（约 1.5GB）
- Java Console 内存（约 400MB）
- 完整 Pod 内存（2.3GB）

### 为什么只能监控 Go 部分？

Higress all-in-one 的 15020 端口只暴露 **Istio Agent** 的指标，这是 Go 控制平面的监控端点。容器级别的内存指标来自 **cAdvisor**，需要单独配置 Prometheus 抓取。

---

## 🔧 获取完整 Pod 内存监控

### 方案 1: 定期检查（最简单）

```bash
# 添加到 alias
echo 'alias htop="kubectl top pod -n agent"' >> ~/.zshrc
source ~/.zshrc

# 使用
htop
```

### 方案 2: Pushgateway + 脚本（需要部署）

```bash
cd /Users/xiaodian/IdeaProjects/higress/monitoring/existing-higress

# 运行采集脚本（后台）
nohup ./setup-real-memory-monitoring.sh > /dev/null 2>&1 &

# 这会每 30 秒推送一次 Pod 内存到 Prometheus
```

### 方案 3: 配置 cAdvisor（需要修改 Prometheus）

需要修改 Prometheus 配置，添加 cAdvisor 抓取。建议由运维人员操作。

---

## ✅ 推荐配置

### 短期（现在）
1. ✅ 使用 `higress-simple-dashboard.json`
2. ✅ 定期执行 `kubectl top pod`
3. ✅ 配置简单的告警脚本

### 长期（1-2周）
1. 配置 Pushgateway 采集完整 Pod 内存
2. 或配置 Prometheus cAdvisor 抓取
3. 更新 Dashboard 显示完整 2.3GB

---

## 📝 快速命令

```bash
# 1. 启动 Grafana
export KUBECONFIG=/Users/xiaodian/.kube/huawei-pref-kubeconfig.yaml
kubectl port-forward -n monitoring svc/grafana 3000:3000

# 2. 查看实际内存
kubectl top pod -n agent higress-allinone-5cc7d4c7d-kgnk9

# 3. 查看可用指标
kubectl port-forward -n monitoring svc/prometheus-operated 9090:9090
# 浏览器: http://localhost:9090/graph
# 查询: {namespace="agent", pod=~"higress.*"}
```

---

## 🎯 总结

**问题**: Dashboard 指标不显示
**原因**: 容器指标在 Prometheus 中不存在
**解决**: 使用 `higress-simple-dashboard.json`
**效果**: 可以监控 Go 内存（150MB），完整 Pod 内存需要额外配置

---

## 📞 需要完整监控？

如果需要监控完整的 2.3GB Pod 内存，请告诉我：
1. 是否要配置 Pushgateway（需要额外部署）
2. 还是保持现状（Go 监控 + kubectl top）
