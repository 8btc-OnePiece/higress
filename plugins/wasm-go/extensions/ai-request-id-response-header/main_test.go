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

func TestOnHttpRequestHeaders(t *testing.T) {
	test.RunTest(t, func(t *testing.T) {
		t.Run("injects x-request-id from x_request_id", func(t *testing.T) {
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

		t.Run("keeps existing x-request-id as shared request ID", func(t *testing.T) {
			host, status := test.NewTestHost(emptyConfig)
			defer host.Reset()
			require.Equal(t, types.OnPluginStartStatusOK, status)

			action := host.CallOnHttpRequestHeaders([][2]string{
				{":authority", "example.com"},
				{upstreamRequestIDHeader, "old-req"},
			})

			require.Equal(t, types.ActionContinue, action)
			value, ok := test.GetHeaderValue(host.GetRequestHeaders(), upstreamRequestIDHeader)
			require.True(t, ok)
			require.Equal(t, "old-req", value)
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
	})
}

func TestOnHttpResponseHeaders(t *testing.T) {
	test.RunTest(t, func(t *testing.T) {
		t.Run("sets request_id from x_request_id", func(t *testing.T) {
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

		t.Run("falls back to x-request-id header", func(t *testing.T) {
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

			require.Equal(t, types.ActionContinue, host.CallOnHttpRequestHeaders([][2]string{
				{":authority", "example.com"},
				{upstreamRequestIDHeader, "-"},
			}))
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

func TestGetRequestID(t *testing.T) {
	test.RunTest(t, func(t *testing.T) {
		t.Run("returns x_request_id", func(t *testing.T) {
			host, status := test.NewTestHost(emptyConfig)
			defer host.Reset()
			require.Equal(t, types.OnPluginStartStatusOK, status)

			require.NoError(t, host.SetRequestId("x-req-123"))
			require.Equal(t, "x-req-123", getRequestID())
		})

		t.Run("falls back to header when property is unusable", func(t *testing.T) {
			host, status := test.NewTestHost(emptyConfig)
			defer host.Reset()
			require.Equal(t, types.OnPluginStartStatusOK, status)

			require.Equal(t, types.ActionContinue, host.CallOnHttpRequestHeaders([][2]string{
				{":authority", "example.com"},
				{"x-request-id", "header-req-456"},
			}))
			require.NoError(t, host.SetRequestId("-"))
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
