# Higress pprof 火焰图获取工具

## 🎯 功能

自动获取 Higress Pod 中 Go 程序的 CPU 和内存火焰图。

## 📁 文件说明

```
/Users/xiaodian/IdeaProjects/higress/monitoring/existing-higress/
├── get-higress-pprof-quick.sh       ⭐ 推荐使用 - 自动化脚本
├── get-higress-pprof.sh             交互式脚本
└── PPROF_GUIDE.md                   完整使用指南
```

## 🚀 快速开始

### 1. 安装依赖

```bash
# Go (包含 pprof)
brew install go

# Graphviz (可选，用于生成调用图)
brew install graphviz
```

### 2. 运行脚本

```bash
cd /Users/xiaodian/IdeaProjects/higress/monitoring/existing-higress
./get-higress-pprof-quick.sh
```

### 3. 查看结果

```bash
# 输出目录
cd /tmp/higress-pprof

# 在浏览器中打开火焰图
open *.svg
```

## 📊 输出文件

| 文件 | 说明 |
|------|------|
| `higress-gateway-cpu.svg` | CPU 火焰图 |
| `higress-gateway-heap.svg` | 内存火焰图 |
| `higress-gateway-cpu.prof` | CPU profile 原始数据 |
| `higress-gateway-heap.prof` | Heap profile 原始数据 |

## 🔧 手动使用

如果自动脚本失败，可以手动获取：

```bash
export KUBECONFIG="/Users/xiaodian/.kube/huawei-pref-kubeconfig.yaml"

# 1. 端口转发
kubectl port-forward -n agent pod/higress-allinone-5cc7d4c7d-kgnk9 8080:8080

# 2. 采集 CPU profile (30秒)
curl "http://localhost:8080/debug/pprof/profile?seconds=30" -o cpu.prof

# 3. 生成火焰图
go tool pprof -raw cpu.prof | /tmp/FlameGraph/flamegraph.pl > cpu.svg

# 4. 打开查看
open cpu.svg
```

## 📚 详细文档

查看 `PPROF_GUIDE.md` 获取：
- 完整使用教程
- 交互式分析指南
- 性能调优案例
- 内存泄漏排查
- 高级技巧

## 🎯 适用场景

- ✅ 性能调优 - CPU 高占用
- ✅ 内存泄漏排查
- ✅ Goroutine 泄漏
- ✅ 锁争用分析
- ✅ 函数调用分析

## ⚠️ 注意事项

1. **性能影响**: pprof 采样开销约 1-2%
2. **采样时间**: 建议 10-30 秒
3. **生产环境**: 谨慎使用，避免高峰期

## 📞 故障排查

### 问题 1: 无法连接 pprof 端点

```bash
# 检查 Pod 是否运行
kubectl get pod -n agent higress-allinone-5cc7d4c7d-kgnk9

# 检查端口是否监听
kubectl exec -n agent higress-allinone-5cc7d4c7d-kgnk9 -- netstat -tlnp | grep 8080
```

### 问题 2: 火焰图生成失败

```bash
# 安装 FlameGraph
git clone https://github.com/brendangregg/FlameGraph /tmp/FlameGraph

# 检查 profile 数据
file cpu.prof
```

### 问题 3: 端口转发失败

```bash
# 检查 kubeconfig
export KUBECONFIG="/Users/xiaodian/.kube/huawei-pref-kubeconfig.yaml"
kubectl cluster-info
```

---

**立即开始性能分析！** 🚀
