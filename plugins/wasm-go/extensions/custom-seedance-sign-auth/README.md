# Custom Sign Auth Plugin

## 功能说明

custom-sign-auth 插件用于为转发的请求添加自定义签名头。该插件实现了特定的签名算法，自动为每个请求生成以下请求头：

- `X-Access-Key`: Access Key（从配置中读取）
- `X-Access-Timestamp`: 秒级 Unix 时间戳（每次请求自动生成）
- `X-Access-Signature`: HMAC-SHA256 签名（根据时间戳和请求体动态生成）
- `Content-Type`: application/json（如果请求体不为空且未设置 Content-Type）

## 签名算法

签名生成规则：

```
signature = HMAC-SHA256(secret_key, timestamp + "\n" + body)
```

其中：
- `secret_key`: 配置中的 Secret Key
- `timestamp`: 当前 Unix 时间戳（秒级）
- `body`: 请求体的字符串内容

## 配置参数

| 参数名 | 类型 | 必填 | 默认值 | 说明 |
|--------|------|------|--------|------|
| `access_key` | string | 是 | - | 用于签名的 Access Key，将设置为 X-Access-Key 请求头的值 |
| `secret_key` | string | 是 | - | 用于生成签名的 Secret Key |
| `enabled` | bool | 否 | true | 是否启用签名功能 |
| `override_existing` | bool | 否 | true | 如果请求中已存在签名头，是否覆盖 |

## 配置示例

```json
{
  "access_key": "ak_xxx",
  "secret_key": "sk_xxx",
  "enabled": true,
  "override_existing": true
}
```

或在 YAML 格式中：

```yaml
access_key: "ak_xxx"
secret_key: "sk_xxx"
enabled: true
override_existing: true
```

## 使用场景

适用于需要自定义签名认证的第三方 API 转发场景，例如：

1. 转发到需要签名认证的视频生成 API
2. 转发到需要自定义认证头的服务
3. 其他需要 HMAC-SHA256 签名的 API 集成

## 构建和部署

### 本地构建

```bash
# 使用 Go 1.24 原生编译（推荐）
./build.sh

# 或使用 TinyGo 编译
BUILD_MODE=tinygo ./build.sh
```

### 构建 Docker 镜像

```bash
docker build -t custom-sign-auth:1.0.0 .
```

### 推送到镜像仓库

```bash
# 标记镜像
docker tag custom-sign-auth:1.0.0 swr.cn-east-3.myhuaweicloud.com/your-namespace/custom-sign-auth:1.0.0

# 推送镜像
docker push swr.cn-east-3.myhuaweicloud.com/your-namespace/custom-sign-auth:1.0.0
```

### 使用构建脚本（推荐）

使用 extensions 目录下的统一构建脚本：

```bash
cd /Users/xiaodian/IdeaProjects/higress/plugins/wasm-go/extensions
./build-and-push-plugin.sh custom-sign-auth
```

## 测试签名算法

可以使用以下 Go 代码验证签名算法的正确性：

```go
package main

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"time"
)

func main() {
	secretKey := "sk_xxx"
	payload := map[string]interface{}{
		"model":    "video_model_v1",
		"prompt":   "生成一个简短视频",
		"duration": 5.0,
	}
	bodyBytes, _ := json.Marshal(payload)
	body := string(bodyBytes)
	timestamp := fmt.Sprintf("%d", time.Now().Unix())
	signText := timestamp + "\n" + body

	mac := hmac.New(sha256.New, []byte(secretKey))
	mac.Write([]byte(signText))
	signature := hex.EncodeToString(mac.Sum(nil))
	fmt.Println(signature)
}
```

## 注意事项

1. 签名头会在请求体读取后添加，确保插件在请求体处理阶段执行
2. 时间戳使用秒级 Unix 时间戳
3. 签名计算包含完整的请求体内容
4. 建议在 Higress Console 中启用详细日志以便调试
