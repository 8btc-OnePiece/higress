# 🔥 Higress pprof 火焰图获取指南

## 📋 前提条件

### 1. 安装必要工具

```bash
# 1. Go (包含 pprof)
brew install go

# 2. Graphviz (用于生成调用图)
brew install graphviz

# 3. 验证安装
go version
dot -V
```

### 2. 检查 pprof 端点

Higress Pod 中的 Go 程序通常支持 pprof：

| 组件 | 端口 | 说明 |
|------|------|------|
| **Higress Gateway** | 8080 | Go 网关 |
| **Pilot Discovery** | 15014 | Istio 控制平面 |
| **API Server** | 18443 | 内部 API |

---

## 🚀 快速开始

### 方法 1: 使用自动化脚本（推荐）

```bash
cd /Users/xiaodian/IdeaProjects/higress/monitoring/existing-higress
./get-higress-pprof-quick.sh
```

**脚本会自动**：
1. ✅ 检测可用的 pprof 端点
2. ✅ 采集 CPU profile（30秒）
3. ✅ 采集 Heap profile
4. ✅ 生成 SVG 火焰图
5. ✅ 显示 Top 10 耗时函数

**输出目录**: `/tmp/higress-pprof/`

---

### 方法 2: 手动获取

#### 步骤 1: 启动端口转发

```bash
export KUBECONFIG="/Users/xiaodian/.kube/huawei-pref-kubeconfig.yaml"

# 转发 Higress Gateway (端口 8080)
kubectl port-forward -n agent pod/higress-allinone-5cc7d4c7d-kgnk9 8080:8080
```

#### 步骤 2: 验证 pprof 端点

```bash
# 测试 pprof 是否可用
curl http://localhost:8080/debug/pprof/

# 应该看到:
# /debug/pprof/
# /debug/pprof/goroutine
# /debug/pprof/heap
# /debug/pprof/profile
# /debug/pprof/threadcreate
# ...
```

#### 步骤 3: 采集 CPU Profile

```bash
# 方法 A: 使用 curl (30秒采样)
curl "http://localhost:8080/debug/pprof/profile?seconds=30" -o cpu.prof

# 方法 B: 使用 go tool pprof (交互式)
go tool pprof http://localhost:8080/debug/pprof/profile?seconds=30
```

#### 步骤 4: 生成火焰图

```bash
# 安装 FlameGraph (如果需要)
git clone https://github.com/brendangregg/FlameGraph /tmp/FlameGraph

# 生成 SVG 火焰图
go tool pprof -raw cpu.prof | /tmp/FlameGraph/flamegraph.pl > cpu.svg

# 在浏览器中打开
open cpu.svg
```

#### 步骤 5: 采集 Heap Profile

```bash
# 获取堆内存 profile
curl http://localhost:8080/debug/pprof/heap -o heap.prof

# 生成内存火焰图
go tool pprof -raw heap.prof | /tmp/FlameGraph/flamegraph.pl > heap.svg
```

---

## 📊 可用的 Profile 类型

| Profile | 端点 | 说明 | 采集命令 |
|---------|------|------|----------|
| **CPU** | `/debug/pprof/profile` | CPU 使用情况 | `curl?seconds=30` |
| **Heap** | `/debug/pprof/heap` | 堆内存分配 | `curl` |
| **Goroutine** | `/debug/pprof/goroutine` | Goroutine 栈 | `curl` |
| **ThreadCreate** | `/debug/pprof/threadcreate` | 线程创建 | `curl` |
| **Block** | `/debug/pprof/block` | 阻塞操作 | `curl` |
| **Mutex** | `/debug/pprof/mutex` | 互斥锁争用 | `curl` |
| **Allocs** | `/debug/pprof/allocs` | 内存分配 | `curl` |

---

## 🛠️ pprof 交互式分析

### 启动交互式分析

```bash
# 分析 CPU profile
go tool pprof cpu.prof

# 分析 Heap profile
go tool pprof heap.prof

# 远程分析（无需下载）
go tool pprof http://localhost:8080/debug/pprof/profile
```

### 常用命令

```bash
# 进入 pprof 后:

(pprof) top                # Top 10 耗时函数
(pprof) top -cum           # 按累积时间排序
(pprof) list functionName  # 查看特定函数的代码
(pprof) web                # 在浏览器中打开调用图
(pprof) pdf                # 生成 PDF 调用图
(pprof) png                # 生成 PNG 调用图
(pprof) flamegraph         # 生成火焰图
(pprof) peek functionName  # 查看函数详情
(pprof) quit               # 退出
```

### 常用选项

```bash
# 采样 30 秒
go tool pprof http://localhost:8080/debug/pprof/profile?seconds=30

# 只显示用户代码 (跳过 Go 运行时)
go tool pprof -no_system cpu.prof

# 按累积时间排序
go tool pprof -cum cpu.prof

# 生成函数调用图
go tool pprof -pdf cpu.prof > callgraph.pdf

# 比较两个 profile
go tool pprof -base cpu1.prof cpu2.prof
```

---

## 🎯 实际应用场景

### 场景 1: 性能调优 - CPU 高占用

**问题**: Higress Gateway CPU 使用率高

**步骤**:
```bash
# 1. 采集 CPU profile (30秒)
curl "http://localhost:8080/debug/pprof/profile?seconds=30" -o cpu.prof

# 2. 查看热点函数
go tool pprof -top cpu.prof

# 3. 生成火焰图
go tool pprof -raw cpu.prof | /tmp/FlameGraph/flamegraph.pl > cpu.svg
open cpu.svg

# 4. 分析热点函数
go tool pprof cpu.prof
(pprof) list hotspotFunction
(pprof) web
```

### 场景 2: 内存泄漏排查

**问题**: 内存持续增长

**步骤**:
```bash
# 1. 采集 Heap profile
curl http://localhost:8080/debug/pprof/heap -o heap1.prof

# 2. 等待一段时间 (例如 10 分钟)
# (运行负载测试)

# 3. 再次采集
curl http://localhost:8080/debug/pprof/heap -o heap2.prof

# 4. 比较差异
go tool pprof -base heap1.prof heap2.prof

# 5. 查看内存分配
go tool pprof -alloc_space heap.prof
```

### 场景 3: Goroutine 泄漏

**问题**: Goroutine 数量持续增长

**步骤**:
```bash
# 1. 采集 Goroutine profile
curl http://localhost:8080/debug/pprof/goroutine -o goroutine.prof

# 2. 查看 Goroutine 数量
go tool pprof goroutine.prof
(pprof) top

# 3. 查看 Goroutine 栈
go tool pprof -tags goroutine.prof
```

---

## 📈 高级技巧

### 1. 实时监控

```bash
# 每隔 10 秒采集一次
for i in {1..10}; do
    curl "http://localhost:8080/debug/pprof/profile?seconds=10" -o "cpu_$i.prof"
    sleep 10
done
```

### 2. 持续性能分析

```bash
# 后台运行 pprof HTTP 服务器
go tool pprof -http=:9999 cpu.prof

# 浏览器访问: http://localhost:9999
```

### 3. 导出数据

```bash
# 导出为文本
go tool pprof cpu.prof > cpu.txt

# 导出为原始格式
go tool pprof -raw cpu.prof > cpu.raw
```

### 4. 过滤和聚焦

```bash
# 只看特定函数
go tool pprof -run_function_name cpu.prof

# 只看特定包
go tool pprof -focus=gateway cpu.prof

# 忽略 Go 运行时
go tool pprof -no_system cpu.prof
```

---

## 🔍 解读火焰图

### 火焰图构成

```
       ▼ (向上生长)
    ┌───┴───┐
    │   A   │  函数 A
  ┌─┴───┬───┴─┐
  │  B  │  C  │  调用 B 和 C
  └─┬───┴───┬─┘
    │  D    │  调用 D
    └───────┘
```

**颜色说明**:
- 🔴 暖色: CPU 使用高
- 🔵 冷色: CPU 使用低
- 🔵 随机: 不同函数

**宽度说明**:
- 宽度 = CPU 时间占比
- 越宽 = 趗时越多

---

## ⚠️ 注意事项

### 1. 性能影响

- ✅ pprof 采样开销很小 (~1-2%)
- ⚠️ 长时间采样会影响性能
- 建议: 生产环境采样 10-30 秒

### 2. 数据准确性

- CPU Profile 是**采样数据**（不是精确值）
- 采样频率: 100 Hz (每 10ms 一次)
- 对于短函数可能采样不到

### 3. 内存分析

- Heap Profile 显示的是**当前**堆内存
- 不是历史最大值
- 如需历史数据，使用 `runtime/metrics`

---

## 🎯 最佳实践

### 1. 采集时机

- ✅ 高峰期
- ✅ 压力测试期间
- ✅ 性能问题出现时
- ❌ 低负载时（看不到问题）

### 2. 对比分析

```bash
# 建立基线
curl "http://localhost:8080/debug/pprof/profile?seconds=30" -o baseline.prof

# 问题发生时
curl "http://localhost:8080/debug/pprof/profile?seconds=30" -o problem.prof

# 对比
go tool pprof -base baseline.prof problem.prof
```

### 3. 多次采样

```bash
# 采集多个样本
for i in {1..5}; do
    curl "?seconds=30" -o "sample_$i.prof"
    sleep 60
done

# 合并分析
go tool pprof sample_*.prof
```

---

## 📚 相关资源

- **Go pprof 官方文档**: https://pkg.go.dev/net/http/pprof
- **FlameGraph**: https://github.com/brendangregg/FlameGraph
- **pprof 指南**: https://github.com/google/pprof

---

## ✅ 总结

**获取火焰图的步骤**:
1. 运行脚本: `./get-higress-pprof-quick.sh`
2. 等待 30 秒采样
3. 查看 `/tmp/higress-pprof/*.svg`

**分析性能问题**:
1. 找最宽的火焰条（耗时最多）
2. 点击查看函数名
3. 定位代码位置

**排查内存问题**:
1. 采集前后两次 Heap profile
2. 对比差异找出增长点
3. 分析代码逻辑

**立即可用！** 🚀
