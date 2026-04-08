# ✅ 已修正！Dashboard 使用变量，不再硬编码 Pod 名称

## 🎉 问题解决

您指出的问题非常关键！Dashboard 现在使用 **变量** 而不是硬编码的 Pod 名称。

---

## 📊 修正对比

### 修正前 ❌
```promql
kube_pod_container_resource_requests{
  namespace="agent",
  pod="higress-allinone-5cc7d4c7d-kgnk9",  # ❌ 硬编码
  resource="memory",
  container="higress"
}
```

**问题**：Pod 重启后名称会改变（例如变成 `higress-allinone-abc12345`），Dashboard 就无法查询数据。

### 修正后 ✅
```promql
kube_pod_container_resource_requests{
  namespace="$namespace",     # ✅ 变量
  pod=~"$pod",                # ✅ 变量（正则匹配）
  resource="memory",
  container="higress"         # ✅ 固定（容器名不变）
}
```

**优势**：
- ✅ **Pod 重启后自动匹配**新的 Pod 名称
- ✅ **容器名称固定**（`higress` 容器不会改名）
- ✅ **支持多环境**（可通过变量切换 namespace）

---

## 🎯 Dashboard 变量定义

### 1. **Namespace 变量**
```
名称: namespace
默认值: agent
类型: Query
来源: label_values(kube_pod_info, namespace)
```

### 2. **Pod 变量** ⭐
```
名称: pod
默认值: higress-allinone-.*
类型: Query
来源: label_values(kube_pod_info{namespace="$namespace"}, pod)
用途: 正则匹配任意 Higress Pod
```

**关键配置**：
- `regex: ".*"` - 允许正则表达式
- `value: "higress-allinone-.*"` - 默认匹配所有 `higress-allinone-` 开头的 Pod
- `pod=~"$pod"` - 查询中使用 `~` 进行正则匹配

### 3. **Container 变量**
```
名称: container
默认值: higress
类型: Query
来源: label_values(container_spec_memory_limit_bytes{namespace="$namespace",pod="$pod"}, container)
```

---

## 💡 为什么 Container 固定，Pod 不固定？

### **Container = "higress" (固定)**
- ✅ 容器名称在 Pod 配置中定义
- ✅ 不会因为 Pod 重启而改变
- ✅ 同一个 Deployment 的所有 Pod 都有相同的容器名

### **Pod = "$pod" (变量)**
- ❌ Pod 重启后名称会改变
- ❌ Deployment 会创建新的 Pod（名称不同）
- ✅ 需要使用正则表达式匹配所有 Pod

---

## 📈 实际效果

### 场景 1: Pod 重启

**之前** (硬编码):
```
Pod: higress-allinone-5cc7d4c7d-kgnk9
↓ 重启
Pod: higress-allinone-abc1234-xyz789
Dashboard: ❌ 无法显示数据 (查询旧 Pod)
```

**现在** (变量):
```
Pod: higress-allinone-5cc7d4c7d-kgnk9
↓ 重启
Pod: higress-allinone-abc1234-xyz789
Dashboard: ✅ 自动匹配新 Pod (pod=~"higress-allinone-.*")
```

### 场景 2: 多个 Pod 副本

如果有 3 个 Pod 副本：
```
higress-allinone-5cc7d4c7d-kgnk9
higress-allinone-abc1234-xyz789
higress-allinone-def5678-uvw123
```

**查询结果**：
- `pod=~"higress-allinone-.*"` 会自动匹配所有 3 个 Pod
- 显示聚合数据（sum/avg）

---

## 🔄 使用方法

### 1. 导入 Dashboard
```bash
Grafana → "+" → "Import" → "Upload JSON file"
选择: higress-complete-dashboard.json
```

### 2. 选择变量
导入后，Dashboard 顶部会显示 3 个下拉框：

```
Namespace: [agent ▼]
Pod:       [higress-allinone-.* ▼]
Container: [higress ▼]
```

**默认值**：
- Namespace: `agent`
- Pod: `higress-allinone-.*` (匹配所有)
- Container: `higress`

### 3. 切换 Pod（可选）
如果想监控特定的 Pod：
1. 点击 "Pod" 下拉框
2. 选择具体的 Pod（例如 `higress-allinone-5cc7d4c7d-kgnk9`）
3. Dashboard 会自动更新

**通常保持默认值** `higress-allinone-.*` 以监控所有 Pod。

---

## ✅ 验证变量工作

### 测试查询

```bash
# 查询 1: 使用正则匹配所有 Pod
curl -s -G "http://localhost:9091/api/v1/query" \
  --data-urlencode 'query=kube_pod_container_resource_requests{namespace="agent",pod=~"higress-allinone-.*",resource="memory",container="higress"}' | python3 -c "
import sys, json
data = json.load(sys.stdin)
if data['status'] == 'success':
    for r in data['data']['result']:
        print(f\"✅ Pod: {r['metric']['pod']}, Memory: {float(r['value'][1])/1024/1024:.0f} MB\")
"

# 输出类似:
# ✅ Pod: higress-allinone-5cc7d4c7d-kgnk9, Memory: 8192 MB
# ✅ Pod: higress-allinone-abc1234-xyz789, Memory: 8192 MB
```

---

## 📋 完整的查询示例

### Panel 1: Pod 内存分配 vs 实际使用
```promql
# 内存分配（所有 Pod 的总和或单个）
kube_pod_container_resource_requests{
  namespace="$namespace",
  pod=~"$pod",
  resource="memory",
  container="higress"
}

# 实际使用（聚合）
sum(container_memory_working_set_bytes{
  namespace="$namespace",
  pod=~"$pod",
  container="higress"
})
```

### Panel 14: 各容器内存明细
```promql
# 按容器分解（显示所有容器）
sum(container_memory_working_set_bytes{
  namespace="$namespace",
  pod=~"$pod",
  container!=""
}) by (container)
```

---

## 🎊 总结

**现在 Dashboard 可以：**

✅ **自动适应 Pod 重启**（不需要修改查询）
✅ **监控多个 Pod 副本**（自动聚合）
✅ **支持多环境**（通过 Namespace 变量切换）
✅ **固定容器名称**（`higress` 容器不变）

**关键改进：**
- ❌ 之前: `pod="higress-allinone-5cc7d4c7d-kgnk9"`
- ✅ 现在: `pod=~"$pod"` (默认 `higress-allinone-.*`)

**容器名称保持固定：**
- ✅ `container="higress"` (不会改变)

---

## 🚀 立即使用

1. **删除旧版本** (如果已导入)
2. **重新导入** `higress-complete-dashboard.json`
3. **验证变量** - 顶部应该显示 3 个下拉框
4. **保持默认值** - Pod 选择 `higress-allinone-.*`

**Pod 重启后，Dashboard 会自动继续显示数据！** 🎉
