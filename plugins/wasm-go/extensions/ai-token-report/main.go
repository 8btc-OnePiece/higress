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
	InputToken  int32  `json:"inputToken"`
	OutputToken int32  `json:"outputToken"`
	TotalToken  int32  `json:"totalToken"`
	Duration    int64  `json:"duration"`
	Timestamp   int64  `json:"timestamp"`
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
		wrapper.ProcessResponseHeaders(onHttpResponseHeaders),
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

	// Extract model from request body and save to context
	requestPath, _ := proxywasm.GetHttpRequestHeader(":path")
	log.Debugf("ai-token-report: processing request path: %s", requestPath)

	// Set request start time
	ctx.SetContext(CtxRequestStart, time.Now().UnixMilli())
	log.Debugf("ai-token-report: set request start time")

	return types.ActionContinue
}

func onHttpRequestBody(ctx wrapper.HttpContext, config TokenUsageReportConfig, body []byte) types.Action {
	// Check if plugin is enabled
	if config.reportApiUrl == "" {
		return types.ActionContinue
	}

	// Extract model from request body and save to context
	if len(body) > 0 && !config.disableOpenaiUsage {
		if model := gjson.GetBytes(body, "model"); model.Exists() {
			ctx.SetContext(CtxKeyModel, model.String())
			log.Debugf("ai-token-report: extracted model from request body: %s", model.String())
		}
	}

	return types.ActionContinue
}

func onHttpResponseHeaders(ctx wrapper.HttpContext, config TokenUsageReportConfig) types.Action {
	// Check if plugin is enabled
	if config.reportApiUrl == "" {
		return types.ActionContinue
	}

	// Check if this is streaming response by looking at content-type
	contentType, _ := proxywasm.GetHttpResponseHeader("content-type")
	if contentType != "" && (strings.Contains(contentType, "text/event-stream") ||
		strings.Contains(contentType, "text/stream")) {
		log.Debugf("ai-token-report: streaming response detected (content-type: %s)", contentType)
	}

	return types.ActionContinue
}

func onHttpStreamingBody(ctx wrapper.HttpContext, config TokenUsageReportConfig, data []byte, endOfStream bool) []byte {
	// Check if plugin is enabled
	if config.reportApiUrl == "" {
		return data
	}

	// Set first token time if not already set
	if ctx.GetContext(CtxFirstToken) == nil {
		ctx.SetContext(CtxFirstToken, time.Now().UnixMilli())
		log.Debugf("ai-token-report: set first token time (streaming)")
	}

	// Extract token usage from response body
	if !config.disableOpenaiUsage {
		if usage := tokenusage.GetTokenUsage(ctx, data); usage.TotalToken > 0 {
			ctx.SetContext(CtxKeyModel, usage.Model)
			ctx.SetContext(CtxKeyInputToken, usage.InputToken)
			ctx.SetContext(CtxKeyOutputToken, usage.OutputToken)
			ctx.SetContext(CtxKeyTotalToken, usage.TotalToken)
			log.Debugf("ai-token-report: extracted token usage (streaming): model=%s, input=%d, output=%d, total=%d",
				usage.Model, usage.InputToken, usage.OutputToken, usage.TotalToken)
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

		log.Debugf("ai-token-report: streaming request duration: %dms", duration)

		// Report token usage
		reportTokenUsageFromContext(ctx, config)
	}

	return data
}

func onHttpResponseBody(ctx wrapper.HttpContext, config TokenUsageReportConfig, body []byte) types.Action {
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

	log.Debugf("ai-token-report: request duration: %dms", duration)

	// Extract token usage from response body
	if !config.disableOpenaiUsage {
		if usage := tokenusage.GetTokenUsage(ctx, body); usage.TotalToken > 0 {
			ctx.SetContext(CtxKeyModel, usage.Model)
			ctx.SetContext(CtxKeyInputToken, usage.InputToken)
			ctx.SetContext(CtxKeyOutputToken, usage.OutputToken)
			ctx.SetContext(CtxKeyTotalToken, usage.TotalToken)
			log.Debugf("ai-token-report: extracted token usage (non-streaming): model=%s, input=%d, output=%d, total=%d",
				usage.Model, usage.InputToken, usage.OutputToken, usage.TotalToken)
		}
	}

	// Report token usage
	reportTokenUsageFromContext(ctx, config)

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

func reportTokenUsage(config TokenUsageReportConfig, ctx wrapper.HttpContext, model string, inputTokens, outputTokens, totalTokens uint64, duration int64) {
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

	// Convert uint64 to int32 for JSON marshaling (API expects Integer)
	inputTokenInt := int32(inputTokens)
	outputTokenInt := int32(outputTokens)
	totalTokenInt := int32(totalTokens)

	// Build request body
	reportReq := TokenUsageReportRequest{
		RequestId:   requestID,
		Model:       model,
		UserKey:     userKey,
		InputToken:  inputTokenInt,
		OutputToken: outputTokenInt,
		TotalToken:  totalTokenInt,
		Duration:    duration,
		Timestamp:   timestamp,
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
		config.reportApiUrl, requestID, model, userKey, inputTokenInt, outputTokenInt, totalTokenInt, duration)

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

	log.Debugf("Token usage report initiated successfully for requestId: %s", requestID)
}

func reportTokenUsageFromContext(ctx wrapper.HttpContext, config TokenUsageReportConfig) {
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
		log.Debugf("InputToken conversion failed, skipping token usage report")
		return
	}

	outputTokens, ok := convertToUInt(outputTokenVal)
	if !ok {
		log.Debugf("OutputToken conversion failed, skipping token usage report")
		return
	}

	totalTokens, ok := convertToUInt(totalTokenVal)
	if !ok {
		log.Debugf("TotalToken conversion failed, skipping token usage report")
		return
	}

	// Get duration from context
	durationVal := ctx.GetContext(LLMServiceDuration)
	duration, ok := convertToUInt(durationVal)
	if !ok {
		log.Debugf("LLMServiceDuration not found, skipping token usage report")
		return
	}

	// Call the report function (in a goroutine-like manner to avoid blocking)
	reportTokenUsage(config, ctx, model, inputTokens, outputTokens, totalTokens, int64(duration))

	// Clean up context to prevent memory leaks
	ctx.SetContext(CtxRequestStart, nil)
	ctx.SetContext(CtxKeyModel, nil)
	ctx.SetContext(CtxKeyInputToken, nil)
	ctx.SetContext(CtxKeyOutputToken, nil)
	ctx.SetContext(CtxKeyTotalToken, nil)
	ctx.SetContext(CtxFirstToken, nil)
	ctx.SetContext(LLMServiceDuration, nil)

	log.Debugf("ai-token-report: cleaned up context data after reporting")
}
