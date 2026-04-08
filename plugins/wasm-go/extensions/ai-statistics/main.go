package main

import (
	"bytes"
	"crypto/rand"
	"encoding/json"
	"errors"
	"fmt"
	"math/big"
	"net/http"
	"net/url"
	"regexp"
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

func main() {}

func init() {
	wrapper.SetCtx(
		"ai-statistics",
		wrapper.ParseConfig(parseConfig),
		wrapper.ProcessRequestHeaders(onHttpRequestHeaders),
		wrapper.ProcessRequestBody(onHttpRequestBody),
		wrapper.ProcessResponseHeaders(onHttpResponseHeaders),
		wrapper.ProcessStreamingResponseBody(onHttpStreamingBody),
		wrapper.ProcessResponseBody(onHttpResponseBody),
		wrapper.WithRebuildAfterRequests[AIStatisticsConfig](1000),
		wrapper.WithRebuildMaxMemBytes[AIStatisticsConfig](200*1024*1024),
	)
}

const (
	defaultMaxBodyBytes        uint32 = 100 * 1024 * 1024
	defaultReportTimeout       int32  = 5000             // 5 seconds
	maxStreamingBodyBufferSize int    = 10 * 1024 * 1024 // 10MB limit for streaming buffer
	// Context consts
	StatisticsRequestStartTime = "ai-statistics-request-start-time"
	StatisticsFirstTokenTime   = "ai-statistics-first-token-time"
	CtxGeneralAtrribute        = "attributes"
	CtxLogAtrribute            = "logAttributes"
	CtxStreamingBodyBuffer     = "streamingBodyBuffer"
	RouteName                  = "route"
	ClusterName                = "cluster"
	APIName                    = "api"
	ConsumerKey                = "x-mse-consumer"
	OriginalAPIKey             = "X-Original-Api-Key"
	RequestID                  = "request_id"
	RequestPath                = "request_path"
	SkipProcessing             = "skip_processing"

	// AI API Paths
	PathOpenAIChatCompletions       = "/v1/chat/completions"
	PathOpenAICompletions           = "/v1/completions"
	PathOpenAIEmbeddings            = "/v1/embeddings"
	PathOpenAIModels                = "/v1/models"
	PathGeminiGenerateContent       = "/generateContent"
	PathGeminiStreamGenerateContent = "/streamGenerateContent"

	// Source Type
	FixedValue            = "fixed_value"
	RequestHeader         = "request_header"
	RequestBody           = "request_body"
	ResponseHeader        = "response_header"
	ResponseStreamingBody = "response_streaming_body"
	ResponseBody          = "response_body"

	// Inner metric & log attributes
	LLMFirstTokenDuration  = "llm_first_token_duration"
	LLMServiceDuration     = "llm_service_duration"
	LLMDurationCount       = "llm_duration_count"
	LLMStreamDurationCount = "llm_stream_duration_count"
	ResponseType           = "response_type"
	ChatID                 = "chat_id"
	ChatRound              = "chat_round"

	// Inner span attributes
	ArmsSpanKind     = "gen_ai.span.kind"
	ArmsModelName    = "gen_ai.model_name"
	ArmsRequestModel = "gen_ai.request.model"
	ArmsInputToken   = "gen_ai.usage.input_tokens"
	ArmsOutputToken  = "gen_ai.usage.output_tokens"
	ArmsTotalToken   = "gen_ai.usage.total_tokens"

	// Extract Rule
	RuleFirst   = "first"
	RuleReplace = "replace"
	RuleAppend  = "append"

	// Built-in attributes
	BuiltinQuestionKey = "question"
	BuiltinAnswerKey   = "answer"

	// Built-in attribute paths
	// Question paths (from request body)
	QuestionPathOpenAI = "messages.@reverse.0.content"
	QuestionPathClaude = "messages.@reverse.0.content" // Claude uses same format

	// Answer paths (from response body - non-streaming)
	AnswerPathOpenAINonStreaming = "choices.0.message.content"
	AnswerPathClaudeNonStreaming = "content.0.text"

	// Answer paths (from response streaming body)
	AnswerPathOpenAIStreaming = "choices.0.delta.content"
	AnswerPathClaudeStreaming = "delta.text"
)

// TracingSpan is the tracing span configuration.
type Attribute struct {
	Key                string `json:"key"`
	ValueSource        string `json:"value_source"`
	Value              string `json:"value"`
	TraceSpanKey       string `json:"trace_span_key,omitempty"`
	DefaultValue       string `json:"default_value,omitempty"`
	Rule               string `json:"rule,omitempty"`
	ApplyToLog         bool   `json:"apply_to_log,omitempty"`
	ApplyToSpan        bool   `json:"apply_to_span,omitempty"`
	AsSeparateLogField bool   `json:"as_separate_log_field,omitempty"`
}

// TokenUsageReportRequest represents the request body for token usage reporting
type TokenUsageReportRequest struct {
	RequestId   string `json:"requestId"`   // 请求唯一ID，格式：{userKey}-{timestamp}-{random}
	Model       string `json:"model"`       // 模型名称
	UserKey     string `json:"userKey"`     // 用户Key（从X-Original-Api-Key header获取）
	InputToken  int32  `json:"inputToken"`  // 输入token数
	OutputToken int32  `json:"outputToken"` // 输出token数
	TotalToken  int32  `json:"totalToken"`  // 总token数
	Duration    int64  `json:"duration"`    // 耗时（毫秒）
	Timestamp   int64  `json:"timestamp"`   // 时间戳（毫秒）
}

// TokenUsageReportResponse represents the response from the reporting API
type TokenUsageReportResponse struct {
	Success   bool   `json:"success"`   // 是否成功
	Message   string `json:"message"`   // 响应消息
	RequestID string `json:"requestId"` // 请求ID
	Duplicate bool   `json:"duplicate"` // 是否重复请求
}

type AIStatisticsConfig struct {
	// Metrics
	// TODO: add more metrics in Gauge and Histogram format
	counterMetrics map[string]proxywasm.MetricCounter
	// Attributes to be recorded in log & span
	attributes []Attribute
	// If there exist attributes extracted from streaming body, chunks should be buffered
	shouldBufferStreamingBody bool
	// If disableOpenaiUsage is true, model/input_token/output_token logs will be skipped
	disableOpenaiUsage bool
	valueLengthLimit   int
	// Path suffixes to enable the plugin on
	enablePathSuffixes []string
	// Content types to enable response body buffering
	enableContentTypes []string

	// Token usage report configuration
	reportClient      wrapper.HttpClient
	reportServiceName string
	reportApiUrl      string
	reportTimeout     int32
}

func generateMetricName(route, cluster, model, consumer, metricName string) string {
	return fmt.Sprintf("route.%s.upstream.%s.model.%s.consumer.%s.metric.%s", route, cluster, model, consumer, metricName)
}

func getRouteName() (string, error) {
	if raw, err := proxywasm.GetProperty([]string{"route_name"}); err != nil {
		return "-", err
	} else {
		return string(raw), nil
	}
}

func getRequestID() (string, error) {
	if raw, err := proxywasm.GetProperty([]string{"request", "id"}); err != nil {
		return "-", err
	} else {
		return string(raw), nil
	}
}

func getAPIName() (string, error) {
	if raw, err := proxywasm.GetProperty([]string{"route_name"}); err != nil {
		return "-", err
	} else {
		parts := strings.Split(string(raw), "@")
		if len(parts) < 3 {
			return "-", errors.New("not api type")
		} else {
			return strings.Join(parts[:3], "@"), nil
		}
	}
}

func getClusterName() (string, error) {
	if raw, err := proxywasm.GetProperty([]string{"cluster_name"}); err != nil {
		return "-", err
	} else {
		return string(raw), nil
	}
}

func (config *AIStatisticsConfig) incrementCounter(metricName string, inc uint64) {
	if inc == 0 {
		return
	}
	counter, ok := config.counterMetrics[metricName]
	if !ok {
		counter = proxywasm.DefineCounterMetric(metricName)
		config.counterMetrics[metricName] = counter
	}
	counter.Increment(inc)
}

// isPathEnabled checks if the request path matches any of the enabled path suffixes
func isPathEnabled(requestPath string, enabledSuffixes []string) bool {
	if len(enabledSuffixes) == 0 {
		return true // If no path suffixes configured, enable for all
	}

	// Remove query parameters from path
	pathWithoutQuery := requestPath
	if queryPos := strings.Index(requestPath, "?"); queryPos != -1 {
		pathWithoutQuery = requestPath[:queryPos]
	}

	// Check if path ends with any enabled suffix
	for _, suffix := range enabledSuffixes {
		if strings.HasSuffix(pathWithoutQuery, suffix) {
			return true
		}
	}
	return false
}

// isContentTypeEnabled checks if the content type matches any of the enabled content types
func isContentTypeEnabled(contentType string, enabledContentTypes []string) bool {
	if len(enabledContentTypes) == 0 {
		return true // If no content types configured, enable for all
	}

	for _, enabledType := range enabledContentTypes {
		if strings.Contains(contentType, enabledType) {
			return true
		}
	}
	return false
}

func parseConfig(configJson gjson.Result, config *AIStatisticsConfig) error {
	// Parse tracing span attributes setting.
	attributeConfigs := configJson.Get("attributes").Array()
	if configJson.Get("value_length_limit").Exists() {
		config.valueLengthLimit = int(configJson.Get("value_length_limit").Int())
	} else {
		config.valueLengthLimit = 4000
	}
	config.attributes = make([]Attribute, len(attributeConfigs))
	for i, attributeConfig := range attributeConfigs {
		attribute := Attribute{}
		err := json.Unmarshal([]byte(attributeConfig.Raw), &attribute)
		if err != nil {
			log.Errorf("parse config failed, %v", err)
			return err
		}
		if attribute.ValueSource == ResponseStreamingBody {
			config.shouldBufferStreamingBody = true
		}
		if attribute.Rule != "" && attribute.Rule != RuleFirst && attribute.Rule != RuleReplace && attribute.Rule != RuleAppend {
			return errors.New("value of rule must be one of [nil, first, replace, append]")
		}
		config.attributes[i] = attribute
	}
	// Metric settings
	config.counterMetrics = make(map[string]proxywasm.MetricCounter)

	// Parse openai usage config setting.
	config.disableOpenaiUsage = configJson.Get("disable_openai_usage").Bool()

	// Parse path suffix configuration
	pathSuffixes := configJson.Get("enable_path_suffixes").Array()
	config.enablePathSuffixes = make([]string, 0, len(pathSuffixes))

	for _, suffix := range pathSuffixes {
		suffixStr := suffix.String()
		if suffixStr == "*" {
			// Clear the suffixes list since * means all paths are enabled
			config.enablePathSuffixes = make([]string, 0)
			break
		}
		config.enablePathSuffixes = append(config.enablePathSuffixes, suffixStr)
	}

	// Parse content type configuration
	contentTypes := configJson.Get("enable_content_types").Array()
	config.enableContentTypes = make([]string, 0, len(contentTypes))

	for _, contentType := range contentTypes {
		contentTypeStr := contentType.String()
		if contentTypeStr == "*" {
			// Clear the content types list since * means all content types are enabled
			config.enableContentTypes = make([]string, 0)
			break
		}
		config.enableContentTypes = append(config.enableContentTypes, contentTypeStr)
	}

	// Parse token usage report configuration
	config.reportApiUrl = configJson.Get("reportApiUrl").String()
	if config.reportApiUrl != "" {
		// Parse URL and extract actual domain and port
		parsedUrl, err := url.Parse(config.reportApiUrl)
		if err != nil {
			log.Errorf("failed to parse reportApiUrl: %v", err)
			return errors.New("invalid reportApiUrl: " + err.Error())
		}

		// Extract actual hostname (for Host header and TLS SNI)
		actualHost := parsedUrl.Hostname()
		if actualHost == "" {
			log.Errorf("failed to extract hostname from reportApiUrl: %s", config.reportApiUrl)
			return errors.New("invalid reportApiUrl: missing hostname")
		}

		// Extract port, default 80 (HTTP) or 443 (HTTPS)
		port := int64(80) // Default HTTP
		if parsedUrl.Scheme == "https" {
			port = 443 // Default HTTPS
		}

		// If port is explicitly specified in URL, use the specified port
		if actualPort := parsedUrl.Port(); actualPort != "" {
			if p, err := strconv.Atoi(actualPort); err == nil {
				port = int64(p)
			}
		}

		config.reportServiceName = configJson.Get("reportServiceName").String()

		// Create HTTP client using DnsCluster
		config.reportClient = wrapper.NewClusterClient(wrapper.DnsCluster{
			ServiceName: config.reportServiceName, // Use domain as service name
			Domain:      actualHost,               // Actual domain (for Host header and TLS SNI)
			Port:        port,                     // Port from URL
		})

		log.Infof("ai-statistics: token usage report configured")
		log.Infof("  reportApiUrl: %s", config.reportApiUrl)
		log.Infof("  DnsCluster.ServiceName (服务名): %s", config.reportServiceName)
		log.Infof("  DnsCluster.Domain (实际域名): %s", actualHost)
		log.Infof("  DnsCluster.Port (服务端口): %d", port)
	}

	if configJson.Get("reportTimeout").Exists() {
		config.reportTimeout = int32(configJson.Get("reportTimeout").Int())
	} else {
		config.reportTimeout = defaultReportTimeout
	}

	return nil
}

func onHttpRequestHeaders(ctx wrapper.HttpContext, config AIStatisticsConfig) types.Action {
	// Check if request path matches enabled suffixes
	requestPath, _ := proxywasm.GetHttpRequestHeader(":path")
	if !isPathEnabled(requestPath, config.enablePathSuffixes) {
		log.Debugf("ai-statistics: skipping request for path %s (not in enabled suffixes)", requestPath)
		// Set skip processing flag and avoid reading request/response body
		ctx.SetContext(SkipProcessing, true)
		ctx.DontReadRequestBody()
		ctx.DontReadResponseBody()
		cleanupContext(ctx)
		return types.ActionContinue
	}

	ctx.DisableReroute()
	route, _ := getRouteName()
	cluster, _ := getClusterName()
	api, apiError := getAPIName()
	if apiError == nil {
		route = api
	}
	ctx.SetContext(RouteName, route)
	ctx.SetContext(ClusterName, cluster)
	ctx.SetUserAttribute(APIName, api)
	ctx.SetContext(StatisticsRequestStartTime, time.Now().UnixMilli())
	if requestPath, _ := proxywasm.GetHttpRequestHeader(":path"); requestPath != "" {
		ctx.SetContext(RequestPath, requestPath)
	}
	if requestID, _ := getRequestID(); requestID != "" && requestID != "-" {
		ctx.SetContext(RequestID, requestID)
	}
	if consumer, _ := proxywasm.GetHttpRequestHeader(ConsumerKey); consumer != "" {
		ctx.SetContext(ConsumerKey, consumer)
	}

	ctx.SetRequestBodyBufferLimit(defaultMaxBodyBytes)

	// Set span attributes for ARMS.
	setSpanAttribute(ArmsSpanKind, "LLM")
	// Set user defined log & span attributes which type is fixed_value
	setAttributeBySource(ctx, config, FixedValue, nil)
	// Set user defined log & span attributes which type is request_header
	setAttributeBySource(ctx, config, RequestHeader, nil)

	return types.ActionContinue
}

func onHttpRequestBody(ctx wrapper.HttpContext, config AIStatisticsConfig, body []byte) types.Action {
	// Check if processing should be skipped first
	if ctx.GetBoolContext(SkipProcessing, false) {
		cleanupContext(ctx)
		return types.ActionContinue
	}

	// Set user defined log & span attributes.
	setAttributeBySource(ctx, config, RequestBody, body)
	// Set span attributes for ARMS.
	requestModel := "UNKNOWN"
	if model := gjson.GetBytes(body, "model"); model.Exists() {
		requestModel = model.String()
	} else {
		requestPath := ctx.GetStringContext(RequestPath, "")
		if strings.Contains(requestPath, "generateContent") || strings.Contains(requestPath, "streamGenerateContent") { // Google Gemini GenerateContent
			reg := regexp.MustCompile(`^.*/(?P<api_version>[^/]+)/models/(?P<model>[^:]+):\w+Content$`)
			matches := reg.FindStringSubmatch(requestPath)
			if len(matches) == 3 {
				requestModel = matches[2]
			}
		}
	}
	setSpanAttribute(ArmsRequestModel, requestModel)
	// Set the number of conversation rounds

	userPromptCount := 0
	if messages := gjson.GetBytes(body, "messages"); messages.Exists() && messages.IsArray() {
		// OpenAI and Claude/Anthropic format - both use "messages" array with "role" field
		for _, msg := range messages.Array() {
			if msg.Get("role").String() == "user" {
				userPromptCount += 1
			}
		}
	} else if contents := gjson.GetBytes(body, "contents"); contents.Exists() && contents.IsArray() {
		// Google Gemini GenerateContent
		for _, content := range contents.Array() {
			if !content.Get("role").Exists() || content.Get("role").String() == "user" {
				userPromptCount += 1
			}
		}
	}
	ctx.SetUserAttribute(ChatRound, userPromptCount)

	// Write log
	_ = ctx.WriteUserAttributeToLogWithKey(wrapper.AILogKey)
	return types.ActionContinue
}

func onHttpResponseHeaders(ctx wrapper.HttpContext, config AIStatisticsConfig) types.Action {
	contentType, _ := proxywasm.GetHttpResponseHeader("content-type")

	if !isContentTypeEnabled(contentType, config.enableContentTypes) {
		log.Debugf("ai-statistics: skipping response for content type %s (not in enabled content types)", contentType)
		// Set skip processing flag and avoid reading response body
		ctx.SetContext(SkipProcessing, true)
		ctx.DontReadResponseBody()
		cleanupContext(ctx)
		return types.ActionContinue
	}

	if !strings.Contains(contentType, "text/event-stream") {
		ctx.BufferResponseBody()
	}

	// Set user defined log & span attributes.
	setAttributeBySource(ctx, config, ResponseHeader, nil)

	return types.ActionContinue
}

func onHttpStreamingBody(ctx wrapper.HttpContext, config AIStatisticsConfig, data []byte, endOfStream bool) []byte {
	// Ensure cleanup only happens at the end of streaming or when skipping
	defer func() {
		// Cleanup when stream ends OR when skipping processing
		if endOfStream || ctx.GetBoolContext(SkipProcessing, false) {
			cleanupContext(ctx)
		}
	}()

	// Check if processing should be skipped
	if ctx.GetBoolContext(SkipProcessing, false) {
		return data
	}

	// Buffer stream body for record log & span attributes
	if config.shouldBufferStreamingBody {
		streamingBodyBuffer, ok := ctx.GetContext(CtxStreamingBodyBuffer).([]byte)
		if !ok {
			streamingBodyBuffer = make([]byte, 0, 1024) // Start with small capacity
		}

		// Check buffer size limit before appending to prevent uncontrolled growth
		currentSize := len(streamingBodyBuffer)
		newSize := currentSize + len(data)

		if newSize > maxStreamingBodyBufferSize {
			log.Warnf("Streaming body buffer exceeds limit (%d bytes), clearing buffer", newSize)
			// Clear the buffer to prevent memory accumulation and immediately free the old buffer
			streamingBodyBuffer = make([]byte, 0, 1024)
		} else {
			// Use append with potentially larger data chunks
			streamingBodyBuffer = append(streamingBodyBuffer, data...)
		}

		ctx.SetContext(CtxStreamingBodyBuffer, streamingBodyBuffer)
	}

	ctx.SetUserAttribute(ResponseType, "stream")
	if chatID := wrapper.GetValueFromBody(data, []string{
		"id",
		"response.id",
		"responseId", // Gemini generateContent
		"message.id", // anthropic/claude messages
	}); chatID != nil {
		ctx.SetUserAttribute(ChatID, chatID.String())
	}

	// Get requestStartTime from http context
	requestStartTime, ok := ctx.GetContext(StatisticsRequestStartTime).(int64)
	if !ok {
		log.Debugf("requestStartTime not found in http context, skipping metrics")
		return data
	}

	// If this is the first chunk, record first token duration metric and span attribute
	if ctx.GetContext(StatisticsFirstTokenTime) == nil {
		firstTokenTime := time.Now().UnixMilli()
		ctx.SetContext(StatisticsFirstTokenTime, firstTokenTime)
		ctx.SetUserAttribute(LLMFirstTokenDuration, firstTokenTime-requestStartTime)
	}

	// Set information about this request
	if !config.disableOpenaiUsage {
		if usage := tokenusage.GetTokenUsage(ctx, data); usage.TotalToken > 0 {
			// Set span attributes for ARMS.
			setSpanAttribute(ArmsTotalToken, usage.TotalToken)
			setSpanAttribute(ArmsModelName, usage.Model)
			setSpanAttribute(ArmsInputToken, usage.InputToken)
			setSpanAttribute(ArmsOutputToken, usage.OutputToken)
		}
	}
	// If the end of the stream is reached, record metrics/logs/spans.
	if endOfStream {
		responseEndTime := time.Now().UnixMilli()
		ctx.SetUserAttribute(LLMServiceDuration, responseEndTime-requestStartTime)

		// Set user defined log & span attributes.
		if config.shouldBufferStreamingBody {
			streamingBodyBuffer, ok := ctx.GetContext(CtxStreamingBodyBuffer).([]byte)
			if !ok {
				return data
			}
			setAttributeBySource(ctx, config, ResponseStreamingBody, streamingBodyBuffer)
			// Immediately clear the buffer after use to free memory
			ctx.SetContext(CtxStreamingBodyBuffer, nil)
		}

		// Write log
		_ = ctx.WriteUserAttributeToLogWithKey(wrapper.AILogKey)

		// Write metrics
		writeMetric(ctx, config)

		// Report token usage to API
		reportTokenUsageFromContext(ctx, config)
	}
	return data
}

// min returns the minimum of two integers
func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

func onHttpResponseBody(ctx wrapper.HttpContext, config AIStatisticsConfig, body []byte) types.Action {
	// Check if processing should be skipped first
	if ctx.GetBoolContext(SkipProcessing, false) {
		cleanupContext(ctx)
		return types.ActionContinue
	}

	// For normal processing, ensure cleanup at the end
	defer cleanupContext(ctx)

	// Get requestStartTime from http context
	requestStartTime, _ := ctx.GetContext(StatisticsRequestStartTime).(int64)

	responseEndTime := time.Now().UnixMilli()
	ctx.SetUserAttribute(LLMServiceDuration, responseEndTime-requestStartTime)

	ctx.SetUserAttribute(ResponseType, "normal")
	if chatID := wrapper.GetValueFromBody(body, []string{
		"id",
		"response.id",
		"responseId", // Gemini generateContent
		"message.id", // anthropic/claude messages
	}); chatID != nil {
		ctx.SetUserAttribute(ChatID, chatID.String())
	}

	// Set information about this request
	if !config.disableOpenaiUsage {
		if usage := tokenusage.GetTokenUsage(ctx, body); usage.TotalToken > 0 {
			// Set span attributes for ARMS.
			setSpanAttribute(ArmsModelName, usage.Model)
			setSpanAttribute(ArmsInputToken, usage.InputToken)
			setSpanAttribute(ArmsOutputToken, usage.OutputToken)
			setSpanAttribute(ArmsTotalToken, usage.TotalToken)
		}
	}

	// Set user defined log & span attributes.
	setAttributeBySource(ctx, config, ResponseBody, body)

	// Write log
	_ = ctx.WriteUserAttributeToLogWithKey(wrapper.AILogKey)

	// Write metrics
	writeMetric(ctx, config)

	// Report token usage to API
	reportTokenUsageFromContext(ctx, config)

	return types.ActionContinue
}

// fetches the tracing span value from the specified source.

func setAttributeBySource(ctx wrapper.HttpContext, config AIStatisticsConfig, source string, body []byte) {
	for _, attribute := range config.attributes {
		var key string
		var value interface{}
		if source == attribute.ValueSource {
			key = attribute.Key
			switch source {
			case FixedValue:
				value = attribute.Value
			case RequestHeader:
				value, _ = proxywasm.GetHttpRequestHeader(attribute.Value)
			case RequestBody:
				value = gjson.GetBytes(body, attribute.Value).Value()
			case ResponseHeader:
				value, _ = proxywasm.GetHttpResponseHeader(attribute.Value)
			case ResponseStreamingBody:
				value = extractStreamingBodyByJsonPath(body, attribute.Value, attribute.Rule)
			case ResponseBody:
				value = gjson.GetBytes(body, attribute.Value).Value()
			default:
			}

			// Handle built-in attributes with Claude/OpenAI protocol fallback logic
			if (value == nil || value == "") && isBuiltinAttribute(key) {
				value = getBuiltinAttributeFallback(ctx, config, key, source, body, attribute.Rule)
				if value != nil && value != "" {
					log.Debugf("[attribute] Used protocol fallback for %s: %+v", key, value)
				}
			}

			if (value == nil || value == "") && attribute.DefaultValue != "" {
				value = attribute.DefaultValue
			}

			log.Debugf("[attribute] source type: %s, key: %s, value: %+v", source, key, value)
			if attribute.ApplyToLog {
				if attribute.AsSeparateLogField {
					// Convert value to string and marshal it
					strValue := fmt.Sprint(value)

					// Truncate if exceeds limit
					if len(strValue) > config.valueLengthLimit {
						halfLen := config.valueLengthLimit / 2
						strValue = strValue[:halfLen] + " [truncated] " + strValue[len(strValue)-halfLen:]
					}

					marshalledJsonStr := wrapper.MarshalStr(strValue)
					if err := proxywasm.SetProperty([]string{key}, []byte(marshalledJsonStr)); err != nil {
						log.Warnf("failed to set %s in filter state, raw is %s, err is %v", key, marshalledJsonStr, err)
					}
				} else {
					// Truncate value if exceeds limit for regular user attributes
					if strValue := fmt.Sprint(value); len(strValue) > config.valueLengthLimit {
						halfLen := config.valueLengthLimit / 2
						value = strValue[:halfLen] + " [truncated] " + strValue[len(strValue)-halfLen:]
					}
					ctx.SetUserAttribute(key, value)
				}
			}
			// for metrics
			if key == tokenusage.CtxKeyModel || key == tokenusage.CtxKeyInputToken || key == tokenusage.CtxKeyOutputToken || key == tokenusage.CtxKeyTotalToken {
				ctx.SetContext(key, value)
			}
			if attribute.ApplyToSpan {
				if attribute.TraceSpanKey != "" {
					key = attribute.TraceSpanKey
				}
				setSpanAttribute(key, value)
			}
		}
	}
}

// isBuiltinAttribute checks if the given key is a built-in attribute
func isBuiltinAttribute(key string) bool {
	return key == BuiltinQuestionKey || key == BuiltinAnswerKey
}

// getBuiltinAttributeFallback provides protocol compatibility fallback for question/answer attributes
func getBuiltinAttributeFallback(ctx wrapper.HttpContext, config AIStatisticsConfig, key, source string, body []byte, rule string) interface{} {
	switch key {
	case BuiltinQuestionKey:
		if source == RequestBody {
			// Try OpenAI/Claude format (both use same messages structure)
			if value := gjson.GetBytes(body, QuestionPathOpenAI).Value(); value != nil && value != "" {
				return value
			}
		}
	case BuiltinAnswerKey:
		if source == ResponseStreamingBody {
			// Try OpenAI format first
			if value := extractStreamingBodyByJsonPath(body, AnswerPathOpenAIStreaming, rule); value != nil && value != "" {
				return value
			}
			// Try Claude format
			if value := extractStreamingBodyByJsonPath(body, AnswerPathClaudeStreaming, rule); value != nil && value != "" {
				return value
			}
		} else if source == ResponseBody {
			// Try OpenAI format first
			if value := gjson.GetBytes(body, AnswerPathOpenAINonStreaming).Value(); value != nil && value != "" {
				return value
			}
			// Try Claude format
			if value := gjson.GetBytes(body, AnswerPathClaudeNonStreaming).Value(); value != nil && value != "" {
				return value
			}
		}
	}
	return nil
}

func extractStreamingBodyByJsonPath(data []byte, jsonPath string, rule string) interface{} {
	// Optimize memory usage by avoiding unnecessary byte slice allocations
	trimmedData := bytes.TrimSpace(wrapper.UnifySSEChunk(data))

	// Always use streaming approach to avoid creating all chunks at once
	// This prevents memory allocation spikes in high-concurrency scenarios
	return extractStreamingBodyLarge(trimmedData, jsonPath, rule)
}

// extractStreamingBodyLarge handles large streaming body data more efficiently
func extractStreamingBodyLarge(data []byte, jsonPath string, rule string) interface{} {
	if rule == RuleFirst {
		// For first rule, scan through data and return first match
		start := 0
		for {
			idx := bytes.Index(data[start:], []byte("\n\n"))
			if idx == -1 {
				// Last chunk
				chunk := data[start:]
				if jsonObj := gjson.GetBytes(chunk, jsonPath); jsonObj.Exists() {
					return jsonObj.Value()
				}
				break
			}
			chunk := data[start : start+idx]
			if jsonObj := gjson.GetBytes(chunk, jsonPath); jsonObj.Exists() {
				return jsonObj.Value()
			}
			start += idx + 2
		}
		return nil
	} else if rule == RuleReplace {
		// For replace rule, return the last matching value
		var lastValue interface{}
		start := 0
		for {
			idx := bytes.Index(data[start:], []byte("\n\n"))
			if idx == -1 {
				chunk := data[start:]
				if jsonObj := gjson.GetBytes(chunk, jsonPath); jsonObj.Exists() {
					lastValue = jsonObj.Value()
				}
				break
			}
			chunk := data[start : start+idx]
			if jsonObj := gjson.GetBytes(chunk, jsonPath); jsonObj.Exists() {
				lastValue = jsonObj.Value()
			}
			start += idx + 2
		}
		return lastValue
	} else if rule == RuleAppend {
		// For append rule with large data, use strings.Builder
		var builder strings.Builder
		builder.Grow(len(data) / 2)
		start := 0
		for {
			idx := bytes.Index(data[start:], []byte("\n\n"))
			if idx == -1 {
				chunk := data[start:]
				if jsonObj := gjson.GetBytes(chunk, jsonPath); jsonObj.Exists() {
					builder.WriteString(jsonObj.String())
				}
				break
			}
			chunk := data[start : start+idx]
			if jsonObj := gjson.GetBytes(chunk, jsonPath); jsonObj.Exists() {
				builder.WriteString(jsonObj.String())
			}
			start += idx + 2
		}
		return builder.String()
	}
	log.Errorf("unsupported rule type: %s", rule)
	return nil
}

// Set the tracing span with value.
func setSpanAttribute(key string, value interface{}) {
	if value != "" {
		traceSpanTag := wrapper.TraceSpanTagPrefix + key
		if e := proxywasm.SetProperty([]string{traceSpanTag}, []byte(fmt.Sprint(value))); e != nil {
			log.Warnf("failed to set %s in filter state: %v", traceSpanTag, e)
		}
	} else {
		log.Debugf("failed to write span attribute [%s], because it's value is empty", key)
	}
}

func writeMetric(ctx wrapper.HttpContext, config AIStatisticsConfig) {
	// Generate usage metrics
	var ok bool
	var route, cluster, model string
	consumer := ctx.GetStringContext(ConsumerKey, "none")
	route, ok = ctx.GetContext(RouteName).(string)
	if !ok {
		log.Info("RouteName type assert failed, skip metric record")
		return
	}
	cluster, ok = ctx.GetContext(ClusterName).(string)
	if !ok {
		log.Info("ClusterName type assert failed, skip metric record")
		return
	}

	if config.disableOpenaiUsage {
		return
	}

	if ctx.GetUserAttribute(tokenusage.CtxKeyModel) == nil || ctx.GetUserAttribute(tokenusage.CtxKeyInputToken) == nil || ctx.GetUserAttribute(tokenusage.CtxKeyOutputToken) == nil || ctx.GetUserAttribute(tokenusage.CtxKeyTotalToken) == nil {
		log.Info("get usage information failed, skip metric record")
		return
	}
	model, ok = ctx.GetUserAttribute(tokenusage.CtxKeyModel).(string)
	if !ok {
		log.Info("Model type assert failed, skip metric record")
		return
	}
	if inputToken, ok := convertToUInt(ctx.GetUserAttribute(tokenusage.CtxKeyInputToken)); ok {

	} else {
		log.Info("InputToken type assert failed, skip metric record")
	}
	if outputToken, ok := convertToUInt(ctx.GetUserAttribute(tokenusage.CtxKeyOutputToken)); ok {
		config.incrementCounter(generateMetricName(route, cluster, model, consumer, tokenusage.CtxKeyOutputToken), outputToken)
	} else {
		log.Info("OutputToken type assert failed, skip metric record")
	}
	if totalToken, ok := convertToUInt(ctx.GetUserAttribute(tokenusage.CtxKeyTotalToken)); ok {
		config.incrementCounter(generateMetricName(route, cluster, model, consumer, tokenusage.CtxKeyTotalToken), totalToken)
	} else {
		log.Info("TotalToken type assert failed, skip metric record")
	}

	// Generate duration metrics
	var llmFirstTokenDuration, llmServiceDuration uint64
	// Is stream response
	if ctx.GetUserAttribute(LLMFirstTokenDuration) != nil {
		llmFirstTokenDuration, ok = convertToUInt(ctx.GetUserAttribute(LLMFirstTokenDuration))
		if !ok {
			log.Info("LLMFirstTokenDuration type assert failed")
			return
		}
		config.incrementCounter(generateMetricName(route, cluster, model, consumer, LLMFirstTokenDuration), llmFirstTokenDuration)
		config.incrementCounter(generateMetricName(route, cluster, model, consumer, LLMStreamDurationCount), 1)
	}
	if ctx.GetUserAttribute(LLMServiceDuration) != nil {
		llmServiceDuration, ok = convertToUInt(ctx.GetUserAttribute(LLMServiceDuration))
		if !ok {
			log.Warnf("LLMServiceDuration type assert failed")
			return
		}
		config.incrementCounter(generateMetricName(route, cluster, model, consumer, LLMServiceDuration), llmServiceDuration)
		config.incrementCounter(generateMetricName(route, cluster, model, consumer, LLMDurationCount), 1)
	}
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

// generateRandomString generates a random string with the given length (max 32)
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
		// Generate a random index
		randomIndex, _ := rand.Int(rand.Reader, big.NewInt(int64(len(charset))))
		result[i] = charset[randomIndex.Int64()]
	}

	return string(result)
}

// reportTokenUsage sends token usage data to the reporting API
func reportTokenUsage(config AIStatisticsConfig, ctx wrapper.HttpContext, model string, inputTokens, outputTokens, totalTokens uint64, duration int64) {
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

	// Get Envoy request ID from context
	envoyRequestID := ""
	if requestIDValue := ctx.GetContext(RequestID); requestIDValue != nil {
		if requestIDStr, ok := requestIDValue.(string); ok && requestIDStr != "" {
			envoyRequestID = requestIDStr
			log.Debugf("Using Envoy request ID: %s", envoyRequestID)
		}
	}

	// Generate requestId: use Envoy request ID if available, otherwise fallback to {userKey}-{timestamp}-{random}
	timestamp := time.Now().UnixMilli()
	var requestID string
	if envoyRequestID != "" {
		// Use Envoy's request ID
		requestID = envoyRequestID
	} else {
		// Fallback to generated ID
		randomSuffix := generateRandomString(8) // Generate 8-character random string
		requestID = fmt.Sprintf("%s%d", randomSuffix, timestamp)
	}
	log.Debugf("reportTokenUsage using generated ID: %s", requestID)

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

	// Make HTTP POST call to report API (non-blocking)
	err = config.reportClient.Post(
		requestPath,
		[][2]string{
			{"Content-Type", "application/json"},
		},
		jsonData,
		func(statusCode int, responseHeaders http.Header, responseBody []byte) {
			log.Debugf("Token usage report callback: status=%d, requestId=%s", statusCode, requestID)
		},
		uint32(config.reportTimeout),
	)

	if err != nil {
		log.Errorf("Failed to dispatch token usage report call: %v", err)
		return
	}

	log.Debugf("Token usage report initiated successfully for requestId: %s", requestID)
}

// reportTokenUsageFromContext extracts token usage from context and reports to API
func reportTokenUsageFromContext(ctx wrapper.HttpContext, config AIStatisticsConfig) {
	// Add panic recovery to ensure this function never interrupts the main flow
	defer func() {
		if r := recover(); r != nil {
			log.Errorf("Panic in reportTokenUsageFromContext (recovered): %v", r)
		}
	}()

	// Skip if OpenAI usage is disabled
	if config.disableOpenaiUsage {
		return
	}

	// Skip if report API client is not configured
	if config.reportClient == nil {
		return
	}

	// Get model and token usage from context attributes
	model, ok := ctx.GetUserAttribute(tokenusage.CtxKeyModel).(string)
	if !ok || model == "" {
		log.Debugf("Model not found in context, skipping token usage report")
		return
	}

	inputTokenVal := ctx.GetUserAttribute(tokenusage.CtxKeyInputToken)
	outputTokenVal := ctx.GetUserAttribute(tokenusage.CtxKeyOutputToken)
	totalTokenVal := ctx.GetUserAttribute(tokenusage.CtxKeyTotalToken)

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
	durationVal := ctx.GetUserAttribute(LLMServiceDuration)
	duration, ok := convertToUInt(durationVal)
	if !ok {
		log.Debugf("LLMServiceDuration not found, skipping token usage report")
		return
	}

	// Call the report function with duration (in a goroutine-like manner to avoid blocking)
	// All errors inside reportTokenUsage are handled internally
	reportTokenUsage(config, ctx, model, inputTokens, outputTokens, totalTokens, int64(duration))
}

// · cleans up the context data to free memory
func cleanupContext(ctx wrapper.HttpContext) {
	// Clear all context data to prevent memory leaks
	ctx.SetContext(CtxStreamingBodyBuffer, nil)
	ctx.SetContext(StatisticsRequestStartTime, nil)
	ctx.SetContext(StatisticsFirstTokenTime, nil)
	ctx.SetContext(RouteName, nil)
	ctx.SetContext(ClusterName, nil)
	ctx.SetContext(RequestID, nil)
	ctx.SetContext(ConsumerKey, nil)
	ctx.SetContext(RequestPath, nil)

	// Clear token usage context attributes
	ctx.SetContext(tokenusage.CtxKeyModel, nil)
	ctx.SetContext(tokenusage.CtxKeyInputToken, nil)
	ctx.SetContext(tokenusage.CtxKeyOutputToken, nil)
	ctx.SetContext(tokenusage.CtxKeyTotalToken, nil)

	// Clear all user attributes to prevent memory leaks in high concurrency scenarios
	// These are set via ctx.SetUserAttribute() and can accumulate over time
	ctx.SetUserAttribute(APIName, nil)
	ctx.SetUserAttribute(ResponseType, nil)
	ctx.SetUserAttribute(ChatID, nil)
	ctx.SetUserAttribute(LLMFirstTokenDuration, nil)
	ctx.SetUserAttribute(LLMServiceDuration, nil)
	ctx.SetUserAttribute(ChatRound, nil)
}
