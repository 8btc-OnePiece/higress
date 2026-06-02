# AI Success Rate Monitor Plugin

Higress WASM 插件，用于监控 API 成功率并发送钉钉告警。

## 功能特性

- 监控 API 响应状态码
- 检测 4xx 和 5xx 错误
- 提取详细的请求信息（URI、Model、Provider、API Key、错误消息）
- 发送钉钉机器人告警
- 支持 Markdown 和文本两种消息格式
- 支持 @ 功能

## 配置说明

```json
{
  "dingTalkWebhook": "钉钉机器人 Webhook URL",
  "enableAlert": true,
  "alertLevelFor4xx": "warning",
  "alertLevelFor5xx": "error",
  "includeRequestBody": false,
  "messageType": "markdown",
  "atAll": false,
  "atMobiles": []
}
```

### 配置项说明

| 配置项 | 类型 | 说明 | 默认值 |
|--------|------|------|--------|
| dingTalkWebhook | string | 钉钉机器人 Webhook URL | 必填 |
| enableAlert | bool | 是否启用告警 | true |
| alertLevelFor4xx | string | 4xx 错误告警级别 (info/warning/error/critical) | warning |
| alertLevelFor5xx | string | 5xx 错误告警级别 | error |
| includeRequestBody | bool | 是否在告警中包含请求体 | false |
| messageType | string | 消息类型 (markdown/text) | markdown |
| atAll | bool | 是否 @所有人 | false |
| atMobiles | array | @的手机号列表 | [] |

## 告警信息

告警消息包含以下信息：

- **接口URI**: 请求的路径
- **请求Model**: AI 模型名称
- **Model Provider**: 服务提供商
- **API Key**: API 密钥（已掩码）
- **响应HTTPCode**: HTTP 状态码
- **响应Msg**: 错误消息

## 构建

```bash
./build.sh
```

## 部署

1. 将构建好的 `main.wasm` 上传到 Higress
2. 在路由配置中添加该插件
3. 配置钉钉 Webhook URL
