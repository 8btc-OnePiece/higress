package main

import (
	"strings"

	"github.com/higress-group/proxy-wasm-go-sdk/proxywasm"
	"github.com/higress-group/proxy-wasm-go-sdk/proxywasm/types"
	"github.com/higress-group/wasm-go/pkg/log"
	"github.com/higress-group/wasm-go/pkg/wrapper"
)

const requestIDHeader = "request_id"

type RequestIDResponseHeaderConfig struct{}

func main() {}

func init() {
	wrapper.SetCtx(
		"ai-request-id-response-header",
		wrapper.ProcessResponseHeaders(onHttpResponseHeaders),
	)
}

func onHttpResponseHeaders(_ wrapper.HttpContext, _ RequestIDResponseHeaderConfig) types.Action {
	requestID := getRequestID()
	if requestID == "" {
		return types.ActionContinue
	}

	if err := proxywasm.ReplaceHttpResponseHeader(requestIDHeader, requestID); err != nil {
		log.Warnf("ai-request-id-response-header: failed to set response header: %v", err)
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
