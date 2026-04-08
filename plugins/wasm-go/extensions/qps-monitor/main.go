// Copyright (c) 2024 Alibaba Group Holding Ltd.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package main

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/higress-group/proxy-wasm-go-sdk/proxywasm"
	"github.com/higress-group/proxy-wasm-go-sdk/proxywasm/types"
	"github.com/higress-group/wasm-go/pkg/log"
	"github.com/higress-group/wasm-go/pkg/wrapper"
	"github.com/tidwall/gjson"
)

func main() {}

func init() {
	wrapper.SetCtx(
		"qps-monitor",
		wrapper.ParseConfig(parseConfig),
		wrapper.ProcessRequestHeaders(onHttpRequestHeaders),
		wrapper.ProcessResponseHeaders(onHttpResponseHeaders),
	)
}

// 已知 AI API 路径后缀，与 ai-proxy 保持一致
var aiApiPathSuffixes = []string{
	// OpenAI
	"/v1/chat/completions",
	"/v1/completions",
	"/v1/embeddings",
	"/v1/audio/speech",
	"/v1/images/generations",
	"/v1/images/edits",
	"/v1/images/variations",
	"/v1/batches",
	"/v1/files",
	"/v1/models",
	"/v1/fine_tuning/jobs",
	"/v1/responses",
	"/v1/videos",
	// Anthropic
	"/v1/messages",
	"/v1/complete",
	// Cohere
	"/v1/rerank",
}

// isAiApiRequest 判断请求路径是否为已知 AI API
func isAiApiRequest(requestPath string) bool {
	path := requestPath
	if idx := strings.Index(path, "?"); idx != -1 {
		path = path[:idx]
	}
	for _, suffix := range aiApiPathSuffixes {
		if strings.HasSuffix(path, suffix) {
			return true
		}
	}
	// Gemini: /.../models/xxx:generateContent 或 :streamGenerateContent
	if strings.Contains(path, ":generateContent") || strings.Contains(path, ":streamGenerateContent") {
		return true
	}
	return false
}

const (
	RouteName   = "route"
	ClusterName = "cluster"
	APIName     = "api"
	ConsumerKey = "x-mse-consumer"
	PathKey     = "path"
	SkipKey     = "skip_ai"

	// Metric names
	MetricRequestTotal    = "request_total"
	MetricRequestSuccess  = "request_success"
	MetricRequestError4xx = "request_error_4xx"
	MetricRequestError5xx = "request_error_5xx"
)

type QpsMonitorConfig struct {
	counterMetrics map[string]proxywasm.MetricCounter
}

func generateMetricName(route, cluster, metricName, requestPath string) string {
	// 和 ai-statistics 完全一致的命名模式（含 model 占位），Envoy tag extraction 才能正确提取所有维度:
	//   route.{route}.upstream.{cluster}.model.{model}.consumer.{consumer}.path.{path}.metric.{metricName}
	// qps-monitor 没有 model 维度，用 "none" 占位
	return fmt.Sprintf("route.%s.upstream.%s.model.none.consumer.%s.metric.%s",
		sanitizeMetricLabel(route), sanitizeMetricLabel(cluster), sanitizeMetricLabel(requestPath), metricName)
}

func getRouteName() (string, error) {
	if raw, err := proxywasm.GetProperty([]string{"route_name"}); err != nil {
		return "unknown", err
	} else {
		return string(raw), nil
	}
}

func getClusterName() (string, error) {
	if raw, err := proxywasm.GetProperty([]string{"cluster_name"}); err != nil {
		return "unknown", err
	} else {
		return string(raw), nil
	}
}

func (config *QpsMonitorConfig) incrementCounter(route, cluster, metricName, requestPath string, inc uint64) {
	if inc == 0 {
		return
	}
	// 每组 (route, cluster, consumer, metric, path) 对应一个独立的 counter
	fullMetricName := generateMetricName(route, cluster, metricName, requestPath)
	counter, ok := config.counterMetrics[fullMetricName]
	if !ok {
		counter = proxywasm.DefineCounterMetric(fullMetricName)
		config.counterMetrics[fullMetricName] = counter
	}
	counter.Increment(inc)
}

// sanitizeMetricLabel 替换指标名称中的特殊字符
func sanitizeMetricLabel(s string) string {
	result := make([]byte, 0, len(s))
	for i := 0; i < len(s); i++ {
		c := s[i]
		if (c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z') || (c >= '0' && c <= '9') || c == '_' || c == '-' || c == '.' {
			result = append(result, c)
		} else {
			result = append(result, '_')
		}
	}
	return string(result)
}

func parseConfig(configJson gjson.Result, config *QpsMonitorConfig) error {
	config.counterMetrics = make(map[string]proxywasm.MetricCounter)
	log.Infof("qps-monitor: config parsed")
	return nil
}

func onHttpRequestHeaders(ctx wrapper.HttpContext, config QpsMonitorConfig) types.Action {
	// 获取请求路径
	requestPath, _ := proxywasm.GetHttpRequestHeader(":path")
	if isAiApiRequest(requestPath) {
		// 跳过已知 AI API 请求，不统计
		ctx.SetContext(SkipKey, true)
		ctx.DontReadResponseBody()
		return types.ActionContinue
	}
	route, _ := getRouteName()
	cluster, _ := getClusterName()

	// 去掉 query string
	path := requestPath
	if idx := strings.Index(path, "?"); idx != -1 {
		path = path[:idx]
	}

	// 存储到 context 中，供响应阶段使用
	ctx.SetContext(RouteName, route)
	ctx.SetContext(ClusterName, cluster)
	ctx.SetContext(PathKey, path)

	// 立即增加请求计数（QPS 统计）
	config.incrementCounter(route, cluster, MetricRequestTotal, path, 1)

	return types.ActionContinue
}

func onHttpResponseHeaders(ctx wrapper.HttpContext, config QpsMonitorConfig) types.Action {
	// 如果是 AI API 请求，跳过统计
	if skip, _ := ctx.GetContext(SkipKey).(bool); skip {
		return types.ActionContinue
	}

	// 获取响应状态码
	statusCode, err := proxywasm.GetHttpResponseHeader(":status")
	if err != nil {
		log.Warnf("qps-monitor: failed to get response status: %v", err)
		return types.ActionContinue
	}

	// 从 context 获取维度信息
	route, _ := ctx.GetContext(RouteName).(string)
	cluster, _ := ctx.GetContext(ClusterName).(string)
	path, _ := ctx.GetContext(PathKey).(string)

	if route == "" {
		route = "unknown"
	}
	if cluster == "" {
		cluster = "unknown"
	}

	// 根据状态码增加对应计数
	status, err := strconv.Atoi(statusCode)
	if err != nil {
		log.Warnf("qps-monitor: invalid status code: %s", statusCode)
		return types.ActionContinue
	}

	switch {
	case status >= 200 && status < 400:
		config.incrementCounter(route, cluster, MetricRequestSuccess, path, 1)
	case status >= 400 && status < 500:
		config.incrementCounter(route, cluster, MetricRequestError4xx, path, 1)
	case status >= 500:
		config.incrementCounter(route, cluster, MetricRequestError5xx, path, 1)
	}

	return types.ActionContinue
}
