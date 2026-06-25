package main

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/higress-group/proxy-wasm-go-sdk/proxywasm"
	"github.com/higress-group/proxy-wasm-go-sdk/proxywasm/types"
	"github.com/higress-group/wasm-go/pkg/log"
	"github.com/higress-group/wasm-go/pkg/wrapper"
	"github.com/tidwall/gjson"
)

const (
	pluginName = "ai-success-rate-monitor"

	// 上下文键
	ctxKeyURI          = "uri"
	ctxKeyMethod       = "method"
	ctxKeyModel        = "model"
	ctxKeyProvider     = "provider"
	ctxKeyAPIKey       = "apiKey"
	ctxKeyRequestBody  = "requestBody"
	ctxKeyQueryParams  = "queryParams"
	ctxKeyStatusCode   = "statusCode"

	// HTTP 头
	OriginalAPIKey = "X-Original-Api-Key"

	// 告警级别
	alertLevelInfo     = "info"
	alertLevelWarning  = "warning"
	alertLevelError    = "error"
	alertLevelCritical = "critical"
)

var (
	pluginConfig PluginConfig
)

// PluginConfig 插件配置
type PluginConfig struct {
	// 钉钉机器人 Webhook URL
	DingTalkWebhook string `json:"dingTalkWebhook"`

	// 钉钉服务名称（用于集群客户端调用）
	DingTalkServiceName string `json:"dingTalkServiceName"`

	// 是否启用告警
	EnableAlert bool `json:"enableAlert"`

	// 告警级别 (4xx=warning, 5xx=error)
	AlertLevelFor4xx string `json:"alertLevelFor4xx"`
	AlertLevelFor5xx string `json:"alertLevelFor5xx"`

	// 钉钉消息类型 (text, markdown)
	MessageType string `json:"messageType"`

	// 是否发送 atAll（@所有人）
	AtAll bool `json:"atAll"`

	// @的手机号列表
	AtMobiles []string `json:"atMobiles"`

	// 忽略的状态码列表（这些状态码不会触发告警）
	IgnoreStatusCodes []int `json:"ignoreStatusCodes"`

	// HTTP 客户端（用于发送钉钉告警）
	dingTalkClient wrapper.HttpClient
}

// DingTalkMessage 钉钉消息结构
type DingTalkMessage struct {
	MsgType string `json:"msgtype"`
	Text    *struct {
		Content string   `json:"content"`
		AtList  []string `json:"atMobiles"`
		AtAll   bool     `json:"isAtAll"`
	} `json:"text,omitempty"`
	Markdown *struct {
		Title string `json:"title"`
		Text  string `json:"text"`
	} `json:"markdown,omitempty"`
	At *struct {
		AtMobiles []string `json:"atMobiles"`
		IsAtAll   bool     `json:"isAtAll"`
	} `json:"at,omitempty"`
}

// AlertInfo 告警信息
type AlertInfo struct {
	RequestID    string
	URI          string
	Method       string
	Model        string
	Provider     string
	APIKey       string
	HTTPCode     int
	ErrorMessage string
	RequestBody  string // 请求 body
	QueryParams  string // GET 请求参数
	ResponseBody string // 响应原报文
	Timestamp    string
	AlertLevel   string
}

func main() {}

func init() {
	wrapper.SetCtx(
		pluginName,
		wrapper.ParseConfigBy(parseConfig),
		wrapper.ProcessRequestHeadersBy(onHttpRequestHeaders),
		wrapper.ProcessRequestBody(onHttpRequestBody),
		wrapper.ProcessResponseHeadersBy(onHttpResponseHeaders),
		wrapper.ProcessResponseBodyBy(onHttpResponseBody),
	)
}

// parseConfig 解析插件配置
func parseConfig(json gjson.Result, config *PluginConfig, log log.Log) error {
	if log != nil {
		log.Infof("parsing config: %s", json.String())
	}

	// 解析钉钉 Webhook
	config.DingTalkWebhook = json.Get("dingTalkWebhook").String()

	// 解析钉钉服务名称
	config.DingTalkServiceName = json.Get("dingTalkServiceName").String()
	if config.DingTalkServiceName == "" {
		config.DingTalkServiceName = "outbound" // 默认使用 outbound
	}

	// 解析启用标志
	config.EnableAlert = json.Get("enableAlert").Bool()

	// 解析告警级别
	config.AlertLevelFor4xx = json.Get("alertLevelFor4xx").String()
	if config.AlertLevelFor4xx == "" {
		config.AlertLevelFor4xx = alertLevelWarning // 默认 warning
	}

	config.AlertLevelFor5xx = json.Get("alertLevelFor5xx").String()
	if config.AlertLevelFor5xx == "" {
		config.AlertLevelFor5xx = alertLevelError // 默认 error
	}

	// 解析其他配置
	config.MessageType = json.Get("messageType").String()
	if config.MessageType == "" {
		config.MessageType = "markdown" // 默认 markdown
	}

	config.AtAll = json.Get("atAll").Bool()

	// 解析 @手机号列表
	atMobilesJson := json.Get("atMobiles")
	if atMobilesJson.Exists() && atMobilesJson.IsArray() {
		config.AtMobiles = make([]string, 0)
		for _, mobile := range atMobilesJson.Array() {
			config.AtMobiles = append(config.AtMobiles, mobile.String())
		}
	}

	// 解析忽略的状态码列表
	ignoreStatusCodesJson := json.Get("ignoreStatusCodes")
	if ignoreStatusCodesJson.Exists() && ignoreStatusCodesJson.IsArray() {
		config.IgnoreStatusCodes = make([]int, 0)
		for _, code := range ignoreStatusCodesJson.Array() {
			if code.Int() > 0 {
				config.IgnoreStatusCodes = append(config.IgnoreStatusCodes, int(code.Int()))
			}
		}
	}

	// 如果配置了钉钉 Webhook，创建 HTTP 客户端
	if config.DingTalkWebhook != "" {
		// 解析 URL 获取主机和端口
		parsedUrl, err := url.Parse(config.DingTalkWebhook)
		if err != nil {
			log.Errorf("failed to parse dingtalk webhook url: %v", err)
			return fmt.Errorf("invalid dingTalkWebhook: %v", err)
		}

		actualHost := parsedUrl.Hostname()
		if actualHost == "" {
			return fmt.Errorf("invalid dingTalkWebhook: missing hostname")
		}

		port := int64(443) // 钉钉使用 HTTPS
		if actualPort := parsedUrl.Port(); actualPort != "" {
			if p, err := strconv.Atoi(actualPort); err == nil {
				port = int64(p)
			}
		}

		// 创建 HTTP 客户端用于发送钉钉告警
		config.dingTalkClient = wrapper.NewClusterClient(wrapper.DnsCluster{
			ServiceName: config.DingTalkServiceName,
			Domain:      actualHost,
			Port:        port,
		})

		log.Infof("created dingtalk client: service=%s, domain=%s, port=%d",
			config.DingTalkServiceName, actualHost, port)
	}

	pluginConfig = *config

	if log != nil {
		log.Infof("config loaded: enableAlert=%v, webhook=%s, messageType=%s",
			config.EnableAlert, config.DingTalkWebhook, config.MessageType)
	}

	return nil
}

// onHttpRequestHeaders 处理请求头
func onHttpRequestHeaders(ctx wrapper.HttpContext, config PluginConfig, log log.Log) types.Action {
	ctx.DisableReroute()

	// 获取请求 URI
	uri, err := proxywasm.GetHttpRequestHeader(":path")
	if err != nil {
		log.Errorf("failed to get request path: %v", err)
		return types.ActionContinue
	}
	ctx.SetContext(ctxKeyURI, uri)

	// 获取请求 method
	if method, err := proxywasm.GetHttpRequestHeader(":method"); err == nil && method != "" {
		ctx.SetContext(ctxKeyMethod, method)
	}

	// 提取 GET 请求参数（:path 中的 query string 部分）
	if idx := strings.Index(uri, "?"); idx != -1 && idx+1 < len(uri) {
		ctx.SetContext(ctxKeyQueryParams, uri[idx+1:])
	}

	// 获取模型名称
	model := getModelFromRequest(ctx)
	if model != "" {
		ctx.SetContext(ctxKeyModel, model)
		log.Debugf("got model: %s", model)
	}

	// 获取 API Key
	apiKey, err := proxywasm.GetHttpRequestHeader(OriginalAPIKey)
	if err == nil && apiKey != "" {
		ctx.SetContext(OriginalAPIKey, apiKey)
		log.Debugf("got api key: %s", maskAPIKey(apiKey))
	}

	// 获取 Provider (从 cluster_name)
	provider := getClusterName(ctx)
	if provider != "" {
		ctx.SetContext(ctxKeyProvider, provider)
		log.Debugf("got provider: %s", provider)
	}


	return types.ActionContinue
}

// onHttpRequestBody 获取请求 body
func onHttpRequestBody(ctx wrapper.HttpContext, config PluginConfig, body []byte) types.Action {
	if len(body) > 0 {
		// 限制请求 body 大小（10KB）
		maxBodySize := 10 * 1024
		if len(body) > maxBodySize {
			body = body[:maxBodySize]
		}
		ctx.SetContext(ctxKeyRequestBody, string(body))
	}
	return types.ActionContinue
}

// onHttpResponseHeaders 处理响应头
func onHttpResponseHeaders(ctx wrapper.HttpContext, config PluginConfig, log log.Log) types.Action {
	if !wrapper.IsResponseFromUpstream() {
		ctx.DontReadResponseBody()
		return types.ActionContinue
	}

	// 检查 HTTP 状态码
	statusCodeStr, err := proxywasm.GetHttpResponseHeader(":status")
	if err != nil {
		// 在某些情况下（如本地响应），:status 可能不可用，这是正常的
		log.Debugf("failed to get response status code: %v", err)
		ctx.DontReadResponseBody()
		return types.ActionContinue
	}

	statusCode, err := strconv.Atoi(statusCodeStr)
	if err != nil {
		log.Debugf("failed to parse status code: %v, value: %q", err, statusCodeStr)
		ctx.DontReadResponseBody()
		return types.ActionContinue
	}

	// 将状态码存入上下文，供 onHttpResponseBody 使用
	ctx.SetContext(ctxKeyStatusCode, statusCode)

	// 如果是 4xx 或 5xx，且配置了忽略列表，检查是否应跳过
	if isErrorCode(statusCode) && config.EnableAlert {
		if shouldIgnoreStatusCode(statusCode, config.IgnoreStatusCodes) {
			log.Debugf("status code %d is in ignore list, skip alert", statusCode)
			ctx.DontReadResponseBody()
			return types.ActionContinue
		}
		ctx.SetResponseBodyBufferLimit(10 * 1024) // 限制 10KB
		return types.ActionContinue
	}

	ctx.DontReadResponseBody()
	return types.ActionContinue
}

// onHttpResponseBody 处理响应体
func onHttpResponseBody(ctx wrapper.HttpContext, config PluginConfig, body []byte, log log.Log) types.Action {
	if !wrapper.IsResponseFromUpstream() {
		return types.ActionContinue
	}

	// 从上下文获取状态码（由 onHttpResponseHeaders 设置）
	statusCodeCtx := ctx.GetContext(ctxKeyStatusCode)
	if statusCodeCtx == nil {
		// 如果上下文中没有状态码，说明不需要发送告警
		return types.ActionContinue
	}

	statusCode, ok := statusCodeCtx.(int)
	if !ok {
		log.Debugf("invalid status code type in context: %T", statusCodeCtx)
		return types.ActionContinue
	}

	// 检查是否需要发送告警
	if !isErrorCode(statusCode) {
		return types.ActionContinue
	}

	if !config.EnableAlert {
		log.Debugf("alert is disabled, skip sending alert")
		return types.ActionContinue
	}

	// 构建告警信息
	alertInfo := buildAlertInfo(ctx, statusCode, body, log)

	log.Infof("triggering dingtalk alert for status code %d", statusCode)

	// 发送钉钉告警（DispatchHttpCall 本身是异步的，不需要 go 关键字）
	sendDingTalkAlert(alertInfo, config, log)

	return types.ActionContinue
}

// buildAlertInfo 构建告警信息
func buildAlertInfo(ctx wrapper.HttpContext, statusCode int, body []byte, log log.Log) AlertInfo {
	alertLevel := getAlertLevel(statusCode)
	requestID := getRequestID()

	info := AlertInfo{
		RequestID:  requestID,
		Timestamp:  time.Now().Format("2006-01-02 15:04:05"),
		HTTPCode:   statusCode,
		AlertLevel: alertLevel,
	}

	// 获取 URI
	if uri, ok := ctx.GetContext(ctxKeyURI).(string); ok {
		info.URI = uri
	}

	// 获取 Method
	if method, ok := ctx.GetContext(ctxKeyMethod).(string); ok {
		info.Method = method
	}

	// 获取 Model
	if model, ok := ctx.GetContext(ctxKeyModel).(string); ok {
		info.Model = model
	}

	// 获取 Provider
	if provider, ok := ctx.GetContext(ctxKeyProvider).(string); ok {
		info.Provider = provider
	}

	// 获取 API Key
	if apiKey, ok := ctx.GetContext(OriginalAPIKey).(string); ok {
		info.APIKey = maskAPIKey(apiKey)
	}

	// 获取请求 body
	if reqBody, ok := ctx.GetContext(ctxKeyRequestBody).(string); ok {
		info.RequestBody = reqBody
	}

	// 获取 GET 请求参数
	if queryParams, ok := ctx.GetContext(ctxKeyQueryParams).(string); ok {
		info.QueryParams = queryParams
	}

	// 获取错误消息和响应原报文
	if len(body) > 0 {
		info.ErrorMessage = extractErrorMessage(body)
		// 保存响应原报文（限制长度）
		bodyStr := string(body)
		maxBodyLength := 2000 // 限制响应报文最大长度
		if len(bodyStr) > maxBodyLength {
			info.ResponseBody = bodyStr[:maxBodyLength] + "...(truncated)"
		} else {
			info.ResponseBody = bodyStr
		}
	} else {
		info.ErrorMessage = fmt.Sprintf("HTTP %d Error", statusCode)
	}

	return info
}

// sendDingTalkAlert 发送钉钉告警
func sendDingTalkAlert(info AlertInfo, config PluginConfig, log log.Log) error {
	if config.DingTalkWebhook == "" {
		log.Errorf("dingtalk webhook is empty")
		return fmt.Errorf("dingtalk webhook is empty")
	}

	if config.dingTalkClient == nil {
		log.Errorf("dingtalk client is not initialized")
		return fmt.Errorf("dingtalk client is not initialized")
	}

	// 构建消息
	var message DingTalkMessage

	if config.MessageType == "markdown" {
		message = buildMarkdownMessage(info, config)
	} else {
		message = buildTextMessage(info, config)
	}

	// 序列化为 JSON
	jsonData, err := json.Marshal(message)
	if err != nil {
		log.Errorf("failed to marshal message: %v", err)
		return fmt.Errorf("failed to marshal message: %w", err)
	}

	// 解析请求路径
	parsedUrl, err := url.Parse(config.DingTalkWebhook)
	if err != nil {
		log.Errorf("failed to parse dingtalk webhook url: %v", err)
		return fmt.Errorf("failed to parse webhook url: %w", err)
	}

	requestPath := parsedUrl.Path
	if requestPath == "" {
		requestPath = "/"
	}
	if parsedUrl.RawQuery != "" {
		requestPath += "?" + parsedUrl.RawQuery
	}

	// 使用集群客户端发送 POST 请求
	err = config.dingTalkClient.Post(
		requestPath,
		[][2]string{{"Content-Type", "application/json"}},
		jsonData,
		func(statusCode int, responseHeaders http.Header, responseBody []byte) {
			handleDingTalkResponse(statusCode, responseBody, log)
		},
		1000, // 1秒超时
	)

	if err != nil {
		log.Errorf("failed to send dingtalk request: %v", err)
		return fmt.Errorf("failed to send dingtalk request: %w", err)
	}

	return nil
}

// handleDingTalkResponse 处理钉钉响应
func handleDingTalkResponse(statusCode int, responseBody []byte, log log.Log) {
	if statusCode >= 200 && statusCode < 300 {
		log.Infof("dingtalk alert sent successfully, status=%d", statusCode)
	} else {
		log.Errorf("dingtalk alert failed, status=%d, body=%s", statusCode, string(responseBody))
	}
}

// buildMarkdownMessage 构建 Markdown 格式消息
func buildMarkdownMessage(info AlertInfo, config PluginConfig) DingTalkMessage {
	// 确定告警图标
	levelIcon := getLevelIcon(info.AlertLevel)

	var content strings.Builder
	content.WriteString(fmt.Sprintf("### %s API调用失败告警\n\n", levelIcon))
	content.WriteString(fmt.Sprintf("**告警级别**: %s\n\n", info.AlertLevel))
	content.WriteString(fmt.Sprintf("**接口URI**: %s %s\n\n", info.Method, info.URI))
	content.WriteString(fmt.Sprintf("**请求状态码**: %d\n\n", info.HTTPCode))

	if info.RequestID != "" {
		content.WriteString(fmt.Sprintf("**请求ID**: %s\n\n", info.RequestID))
	}

	if info.Model != "" {
		content.WriteString(fmt.Sprintf("**请求Model**: %s\n\n", info.Model))
	}

	if info.Provider != "" {
		content.WriteString(fmt.Sprintf("**Model Provider**: %s\n\n", info.Provider))
	}

	if info.APIKey != "" {
		content.WriteString(fmt.Sprintf("**API Key**: %s\n\n", info.APIKey))
	}

	if info.RequestBody != "" {
		content.WriteString(fmt.Sprintf("**请求Body**:\n```\n%s\n```\n\n", info.RequestBody))
	}

	if info.QueryParams != "" {
		content.WriteString(fmt.Sprintf("**请求参数**: %s\n\n", info.QueryParams))
	}

	if info.ErrorMessage != "" {
		// 转义 Markdown 特殊字符
		escapedMsg := escapeMarkdown(info.ErrorMessage)
		content.WriteString(fmt.Sprintf("**错误信息**: %s\n\n", escapedMsg))
	}

	if info.ResponseBody != "" {
		// 转义 Markdown 特殊字符并使用代码块显示响应原报文
		escapedBody := escapeMarkdown(info.ResponseBody)
		content.WriteString(fmt.Sprintf("**响应原报文**:\n```\n%s\n```\n\n", escapedBody))
	}

	message := DingTalkMessage{
		MsgType: "markdown",
		Markdown: &struct {
			Title string `json:"title"`
			Text  string `json:"text"`
		}{
			Title: fmt.Sprintf("API失败告警 - %d", info.HTTPCode),
			Text:  content.String(),
		},
	}

	// 添加 @ 信息
	if config.AtAll || len(config.AtMobiles) > 0 {
		message.At = &struct {
			AtMobiles []string `json:"atMobiles"`
			IsAtAll   bool     `json:"isAtAll"`
		}{
			AtMobiles: config.AtMobiles,
			IsAtAll:   config.AtAll,
		}
	}

	return message
}

// buildTextMessage 构建文本格式消息
func buildTextMessage(info AlertInfo, config PluginConfig) DingTalkMessage {
	var content strings.Builder
	content.WriteString(fmt.Sprintf("【API调用失败告警】\n"))
	content.WriteString(fmt.Sprintf("告警级别: %s\n", info.AlertLevel))
	content.WriteString(fmt.Sprintf("接口URI: %s %s\n", info.Method, info.URI))
	content.WriteString(fmt.Sprintf("请求状态码: %d\n", info.HTTPCode))

	if info.RequestID != "" {
		content.WriteString(fmt.Sprintf("请求ID: %s\n", info.RequestID))
	}

	if info.Model != "" {
		content.WriteString(fmt.Sprintf("请求Model: %s\n", info.Model))
	}

	if info.Provider != "" {
		content.WriteString(fmt.Sprintf("Model Provider: %s\n", info.Provider))
	}

	if info.APIKey != "" {
		content.WriteString(fmt.Sprintf("API Key: %s\n", info.APIKey))
	}

	if info.RequestBody != "" {
		content.WriteString(fmt.Sprintf("请求Body: %s\n", info.RequestBody))
	}

	if info.QueryParams != "" {
		content.WriteString(fmt.Sprintf("请求参数: %s\n", info.QueryParams))
	}

	if info.ErrorMessage != "" {
		content.WriteString(fmt.Sprintf("错误信息: %s\n", info.ErrorMessage))
	}

	if info.ResponseBody != "" {
		content.WriteString(fmt.Sprintf("响应原报文: %s\n", info.ResponseBody))
	}

	// 添加 @ 信息
	if config.AtAll {
		content.WriteString(" @all")
	} else if len(config.AtMobiles) > 0 {
		for _, mobile := range config.AtMobiles {
			content.WriteString(fmt.Sprintf(" @%s", mobile))
		}
	}

	message := DingTalkMessage{
		MsgType: "text",
		Text: &struct {
			Content string   `json:"content"`
			AtList  []string `json:"atMobiles"`
			AtAll   bool     `json:"isAtAll"`
		}{
			Content: content.String(),
			AtAll:   config.AtAll,
		},
	}

	return message
}

// getModelFromRequest 从请求中获取模型名称
func getModelFromRequest(ctx wrapper.HttpContext) string {
	// 优先从 Envoy 属性中获取（由之前的插件设置）
	if requestModel, err := proxywasm.GetProperty([]string{"wasm.requestModel"}); err == nil {
		model := string(requestModel)
		if model != "" {
			return model
		}
	}

	return ""
}

// getClusterName 获取当前路由的服务提供者名称（cluster_name）
func getClusterName(ctx wrapper.HttpContext) string {
	if raw, err := proxywasm.GetProperty([]string{"cluster_name"}); err == nil {
		clusterName := string(raw)
		if clusterName != "" {
			// cluster_name 格式: outbound|443||llm-RightCodes.internal.dns
			// 提取服务名称部分
			parts := strings.Split(clusterName, "||")
			if len(parts) >= 2 {
				serviceName := parts[1]

				// 移除 llm- 前缀
				serviceName = strings.TrimPrefix(serviceName, "llm-")

				// 提取第一个点之前的部分
				if idx := strings.Index(serviceName, "."); idx > 0 {
					serviceName = serviceName[:idx]
				}
				return serviceName
			}
		}
	}
	return ""
}

// isErrorCode 检查是否是错误状态码
func isErrorCode(statusCode int) bool {
	return statusCode >= 400 && statusCode < 600
}

// shouldIgnoreStatusCode 检查状态码是否在忽略列表中
func shouldIgnoreStatusCode(statusCode int, ignoreList []int) bool {
	for _, code := range ignoreList {
		if code == statusCode {
			return true
		}
	}
	return false
}

// getRequestID 获取 Envoy 请求 ID
func getRequestID() string {
	// 从 x-request-id header 获取（Higress 的方式）
	if requestIDHeader, err := proxywasm.GetHttpRequestHeader("x-request-id"); err == nil {
		if requestIDHeader != "" && requestIDHeader != "-" {
			return requestIDHeader
		}
	}
	return ""
}

// getAlertLevel 根据状态码获取告警级别
func getAlertLevel(statusCode int) string {
	if statusCode >= 400 && statusCode < 500 {
		return pluginConfig.AlertLevelFor4xx
	}
	if statusCode >= 500 && statusCode < 600 {
		return pluginConfig.AlertLevelFor5xx
	}
	return alertLevelInfo
}

// getLevelIcon 获取告警级别图标
func getLevelIcon(level string) string {
	switch level {
	case alertLevelCritical:
		return "🔴"
	case alertLevelError:
		return "🟠"
	case alertLevelWarning:
		return "🟡"
	default:
		return "🟢"
	}
}

// maskAPIKey 掩码 API Key
func maskAPIKey(apiKey string) string {
	if len(apiKey) <= 8 {
		return "****"
	}
	return apiKey[:6] + "****" + apiKey[len(apiKey)-6:]
}

// maskWebhook 掩码 Webhook URL
func maskWebhook(webhook string) string {
	if webhook == "" {
		return ""
	}
	// 只显示协议和主机名，隐藏路径和 access_token
	if idx := strings.Index(webhook, "/send"); idx > 0 {
		return webhook[:idx] + "/send***"
	}
	// 如果找不到 /send，返回前 50 个字符
	if len(webhook) > 50 {
		return webhook[:50] + "***"
	}
	return webhook[:len(webhook)/2] + "***"
}

// extractErrorMessage 从响应体中提取错误消息
func extractErrorMessage(body []byte) string {
	if len(body) == 0 {
		return ""
	}

	// 限制处理的 body 大小，避免处理过大的响应体
	maxBodySize := 10 * 1024 // 10KB
	if len(body) > maxBodySize {
		body = body[:maxBodySize]
	}

	// 尝试解析 JSON
	jsonData := gjson.ParseBytes(body)

	// 尝试获取常见的错误字段
	errorFields := []string{"error.message", "message", "error", "msg", "error.msg"}
	for _, field := range errorFields {
		if value := jsonData.Get(field); value.Exists() && value.String() != "" {
			msg := value.String()
			// 限制消息长度
			if len(msg) > 1000 {
				return msg[:1000] + "...(truncated)"
			}
			return msg
		}
	}

	// 如果无法解析，返回原始内容（限制长度）
	bodyStr := string(body)
	if len(bodyStr) > 1000 {
		return bodyStr[:1000] + "...(truncated)"
	}
	return bodyStr
}

// escapeMarkdown 转义 Markdown 特殊字符
func escapeMarkdown(s string) string {
	// 转义 Markdown 特殊字符
	replacer := strings.NewReplacer(
		"_", "\\_",
		"*", "\\*",
		"[", "\\[",
		"]", "\\]",
		"(", "\\(",
		")", "\\)",
		"~", "\\~",
		"`", "\\`",
		">", "\\>",
		"#", "\\#",
		"+", "\\+",
		"-", "\\-",
		"=", "\\=",
		"|", "\\|",
		"{", "\\{",
		"}", "\\}",
		".", "\\.",
		"!", "\\!",
	)
	return replacer.Replace(s)
}
