# Higress WASM 插件构建脚本

## 📝 概述

本目录包含两个脚本用于构建和推送 Higress WASM 插件到华为云 SWR。

## 🚀 快速开始

### 方法 1: 使用快速版脚本（推荐）

```bash
# 从 extensions 目录执行（自动查找子目录）
./quick-build.sh consumer-group-mapping

# 从插件目录内执行
cd consumer-group-mapping
../quick-build.sh consumer-group-mapping .

# 指定插件目录路径
./quick-build.sh consumer-group-mapping ./consumer-group-mapping
```

### 方法 2: 使用完整版脚本

```bash
# 从 extensions 目录执行（自动查找子目录）
./build-and-push-plugin.sh consumer-group-mapping

# 从插件目录内执行
cd consumer-group-mapping
../build-and-push-plugin.sh consumer-group-mapping .

# 指定插件目录路径
./build-and-push-plugin.sh consumer-group-mapping ./consumer-group-mapping
```

### 💡 使用提示

- **默认行为**：当从 `extensions` 目录执行时，脚本会自动在当前目录下查找名为 `<插件名>` 的子目录
- **灵活指定**：您也可以显式指定插件目录路径
- **从插件内执行**：在插件目录内执行时，使用 `.` 作为插件目录路径

## 📋 脚本功能

两个脚本都自动执行以下步骤：

1. ✅ **清理旧的 WASM 文件**
2. ✅ **执行构建脚本** (如果有 `build.sh`)
3. ✅ **检查 WASM 文件** (如果没有 `build.sh`，使用现有的 `main.wasm`)
4. ✅ **构建 Docker 镜像**
5. ✅ **标记镜像** 为 SWR 格式
6. ✅ **推送到华为云 SWR**

## 💡 使用场景

### 场景 1: 从 extensions 目录执行（最简单）

```bash
cd /path/to/higress/plugins/wasm-go/extensions
./build-and-push-plugin.sh consumer-group-mapping
```

**说明**: 脚本会自动在当前目录下查找 `consumer-group-mapping` 子目录

### 场景 2: 从插件目录内执行

```bash
cd /path/to/higress/plugins/wasm-go/extensions/consumer-group-mapping
../build-and-push-plugin.sh consumer-group-mapping .
```

**说明**: 使用 `.` 表示当前目录作为插件目录

### 场景 3: 使用现有 main.wasm（无需重新编译）

```bash
cd /path/to/higress/plugins/wasm-go/extensions
./build-and-push-plugin.sh consumer-group-mapping
```

**说明**: 脚本会自动检测，如果没有 `build.sh`，则使用现有的 `main.wasm`

## 📦 镜像格式

**本地镜像:** `<插件名>:<时间戳>`
**SWR 镜像:** `swr.cn-east-3.myhuaweicloud.com/tob/higress-<插件名>:<时间戳>`

**示例:**
```
本地: consumer-group-mapping:20260402115130
SWR: swr.cn-east-3.myhuaweicloud.com/tob/higress-consumer-group-mapping:20260402115130
```

## 🎯 在 Higress 中使用

1. 打开 Higress 控制台
2. 进入插件管理
3. 创建或编辑插件
4. 镜像地址填写: `swr.cn-east-3.myhuaweicloud.com/tob/higress-<插件名>:<时间戳>`
5. 配置插件参数
6. 启用插件

## ⚠️ 注意事项

1. **华为云 SWR 认证:**
   ```bash
   docker login swr.cn-east-3.myhuaweicloud.com
   ```

2. **脚本灵活性:**
   - 支持从任何目录执行
   - 自动检测插件目录
   - 支持使用现有 `main.wasm` 或重新编译

3. **错误处理:**
   - 完整版脚本有详细的错误检查
   - 快速版脚本简洁高效

## 🔧 前置要求

- ✅ Docker
- ✅ 华为云 SWR 访问权限
- ✅ 插件目录包含 `build.sh` 或 `main.wasm`
- ✅ 插件目录包含 `Dockerfile`

## 📂 目录结构示例

```
extensions/
├── build-and-push-plugin.sh    # 完整版脚本
├── quick-build.sh               # 快速版脚本
├── consumer-group-mapping/
│   ├── build.sh                 # 构建脚本
│   ├── main.wasm                # WASM 文件
│   ├── Dockerfile               # Docker 镜像文件
│   └── main.go                  # Go 源代码
└── other-plugin/
    ├── build.sh
    ├── main.wasm
    ├── Dockerfile
    └── main.go
```

## 📞 故障排除

### 问题: 找不到 build.sh

**解决方案:** 脚本会自动使用现有的 `main.wasm` 文件，无需重新编译。

### 问题: 找不到插件目录

**解决方案:** 指定正确的插件目录路径：
```bash
./build-and-push-plugin.sh plugin-name /full/path/to/plugin
```

### 问题: Docker 推送失败

**解决方案:** 检查华为云 SWR 认证：
```bash
docker login swr.cn-east-3.myhuaweicloud.com
```

## 🔄 更新日志

### v2.0 (最新)
- ✅ 支持从任何目录执行
- ✅ 自动检测插件目录
- ✅ 支持使用现有 main.wasm 文件
- ✅ 改进的错误处理

### v1.0
- 初始版本
