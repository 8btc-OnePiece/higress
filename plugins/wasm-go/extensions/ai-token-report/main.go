package main

import (
	"crypto/rand"
	"encoding/json"
	"fmt"
	"math/big"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/higress-group/proxy-wasm-go-sdk/proxywasm"
	"github.com/higress-group/proxy-wasm-go-sdk/proxywasm/types"
	"github.com/higress-group/wasm-go/pkg/log"
	"github.com/higress-group/wasm-go/pkg/tokenusage"
	"github.com/higress-group/wasm-go/pkg/wrapper"
	"github.com/tidwall/gjson"
)

const (
	// Report configuration
	defaultReportTimeout int32 = 5000 // 5 seconds

	// Context keys
	CtxKeyModel      = "token_model"
	CtxKeyInputToken = "token_input"
	CtxKeyOutputToken = "token_output"
	CtxKeyTotalToken  = "token_total"
	CtxRequestStart    = "request_start_time"
	CtxFirstToken     = "first_token_time"
	CtxTokenUsage      = "token_usage_raw"
	CtxSkipPlugin     = "skip_token_report"
	providerTypeKey   = "providerType"

	// HTTP headers
	OriginalAPIKey = "X-Original-Api-Key"

	// Metrics
	LLMServiceDuration = "llm_service_duration"

	// Default disableOpenaiUsage value
	defaultDisableOpenaiUsage = false
)

// TokenUsageReportRequest represents the request body for token usage reporting
type TokenUsageReportRequest struct {
	RequestId   string `json:"requestId"`
	Model       string `json:"model"`
	UserKey     string `json:"userKey"`
	InputToken  int64  `json:"inputToken"`
	OutputToken int64  `json:"outputToken"`
	TotalToken  int64  `json:"totalToken"`
	Duration    int64  `json:"duration"`
	Timestamp   int64  `json:"timestamp"`
	StartTime   int64  `json:"startTime"`
	EndTime     int64  `json:"endTime"`
	TokenUsage  string `json:"tokenUsage"`
}

// TokenUsageReportConfig holds the configuration for token usage reporting
type TokenUsageReportConfig struct {
	reportClient      wrapper.HttpClient
	reportApiUrl      string
	reportServiceName string
	reportTimeout     int32
	disableOpenaiUsage bool
}

func main() {}

func init() {
	wrapper.SetCtx(
		"ai-token-report",
		wrapper.ParseConfig(parseConfig),
		wrapper.ProcessRequestHeaders(onHttpRequestHeaders),
		wrapper.ProcessRequestBody(onHttpRequestBody),
		wrapper.ProcessStreamingResponseBody(onHttpStreamingBody),
		wrapper.ProcessResponseBody(onHttpResponseBody),
	)
}

func parseConfig(json gjson.Result, config *TokenUsageReportConfig) error {
	// Parse reportApiUrl (required)
	config.reportApiUrl = json.Get("reportApiUrl").String()
	if config.reportApiUrl == "" {
		log.Infof("ai-token-report: reportApiUrl not configured, plugin will be disabled")
		return nil
	}

	// Parse reportServiceName (optional)
	config.reportServiceName = json.Get("reportServiceName").String()
	if config.reportServiceName == "" {
		config.reportServiceName = "default"
		log.Warnf("reportServiceName not configured, using default: %s", "default")
	}

	// Parse reportTimeout (optional)
	if json.Get("reportTimeout").Exists() {
		config.reportTimeout = int32(json.Get("reportTimeout").Int())
	} else {
		config.reportTimeout = defaultReportTimeout
		log.Infof("reportTimeout not configured, using default: %dms", defaultReportTimeout)
	}

	// Parse disableOpenaiUsage (optional)
	if json.Get("disableOpenaiUsage").Exists() {
		config.disableOpenaiUsage = json.Get("disableOpenaiUsage").Bool()
	} else {
		config.disableOpenaiUsage = defaultDisableOpenaiUsage
		log.Infof("disableOpenaiUsage not configured, using default: %v", defaultDisableOpenaiUsage)
	}

	// Parse URL and extract actual domain and port
	parsedUrl, err := url.Parse(config.reportApiUrl)
	if err != nil {
		log.Errorf("failed to parse reportApiUrl: %v", err)
		return fmt.Errorf("invalid reportApiUrl: %v", err)
	}

	// Extract actual hostname (for Host header and TLS SNI)
	actualHost := parsedUrl.Hostname()
	if actualHost == "" {
		log.Errorf("failed to extract hostname from reportApiUrl: %s", config.reportApiUrl)
		return fmt.Errorf("invalid reportApiUrl: missing hostname")
	}

	// Extract port, default 80 (HTTP) or 443 (HTTPS)
	port := int64(80)
	if parsedUrl.Scheme == "https" {
		port = 443
	}

	// If port is explicitly specified in URL, use the specified port
	if actualPort := parsedUrl.Port(); actualPort != "" {
		if p, err := strconv.Atoi(actualPort); err == nil {
			port = int64(p)
		}
	}

	// Create HTTP client using DnsCluster
	config.reportClient = wrapper.NewClusterClient(wrapper.DnsCluster{
		ServiceName: config.reportServiceName,
		Domain:      actualHost,
		Port:        port,
	})

	log.Infof("ai-token-report: token usage report configured")
	log.Infof("  reportApiUrl: %s", config.reportApiUrl)
	log.Infof("  reportServiceName (服务名): %s", config.reportServiceName)
	log.Infof("  DnsCluster.ServiceName (服务名): %s", config.reportServiceName)
	log.Infof("  DnsCluster.Domain (实际域名): %s", actualHost)
	log.Infof("  DnsCluster.Port (服务端口): %d", port)
	log.Infof("  disableOpenaiUsage: %v", config.disableOpenaiUsage)
	log.Infof("  reportTimeout: %dms", config.reportTimeout)

	return nil
}

func onHttpRequestHeaders(ctx wrapper.HttpContext, config TokenUsageReportConfig) types.Action {
	// Check if plugin is enabled
	if config.reportApiUrl == "" {
		return types.ActionContinue
	}

	// 检查 providerType，如果为空则跳过本插件（非 AI 路由）
	providerType := getProviderType()
	if providerType == "" {
		log.Debugf("ai-token-report: providerType is empty, skipping (not an AI route)")
		ctx.DontReadRequestBody()
		ctx.DontReadResponseBody()
		ctx.SetContext(CtxSkipPlugin, true)
		return types.ActionContinue
	}

	// Set request start time
	ctx.SetContext(CtxRequestStart, time.Now().UnixMilli())

	requestModel := getRequestModel()
	ctx.SetContext(CtxKeyModel, requestModel)

	log.Debugf("ai-token-report: processing AI request, providerType=%s, requestModel=%s", providerType, requestModel)

	return types.ActionContinue
}

func onHttpRequestBody(ctx wrapper.HttpContext, config TokenUsageReportConfig, body []byte) types.Action {
	// 检查是否设置了跳过标记
	if ctx.GetContext(CtxSkipPlugin) != nil {
		return types.ActionContinue
	}

	// 检查 plugin 是否启用
	if config.reportApiUrl == "" {
		return types.ActionContinue
	}

	// 如果 requestModel 为空，尝试从 body 中提取
	model, _ := ctx.GetContext(CtxKeyModel).(string)
	if model == "" {
		parsed := gjson.ParseBytes(body)
		model = parsed.Get("model").String()
		if model != "" {
			ctx.SetContext(CtxKeyModel, model)
			log.Debugf("ai-token-report: extracted model from request body: %s", model)
		}
	}

    // 如果 model 为空，则跳过读取响应体
	if model == "" {
		log.Debugf("ai-token-report: request model is empty, skipping response body read")
		ctx.DontReadResponseBody()
	}

	return types.ActionContinue
}

func getProviderType() string {
	propertyKey := "wasm." + providerTypeKey
	if providerType, err := proxywasm.GetProperty([]string{propertyKey}); err == nil {
		return string(providerType)
	}
	return ""
}

func getRequestModel() string {
	propertyKey := "wasm.requestModel"
	if requestModel, err := proxywasm.GetProperty([]string{propertyKey}); err == nil {
		return string(requestModel)
	}
	return ""
}

func onHttpStreamingBody(ctx wrapper.HttpContext, config TokenUsageReportConfig, data []byte, endOfStream bool) []byte {
	// 检查是否设置了跳过标记
	if ctx.GetContext(CtxSkipPlugin) != nil {
		return data
	}

	// Check if plugin is enabled
	if config.reportApiUrl == "" {
		return data
	}

	// Set first token time if not already set
	if ctx.GetContext(CtxFirstToken) == nil {
		ctx.SetContext(CtxFirstToken, time.Now().UnixMilli())
	}

	// Extract token usage from response body
	if !config.disableOpenaiUsage {
		if usage := tokenusage.GetTokenUsage(ctx, data); usage.TotalToken > 0 {
// 			ctx.SetContext(CtxKeyModel, usage.Model)
			ctx.SetContext(CtxKeyInputToken, usage.InputToken)
			ctx.SetContext(CtxKeyOutputToken, usage.OutputToken)
			ctx.SetContext(CtxKeyTotalToken, usage.TotalToken)
			// 存储原始的 tokenUsage JSON 字符串
			usageJSON, err := json.Marshal(usage)
			if err == nil {
				ctx.SetContext(CtxTokenUsage, string(usageJSON))
			}
		}
	}

	// Report token usage at the end of stream
	if endOfStream {
		// Calculate request duration for streaming response
		requestStartTime, ok := ctx.GetContext(CtxRequestStart).(int64)
		if !ok {
			log.Warn("ai-token-report: request start time not found in context (streaming), using current time")
			requestStartTime = time.Now().UnixMilli()
		}

		responseEndTime := time.Now().UnixMilli()
		duration := responseEndTime - requestStartTime
		ctx.SetContext(LLMServiceDuration, duration)

		// Report token usage
		reportTokenUsageFromContext(ctx, config, data)
	}

	return data
}

func onHttpResponseBody(ctx wrapper.HttpContext, config TokenUsageReportConfig, body []byte) types.Action {
	// 检查是否设置了跳过标记
	if ctx.GetContext(CtxSkipPlugin) != nil {
		return types.ActionContinue
	}

	// Check if plugin is enabled
	if config.reportApiUrl == "" {
		return types.ActionContinue
	}

	// Calculate request duration
	requestStartTime, ok := ctx.GetContext(CtxRequestStart).(int64)
	if !ok {
		log.Warn("ai-token-report: request start time not found in context, using current time")
		requestStartTime = time.Now().UnixMilli()
	}

	responseEndTime := time.Now().UnixMilli()
	duration := responseEndTime - requestStartTime
	ctx.SetContext(LLMServiceDuration, duration)

	// Extract token usage from response body
	if !config.disableOpenaiUsage {
		if usage := tokenusage.GetTokenUsage(ctx, body); usage.TotalToken > 0 {
// 			ctx.SetContext(CtxKeyModel, usage.Model)
			ctx.SetContext(CtxKeyInputToken, usage.InputToken)
			ctx.SetContext(CtxKeyOutputToken, usage.OutputToken)
			ctx.SetContext(CtxKeyTotalToken, usage.TotalToken)
			// 存储原始的 tokenUsage JSON 字符串
			usageJSON, err := json.Marshal(usage)
			if err == nil {
				ctx.SetContext(CtxTokenUsage, string(usageJSON))
			}
			log.Debugf("ai-token-report: extracted token usage (non-streaming): model=%s, input=%d, output=%d, total=%d",
				usage.Model, usage.InputToken, usage.OutputToken, usage.TotalToken)
		}
	}

	// Report token usage
	reportTokenUsageFromContext(ctx, config, body)

	return types.ActionContinue
}

func generateRandomString(length int) string {
	if length > 32 {
		length = 32
	}
	if length <= 0 {
		length = 8
	}

	const charset = "abcdefghijklmnopqrstuvwxyz0123456789"
	result := make([]byte, length)

	for i := range result {
		randomIndex, _ := rand.Int(rand.Reader, big.NewInt(int64(len(charset))))
		result[i] = charset[randomIndex.Int64()]
	}

	return string(result)
}

func convertToUInt(val interface{}) (uint64, bool) {
	switch v := val.(type) {
	case float32:
		return uint64(v), true
	case float64:
		return uint64(v), true
	case int32:
		return uint64(v), true
	case int64:
		return uint64(v), true
	case uint32:
		return uint64(v), true
	case uint64:
		return v, true
	default:
		return 0, false
	}
}

func reportTokenUsage(config TokenUsageReportConfig, ctx wrapper.HttpContext, model string, inputTokens, outputTokens,
                      totalTokens uint64, duration int64, startTime, endTime int64, tokenUsage string) {
	// Add panic recovery to ensure reporting never crashes the main flow
	defer func() {
		if r := recover(); r != nil {
			log.Errorf("Panic in reportTokenUsage (recovered): %v", r)
		}
	}()

	// Skip if report API client is not configured
	if config.reportClient == nil {
		log.Debugf("reportClient not configured, skipping token usage report")
		return
	}

	// Get userKey from X-Original-Api-Key header
	userKey, err := proxywasm.GetHttpRequestHeader(OriginalAPIKey)
	if err != nil || userKey == "" {
		log.Debugf("X-Original-Api-Key header not found, skipping token usage report")
		return
	}

	// Get Envoy request ID from property
	envoyRequestID := ""
	if requestIDValue, err := proxywasm.GetProperty([]string{"request", "id"}); err == nil {
		envoyRequestID = string(requestIDValue)
		if envoyRequestID != "" && envoyRequestID != "-" {
			log.Debugf("Using Envoy request ID: %s", envoyRequestID)
		}
	}

	timestamp := time.Now().UnixMilli()
	var requestID string
	if envoyRequestID != "" {
		requestID = envoyRequestID
		log.Debugf("reportTokenUsage using Envoy request ID: %s", envoyRequestID)
	} else {
		randomSuffix := generateRandomString(8)
		requestID = fmt.Sprintf("%s%d", randomSuffix, timestamp)
		log.Debugf("reportTokenUsage using generated ID: %s", requestID)
	}

	// Build request body
	reportReq := TokenUsageReportRequest{
		RequestId:   requestID,
		Model:       model,
		UserKey:     userKey,
		InputToken:  int64(inputTokens),
		OutputToken: int64(outputTokens),
		TotalToken:  int64(totalTokens),
		Duration:    duration,
		Timestamp:   timestamp,
		StartTime:   startTime,
		EndTime:     endTime,
		TokenUsage:  tokenUsage,
	}

	// Marshal request body
	jsonData, err := json.Marshal(reportReq)
	if err != nil {
		log.Errorf("failed to marshal token usage report request: %v", err)
		return
	}

	// Parse URL to get path
	parsedUrl, _ := url.Parse(config.reportApiUrl)
	requestPath := parsedUrl.Path
	if requestPath == "" {
		requestPath = "/"
	}

	// Add query parameters if present
	if parsedUrl.RawQuery != "" {
		requestPath = requestPath + "?" + parsedUrl.RawQuery
	}

	// Call the reporting API asynchronously
	log.Infof("Reporting token usage to %s: requestId=%s, model=%s, userKey=%s, inputToken=%d, outputToken=%d, totalToken=%d, duration=%d",
		config.reportApiUrl, requestID, model, userKey, inputTokens, outputTokens, totalTokens, duration)

	// Use a simple callback that doesn't capture large variables
	err = config.reportClient.Post(
		requestPath,
		[][2]string{
			{"Content-Type", "application/json"},
		},
		jsonData,
		func(statusCode int, responseHeaders http.Header, responseBody []byte) {
			log.Debugf("Token usage report callback: status=%d", statusCode)
		},
		uint32(config.reportTimeout),
	)

	if err != nil {
		log.Errorf("Failed to dispatch token usage report call: %v", err)
		return
	}
}

func reportTokenUsageFromContext(ctx wrapper.HttpContext, config TokenUsageReportConfig, responseBody []byte) {
	// Add panic recovery to ensure this function never interrupts the main flow
	defer func() {
		if r := recover(); r != nil {
			log.Errorf("Panic in reportTokenUsageFromContext (recovered): %v", r)
		}
	}()

	// Skip if disableOpenaiUsage is enabled
	if config.disableOpenaiUsage {
		log.Debugf("disableOpenaiUsage is enabled, skipping token usage report")
		return
	}

	// Skip if report API client is not configured
	if config.reportClient == nil {
		return
	}

	// Get model from context attributes
	model, ok := ctx.GetContext(CtxKeyModel).(string)
	if !ok || model == "" {
		log.Debugf("Model not found in context, skipping token usage report")
		return
	}

	// Get token values from context
	inputTokenVal := ctx.GetContext(CtxKeyInputToken)
	outputTokenVal := ctx.GetContext(CtxKeyOutputToken)
	totalTokenVal := ctx.GetContext(CtxKeyTotalToken)

	// Convert token values to uint64
	inputTokens, ok := convertToUInt(inputTokenVal)
	if !ok {
		log.Warnf("InputToken conversion failed, skipping token usage report, model=%s, responseBody=%s", model, string(responseBody))
		return
	}

	outputTokens, ok := convertToUInt(outputTokenVal)
	if !ok {
		return
	}

	totalTokens, ok := convertToUInt(totalTokenVal)
	if !ok {
		return
	}

	// Get duration from context
	durationVal := ctx.GetContext(LLMServiceDuration)
	duration, ok := convertToUInt(durationVal)
	if !ok {
		log.Debugf("LLMServiceDuration not found, skipping token usage report")
		return
	}

	// Get start time from context
	startTimeVal := ctx.GetContext(CtxRequestStart)
	startTime, ok := convertToUInt(startTimeVal)
	if !ok {
		startTime = 0
	}

	// Get end time (current time)
	endTime := time.Now().UnixMilli()

	// Get tokenUsage JSON from context
	tokenUsage := ""
	if tokenUsageVal := ctx.GetContext(CtxTokenUsage); tokenUsageVal != nil {
		if tokenUsageStr, ok := tokenUsageVal.(string); ok {
			// 将驼峰命名转换为下划线命名
			tokenUsage = convertCamelToSnakeJSON(tokenUsageStr)
		}
	}

	// Call the report function (in a goroutine-like manner to avoid blocking)
	reportTokenUsage(config, ctx, model, inputTokens, outputTokens, totalTokens, int64(duration), int64(startTime), endTime, tokenUsage)

	// Clean up context to prevent memory leaks
	ctx.SetContext(CtxRequestStart, nil)
	ctx.SetContext(CtxKeyModel, nil)
	ctx.SetContext(CtxKeyInputToken, nil)
	ctx.SetContext(CtxKeyOutputToken, nil)
	ctx.SetContext(CtxKeyTotalToken, nil)
	ctx.SetContext(CtxFirstToken, nil)
	ctx.SetContext(CtxTokenUsage, nil)
	ctx.SetContext(LLMServiceDuration, nil)
}

// convertCamelToSnakeJSON 将 JSON 字符串中第一层的驼峰命名转换为下划线命名
// 例如：{"InputToken":123,"Inner":{"Key":"val"}} -> {"input_token":123,"Inner":{"Key":"val"}}
func convertCamelToSnakeJSON(jsonStr string) string {
	var data map[string]interface{}
	if err := json.Unmarshal([]byte(jsonStr), &data); err != nil {
		// 解析失败，返回原字符串
		return jsonStr
	}

	// 只转换第一层的 key
	result := make(map[string]interface{})
	for key, value := range data {
		newKey := camelToSnake(key)
		result[newKey] = value
	}

	output, _ := json.Marshal(result)
	return string(output)
}

// camelToSnake 将驼峰命名转换为下划线命名
func camelToSnake(s string) string {
	var result []rune
	for i, r := range s {
		if i > 0 && r >= 'A' && r <= 'Z' {
			result = append(result, '_')
		}
		result = append(result, r)
	}
	return strings.ToLower(string(result))
}
