package main

import (
	"encoding/json"
	"testing"

	"github.com/higress-group/proxy-wasm-go-sdk/proxywasm/types"
	"github.com/higress-group/wasm-go/pkg/test"
	"github.com/stretchr/testify/require"
)

var emptyConfig = func() json.RawMessage {
	data, _ := json.Marshal(map[string]interface{}{})
	return data
}()

func TestGetCanonicalRequestID(t *testing.T) {
	test.RunTest(t, func(t *testing.T) {
		t.Run("reads canonical request-id from wasm property", func(t *testing.T) {
			host, status := test.NewTestHost(emptyConfig)
			defer host.Reset()
			require.Equal(t, types.OnPluginStartStatusOK, status)

			require.Equal(t, types.ActionContinue, host.CallOnHttpRequestHeaders([][2]string{
				{":authority", "example.com"},
			}))
			require.NoError(t, host.SetProperty([]string{canonicalRequestIDKey}, []byte("187e99ba-5b64-9ffe-8f69-01dafbaf6ed7-a1b2c3d4")))
			require.Equal(t, "187e99ba-5b64-9ffe-8f69-01dafbaf6ed7-a1b2c3d4", getCanonicalRequestID())
			host.CompleteHttp()
		})

		t.Run("prefers wasm property over request-id header", func(t *testing.T) {
			host, status := test.NewTestHost(emptyConfig)
			defer host.Reset()
			require.Equal(t, types.OnPluginStartStatusOK, status)

			require.Equal(t, types.ActionContinue, host.CallOnHttpRequestHeaders([][2]string{
				{":authority", "example.com"},
				{"request-id", "header-id-deadbeef"},
			}))
			require.NoError(t, host.SetProperty([]string{canonicalRequestIDKey}, []byte("property-id-a1b2c3d4")))
			require.Equal(t, "property-id-a1b2c3d4", getCanonicalRequestID())
			host.CompleteHttp()
		})

		t.Run("reads canonical request-id from request-id header", func(t *testing.T) {
			host, status := test.NewTestHost(emptyConfig)
			defer host.Reset()
			require.Equal(t, types.OnPluginStartStatusOK, status)

			require.Equal(t, types.ActionContinue, host.CallOnHttpRequestHeaders([][2]string{
				{":authority", "example.com"},
				{"request-id", "187e99ba-5b64-9ffe-8f69-01dafbaf6ed7-a1b2c3d4"},
			}))
			require.Equal(t, "187e99ba-5b64-9ffe-8f69-01dafbaf6ed7-a1b2c3d4", getCanonicalRequestID())
			host.CompleteHttp()
		})

		t.Run("returns empty when request-id header missing", func(t *testing.T) {
			host, status := test.NewTestHost(emptyConfig)
			defer host.Reset()
			require.Equal(t, types.OnPluginStartStatusOK, status)

			require.Equal(t, types.ActionContinue, host.CallOnHttpRequestHeaders([][2]string{
				{":authority", "example.com"},
			}))
			require.Empty(t, getCanonicalRequestID())
			host.CompleteHttp()
		})

		t.Run("returns empty for dash value", func(t *testing.T) {
			host, status := test.NewTestHost(emptyConfig)
			defer host.Reset()
			require.Equal(t, types.OnPluginStartStatusOK, status)

			require.Equal(t, types.ActionContinue, host.CallOnHttpRequestHeaders([][2]string{
				{":authority", "example.com"},
				{"request-id", "-"},
			}))
			require.Empty(t, getCanonicalRequestID())
			host.CompleteHttp()
		})
	})
}

func TestGetRequestID(t *testing.T) {
	test.RunTest(t, func(t *testing.T) {
		t.Run("returns x_request_id property", func(t *testing.T) {
			host, status := test.NewTestHost(emptyConfig)
			defer host.Reset()
			require.Equal(t, types.OnPluginStartStatusOK, status)

			require.NoError(t, host.SetRequestId("x-req-123"))
			require.Equal(t, "x-req-123", getRequestID())
		})

		t.Run("falls back to x-request-id header", func(t *testing.T) {
			host, status := test.NewTestHost(emptyConfig)
			defer host.Reset()
			require.Equal(t, types.OnPluginStartStatusOK, status)

			require.Equal(t, types.ActionContinue, host.CallOnHttpRequestHeaders([][2]string{
				{":authority", "example.com"},
				{"x-request-id", "header-req-123"},
			}))
			require.NoError(t, host.SetRequestId("-"))
			require.Equal(t, "header-req-123", getRequestID())
			host.CompleteHttp()
		})

		t.Run("returns empty for dash request ID", func(t *testing.T) {
			host, status := test.NewTestHost(emptyConfig)
			defer host.Reset()
			require.Equal(t, types.OnPluginStartStatusOK, status)

			require.Equal(t, types.ActionContinue, host.CallOnHttpRequestHeaders([][2]string{
				{":authority", "example.com"},
			}))
			require.NoError(t, host.SetRequestId("-"))
			require.Empty(t, getRequestID())
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
