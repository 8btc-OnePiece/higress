# Volcengine Sign Auth Plugin

## 功能说明

volcengine-sign-auth 插件用于为转发的请求添加火山引擎签名 v4 认证头。该插件实现了火山引擎的签名算法 v4（类似 AWS Signature v4），自动为每个请求生成以下请求头：

- `X-Date`: UTC 时间戳，格式为 YYYYMMDDTHHMMSSZ
- `X-Content-Sha256`: 请求体的 SHA256 哈希值（十六进制）
- `Authorization`: HMAC-SHA256 签名认证头
- `Content-Type`: application/json; charset=utf-8（如果请求体不为空且未设置 Content-Type）

## 签名算法

该插件实现了火山引擎签名 v4 算法，主要步骤如下：

1. **构造规范请求（Canonical Request）**：
   - HTTP 方法（POST）
   - 请求路径
   - 规范查询字符串（URL 编码并按 key 排序）
   - 规范请求头（content-type; host; x-content-sha256; x-date）
   - 签名字符串（请求体的 SHA256）

2. **构造待签名字符串（String to Sign）**：
   ```
   HMAC-SHA256
   X-Date
   CredentialScope
   HashedCanonicalRequest
   ```

3. **计算签名**：
   - 派生签名密钥：k_date -> k_region -> k_service -> k_signing
   - 使用 HMAC-SHA256 计算最终签名

4. **构造 Authorization 头**：
   ```
   HMAC-SHA256 Credential={AccessKeyId}/{CredentialScope}, SignedHeaders={SignedHeaders}, Signature={Signature}
   ```

## 配置参数

| 参数名 | 类型 | 必填 | 默认值 | 说明 |
|--------|------|------|--------|------|
| `access_key_id` | string | 是 | - | 火山引擎 Access Key ID |
| `secret_access_key` | string | 是 | - | 火山引擎 Secret Access Key |
| `region` | string | 否 | ap-southeast-1 | 服务区域，支持 ap-southeast-1、cn-beijing 等 |
| `service` | string | 否 | ark | 服务名，默认为 ark |
| `enabled` | bool | 否 | true | 是否启用签名功能 |
| `override_existing` | bool | 否 | true | 如果请求中已存在签名头，是否覆盖 |

## 配置示例

```json
{
  "access_key_id": "AKLTXXXXXXXXXXXXXXXX",
  "secret_access_key": "XXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXX",
  "region": "ap-southeast-1",
  "service": "ark",
  "enabled": true,
  "override_existing": true
}
```

或在 YAML 格式中：

```yaml
access_key_id: "AKLTXXXXXXXXXXXXXXXX"
secret_access_key: "XXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXX"
region: "ap-southeast-1"
service: "ark"
enabled: true
override_existing: true
```

## 使用场景

适用于需要火山引擎签名 v4 认证的 API 转发场景：

1. 转发到火山引擎 ARK 服务（openai.pixmax.ai）
2. 转发到需要火山引擎认证的其他服务
3. 与火山引擎的 AI 服务、媒体处理服务等集成

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
docker build -t volcengine-sign-auth:1.0.0 .
```

### 推送到镜像仓库

```bash
# 标记镜像
docker tag volcengine-sign-auth:1.0.0 swr.cn-east-3.myhuaweicloud.com/your-namespace/volcengine-sign-auth:1.0.0

# 推送镜像
docker push swr.cn-east-3.myhuaweicloud.com/your-namespace/volcengine-sign-auth:1.0.0
```

### 使用构建脚本（推荐）

使用 extensions 目录下的统一构建脚本：

```bash
cd /Users/xiaodian/IdeaProjects/higress/plugins/wasm-go/extensions
./build-and-push-plugin.sh volcengine-sign-auth
```

## 测试签名算法

可以使用以下 Go 代码验证签名算法的正确性：

```go
package main

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"time"
)

func main() {
	secretAccessKey := "XXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXX"
	accessKeyID := "AKLTXXXXXXXXXXXXXXXX"
	region := "ap-southeast-1"
	service := "ark"

	xDate := time.Now().UTC().Format("20060102T150405Z")
	body := `{"model":"gpt-4","messages":[{"role":"user","content":"Hello"}]}`

	// 计算请求体 SHA256
	hash := sha256.Sum256([]byte(body))
	xContentSha256 := hex.EncodeToString(hash[:])

	// 派生签名密钥
	kDate := hmacSha256([]byte(secretAccessKey), xDate[:8])
	kRegion := hmacSha256(kDate, region)
	kService := hmacSha256(kRegion, service)
	kSigning := hmacSha256(kService, "request")

	// 构造规范请求
	canonicalRequest := fmt.Sprintf(
		"POST\n/ai-api/volcengine/openapi/\nAction=CreateChatCompletion&Version=2024-01-01\n"+
		"content-type:application/json; charset=utf-8\n"+
		"host:openai.pixmax.ai\n"+
		"x-content-sha256:%s\n"+
		"x-date:%s\n\n"+
		"content-type;host;x-content-sha256;x-date\n%s",
		xContentSha256, xDate, xContentSha256,
	)

	// 计算签名
	hashedCanonicalRequest := sha256.Sum256([]byte(canonicalRequest))
	credentialScope := fmt.Sprintf("%s/%s/%s/request", xDate[:8], region, service)
	stringToSign := fmt.Sprintf("HMAC-SHA256\n%s\n%s\n%s", xDate, credentialScope, hex.EncodeToString(hashedCanonicalRequest[:]))
	signature := hmacSha256Hex(kSigning, stringToSign)

	authorization := fmt.Sprintf("HMAC-SHA256 Credential=%s/%s, SignedHeaders=content-type;host;x-content-sha256;x-date, Signature=%s", accessKeyID, credentialScope, signature)

	fmt.Println("Authorization:", authorization)
}

func hmacSha256(key []byte, content string) []byte {
	mac := hmac.New(sha256.New, key)
	mac.Write([]byte(content))
	return mac.Sum(nil)
}

func hmacSha256Hex(key []byte, content string) string {
	mac := hmac.New(sha256.New, key)
	mac.Write([]byte(content))
	return hex.EncodeToString(mac.Sum(nil))
}
```

## 注意事项

1. 签名头会在请求体读取后添加，确保插件在请求体处理阶段执行
2. X-Date 使用 UTC 时间，格式为 YYYYMMDDTHHMMSSZ
3. 签名计算包含完整的请求体内容，确保请求体在签名过程中不变
4. 建议在 Higress Console 中启用详细日志以便调试
5. 该插件完全兼容火山引擎的签名 v4 规范，可用于调用火山引擎的各种 API 服务
