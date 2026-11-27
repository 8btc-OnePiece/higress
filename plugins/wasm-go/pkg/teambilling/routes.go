// Package teambilling classifies first-party model operations on the resolved gateway route.
package teambilling

import (
	"fmt"
	"net/http"
	"strings"

	"github.com/higress-group/proxy-wasm-go-sdk/proxywasm"
)

const OperationProperty = "wasm.wujieTeamBillingOperation"
const BananaSKU = "gemini-3.1-flash-image-preview-times"
const SearchSKU = "deepseek-search"

func IsManagedRoute(name string) bool {
	// Enterprise JWT routes authenticate without the personal API key used by Claw.
	return strings.HasPrefix(name, "ai-route-") && strings.HasSuffix(name, ".internal") &&
		!strings.HasPrefix(name, "ai-route-enterprise-jwt")
}

func PerCallModel(operation string) string {
	switch operation {
	case "image":
		return BananaSKU
	case "image-pro":
		return "gemini-3.0-pro-image-preview-times"
	case "search":
		return SearchSKU
	}
	return ""
}

func Operation() (string, error) {
	route, err := proxywasm.GetProperty([]string{"route_name"})
	if err != nil || len(route) == 0 {
		// A local 404 has no selected upstream. An AI stream must have a resolved route.
		if provider, _ := proxywasm.GetProperty([]string{"wasm.providerType"}); len(provider) == 0 {
			return "external", nil
		}
		return "", fmt.Errorf("resolved model route unavailable")
	}
	name := string(route)
	if name != "wujieai-open-platform" && !IsManagedRoute(name) {
		// Older/custom model ingresses do not all follow the generated name convention.
		// Use the same trusted ai-proxy property the legacy usage filter used to recognize them.
		provider, _ := proxywasm.GetProperty([]string{"wasm.providerType"})
		if len(provider) == 0 || strings.HasPrefix(name, "wujieai-") || strings.HasPrefix(name, "ai-route-enterprise-jwt") {
			return "external", nil
		}
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
	if name == "wujieai-open-platform" {
		switch path {
		case "/wj-open/v2/open-platform/task/task_create":
			if method == http.MethodPost {
				return "task_create", nil
			}
		case "/wj-open/v2/open-platform/task/task_create_sync":
			if method == http.MethodPost {
				return "task_create_sync", nil
			}
		case "/wj-open/v2/open-platform/task/v2/task_info", "/wj-open/v2/open-platform/task/task_info":
			if method == http.MethodGet {
				return "read", nil
			}
		}
		return "external", nil
	}
	path = strings.ToLower(path)
	if method == http.MethodGet || method == http.MethodHead || method == http.MethodOptions {
		return "read", nil
	}
	if method != http.MethodPost {
		return "unclassified", nil
	}
	if path == "/v1/tools/search" && strings.HasPrefix(name, "ai-route-deepseek") {
		return "search", nil
	}
	switch path {
	case "/v1/images/generations", "/v1/images/edits":
		switch name {
		case "ai-route-" + BananaSKU + ".internal":
			return "image", nil
		case "ai-route-gemini-3.0-pro-image-preview-times.internal":
			return "image-pro", nil
		}
		return "usage", nil
	case "/v1/files":
		return "read", nil
	case "/v1/chat/completions", "/v1/completions", "/v1/responses", "/v1/messages", "/anthropic/v1/messages",
		"/v1/embeddings", "/v1/audio/speech", "/v1/audio/transcriptions", "/v1/audio/translations",
		"/api/v3/chat/completions", "/v1/images/variations", "/v1/rerank", "/v1/complete":
		return "usage", nil
	}
	// Provider adapters may already have translated the path (Azure deployments,
	// native Gemini APIs, etc.). Keep the provider's existing usage extraction
	// and price model, with the same admitted team as OpenAI-compatible calls.
	if provider, _ := proxywasm.GetProperty([]string{"wasm.providerType"}); len(provider) != 0 {
		return "usage", nil
	}
	// An unknown operation on a model ingress must not enter legacy personal settlement.
	return "unclassified", nil
}
