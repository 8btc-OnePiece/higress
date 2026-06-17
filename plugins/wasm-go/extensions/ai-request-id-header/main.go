package main

import (
	"strings"

	"github.com/higress-group/proxy-wasm-go-sdk/proxywasm"
	"github.com/higress-group/proxy-wasm-go-sdk/proxywasm/types"
	"github.com/higress-group/wasm-go/pkg/log"
	"github.com/higress-group/wasm-go/pkg/wrapper"
)

const (
	responseRequestIDHeader = "request_id"
	upstreamRequestIDHeader = "x-request-id"
)

type RequestIDHeaderConfig struct{}

func main() {}

func init() {
	wrapper.SetCtx(
		"ai-request-id-header",
		wrapper.ProcessRequestHeaders(onHttpRequestHeaders),
		wrapper.ProcessResponseHeaders(onHttpResponseHeaders),
	)
}

func onHttpRequestHeaders(_ wrapper.HttpContext, _ RequestIDHeaderConfig) types.Action {
	requestID := getRequestID()
	if requestID == "" {
		return types.ActionContinue
	}

	if err := proxywasm.ReplaceHttpRequestHeader(upstreamRequestIDHeader, requestID); err != nil {
		log.Warnf("ai-request-id-header: failed to set upstream request header: %v", err)
	}

	return types.ActionContinue
}

func onHttpResponseHeaders(_ wrapper.HttpContext, _ RequestIDHeaderConfig) types.Action {
	requestID := getRequestID()
	if requestID == "" {
		return types.ActionContinue
	}

	if err := proxywasm.ReplaceHttpResponseHeader(responseRequestIDHeader, requestID); err != nil {
		log.Warnf("ai-request-id-header: failed to set response header: %v", err)
	}

	return types.ActionContinue
}

func getRequestID() string {
	if value, err := proxywasm.GetProperty([]string{"x_request_id"}); err == nil {
		if requestID := normalizeRequestID(string(value)); requestID != "" {
			return requestID
		}
	}

	if value, err := proxywasm.GetHttpRequestHeader("x-request-id"); err == nil {
		return normalizeRequestID(value)
	}

	return ""
}

func normalizeRequestID(value string) string {
	requestID := strings.TrimSpace(value)
	if requestID == "" || requestID == "-" {
		return ""
	}
	return requestID
}
