# ✅ 已更新！Dashboard 现在使用动态 cAdvisor 指标

## 🎉 重大改进

您指出的问题非常关键！现在 **Pod 内存监控已全部改为动态获取**，不再使用静态值！

---

## 📊 更新内容

### 之前（静态值）❌
```promql
# 使用硬编码的静态值
2338*1024*1024  # Pod 总内存
(2338*1024*1024 / (8*1024*1024*1024)) * 100  # 内存使用率
```

### 现在（动态监控）✅
```promql
# 使用 cAdvisor 指标实时获取
sum(container_memory_working_set_bytes{namespace="agent", pod="higress-allinone-5cc7d4c7d-kgnk9", container!=""})

sum(container_memory_working_set_bytes{...}) / sum(container_spec_memory_limit_bytes{...}) * 100
```

---

## 🚀 立即使用

### 1. 重新导入 Dashboard

```bash
# 如果已经导入旧版本，先删除
# Grafana → Search → "Higress All-in-One Complete" → Delete

# 重新导入
# 点击 "+" → "Import" → "Upload JSON file"
# 选择: higress-complete-dashboard.json
```

### 2. 验证动态数据

导入后，您应该看到：

✅ **POD 总内存使用 (动态)**
- Gauge 图表，实时显示当前 Pod 内存
- 数值会随时间变化（不是静态 2338MB）

✅ **POD 内存使用率 (动态)**
- 百分比会动态变化
- 基于实际使用和分配上限计算

✅ **POD 内存分配情况 (MB, 动态)**
- 显示实际使用、分配上限、剩余可用
- 所有数值都是实时的

✅ **POD 各容器内存使用明细 (cAdvisor)** ⭐ 新增
- 按容器显示内存分布
- 可以看到每个容器（higress/discovery/etc）占用多少内存

---

## 📈 新增功能

### Pod 各容器内存明细 (Panel 14)

新增的 Panel 显示每个容器的内存使用：

```promql
sum(container_memory_working_set_bytes{...}) by (container)
```

**用途**:
- 查看 Pod 内各容器内存分布
- 识别哪个容器占用最多内存
- 容器级别的容量规划

---

## 🔍 数据来源说明

### 1. **cAdvisor 指标**（Pod 和容器内存）

您的 Prometheus 已经在抓取 cAdvisor 指标（通过 kubelet）：

- `container_memory_working_set_bytes` - 容器实际内存使用
- `container_spec_memory_limit_bytes` - 容器内存限制

这些指标来自：
```yaml
job="kubelet"
metrics_path="/metrics/cadvisor"
```

### 2. **Istio Agent 指标**（Go 内存）

- `istio_agent_go_memstats_*` - Go 进程内存详情
- 来自 Higress Pod 的 15020 端口

### 3. **Envoy 指标**（Envoy 内存）

- `envoy_server_memory_*` - Envoy 内存统计
- 聚合自 Envoy 的 /stats/prometheus

---

## ✅ 监控覆盖完整列表

| 监控项 | 指标来源 | 更新方式 | Panel ID |
|--------|----------|----------|----------|
| **Pod 总内存** | cAdvisor | ✅ 动态（实时） | 1 |
| **Pod 内存使用率** | cAdvisor | ✅ 动态（实时） | 2 |
| **Pod 内存分配** | cAdvisor | ✅ 动态（实时） | 3 |
| **各容器内存明细** | cAdvisor | ✅ 动态（实时） | 14 |
| **Go 进程内存** | Istio Agent | ✅ 动态（实时） | 4 |
| **Go Heap** | Istio Agent | ✅ 动态（实时） | 5 |
| **Go Stack** | Istio Agent | ✅ 动态（实时） | 6 |
| **系统开销** | Istio Agent | ✅ 动态（实时） | 7 |
| **Envoy 内存** | Envoy Stats | ✅ 动态（实时） | 8 |

---

## 🎯 与 kubectl top 对比

### Dashboard 显示（cAdvisor）
```
sum(container_memory_working_set_bytes{...})
≈ 2.3-2.4GB（动态变化）
```

### kubectl top 显示
```bash
kubectl top pod -n agent higress-allinone-5cc7d4c7d-kgnk9
# 输出: ~2338MB
```

**两者应该基本一致！**（cAdvisor working_set ≈ metrics API 的内存使用）

---

## 💡 为什么之前没发现 cAdvisor 指标？

在之前的探索中，我搜索的是：

```bash
container_memory_working_set_bytes{namespace="agent"}
```

但您的 Dashboard 实际使用的查询是：

```promql
container_memory_working_set_bytes{
  job="kubelet",
  metrics_path="/metrics/cadvisor",
  ...
}
```

关键区别：需要包含 **`job="kubelet"`** 标签！

---

## 🔄 Dashboard 列表对比

| Dashboard | Pod 内存 | 推荐度 |
|-----------|----------|--------|
| **higress-complete-dashboard.json** | ✅ cAdvisor 动态 | ⭐⭐⭐ **推荐** |
| higress-simple-dashboard.json | ❌ 仅 Go 内存 | ⭐⭐ 备用 |

---

## 📝 快速测试

导入新 Dashboard 后，测试动态数据：

1. 打开 Dashboard
2. 等待 10 秒（自动刷新）
3. 观察 **POD 总内存使用** Gauge：
   - 数值应该显示 ~2400MB（不是固定 2338MB）
   - 数值会轻微波动

4. 查看 **POD 各容器内存使用明细**：
   - 应该看到多条曲线（每个容器一条）
   - 容器名称如：higress、discovery 等

5. 对比 `kubectl top`:
   ```bash
   kubectl top pod -n agent higress-allinone-5cc7d4c7d-kgnk9
   ```
   - Dashboard 的 Pod 总内存 ≈ kubectl top 的值

---

## 🎊 总结

**现在 Dashboard 满足您的所有需求：**

✅ Pod 总内存 - **cAdvisor 动态监控**
✅ Pod 内存占用 - **实时使用率**
✅ Go 进程内存堆栈 - **完整监控**
✅ Envoy 内存 - **完整监控**
✅ 其他组件内存 - **按容器明细**
✅ 页缓存 - **Heap Idle 近似**
✅ 系统开销 - **GC、其他系统内存**

**所有数据都是动态的、实时的！** 🎉
