package main

import (
	"crypto/hmac"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/hex"
	"strings"

	"github.com/google/uuid"
	"github.com/higress-group/proxy-wasm-go-sdk/proxywasm"
	"github.com/higress-group/proxy-wasm-go-sdk/proxywasm/types"
	"github.com/higress-group/wasm-go/pkg/log"
	"github.com/higress-group/wasm-go/pkg/wrapper"
	"github.com/tidwall/gjson"
)

const (
	responseRequestIDHeader = "request-id"
	upstreamRequestIDHeader = "request-id"
	canonicalRequestIDKey   = "wasm.canonicalRequestID"
	providerTypePropertyKey = "wasm.providerType"
	clusterNameProperty     = "cluster_name"
	defaultSignatureLength  = 8
)

var defaultSkipClusterNamePatterns = []string{"llm-"}

type RequestIDHeaderConfig struct {
	SkipClusterNamePatterns []string
	SignatureSecret         string
	SignatureLength         int
}

func main() {}

func init() {
	wrapper.SetCtx(
		"ai-request-id-header",
		wrapper.ParseConfig(parseConfig),
		wrapper.ProcessRequestHeaders(onHttpRequestHeaders),
		wrapper.ProcessResponseHeaders(onHttpResponseHeaders),
	)
}

func parseConfig(json gjson.Result, config *RequestIDHeaderConfig) error {
	config.SkipClusterNamePatterns = defaultSkipClusterNamePatterns
	if patterns := json.Get("skipClusterNamePatterns"); patterns.IsArray() {
		var got []string
		patterns.ForEach(func(_, v gjson.Result) bool {
			if s := strings.TrimSpace(v.String()); s != "" {
				got = append(got, s)
			}
			return true
		})
		if len(got) > 0 {
			config.SkipClusterNamePatterns = got
		}
	}

	config.SignatureSecret = json.Get("signatureSecret").String()
	if config.SignatureSecret == "" {
		log.Warnf("ai-request-id-header: signatureSecret not configured, signature verification disabled")
	}

	config.SignatureLength = defaultSignatureLength
	if json.Get("signatureLength").Exists() {
		l := int(json.Get("signatureLength").Int())
		if l >= 4 && l <= 64 {
			config.SignatureLength = l
		}
	}

	return nil
}

func onHttpRequestHeaders(_ wrapper.HttpContext, config RequestIDHeaderConfig) types.Action {
	canonicalID := resolveCanonicalRequestID(config)
	if canonicalID != "" {
		setCanonicalRequestIDProperty(canonicalID)
	}

	if shouldSkipForAIProvider(config) {
		return types.ActionContinue
	}

	if canonicalID == "" {
		return types.ActionContinue
	}

	if err := proxywasm.ReplaceHttpRequestHeader(upstreamRequestIDHeader, canonicalID); err != nil {
		log.Warnf("ai-request-id-header: failed to set upstream request header: %v", err)
	}

	return types.ActionContinue
}

func onHttpResponseHeaders(_ wrapper.HttpContext, config RequestIDHeaderConfig) types.Action {
	canonicalID := readCanonicalRequestIDProperty()
	if canonicalID == "" {
		canonicalID = readCanonicalRequestIDFromHeader()
	}
	if canonicalID == "" {
		canonicalID = resolveCanonicalRequestID(config)
	}
	if canonicalID == "" {
		return types.ActionContinue
	}

	if err := proxywasm.ReplaceHttpResponseHeader(responseRequestIDHeader, canonicalID); err != nil {
		log.Warnf("ai-request-id-header: failed to set response header: %v", err)
	}

	return types.ActionContinue
}

// resolveCanonicalRequestID produces or validates a canonical request-id with signature.
func resolveCanonicalRequestID(config RequestIDHeaderConfig) string {
	if config.SignatureSecret == "" {
		return getRequestID()
	}

	incoming := getIncomingRequestID()

	if incoming != "" {
		rawID, sig := splitSignature(incoming)
		if rawID != "" && sig != "" {
			expected := computeSignature(rawID, config.SignatureSecret, config.SignatureLength)
			if constantTimeEqual(sig, expected) {
				return incoming
			}
		}
	}

	rawID := getRawEnvoyRequestID()
	if rawID == "" {
		rawID = uuid.New().String()
	}

	sig := computeSignature(rawID, config.SignatureSecret, config.SignatureLength)
	return rawID + "-" + sig
}

func setCanonicalRequestIDProperty(canonicalID string) {
	if err := proxywasm.SetProperty([]string{canonicalRequestIDKey}, []byte(canonicalID)); err != nil {
		log.Warnf("ai-request-id-header: failed to set canonical request-id property: %v", err)
	}
}

func readCanonicalRequestIDProperty() string {
	if value, err := proxywasm.GetProperty([]string{canonicalRequestIDKey}); err == nil {
		if id := normalizeRequestID(string(value)); id != "" {
			return id
		}
	}
	return ""
}

// readCanonicalRequestIDFromHeader reads the canonical request-id from request headers when it was injected upstream.
func readCanonicalRequestIDFromHeader() string {
	if value, err := proxywasm.GetHttpRequestHeader(responseRequestIDHeader); err == nil {
		if id := normalizeRequestID(value); id != "" {
			return id
		}
	}
	return ""
}

func shouldSkipForAIProvider(config RequestIDHeaderConfig) bool {
	if v, err := proxywasm.GetProperty([]string{providerTypePropertyKey}); err == nil && len(v) > 0 {
		log.Debugf("ai-request-id-header: skip injection, providerType=%s", string(v))
		return true
	}

	if len(config.SkipClusterNamePatterns) == 0 {
		return false
	}

	clusterName, err := proxywasm.GetProperty([]string{clusterNameProperty})
	if err != nil || len(clusterName) == 0 {
		return false
	}

	for _, pattern := range config.SkipClusterNamePatterns {
		if pattern == "" {
			continue
		}
		if strings.Contains(string(clusterName), pattern) {
			log.Debugf("ai-request-id-header: skip injection, cluster_name=%s matches pattern=%s", string(clusterName), pattern)
			return true
		}
	}
	return false
}

// getIncomingRequestID reads the request-id from request headers (client-provided).
func getIncomingRequestID() string {
	for _, header := range []string{responseRequestIDHeader, "x-request-id"} {
		if value, err := proxywasm.GetHttpRequestHeader(header); err == nil {
			if id := normalizeRequestID(value); id != "" {
				return id
			}
		}
	}
	return ""
}

// getRawEnvoyRequestID reads the Envoy-generated x_request_id property.
func getRawEnvoyRequestID() string {
	if value, err := proxywasm.GetProperty([]string{"x_request_id"}); err == nil {
		if id := normalizeRequestID(string(value)); id != "" {
			return id
		}
	}
	return ""
}

// getRequestID returns the best available raw request ID (backward compat, no signature logic).
func getRequestID() string {
	if value, err := proxywasm.GetProperty([]string{"x_request_id"}); err == nil {
		if requestID := normalizeRequestID(string(value)); requestID != "" {
			return requestID
		}
	}

	for _, header := range []string{upstreamRequestIDHeader, "x-request-id"} {
		if value, err := proxywasm.GetHttpRequestHeader(header); err == nil {
			if requestID := normalizeRequestID(value); requestID != "" {
				return requestID
			}
		}
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

// splitSignature splits a canonical request-id into rawID and signature at the last '-'.
// UUID format is 8-4-4-4-12, so the last '-' after the 5th segment is the signature delimiter.
func splitSignature(id string) (string, string) {
	idx := strings.LastIndex(id, "-")
	if idx <= 0 || idx == len(id)-1 {
		return "", ""
	}
	rawID := id[:idx]
	sig := id[idx+1:]
	if !isHexString(sig) {
		return "", ""
	}
	return rawID, sig
}

func isHexString(s string) bool {
	for _, c := range s {
		if !((c >= '0' && c <= '9') || (c >= 'a' && c <= 'f') || (c >= 'A' && c <= 'F')) {
			return false
		}
	}
	return len(s) > 0
}

func computeSignature(rawID, secret string, length int) string {
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write([]byte(rawID))
	fullSig := hex.EncodeToString(mac.Sum(nil))
	if length > len(fullSig) {
		length = len(fullSig)
	}
	return fullSig[:length]
}

func constantTimeEqual(a, b string) bool {
	return subtle.ConstantTimeCompare([]byte(a), []byte(b)) == 1
}
