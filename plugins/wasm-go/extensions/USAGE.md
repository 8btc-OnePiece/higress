# Higress 插件构建脚本使用说明

## 🎯 最简单的使用方法

### 从 extensions 目录执行（推荐）

```bash
cd /Users/xiaodian/IdeaProjects/higress/plugins/wasm-go/extensions
./build-and-push-plugin.sh consumer-group-mapping
```

就这么简单！脚本会自动：
1. 在当前目录下查找 `consumer-group-mapping` 子目录
2. 执行构建（如果有 `build.sh`）或使用现有的 `main.wasm`
3. 构建 Docker 镜像
4. 推送到华为云 SWR

## 📋 其他使用方式

### 使用快速版脚本

```bash
./quick-build.sh consumer-group-mapping
```

### 从插件目录内执行

```bash
cd consumer-group-mapping
../build-and-push-plugin.sh consumer-group-mapping .
```

### 显式指定插件目录

```bash
./build-and-push-plugin.sh consumer-group-mapping ./consumer-group-mapping
```

## 🎁 生成的镜像

**本地镜像**: `consumer-group-mapping:20260402125749`
**SWR 镜像**: `swr.cn-east-3.myhuaweicloud.com/tob/higress-consumer-group-mapping:20260402125749`

在 Higress 中使用 SWR 镜像地址即可。

## ✨ 脚本特性

- ✅ **自动查找插件目录**: 从 `extensions` 目录执行时，自动查找子目录
- ✅ **灵活构建**: 支持 `build.sh` 编译或使用现有 `main.wasm`
- ✅ **自动标记**: 自动为镜像打上 SWR 标签
- ✅ **自动推送**: 自动推送到华为云 SWR

## 🔧 故障排除

### 问题: 找不到插件目录

确保您在正确的目录下执行脚本：
```bash
pwd
# 应该显示: /Users/xiaodian/IdeaProjects/higress/plugins/wasm-go/extensions
```

### 问题: Docker 登录失败

重新登录华为云 SWR：
```bash
docker login swr.cn-east-3.myhuaweicloud.com
```

## 📞 更多信息

详细文档请查看 `BUILD_SCRIPTS_README.md`
