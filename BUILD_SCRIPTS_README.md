# Higress 插件构建脚本使用指南

本项目提供了两个脚本用于构建和推送 Higress WASM 插件到华为云 SWR。

## 📁 脚本文件

### 1. `build-and-push-plugin.sh` (完整版)
功能齐全的脚本，包含详细的步骤提示、错误检查和用户交互。

**特点:**
- ✅ 详细的步骤提示和进度显示
- ✅ 完整的错误检查和处理
- ✅ 彩色输出，易于阅读
- ✅ 执行后询问是否删除本地镜像
- ✅ 详细的错误信息

**使用方法:**
```bash
# 基本使用
./build-and-push-plugin.sh <插件名>

# 指定自定义 Dockerfile
./build-and-push-plugin.sh <插件名> <Dockerfile路径>

# 示例
./build-and-push-plugin.sh my-wasm-plugin
./build-and-push-plugin.sh my-wasm-plugin ./Dockerfile.custom
```

### 2. `quick-build.sh` (快速版)
简洁版本，适合快速开发和测试。

**特点:**
- ⚡ 简洁快速，无多余输出
- ⚡ 自动执行所有步骤
- ⚡ 适合频繁的开发迭代

**使用方法:**
```bash
# 基本使用
./quick-build.sh <插件名>

# 指定自定义 Dockerfile
./quick-build.sh <插件名> <Dockerfile路径>

# 示例
./quick-build.sh my-wasm-plugin
./quick-build.sh my-wasm-plugin ./Dockerfile.custom
```

## 🔧 工作流程

两个脚本都执行以下步骤：

1. **清理旧的 WASM 文件** - 删除 `main.wasm`
2. **执行构建脚本** - 运行 `./build.sh` 编译 WASM
3. **构建 Docker 镜像** - 使用当前目录的 Dockerfile
4. **标记镜像** - 为镜像打上 SWR 仓库标签
5. **推送到 SWR** - 推送到华为云容器镜像服务

## 📦 生成的镜像格式

**本地镜像:**
```
<插件名>:<时间戳>
```

**华为云 SWR 镜像:**
```
swr.cn-east-3.myhuaweicloud.com/tob/higress-<插件名>:<时间戳>
```

**示例:**
```
本地: my-wasm-plugin:20260402095123
SWR: swr.cn-east-3.myhuaweicloud.com/tob/higress-my-wasm-plugin:20260402095123
```

## 📋 前置要求

### 必需文件:
- ✅ `build.sh` - WASM 编译脚本
- ✅ `Dockerfile` - Docker 镜像构建文件

### 必需工具:
- ✅ Docker
- ✅ 华为云 SWR 访问权限 (已登录)

### Docker 登录华为云 SWR:
```bash
docker login swr.cn-east-3.myhuaweicloud.com
# 输入华为云账号和密码
```

## 🚀 使用示例

### 示例 1: 构建标准插件
```bash
cd /path/to/your/plugin
./build-and-push-plugin.sh auth-plugin
```

**输出:**
```
[INFO] ======================================
[INFO] Higress 插件构建和推送脚本
[INFO] ======================================
[INFO] 插件名称: auth-plugin
[INFO] 版本号: 20260402095123
[INFO] Dockerfile: ./Dockerfile
[INFO] SWR 仓库: swr.cn-east-3.myhuaweicloud.com/tob/higress-auth-plugin:20260402095123
[INFO] ======================================
[INFO] 步骤 1: 清理旧的 WASM 文件...
[SUCCESS] 已删除旧的 main.wasm
[INFO] 步骤 2: 检查构建脚本...
[INFO] 步骤 3: 执行构建脚本...
[SUCCESS] 构建脚本执行成功
[INFO] 步骤 4: 检查 WASM 文件...
[SUCCESS] main.wasm 文件已生成
[INFO] 步骤 5: 检查 Dockerfile...
[INFO] 步骤 6: 构建 Docker 镜像...
[SUCCESS] Docker 镜像构建成功: auth-plugin:20260402095123
[INFO] 步骤 7: 标记镜像为 SWR 格式...
[SUCCESS] 镜像标记成功: swr.cn-east-3.myhuaweicloud.com/tob/higress-auth-plugin:20260402095123
[INFO] 步骤 8: 推送镜像到华为云 SWR...
[SUCCESS] 镜像推送成功!
[SUCCESS] 构建和推送完成！
[INFO] ======================================
[SUCCESS] 本地镜像: auth-plugin:20260402095123
[SUCCESS] SWR 镜像: swr.cn-east-3.myhuaweicloud.com/tob/higress-auth-plugin:20260402095123
[INFO] 使用方法:
[INFO]   在 Higress 中配置插件时使用: swr.cn-east-3.myhuaweicloud.com/tob/higress-auth-plugin:20260402095123
[INFO] ======================================
```

### 示例 2: 使用快速版本
```bash
./quick-build.sh rate-limit-plugin
```

**输出:**
```
🚀 开始构建 Higress 插件: rate-limit-plugin
📅 版本号: 20260402095123
🧹 清理旧的 WASM 文件...
🔨 执行构建脚本...
🐳 构建 Docker 镜像...
🏷️  标记镜像...
📤 推送到华为云 SWR...
✅ 完成！
📦 本地镜像: rate-limit-plugin:20260402095123
🌐 SWR 镜像: swr.cn-east-3.myhuaweicloud.com/tob/higress-rate-limit-plugin:20260402095123

💡 在 Higress 中使用: swr.cn-east-3.myhuaweicloud.com/tob/higress-rate-limit-plugin:20260402095123
```

### 示例 3: 使用自定义 Dockerfile
```bash
./build-and-push-plugin.sh custom-plugin ./Dockerfile.production
```

## 🎯 在 Higress 中使用

1. **登录 Higress 控制台**
2. **进入插件管理**
3. **创建或编辑插件**
4. **镜像地址填写:**
   ```
   swr.cn-east-3.myhuaweicloud.com/tob/higress-<插件名>:<时间戳>
   ```
5. **配置插件参数**
6. **启用插件**

## ⚠️ 注意事项

1. **华为云 SWR 认证:**
   - 确保已经登录到华为云 SWR
   - 有推送权限到 `tob` 命名空间

2. **网络连接:**
   - 确保能够访问华为云 SWR (`swr.cn-east-3.myhuaweicloud.com`)
   - Docker 镜像推送可能需要较长时间

3. **时间戳格式:**
   - 格式: `YYYYMMDDHHmmss`
   - 示例: `20260402095123`

4. **插件命名:**
   - 避免使用特殊字符
   - 建议使用小写字母和连字符

5. **清理本地镜像:**
   - 完整版脚本会询问是否删除本地镜像
   - 可以节省磁盘空间

## 🔍 故障排除

### 问题 1: Docker 登录失败
```bash
# 解决方案：重新登录
docker logout swr.cn-east-3.myhuaweicloud.com
docker login swr.cn-east-3.myhuaweicloud.com
```

### 问题 2: build.sh 不存在
```bash
# 解决方案：确保在正确的目录
ls -la build.sh
pwd
```

### 问题 3: WASM 文件未生成
```bash
# 解决方案：检查构建脚本输出
./build.sh
ls -la main.wasm
```

### 问题 4: Docker 构建失败
```bash
# 解决方案：检查 Dockerfile 语法
docker build -t test:latest -f ./Dockerfile .
```

## 📞 支持

如有问题，请检查：
1. Docker 是否正常运行
2. 网络连接是否正常
3. 华为云 SWR 权限是否正确
4. 构建脚本和 Dockerfile 是否正确

## 🔄 版本历史

- **v1.0** - 初始版本
  - 完整版脚本 `build-and-push-plugin.sh`
  - 快速版脚本 `quick-build.sh`
