# ✅ Higress pprof 火焰图获取 - 完成！

## 🎉 已完成的工作

为您创建了完整的 Higress pprof 性能分析工具集！

---

## 📁 文件清单

### 主脚本

| 文件 | 说明 | 推荐度 |
|------|------|--------|
| **pprof-simple.sh** | 一键采集脚本 | ⭐⭐⭐ **推荐** |
| get-higress-pprof-quick.sh | 交互式脚本 | ⭐⭐ 备用 |
| get-higress-pprof.sh | 完整功能脚本 | ⭐ 高级用户 |

### 文档

| 文件 | 说明 |
|------|------|
| **PPROF_QUICKSTART.md** | 本文档 - 快速开始 |
| PPROF_README.md | 快速参考 |
| PPROF_GUIDE.md | 完整使用指南 |

---

## 🚀 立即使用

### **一键获取火焰图**（推荐）

```bash
cd /Users/xiaodian/IdeaProjects/higress/monitoring/existing-higress

# 默认采样 30 秒
./pprof-simple.sh

# 或者自定义采样时长（例如 60 秒）
./pprof-simple.sh 60
```

**脚本会自动**：
1. ✅ 检测可用的 pprof 端点
2. ✅ 采集 CPU profile（默认30秒）
3. ✅ 采集 Heap profile
4. ✅ 生成 SVG 火焰图
5. ✅ 显示 Top 10 耗时函数
6. ✅ 所有数据保存到带时间戳的目录

**输出示例**：
```
/tmp/higress-pprof-20250228_103000/
├── higress-gateway-cpu.prof       # CPU profile 原始数据
├── higress-gateway-cpu.svg        # CPU 火焰图 ⭐
├── higress-gateway-heap.prof      # Heap profile 原始数据
├── higress-gateway-heap.svg       # 内存火焰图 ⭐
├── pilot-discovery-cpu.prof
├── pilot-discovery-cpu.svg
├── pilot-discovery-heap.prof
└── pilot-discovery-heap.svg
```

---

## 📊 查看火焰图

### 方法 1: 直接打开 SVG

```bash
cd /tmp/higress-pprof-*

# 在浏览器中打开所有 SVG
open *.svg
```

### 方法 2: Web UI 分析

```bash
cd /tmp/higress-pprof-*

# 启动 Web UI
go tool pprof -http=:9999 higress-gateway-cpu.prof

# 浏览器访问
open http://localhost:9999
```

**Web UI 功能**：
- 🔥 火焰图可视化
- 📊 调用图
- 📈 Top 函数列表
- 🔍 函数源码查看

---

## 🔧 前置要求

### 检查安装

```bash
# 1. Go (必须)
go version
# 输出: go version go1.xx.x darwin/arm64

# 2. pprof (Go 自带)
go tool pprof -h
# 应该显示帮助信息

# 3. FlameGraph (自动下载)
# 脚本会自动下载到 /tmp/FlameGraph/
```

### 安装 Go（如果没有）

```bash
brew install go
```

---

## 🎯 使用场景

### **场景 1: CPU 性能问题**

**症状**: Higress CPU 使用率高

**步骤**:
```bash
# 1. 运行脚本（采样30秒）
./pprof-simple.sh 30

# 2. 打开 CPU 火焰图
open /tmp/higress-pprof-*/higress-gateway-cpu.svg

# 3. 找最宽的火焰条（热点函数）
# 4. 点击查看完整调用栈
```

### **场景 2: 内存泄漏**

**症状**: 内存持续增长

**步骤**:
```bash
# 1. 第一次采样
./pprof-simple.sh
mv /tmp/higress-pprof-* /tmp/before/

# 2. 运行负载测试或等待问题复现（例如10分钟）

# 3. 第二次采样
./pprof-simple.sh
mv /tmp/higress-pprof-* /tmp/after/

# 4. 对比 Heap profile
cd /tmp/after
go tool pprof -base ../before/higress-gateway-heap.prof \
                higress-gateway-heap.prof
```

### **场景 3: 定位特定函数**

**步骤**:
```bash
cd /tmp/higress-pprof-*

# 启动交互式分析
go tool pprof higress-gateway-cpu.prof

# pprof 交互命令:
(pprof) top                    # Top 10 耗时函数
(pprof) top -cum               # 按累积时间排序
(pprof) list functionName      # 查看函数代码
(pprof) web                    # 浏览器打开调用图
(pprof) pdf                    # 生成 PDF 调用图
(pprof) peek functionName      # 查看函数详情
```

---

## 📈 解读火焰图

### 火焰图结构

```
       ▲ (向上生长)
    ┌───┴───┐
    │ main  │  ← 根函数
  ┌─┴───┬───┴─┐
  │ funcA│funcB│  ← 调用 A 和 B
  └─┬───┴───┬─┘
    │hotspot│  ← 热点函数
    └───────┘
```

**颜色说明**：
- 🔴 暖色（红/橙/黄）- CPU 使用高
- 🔵 冷色（绿/蓝）- CPU 使用低
- 🔵 随机颜色 - 区分不同函数

**宽度说明**：
- 宽度 = CPU 采样次数
- 越宽 = 耗时越多
- 关注最宽的火焰条

---

## ⚡ 性能分析技巧

### 1. 快速定位热点

```bash
# 查看 Top 10
go tool pprof -top -nodecount=10 cpu.prof

# 输出示例:
# flat  flat%   sum%        cum   cum%
# 120ms 10.0% 10.0%       600ms 50.0%  runtime.heapalloc
#  80ms  6.67% 16.67%      300ms 25.0%  main.processRequest
```

### 2. 查看函数调用链

```bash
# 列出特定函数
go tool pprof -list processRequest cpu.prof

# 查看调用图
go tool pprof -web cpu.prof
```

### 3. 忽略 Go 运行时

```bash
# 只看用户代码
go tool pprof -no_system cpu.prof
```

---

## ⚠️ 注意事项

### 1. 性能影响

- ✅ pprof 采样开销很小（~1-2%）
- ⚠️ 长时间采样会影响性能
- 建议：生产环境采样 10-30 秒

### 2. 数据准确性

- CPU Profile 是**采样数据**（100 Hz）
- 短函数可能采样不到
- 需要足够的采样时长

### 3. 采样时机

- ✅ 高峰期
- ✅ 压力测试时
- ✅ 问题复现时
- ❌ 低负载时（看不到问题）

---

## 📚 进阶阅读

- **完整指南**: `PPROF_GUIDE.md`
- **快速参考**: `PPROF_README.md`
- **Go pprof 文档**: https://pkg.go.dev/net/http/pprof
- **FlameGraph**: https://github.com/brendangregg/FlameGraph

---

## 🎯 总结

### 已创建的工具

| 工具 | 文件 | 用途 |
|------|------|------|
| **一键脚本** | `pprof-simple.sh` | 自动采集火焰图 |
| **交互式** | `get-higress-pprof-quick.sh` | 可选组件 |
| **完整版** | `get-higress-pprof.sh` | 高级功能 |

### 使用流程

```bash
# 1. 运行脚本
cd /Users/xiaodian/IdeaProjects/higress/monitoring/existing-higress
./pprof-simple.sh

# 2. 等待 30 秒采样
# (或自定义时长: ./pprof-simple.sh 60)

# 3. 查看火焰图
open /tmp/higress-pprof-*/higress-gateway-cpu.svg

# 4. 分析热点
# 找最宽的火焰条 → 查看函数名 → 定位代码
```

### 下一步

1. ✅ 运行 `pprof-simple.sh`
2. ✅ 打开生成的 SVG 文件
3. ✅ 找出 CPU 热点函数
4. ✅ 优化代码
5. ✅ 再次采样验证效果

---

**开始性能分析之旅吧！** 🚀
