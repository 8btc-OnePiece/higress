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

func TestOnHttpResponseHeaders(t *testing.T) {
	test.RunTest(t, func(t *testing.T) {
		t.Run("sets request_id from Envoy request ID", func(t *testing.T) {
			host, status := test.NewTestHost(emptyConfig)
			defer host.Reset()
			require.Equal(t, types.OnPluginStartStatusOK, status)

			require.NoError(t, host.SetProperty([]string{"request", "id"}, []byte("envoy-req-123")))

			action := host.CallOnHttpResponseHeaders([][2]string{
				{":status", "200"},
			})

			require.Equal(t, types.ActionContinue, action)
			require.True(t, test.HasHeaderWithValue(host.GetResponseHeaders(), requestIDHeader, "envoy-req-123"))
			host.CompleteHttp()
		})

		t.Run("skips empty request ID", func(t *testing.T) {
			host, status := test.NewTestHost(emptyConfig)
			defer host.Reset()
			require.Equal(t, types.OnPluginStartStatusOK, status)

			action := host.CallOnHttpResponseHeaders([][2]string{
				{":status", "200"},
			})

			require.Equal(t, types.ActionContinue, action)
			require.False(t, test.HasHeader(host.GetResponseHeaders(), requestIDHeader))
			host.CompleteHttp()
		})

		t.Run("skips dash request ID", func(t *testing.T) {
			host, status := test.NewTestHost(emptyConfig)
			defer host.Reset()
			require.Equal(t, types.OnPluginStartStatusOK, status)

			require.NoError(t, host.SetProperty([]string{"request", "id"}, []byte("-")))

			action := host.CallOnHttpResponseHeaders([][2]string{
				{":status", "200"},
			})

			require.Equal(t, types.ActionContinue, action)
			require.False(t, test.HasHeader(host.GetResponseHeaders(), requestIDHeader))
			host.CompleteHttp()
		})
	})
}

func TestGetEnvoyRequestID(t *testing.T) {
	test.RunTest(t, func(t *testing.T) {
		t.Run("trims whitespace", func(t *testing.T) {
			host, status := test.NewTestHost(emptyConfig)
			defer host.Reset()
			require.Equal(t, types.OnPluginStartStatusOK, status)

			require.NoError(t, host.SetProperty([]string{"request", "id"}, []byte(" envoy-req-456 ")))
			require.Equal(t, "envoy-req-456", getEnvoyRequestID())
		})
	})
}

func TestNormalizeEnvoyRequestID(t *testing.T) {
	require.Equal(t, "envoy-req-123", normalizeEnvoyRequestID("envoy-req-123"))
	require.Equal(t, "envoy-req-456", normalizeEnvoyRequestID(" envoy-req-456 "))
	require.Empty(t, normalizeEnvoyRequestID(""))
	require.Empty(t, normalizeEnvoyRequestID("   "))
	require.Empty(t, normalizeEnvoyRequestID("-"))
}
