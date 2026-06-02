// Copyright (c) 2025 Alibaba Group Holding Ltd.
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
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"strconv"
	"time"

	"github.com/higress-group/proxy-wasm-go-sdk/proxywasm"
	"github.com/higress-group/proxy-wasm-go-sdk/proxywasm/types"
	"github.com/higress-group/wasm-go/pkg/log"
	"github.com/higress-group/wasm-go/pkg/wrapper"
	"github.com/tidwall/gjson"
)

func main() {}

func init() {
	wrapper.SetCtx(
		"custom-sign-auth",
		wrapper.ParseConfig(parseConfig),
		wrapper.ProcessRequestHeadersBy(onHttpRequestHeaders),
		wrapper.ProcessRequestBodyBy(onHttpRequestBody),
	)
}

const (
	pluginName = "custom-sign-auth"

	// 签名请求头名称
	HeaderAccessKey   = "X-Access-Key"
	HeaderTimestamp   = "X-Access-Timestamp"
	HeaderSignature   = "X-Access-Signature"
	HeaderContentType = "Content-Type"
)

// CustomSignAuthConfig 插件配置
type CustomSignAuthConfig struct {
	// @Title Access Key
	// @Description 用于签名的 Access Key
	AccessKey string `yaml:"access_key" json:"access_key"`

	// @Title Secret Key
	// @Description 用于生成签名的 Secret Key
	SecretKey string `yaml:"secret_key" json:"secret_key"`

	// @Title 是否启用签名
	// @Description 是否启用签名功能，默认为 true
	Enabled bool `yaml:"enabled" json:"enabled"`

	// @Title 是否覆盖已存在的签名头
	// @Description 如果请求中已存在签名头，是否覆盖，默认为 true
	OverrideExisting bool `yaml:"override_existing" json:"override_existing"`
}

func parseConfig(json gjson.Result, config *CustomSignAuthConfig) error {
	// 解析 access_key
	config.AccessKey = json.Get("access_key").String()
	if config.AccessKey == "" {
		return fmt.Errorf("access_key is required")
	}

	// 解析 secret_key
	config.SecretKey = json.Get("secret_key").String()
	if config.SecretKey == "" {
		return fmt.Errorf("secret_key is required")
	}

	// 解析 enabled，默认为 true
	config.Enabled = json.Get("enabled").Bool()
	if !json.Get("enabled").Exists() {
		config.Enabled = true
	}

	// 解析 override_existing，默认为 true
	config.OverrideExisting = json.Get("override_existing").Bool()
	if !json.Get("override_existing").Exists() {
		config.OverrideExisting = true
	}

	log.Infof("custom-sign-auth plugin loaded: enabled=%v, access_key=%s, override_existing=%v",
		config.Enabled, config.AccessKey, config.OverrideExisting)

	return nil
}

func onHttpRequestHeaders(ctx wrapper.HttpContext, config CustomSignAuthConfig, log log.Log) types.Action {
	// 如果禁用签名功能，直接放行
	if !config.Enabled {
		return types.ActionContinue
	}

	// 检查是否需要覆盖已存在的签名头
	if !config.OverrideExisting {
		// 检查是否已存在签名头
		existingKey, err := proxywasm.GetHttpRequestHeader(HeaderAccessKey)
		if err == nil && existingKey != "" {
			log.Debugf("custom-sign-auth: existing signature headers found, skipping (override_existing=false)")
			return types.ActionContinue
		}
	}

	// 生成时间戳并保存到上下文
	timestamp := strconv.FormatInt(time.Now().Unix(), 10)
	ctx.SetContext("timestamp", timestamp)

	log.Debugf("custom-sign-auth: timestamp=%s, pausing to read request body", timestamp)

	// 返回 ActionPause 暂停请求处理
	// 这样 onHttpRequestBody 会被调用，我们可以在那里添加请求头
	return types.ActionPause
}

func onHttpRequestBody(ctx wrapper.HttpContext, config CustomSignAuthConfig, body []byte, log log.Log) types.Action {
	if !config.Enabled {
		// 如果未启用，恢复请求
		proxywasm.ResumeHttpRequest()
		return types.ActionContinue
	}

	// 从上下文获取时间戳
	timestampValue := ctx.GetContext("timestamp")
	if timestampValue == nil {
		log.Errorf("custom-sign-auth: timestamp not found in context")
		proxywasm.ResumeHttpRequest()
		return types.ActionContinue
	}

	timestamp, ok := timestampValue.(string)
	if !ok {
		log.Errorf("custom-sign-auth: invalid timestamp type in context")
		proxywasm.ResumeHttpRequest()
		return types.ActionContinue
	}

	// 将请求体转换为字符串
	bodyStr := string(body)

	log.Debugf("custom-sign-auth: generating signature, body length=%d", len(bodyStr))

	// 生成签名
	signature := generateSignature(config.SecretKey, timestamp, bodyStr)

	log.Debugf("custom-sign-auth: generated signature=%s", maskSignature(signature))

	// 在请求体阶段，在恢复请求之前添加签名头
	// 注意：这需要在调用 ResumeHttpRequest() 之前完成

	// 先移除已存在的签名头（如果有）
	proxywasm.RemoveHttpRequestHeader(HeaderAccessKey)
	proxywasm.RemoveHttpRequestHeader(HeaderTimestamp)
	proxywasm.RemoveHttpRequestHeader(HeaderSignature)

	// 添加签名请求头
	// 根据 Proxy-Wasm 的设计，在 onHttpRequestBody 中添加请求头需要特殊处理
	// 我们先尝试添加，看是否能成功
	if err := proxywasm.AddHttpRequestHeader(HeaderAccessKey, config.AccessKey); err != nil {
		log.Errorf("custom-sign-auth: failed to add %s header: %v", HeaderAccessKey, err)
	}
	if err := proxywasm.AddHttpRequestHeader(HeaderTimestamp, timestamp); err != nil {
		log.Errorf("custom-sign-auth: failed to add %s header: %v", HeaderTimestamp, err)
	}
	if err := proxywasm.AddHttpRequestHeader(HeaderSignature, signature); err != nil {
		log.Errorf("custom-sign-auth: failed to add %s header: %v", HeaderSignature, err)
	}

	// 确保设置 Content-Type（如果请求体不为空）
	if len(bodyStr) > 0 {
		existingContentType, _ := proxywasm.GetHttpRequestHeader(HeaderContentType)
		if existingContentType == "" {
			proxywasm.RemoveHttpRequestHeader(HeaderContentType)
			if err := proxywasm.AddHttpRequestHeader(HeaderContentType, "application/json"); err != nil {
				log.Errorf("custom-sign-auth: failed to set %s header: %v", HeaderContentType, err)
			}
		}
	}

	log.Infof("custom-sign-auth: signature headers added, resuming request")

	// 恢复请求处理
	proxywasm.ResumeHttpRequest()
	return types.ActionContinue
}

// generateSignature 根据签名规则生成 HMAC-SHA256 签名
// 签名规则：signature = HMAC-SHA256(secret_key, timestamp + "\n" + body)
func generateSignature(secretKey, timestamp, body string) string {
	// 按照签名规则：timestamp + "\n" + body
	signText := timestamp + "\n" + body

	// 使用 HMAC-SHA256 生成签名
	mac := hmac.New(sha256.New, []byte(secretKey))
	mac.Write([]byte(signText))
	signature := hex.EncodeToString(mac.Sum(nil))

	return signature
}

// maskSignature 遮蔽签名用于日志输出（只显示前8位和后8位）
func maskSignature(signature string) string {
	if len(signature) <= 16 {
		return "****"
	}
	return signature[:8] + "..." + signature[len(signature)-8:]
}
