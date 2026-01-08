# Go 1.24 原生编译升级说明

## 升级概述

本次升级将插件从 **TinyGo 0.28 + Go 1.21** 编译方案迁移到 **Go 1.24 原生 WASM 编译**方案。

这是 Higress 官方推荐的最新编译方式，具有以下优势：
- ✅ 官方推荐的标准方式
- ✅ 更好的性能和兼容性
- ✅ 统一的工具链（无需额外安装 TinyGo）
- ✅ 更小的二进制文件

## 升级时间

**升级日期**: 2026-01-06
**升级版本**: 1.0.0 (Go 1.24 Native)

## 变更内容

### 1. go.mod
```diff
- go 1.21
+ go 1.24
```

### 2. build.sh
- ✅ 默认使用 **Go 1.24 原生编译**
- ✅ 保留 TinyGo 编译选项（通过 `BUILD_MODE=tinygo` 切换）
- ✅ 自动检测 Go 版本

**新的编译命令**：
```bash
# 默认使用 Go 1.24 编译（推荐）
./build.sh

# 或者使用 TinyGo 编译（兼容模式）
BUILD_MODE=tinygo ./build.sh
```

### 3. Dockerfile
```diff
- COPY main.wasm /plugin/main.wasm
+ COPY main.wasm plugin.wasm
```

更新为官方推荐的路径格式，同时保留完整的元数据 Labels。

### 4. main.go
- ✅ **无需修改** - 代码已经兼容 Go 1.24
- ✅ `main()` 函数为空
- ✅ 初始化逻辑在 `init()` 函数中

## 使用方法

### 前置要求

**Go 1.24 或更高版本**

检查版本：
```bash
go version
# 输出应类似：go version go1.24.x darwin/amd64
```

如果版本低于 1.24，请安装最新版本：
- macOS: `brew install go`
- 或访问：https://go.dev/dl/

### 构建步骤

```bash
# 1. 进入插件目录
cd consumer-group-mapping

# 2. 更新依赖
go mod tidy

# 3. 编译 wasm 文件（默认使用 Go 1.24）
./build.sh

# 4. 构建 Docker 镜像
docker build -t consumer-group-mapping:1.0.0 .

# 5. 推送到镜像仓库
docker push your-registry/consumer-group-mapping:1.0.0
```

### 降级到 TinyGo（可选）

如果需要使用 TinyGo 编译：
```bash
BUILD_MODE=tinygo ./build.sh
```

## 兼容性说明

### ✅ 完全兼容
- **功能**：所有功能保持不变
- **配置**：无需修改任何配置文件
- **API**：插件接口和行为完全一致
- **性能**：性能相当或更优

### ⚠️ 环境要求
- **Go 版本**：需要 Go 1.24 或更高版本
- **Higress 版本**：建议使用最新版本的 Higress

### 🔄 迁移路径
如果您的环境不支持 Go 1.24，可以：
1. 使用 `BUILD_MODE=tinygo` 继续使用 TinyGo 编译
2. 升级 Go 版本到 1.24+
3. 升级 Higress 到最新版本

## 验证升级

构建完成后，验证 wasm 文件：
```bash
ls -lh main.wasm
```

文件大小应在 2-3 MB 范围内。

## 常见问题

### Q1: 编译时提示 "go: module X: not found"

**解决方法**：
```bash
go mod tidy
```

### Q2: Go 版本检查失败

**错误信息**：
```
Error: Go version must be 1.24 or later
Current version: 1.21
```

**解决方法**：
1. 升级 Go 到 1.24+
2. 或使用 TinyGo 编译：`BUILD_MODE=tinygo ./build.sh`

### Q3: Docker 构建失败

**解决方法**：
确保 main.wasm 文件存在：
```bash
ls -la main.wasm
```

如果不存在，先运行 `./build.sh` 生成。

### Q4: 插件在 Higress 中无法加载

**排查步骤**：
1. 检查镜像是否正确构建
2. 查看 Higress 日志：`kubectl logs -n higress-system deployment/higress-controller`
3. 验证 wasm 文件路径：应该在根目录，名为 `plugin.wasm`

## 技术细节

### Go 1.24 vs TinyGo 对比

| 特性 | Go 1.24 Native | TinyGo |
|------|----------------|--------|
| **官方支持** | ✅ Higress 官方推荐 | ⚠️ 旧方案 |
| **工具链** | 标准 Go 编译器 | 需要单独安装 |
| **性能** | 优秀 | 良好 |
| **兼容性** | 100% | 100% |
| **文件大小** | 较小 | 较大 |
| **编译速度** | 快 | 较慢 |

### 代码结构要求

Go 1.24 原生编译要求：
```go
package main

func main() {}  // main 函数必须为空

func init() {
    // 所有初始化逻辑放在 init 函数中
    wrapper.SetCtx(...)
}
```

**本插件已经符合此要求**，无需修改代码。

## 滚回方案

如果升级后遇到问题，可以快速回滚：

```bash
# 方法 1: 使用 TinyGo 编译
BUILD_MODE=tinygo ./build.sh
docker build -t consumer-group-mapping:1.0.0-tinygo .

# 方法 2: 从 Git 恢复
git checkout HEAD~1 go.mod build.sh Dockerfile
```

## 相关资源

- [Higress 自定义插件开发](https://higress.cn/docs/latest/plugins/custom/)
- [Go 1.24 WASM 支持文档](https://go.dev/wiki/wasm)
- [Wasm 插件镜像规范](https://higress.cn/docs/latest/user/wasm-image-spec/)

## 技术支持

如有问题，请通过以下方式获取帮助：
- GitHub Issues: [alibaba/higress](https://github.com/alibaba/higress/issues)
- 钉钉群：34846887
- 邮件：higress@googlegroups.com
