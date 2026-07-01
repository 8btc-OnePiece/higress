package main

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/higress-group/proxy-wasm-go-sdk/proxywasm"
	"github.com/higress-group/proxy-wasm-go-sdk/proxywasm/types"
	"github.com/higress-group/wasm-go/pkg/test"
	"github.com/stretchr/testify/require"
)

var emptyConfig = func() json.RawMessage {
	data, _ := json.Marshal(map[string]interface{}{})
	return data
}()

func configWithSkipPatterns(patterns []string) json.RawMessage {
	data, _ := json.Marshal(map[string]interface{}{
		"skipClusterNamePatterns": patterns,
	})
	return data
}

func configWithSignature(secret string, length int) json.RawMessage {
	data, _ := json.Marshal(map[string]interface{}{
		"signatureSecret": secret,
		"signatureLength": length,
	})
	return data
}

func configWithSignatureDefault(secret string) json.RawMessage {
	data, _ := json.Marshal(map[string]interface{}{
		"signatureSecret": secret,
	})
	return data
}

func TestOnHttpRequestHeaders(t *testing.T) {
	test.RunTest(t, func(t *testing.T) {
		t.Run("injects request-id from x_request_id (no signature)", func(t *testing.T) {
			host, status := test.NewTestHost(emptyConfig)
			defer host.Reset()
			require.Equal(t, types.OnPluginStartStatusOK, status)

			require.NoError(t, host.SetRequestId("x-req-123"))

			action := host.CallOnHttpRequestHeaders([][2]string{
				{":authority", "example.com"},
			})

			require.Equal(t, types.ActionContinue, action)
			require.True(t, test.HasHeaderWithValue(host.GetRequestHeaders(), upstreamRequestIDHeader, "x-req-123"))
			host.CompleteHttp()
		})

		t.Run("skips dash request ID", func(t *testing.T) {
			host, status := test.NewTestHost(emptyConfig)
			defer host.Reset()
			require.Equal(t, types.OnPluginStartStatusOK, status)

			require.NoError(t, host.SetRequestId("-"))

			action := host.CallOnHttpRequestHeaders([][2]string{
				{":authority", "example.com"},
				{upstreamRequestIDHeader, "-"},
			})

			require.Equal(t, types.ActionContinue, action)
			require.True(t, test.HasHeaderWithValue(host.GetRequestHeaders(), upstreamRequestIDHeader, "-"))
			host.CompleteHttp()
		})

		t.Run("skips injection when ai-proxy set providerType", func(t *testing.T) {
			host, status := test.NewTestHost(configWithSignatureDefault("mysecret"))
			defer host.Reset()
			require.Equal(t, types.OnPluginStartStatusOK, status)

			require.NoError(t, host.SetRequestId("x-req-ai"))
			require.NoError(t, host.SetProperty([]string{providerTypePropertyKey}, []byte("openai")))

			action := host.CallOnHttpRequestHeaders([][2]string{
				{":authority", "api.openai.com"},
			})

			require.Equal(t, types.ActionContinue, action)
			require.False(t, test.HasHeader(host.GetRequestHeaders(), upstreamRequestIDHeader))
			propertyValue, err := proxywasm.GetProperty([]string{canonicalRequestIDKey})
			require.NoError(t, err)
			require.True(t, strings.HasPrefix(string(propertyValue), "x-req-ai-"))
			host.CompleteHttp()
		})

		t.Run("skips injection when cluster_name matches default llm- pattern", func(t *testing.T) {
			host, status := test.NewTestHost(emptyConfig)
			defer host.Reset()
			require.Equal(t, types.OnPluginStartStatusOK, status)

			require.NoError(t, host.SetRequestId("x-req-llm"))
			require.NoError(t, host.SetClusterName("outbound|443||llm-RightCodes.internal.dns"))

			action := host.CallOnHttpRequestHeaders([][2]string{
				{":authority", "www.right.codes"},
			})

			require.Equal(t, types.ActionContinue, action)
			host.CompleteHttp()
		})

		t.Run("skips injection when cluster_name matches configured pattern", func(t *testing.T) {
			host, status := test.NewTestHost(configWithSkipPatterns([]string{"right-codes.dns"}))
			defer host.Reset()
			require.Equal(t, types.OnPluginStartStatusOK, status)

			require.NoError(t, host.SetRequestId("x-req-direct"))
			require.NoError(t, host.SetClusterName("outbound|443||right-codes.dns"))

			action := host.CallOnHttpRequestHeaders([][2]string{
				{":authority", "www.right.codes"},
			})

			require.Equal(t, types.ActionContinue, action)
			host.CompleteHttp()
		})

		t.Run("still injects when cluster_name does not match patterns", func(t *testing.T) {
			host, status := test.NewTestHost(emptyConfig)
			defer host.Reset()
			require.Equal(t, types.OnPluginStartStatusOK, status)

			require.NoError(t, host.SetRequestId("x-req-biz"))
			require.NoError(t, host.SetClusterName("outbound|80||onepiece-open-platform.onepiece.svc.cluster.local"))

			action := host.CallOnHttpRequestHeaders([][2]string{
				{":authority", "biz.example.com"},
			})

			require.Equal(t, types.ActionContinue, action)
			require.True(t, test.HasHeaderWithValue(host.GetRequestHeaders(), upstreamRequestIDHeader, "x-req-biz"))
			host.CompleteHttp()
		})
	})
}

func TestOnHttpResponseHeaders(t *testing.T) {
	test.RunTest(t, func(t *testing.T) {
		t.Run("sets request-id from x_request_id (no signature)", func(t *testing.T) {
			host, status := test.NewTestHost(emptyConfig)
			defer host.Reset()
			require.Equal(t, types.OnPluginStartStatusOK, status)

			require.NoError(t, host.SetRequestId("x-req-123"))

			action := host.CallOnHttpResponseHeaders([][2]string{
				{":status", "200"},
			})

			require.Equal(t, types.ActionContinue, action)
			require.True(t, test.HasHeaderWithValue(host.GetResponseHeaders(), responseRequestIDHeader, "x-req-123"))
			host.CompleteHttp()
		})

		t.Run("falls back to legacy x-request-id header", func(t *testing.T) {
			host, status := test.NewTestHost(emptyConfig)
			defer host.Reset()
			require.Equal(t, types.OnPluginStartStatusOK, status)

			require.Equal(t, types.ActionContinue, host.CallOnHttpRequestHeaders([][2]string{
				{":authority", "example.com"},
				{"x-request-id", "header-req-123"},
			}))
			require.NoError(t, host.SetRequestId("-"))

			action := host.CallOnHttpResponseHeaders([][2]string{
				{":status", "200"},
			})

			require.Equal(t, types.ActionContinue, action)
			require.True(t, test.HasHeaderWithValue(host.GetResponseHeaders(), responseRequestIDHeader, "header-req-123"))
			host.CompleteHttp()
		})

		t.Run("skips dash request ID", func(t *testing.T) {
			host, status := test.NewTestHost(emptyConfig)
			defer host.Reset()
			require.Equal(t, types.OnPluginStartStatusOK, status)

			require.NoError(t, host.SetRequestId("-"))

			action := host.CallOnHttpResponseHeaders([][2]string{
				{":status", "200"},
			})

			require.Equal(t, types.ActionContinue, action)
			require.False(t, test.HasHeader(host.GetResponseHeaders(), responseRequestIDHeader))
			host.CompleteHttp()
		})
	})
}

func TestCanonicalRequestIDSignature(t *testing.T) {
	test.RunTest(t, func(t *testing.T) {
		t.Run("generates canonical request-id with signature", func(t *testing.T) {
			host, status := test.NewTestHost(configWithSignatureDefault("test-secret"))
			defer host.Reset()
			require.Equal(t, types.OnPluginStartStatusOK, status)

			require.NoError(t, host.SetRequestId("187e99ba-5b64-9ffe-8f69-01dafbaf6ed7"))

			action := host.CallOnHttpRequestHeaders([][2]string{
				{":authority", "example.com"},
			})

			require.Equal(t, types.ActionContinue, action)
			value, ok := test.GetHeaderValue(host.GetRequestHeaders(), upstreamRequestIDHeader)
			require.True(t, ok)
			// Should have format: rawID-signature (8 hex chars)
			require.True(t, strings.HasPrefix(value, "187e99ba-5b64-9ffe-8f69-01dafbaf6ed7-"))
			parts := strings.Split(value, "-")
			sig := parts[len(parts)-1]
			require.Len(t, sig, 8)
			require.True(t, isHexString(sig))
			host.CompleteHttp()
		})

		t.Run("validates and reuses valid signed request-id", func(t *testing.T) {
			host, status := test.NewTestHost(configWithSignatureDefault("test-secret"))
			defer host.Reset()
			require.Equal(t, types.OnPluginStartStatusOK, status)

			rawID := "187e99ba-5b64-9ffe-8f69-01dafbaf6ed7"
			validSig := computeSignature(rawID, "test-secret", 8)
			canonicalID := rawID + "-" + validSig

			require.NoError(t, host.SetRequestId(rawID))
			action := host.CallOnHttpRequestHeaders([][2]string{
				{":authority", "example.com"},
				{"request-id", canonicalID},
			})

			require.Equal(t, types.ActionContinue, action)
			value, ok := test.GetHeaderValue(host.GetRequestHeaders(), upstreamRequestIDHeader)
			require.True(t, ok)
			require.Equal(t, canonicalID, value)
			host.CompleteHttp()
		})

		t.Run("rejects invalid signature and generates new canonical ID", func(t *testing.T) {
			host, status := test.NewTestHost(configWithSignatureDefault("test-secret"))
			defer host.Reset()
			require.Equal(t, types.OnPluginStartStatusOK, status)

			rawID := "187e99ba-5b64-9ffe-8f69-01dafbaf6ed7"
			fakeCanonicalID := rawID + "-deadbeef"

			require.NoError(t, host.SetRequestId(rawID))
			action := host.CallOnHttpRequestHeaders([][2]string{
				{":authority", "example.com"},
				{"request-id", fakeCanonicalID},
			})

			require.Equal(t, types.ActionContinue, action)
			value, ok := test.GetHeaderValue(host.GetRequestHeaders(), upstreamRequestIDHeader)
			require.True(t, ok)
			// Should NOT be the fake canonical ID
			require.NotEqual(t, fakeCanonicalID, value)
			// Should be based on the Envoy raw ID
			require.True(t, strings.HasPrefix(value, rawID+"-"))
			host.CompleteHttp()
		})

		t.Run("generates UUID fallback when envoy request-id is empty", func(t *testing.T) {
			host, status := test.NewTestHost(configWithSignatureDefault("test-secret"))
			defer host.Reset()
			require.Equal(t, types.OnPluginStartStatusOK, status)

			require.NoError(t, host.SetRequestId("-"))

			action := host.CallOnHttpRequestHeaders([][2]string{
				{":authority", "example.com"},
			})

			require.Equal(t, types.ActionContinue, action)
			value, ok := test.GetHeaderValue(host.GetRequestHeaders(), upstreamRequestIDHeader)
			require.True(t, ok)
			// Should be a UUID-like string with a signature appended
			require.NotEmpty(t, value)
			parts := strings.Split(value, "-")
			require.True(t, len(parts) >= 6) // UUID has 5 parts + 1 signature
			sig := parts[len(parts)-1]
			require.Len(t, sig, 8)
			require.True(t, isHexString(sig))
			host.CompleteHttp()
		})

		t.Run("custom signature length", func(t *testing.T) {
			host, status := test.NewTestHost(configWithSignature("test-secret", 16))
			defer host.Reset()
			require.Equal(t, types.OnPluginStartStatusOK, status)

			require.NoError(t, host.SetRequestId("187e99ba-5b64-9ffe-8f69-01dafbaf6ed7"))

			action := host.CallOnHttpRequestHeaders([][2]string{
				{":authority", "example.com"},
			})

			require.Equal(t, types.ActionContinue, action)
			value, ok := test.GetHeaderValue(host.GetRequestHeaders(), upstreamRequestIDHeader)
			require.True(t, ok)
			parts := strings.Split(value, "-")
			sig := parts[len(parts)-1]
			require.Len(t, sig, 16)
			require.True(t, isHexString(sig))
			host.CompleteHttp()
		})

		t.Run("response header returns canonical request-id", func(t *testing.T) {
			host, status := test.NewTestHost(configWithSignatureDefault("test-secret"))
			defer host.Reset()
			require.Equal(t, types.OnPluginStartStatusOK, status)

			require.NoError(t, host.SetRequestId("187e99ba-5b64-9ffe-8f69-01dafbaf6ed7"))

			// Request phase generates and stores canonical ID
			action := host.CallOnHttpRequestHeaders([][2]string{
				{":authority", "example.com"},
			})
			require.Equal(t, types.ActionContinue, action)

			// Response phase returns it
			action = host.CallOnHttpResponseHeaders([][2]string{
				{":status", "200"},
			})
			require.Equal(t, types.ActionContinue, action)

			value, ok := test.GetHeaderValue(host.GetResponseHeaders(), responseRequestIDHeader)
			require.True(t, ok)
			require.True(t, strings.HasPrefix(value, "187e99ba-5b64-9ffe-8f69-01dafbaf6ed7-"))
			host.CompleteHttp()
		})

		t.Run("AI provider route stores canonical ID without upstream request-id header", func(t *testing.T) {
			host, status := test.NewTestHost(configWithSignatureDefault("test-secret"))
			defer host.Reset()
			require.Equal(t, types.OnPluginStartStatusOK, status)

			require.NoError(t, host.SetRequestId("ai-route-req-id"))
			require.NoError(t, host.SetProperty([]string{providerTypePropertyKey}, []byte("openai")))

			action := host.CallOnHttpRequestHeaders([][2]string{
				{":authority", "api.openai.com"},
			})
			require.Equal(t, types.ActionContinue, action)

			require.False(t, test.HasHeader(host.GetRequestHeaders(), "request-id"))
			propertyValue, err := proxywasm.GetProperty([]string{canonicalRequestIDKey})
			require.NoError(t, err)
			require.True(t, strings.HasPrefix(string(propertyValue), "ai-route-req-id-"))
			host.CompleteHttp()
		})
	})
}

func TestGetRequestID(t *testing.T) {
	test.RunTest(t, func(t *testing.T) {
		t.Run("returns x_request_id", func(t *testing.T) {
			host, status := test.NewTestHost(emptyConfig)
			defer host.Reset()
			require.Equal(t, types.OnPluginStartStatusOK, status)

			require.NoError(t, host.SetRequestId("x-req-123"))
			require.Equal(t, "x-req-123", getRequestID())
		})

		t.Run("falls back to legacy x-request-id header when property is unusable", func(t *testing.T) {
			host, status := test.NewTestHost(emptyConfig)
			defer host.Reset()
			require.Equal(t, types.OnPluginStartStatusOK, status)

			require.Equal(t, types.ActionContinue, host.CallOnHttpRequestHeaders([][2]string{
				{":authority", "example.com"},
				{"x-request-id", "header-req-456"},
			}))
			require.Equal(t, "header-req-456", getRequestID())
			host.CompleteHttp()
		})

		t.Run("trims whitespace", func(t *testing.T) {
			host, status := test.NewTestHost(emptyConfig)
			defer host.Reset()
			require.Equal(t, types.OnPluginStartStatusOK, status)

			require.NoError(t, host.SetRequestId(" x-req-456 "))
			require.Equal(t, "x-req-456", getRequestID())
		})
	})
}

func TestNormalizeRequestID(t *testing.T) {
	require.Equal(t, "req-123", normalizeRequestID("req-123"))
	require.Equal(t, "req-456", normalizeRequestID(" req-456 "))
	require.Empty(t, normalizeRequestID(""))
	require.Empty(t, normalizeRequestID("   "))
	require.Empty(t, normalizeRequestID("-"))
}

func TestSplitSignature(t *testing.T) {
	t.Run("splits valid canonical ID", func(t *testing.T) {
		rawID, sig := splitSignature("187e99ba-5b64-9ffe-8f69-01dafbaf6ed7-a1b2c3d4")
		require.Equal(t, "187e99ba-5b64-9ffe-8f69-01dafbaf6ed7", rawID)
		require.Equal(t, "a1b2c3d4", sig)
	})

	t.Run("returns empty for non-hex signature", func(t *testing.T) {
		rawID, sig := splitSignature("187e99ba-5b64-9ffe-8f69-01dafbaf6ed7-ZZZZZZZZ")
		require.Empty(t, rawID)
		require.Empty(t, sig)
	})

	t.Run("returns empty for no dash", func(t *testing.T) {
		rawID, sig := splitSignature("nodash")
		require.Empty(t, rawID)
		require.Empty(t, sig)
	})

	t.Run("returns empty for trailing dash", func(t *testing.T) {
		rawID, sig := splitSignature("abc-")
		require.Empty(t, rawID)
		require.Empty(t, sig)
	})
}

func TestComputeSignature(t *testing.T) {
	t.Run("deterministic", func(t *testing.T) {
		sig1 := computeSignature("test-raw-id", "secret", 8)
		sig2 := computeSignature("test-raw-id", "secret", 8)
		require.Equal(t, sig1, sig2)
	})

	t.Run("different secrets produce different signatures", func(t *testing.T) {
		sig1 := computeSignature("test-raw-id", "secret1", 8)
		sig2 := computeSignature("test-raw-id", "secret2", 8)
		require.NotEqual(t, sig1, sig2)
	})

	t.Run("respects length", func(t *testing.T) {
		sig := computeSignature("test-raw-id", "secret", 16)
		require.Len(t, sig, 16)
	})
}

func TestIsHexString(t *testing.T) {
	require.True(t, isHexString("0123456789abcdef"))
	require.True(t, isHexString("ABCDEF"))
	require.False(t, isHexString(""))
	require.False(t, isHexString("xyz"))
	require.False(t, isHexString("12g4"))
}
