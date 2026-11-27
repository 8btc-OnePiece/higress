// Package teambilling identifies the first-party operations migrated to team settlement.
package teambilling

import (
	"fmt"
	"net/http"
	"strings"

	"github.com/higress-group/proxy-wasm-go-sdk/proxywasm"
)

const OperationProperty = "wasm.wujieTeamBillingOperation"

const BananaSKU = "gemini-3.1-flash-image-preview-times"

type endpoint struct {
	method string
	path   string
}

// Higress uses the Ingress name for route_name in the gateway namespace.
var operations = map[string]map[endpoint]string{
	"wujieai-open-platform": {
		{http.MethodPost, "/wj-open/v2/open-platform/task/task_create"}: "forward",
		{http.MethodGet, "/wj-open/v2/open-platform/task/v2/task_info"}: "read",
	},
	"ai-route-deepseek-v4-flash.internal": {
		{http.MethodPost, "/v1/chat/completions"}: "usage",
	},
	"ai-route-" + BananaSKU + ".internal": {
		{http.MethodPost, "/v1/images/generations"}: "image",
		{http.MethodPost, "/v1/images/edits"}:       "image",
	},
}

func IsManagedRoute(name string) bool { return operations[name] != nil }

// Operation uses the selected Envoy route, independent of user-supplied model headers.
func Operation() (string, error) {
	route, err := proxywasm.GetProperty([]string{"route_name"})
	if err != nil || len(route) == 0 {
		return "", fmt.Errorf("resolved route unavailable")
	}
	rules := operations[string(route)]
	if rules == nil {
		return "", nil
	}
	method, err := proxywasm.GetHttpRequestHeader(":method")
	if err != nil || method == "" {
		return "", fmt.Errorf("request method unavailable")
	}
	raw, err := proxywasm.GetHttpRequestHeader(":path")
	if err != nil || raw == "" {
		return "", fmt.Errorf("request path unavailable")
	}
	path, _, _ := strings.Cut(raw, "?")
	// Both model ingresses match paths case-insensitively. Open Platform is case-sensitive.
	if string(route) != "wujieai-open-platform" {
		path = strings.ToLower(path)
	}
	return rules[endpoint{method, path}], nil
}
