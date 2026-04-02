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
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"strings"

	"github.com/higress-group/proxy-wasm-go-sdk/proxywasm"
	"github.com/higress-group/proxy-wasm-go-sdk/proxywasm/types"
	"github.com/higress-group/wasm-go/pkg/log"
	"github.com/higress-group/wasm-go/pkg/tokenusage"
	"github.com/higress-group/wasm-go/pkg/wrapper"
	"github.com/tidwall/gjson"
	"github.com/tidwall/resp"

	"ai-keyname-quota/util"
)

const (
	pluginName = "ai-keyname-quota"

	// OriginalApiKeyHeader 保存原始API Key的header名称
	OriginalApiKeyHeader = "X-Original-Api-Key"
	// DefaultAuthHeader 默认的认证header名称
	DefaultAuthHeader = "Authorization"
	// DefaultTimeout 默认超时时间(毫秒)
	DefaultTimeout = 5000
)

func main() {}

func init() {
	wrapper.SetCtx(
		pluginName,
		wrapper.ParseConfig(parseConfig),
		wrapper.ProcessRequestHeaders(onHttpRequestHeaders),
		wrapper.ProcessResponseBody(onHttpResponseBody),
		wrapper.ProcessStreamingResponseBody(onHttpStreamingResponseBody),
	)
}

// QuotaPool 配额池配置
type QuotaPool struct {
	// @Title KeyName 前缀
	// @Description 匹配的 keyname 前缀，例如 "外部培训", "内部"
	KeyNamePrefix string `yaml:"key_name_prefix" json:"key_name_prefix"`

	// @Title 配额限制
	// @Description 该配额池的 token 数量限制，例如 1000000 (100w)
	QuotaLimit int `yaml:"quota_limit" json:"quota_limit"`

	// @Title Redis Key 前缀
	// @Description 该配额池在 Redis 中的 key 前缀，默认使用 key_name_prefix
	RedisKeyPrefix string `yaml:"redis_key_prefix" json:"redis_key_prefix"`
}

// KeyNameQuotaConfig 插件配置
type KeyNameQuotaConfig struct {
	// HTTP 客户端（用于查询 keyname）
	apiClient wrapper.HttpClient

	// Redis 客户端（用于配额管理）
	redisClient wrapper.RedisClient

	// @Title 认证Header名称
	// @Description 从哪个请求头获取 API Key，默认为 Authorization
	AuthHeader string `yaml:"authHeader"`

	// @Title API Key参数名称
	// @Description 调用 keyname 查询接口时，传递 API Key 的参数名（已废弃，固定使用 apiKey）
	ApiKeyParamName string `yaml:"apiKeyParamName"`

	// @Title KeyName 查询服务配置
	// @Description 用于查询 keyname 的远端 API 服务配置
	KeyNameService struct {
		// @Title 服务名称
		// @Description Higress Console 中的服务名称
		ServiceName string `yaml:"service_name"`

		// @Title API 完整 URL
		// @Description 查询 keyname 接口的完整 URL
		ApiUrl string `yaml:"api_url"`

		// @Title 超时时间(毫秒)
		// @Description HTTP 调用超时时间，默认为 5000ms
		Timeout int64 `yaml:"timeout"`
	} `yaml:"keyNameService"`

	// @Title Redis 配置
	// @Description Redis 连接配置
	Redis struct {
		// @Title 服务名称
		ServiceName string `yaml:"service_name"`

		// @Title 服务端口
		ServicePort int `yaml:"service_port"`

		// @Title 用户名
		Username string `yaml:"username"`

		// @Title 密码
		Password string `yaml:"password"`

		// @Title 超时时间
		Timeout int `yaml:"timeout"`

		// @Title 数据库
		Database int `yaml:"database"`
	} `yaml:"redis"`

	// @Title 配额池列表
	// @Description 配额池配置列表，按顺序匹配
	QuotaPools []QuotaPool `yaml:"quota_pools"`

	// @Title 全局 Redis Key 前缀
	// @Description 所有配额池的 Redis key 前缀，默认为 "keyname_quota:"
	GlobalRedisKeyPrefix string `yaml:"global_redis_key_prefix"`

	// @Title 未匹配时的行为
	// @Description 当 keyname 不匹配任何配额池时的行为：continue(放行) 或 reject(拒绝)，默认为 continue
	UnmatchedAction string `yaml:"unmatched_action"`
}

func parseConfig(json gjson.Result, config *KeyNameQuotaConfig) error {
	log.Debugf("parse config()")

	// 解析 authHeader，默认为 Authorization
	config.AuthHeader = json.Get("authHeader").String()
	if config.AuthHeader == "" {
		config.AuthHeader = DefaultAuthHeader
	}

	// 解析 apiKeyParamName，默认为 key
	config.ApiKeyParamName = json.Get("apiKeyParamName").String()
	if config.ApiKeyParamName == "" {
		config.ApiKeyParamName = "key"
	}

	// 解析 KeyName 查询服务配置（遵循 OpenApiKeyEndpoint.info 接口规范）
	keyNameService := json.Get("keyNameService")
	if !keyNameService.Exists() {
		return errors.New("missing keyNameService in config")
	}

	config.KeyNameService.ServiceName = keyNameService.Get("service_name").String()
	if config.KeyNameService.ServiceName == "" {
		return errors.New("keyNameService.service_name is required")
	}

	config.KeyNameService.ApiUrl = keyNameService.Get("api_url").String()
	if config.KeyNameService.ApiUrl == "" {
		return errors.New("keyNameService.api_url is required")
	}

	config.KeyNameService.Timeout = keyNameService.Get("timeout").Int()
	if config.KeyNameService.Timeout == 0 {
		config.KeyNameService.Timeout = DefaultTimeout
	}

	// 解析 Redis 配置（参考 ai-quota）
	redisConfig := json.Get("redis")
	if !redisConfig.Exists() {
		return errors.New("missing redis in config")
	}

	serviceName := redisConfig.Get("service_name").String()
	if serviceName == "" {
		return errors.New("redis service_name is required")
	}

	servicePort := int(redisConfig.Get("service_port").Int())
	if servicePort == 0 {
		if strings.HasSuffix(serviceName, ".static") {
			// use default logic port which is 80 for static service
			servicePort = 80
		} else {
			servicePort = 6379
		}
	}

	username := redisConfig.Get("username").String()
	password := redisConfig.Get("password").String()
	timeout := int(redisConfig.Get("timeout").Int())
	if timeout == 0 {
		timeout = 1000
	}
	database := int(redisConfig.Get("database").Int())

	config.Redis.ServiceName = serviceName
	config.Redis.ServicePort = servicePort
	config.Redis.Username = username
	config.Redis.Password = password
	config.Redis.Timeout = timeout
	config.Redis.Database = database

	// 解析配额池配置
	quotaPools := json.Get("quota_pools")
	if !quotaPools.Exists() || len(quotaPools.Array()) == 0 {
		return errors.New("missing quota_pools in config")
	}

	for _, pool := range quotaPools.Array() {
		keyNamePrefix := pool.Get("key_name_prefix").String()
		quotaLimit := int(pool.Get("quota_limit").Int())
		redisKeyPrefix := pool.Get("redis_key_prefix").String()

		if keyNamePrefix == "" {
			return errors.New("quota_pools.key_name_prefix is required")
		}
		if quotaLimit <= 0 {
			return errors.New("quota_pools.quota_limit must be greater than 0")
		}

		// 如果没有指定 redis_key_prefix，使用 key_name_prefix
		if redisKeyPrefix == "" {
			redisKeyPrefix = keyNamePrefix
		}

		config.QuotaPools = append(config.QuotaPools, QuotaPool{
			KeyNamePrefix:  keyNamePrefix,
			QuotaLimit:     quotaLimit,
			RedisKeyPrefix: redisKeyPrefix,
		})
	}

	// 解析全局 Redis Key 前缀
	config.GlobalRedisKeyPrefix = json.Get("global_redis_key_prefix").String()
	if config.GlobalRedisKeyPrefix == "" {
		config.GlobalRedisKeyPrefix = "keyname_quota:"
	}

	// 解析未匹配时的行为
	config.UnmatchedAction = json.Get("unmatched_action").String()
	if config.UnmatchedAction == "" {
		config.UnmatchedAction = "continue"
	}
	if config.UnmatchedAction != "continue" && config.UnmatchedAction != "reject" {
		return errors.New("unmatched_action must be 'continue' or 'reject'")
	}

	// 初始化 KeyName 查询服务 HTTP 客户端（参考 consumer-group-mapping）
	// 从完整的 URL 中提取域名和端口
	var err error
	parsedUrl, err := url.Parse(config.KeyNameService.ApiUrl)
	if err != nil {
		return errors.New("invalid keyNameService.api_url: " + err.Error())
	}

	actualHost := parsedUrl.Hostname()
	if actualHost == "" {
		return errors.New("invalid keyNameService.api_url: missing hostname")
	}

	port := int64(80)
	if parsedUrl.Scheme == "https" {
		port = 443
	}
	if actualPort := parsedUrl.Port(); actualPort != "" {
		if p, err := strconv.Atoi(actualPort); err == nil {
			port = int64(p)
		}
	}

	config.apiClient = wrapper.NewClusterClient(wrapper.DnsCluster{
		ServiceName: config.KeyNameService.ServiceName,
		Domain:      actualHost,
		Port:        port,
	})

	log.Debugf("HTTP client initialized: serviceName=%s, domain=%s, port=%d",
		config.KeyNameService.ServiceName, actualHost, port)

	config.redisClient = wrapper.NewRedisClusterClient(wrapper.FQDNCluster{
		FQDN: config.Redis.ServiceName,
		Port: int64(config.Redis.ServicePort),
	})

	log.Debugf("Redis client initialized: ServiceName=%s, port=%d, timeout=%dms, database=%d",
		config.Redis.ServiceName, config.Redis.ServicePort, config.Redis.Timeout, config.Redis.Database)

	err = config.redisClient.Init(config.Redis.Username, config.Redis.Password,
		int64(config.Redis.Timeout), wrapper.WithDataBase(config.Redis.Database))
	if err != nil {
		return errors.New("failed to init redis client: " + err.Error())
	}

	log.Debugf("Redis client init successful")

	log.Debugf("ai-keyname-quota plugin loaded:")
	log.Debugf("  authHeader: %s", config.AuthHeader)
	log.Debugf("  quotaPools count: %d", len(config.QuotaPools))
	//for i, pool := range config.QuotaPools {
	//	log.Debugf("  quota_pool[%d]: prefix=%s, limit=%d", i, pool.KeyNamePrefix, pool.QuotaLimit)
	//}

	return nil
}

func onHttpRequestHeaders(ctx wrapper.HttpContext, config KeyNameQuotaConfig) types.Action {
	ctx.DisableReroute()
	log.Debugf("onHttpRequestHeaders()")

	// 1. 获取原始 API Key
	// 优先从 X-Original-Api-Key header 获取（由 consumer-group-mapping 等插件设置）
	// 如果没有，则从 Authorization header 提取
	apiKey, err := proxywasm.GetHttpRequestHeader(OriginalApiKeyHeader)
	if err != nil || apiKey == "" {
		// 如果 X-Original-Api-Key 不存在，尝试从 Authorization header 提取
		authValue, err := proxywasm.GetHttpRequestHeader(config.AuthHeader)
		if err != nil || authValue == "" {
			log.Debugf("no %s or %s header, skipping", OriginalApiKeyHeader, config.AuthHeader)
			return types.ActionContinue
		}

		apiKey = extractApiKey(authValue)
		if apiKey == "" {
			log.Debugf("failed to extract API key from %s, skipping", config.AuthHeader)
			return types.ActionContinue
		}
	}

	log.Debugf("got api_key: %s from header: %s", maskApiKey(apiKey), OriginalApiKeyHeader)

	// 保存原始 API Key
	ctx.SetContext("api_key", apiKey)

	// 2. 调用远端 API 查询 keyname（遵循 OpenApiKeyEndpoint.info 接口规范）
	// 接口规范: GET {api_url}?apiKey={apiKey}
	// 示例: https://pref-gate.wujieai.com/open-platform-private/apiKey/info?apiKey={apiKey}
	// 返回格式: {"id": 123, "apiKeyName": "外部培训-key-001", "appid": "app-001", "apiKey": "sk-xxx", "groups": []}
	parsedUrl, _ := url.Parse(config.KeyNameService.ApiUrl)
	requestPath := parsedUrl.Path
	if requestPath == "" {
		requestPath = "/"
	}

	// 添加查询参数（使用固定的 apiKey 参数名）
	if strings.Contains(requestPath, "?") {
		requestPath += "&apiKey=" + url.QueryEscape(apiKey)
	} else {
		requestPath += "?apiKey=" + url.QueryEscape(apiKey)
	}

	log.Debugf("querying keyname for api_key: %s", maskApiKey(apiKey))
	log.Debugf("HTTP client config: serviceName=%s, timeout=%dms", config.KeyNameService.ServiceName, config.KeyNameService.Timeout)
	log.Debugf("HTTP request: path=%s", requestPath)

	// 3. 发起异步 HTTP 调用
	log.Debugf("initiating HTTP GET request...")
	err = config.apiClient.Get(
		requestPath,
		nil,
		func(statusCode int, responseHeaders http.Header, responseBody []byte) {
			log.Debugf("keyname query callback, status=%d", statusCode)

			defer func() {
				if err := recover(); err != nil {
					log.Errorf("panic in callback: %v", err)
				}
			}()

			if statusCode != http.StatusOK {
				log.Debugf("keyname query API error: status=%d, response=%s, resuming request",
					statusCode, string(responseBody))
				// API 调用失败，放行请求
				proxywasm.ResumeHttpRequest()
				return
			}

			// 4. 解析响应获取 keyname
			// 接口返回格式: {"id": 123, "apiKeyName": "外部培训-key-001", "appid": "app-001", "apiKey": "sk-xxx", "groups": []}
			responseStr := string(responseBody)
			if responseStr == "" {
				log.Debugf("received empty response body, resuming request")
				proxywasm.ResumeHttpRequest()
				return
			}

			result := gjson.Parse(responseStr)
			if !result.Exists() || result.Type == gjson.Null {
				log.Debugf("received null or invalid response, resuming request")
				proxywasm.ResumeHttpRequest()
				return
			}

			// 提取 keyname（遵循 OpenApiKeyResponse 规范）
			var keyname string

			// 优先使用 apiKeyName 字段（标准字段）
			keyname = result.Get("api_key_name").String()

			// 如果 apiKeyName 为空，尝试其他字段（兼容性处理）
			if keyname == "" {
				keyname = result.Get("api_key_name").String()
			}

			if keyname == "" {
				log.Debugf("keyname not found in response, resuming request")
				proxywasm.ResumeHttpRequest()
				return
			}

			log.Debugf("queried keyname: %s for api_key: %s", keyname, maskApiKey(apiKey))
			ctx.SetContext("keyname", keyname)

			// 5. 匹配配额池
			matchedPool := matchQuotaPool(config, keyname)
			if matchedPool == nil {
				log.Debugf("keyname '%s' does not match any quota pool, resuming request", keyname)
				// 不匹配则放行，不进行限额检查
				proxywasm.ResumeHttpRequest()
				return
			}

			log.Debugf("keyname '%s' matched quota pool: prefix=%s, limit=%d",
				keyname, matchedPool.KeyNamePrefix, matchedPool.QuotaLimit)

			ctx.SetContext("matched_pool", matchedPool)
			ctx.SetContext("keyname", keyname)

			// 6. 检查配额（参考 ai-quota）
			apiKey := ctx.GetContext("api_key").(string)
			redisKey := config.GlobalRedisKeyPrefix + matchedPool.RedisKeyPrefix + ":" + apiKey
			log.Debugf("checking quota for keyname '%s', redis_key=%s", keyname, redisKey)

			config.redisClient.Get(redisKey, func(response resp.Value) {
				log.Debugf("redis get callback for keyname '%s', redis_key=%s", keyname, redisKey)

				// 如果 Redis 出错，放行请求
				if err := response.Error(); err != nil {
					log.Debugf("redis get error: %v, resuming request", err)
					proxywasm.ResumeHttpRequest()
					return
				}

				// 如果 key 不存在（首次访问），初始化配额并放行
				if response.IsNull() {
					log.Debugf("redis key not found (first access), initializing quota: %d", matchedPool.QuotaLimit)

					config.redisClient.Set(redisKey, matchedPool.QuotaLimit, func(setResponse resp.Value) {
						if err := setResponse.Error(); err != nil {
							log.Debugf("failed to initialize quota: %v, resuming request", err)
						} else {
							log.Debugf("quota initialized successfully for keyname '%s': %d", keyname, matchedPool.QuotaLimit)
						}
						// 无论初始化成功与否，都放行请求
						proxywasm.ResumeHttpRequest()
					})
					return
				}

				// 如果配额耗尽（值 <= 0），拒绝请求
				if response.Integer() <= 0 {
					log.Debugf("quota exhausted for keyname '%s': %d", keyname, response.Integer())
					util.SendResponse(http.StatusForbidden,
						"ai-keyname-quota.no_quota",
						"text/plain",
						fmt.Sprintf("Quota exhausted for keyname '%s' (prefix: %s)",
							keyname, matchedPool.KeyNamePrefix))
					return
				}

				// 配额足够，放行请求
				log.Debugf("quota check passed for keyname '%s': remaining=%d",
					keyname, response.Integer())
				proxywasm.ResumeHttpRequest()
			})
		},
		uint32(config.KeyNameService.Timeout),
	)

	if err != nil {
		log.Errorf("keyname query HTTP call failed: %v", err)
		if config.UnmatchedAction == "reject" {
			util.SendResponse(http.StatusServiceUnavailable,
				"ai-keyname-quota.http_error",
				"text/plain",
				"Failed to query keyname from remote API")
		} else {
			log.Debugf("HTTP call failed but unmatched_action is continue, resuming request")
			proxywasm.ResumeHttpRequest()
			return types.ActionContinue
		}
		return types.ActionPause
	}

	log.Debugf("HTTP call initiated successfully, pausing request")
	return types.HeaderStopAllIterationAndWatermark
}

func onHttpResponseBody(ctx wrapper.HttpContext, config KeyNameQuotaConfig, body []byte) types.Action {
	log.Debugf("onHttpResponseBody() called, body length=%d", len(body))

	// 处理非流式响应的配额扣减
	matchedPool := ctx.GetContext("matched_pool")
	if matchedPool == nil {
		log.Debugf("onHttpResponseBody(): no matched_pool found, skipping")
		return types.ActionContinue
	}

	pool, ok := matchedPool.(*QuotaPool)
	if !ok {
		log.Debugf("onHttpResponseBody(): failed to cast matched_pool to *QuotaPool, skipping")
		return types.ActionContinue
	}

	// 提取 token 使用量
	if usage := tokenusage.GetTokenUsage(ctx, body); usage.TotalToken > 0 {
		inputToken := usage.InputToken
		outputToken := usage.OutputToken
		keyname := ctx.GetContext("keyname").(string)
		apiKey := ctx.GetContext("api_key").(string)
		totalToken := int(inputToken + outputToken)

		redisKey := config.GlobalRedisKeyPrefix + pool.RedisKeyPrefix + ":" + apiKey
		log.Debugf("deducting quota (non-stream) for keyname '%s': tokens=%d, redis_key=%s",
			keyname, totalToken, redisKey)

		config.redisClient.DecrBy(redisKey, totalToken, nil)

		log.Debugf("onHttpResponseBody(): quota deducted successfully for keyname '%s'", keyname)
	} else {
		log.Debugf("onHttpResponseBody(): no token usage found in response")
	}

	log.Debugf("onHttpResponseBody() completed")
	return types.ActionContinue
}

func onHttpStreamingResponseBody(ctx wrapper.HttpContext, config KeyNameQuotaConfig, data []byte, endOfStream bool) []byte {
	log.Debugf("onHttpStreamingResponseBody() called, data length=%d, endOfStream=%v", len(data), endOfStream)

	// 1. 提取 token 使用量（参考 ai-quota）
	if usage := tokenusage.GetTokenUsage(ctx, data); usage.TotalToken > 0 {
		ctx.SetContext(tokenusage.CtxKeyInputToken, usage.InputToken)
		ctx.SetContext(tokenusage.CtxKeyOutputToken, usage.OutputToken)
		log.Debugf("onHttpStreamingResponseBody(): token usage updated, input=%d, output=%d, total=%d",
			usage.InputToken, usage.OutputToken, usage.TotalToken)
	}

	// 2. 在流结束时扣减配额（参考 ai-quota）
	if !endOfStream {
		log.Debugf("onHttpStreamingResponseBody(): stream not ended yet, skipping quota deduction")
		return data
	}

	log.Debugf("onHttpStreamingResponseBody(): stream ended, processing quota deduction")

	matchedPool := ctx.GetContext("matched_pool")
	if matchedPool == nil {
		log.Debugf("onHttpStreamingResponseBody(): no matched_pool found, skipping quota deduction")
		return data
	}

	pool, ok := matchedPool.(*QuotaPool)
	if !ok {
		log.Debugf("onHttpStreamingResponseBody(): failed to cast matched_pool to *QuotaPool, skipping")
		return data
	}

	if ctx.GetContext(tokenusage.CtxKeyInputToken) == nil ||
		ctx.GetContext(tokenusage.CtxKeyOutputToken) == nil {
		log.Debugf("onHttpStreamingResponseBody(): token context not found, skipping quota deduction")
		return data
	}

	inputToken := ctx.GetContext(tokenusage.CtxKeyInputToken).(int64)
	outputToken := ctx.GetContext(tokenusage.CtxKeyOutputToken).(int64)
	keyname := ctx.GetContext("keyname").(string)
	apiKey := ctx.GetContext("api_key").(string)
	totalToken := int(inputToken + outputToken)

	redisKey := config.GlobalRedisKeyPrefix + pool.RedisKeyPrefix + ":" + apiKey
	log.Debugf("deducting quota for keyname '%s': tokens=%d, redis_key=%s",
		keyname, totalToken, redisKey)

	config.redisClient.DecrBy(redisKey, totalToken, nil)

	log.Debugf("onHttpStreamingResponseBody() completed, quota deducted for keyname '%s'", keyname)
	return data
}

// matchQuotaPool 根据 keyname 前缀匹配配额池
func matchQuotaPool(config KeyNameQuotaConfig, keyname string) *QuotaPool {
	for i := range config.QuotaPools {
		pool := &config.QuotaPools[i]
		if strings.HasPrefix(keyname, pool.KeyNamePrefix) {
			return pool
		}
	}
	return nil
}

// extractApiKey 从 Authorization header 中提取 API Key（参考 consumer-group-mapping）
func extractApiKey(authValue string) string {
	authValue = strings.TrimSpace(authValue)

	if strings.HasPrefix(strings.ToLower(authValue), "bearer ") {
		parts := strings.SplitN(authValue, " ", 2)
		if len(parts) == 2 {
			return strings.TrimSpace(parts[1])
		}
	}

	return authValue
}

// maskApiKey 遮蔽 API Key 用于日志输出（参考 consumer-group-mapping）
func maskApiKey(apiKey string) string {
	if len(apiKey) <= 8 {
		return "****"
	}
	return apiKey[:4] + "..." + apiKey[len(apiKey)-4:]
}
