package main

import (
	"encoding/json"
	"testing"

	"github.com/higress-group/proxy-wasm-go-sdk/proxywasm/types"
	"github.com/higress-group/wasm-go/pkg/test"
	"github.com/stretchr/testify/require"
	"github.com/tidwall/gjson"
)

func TestSourceConfig(t *testing.T) {
	c, err := parseSourceConfig(gjson.Parse(`{}`))
	require.NoError(t, err)
	require.Equal(t, "x-wujie-source", c.headerName)
	require.Equal(t, "unknown", c.normalize("wujieai")) // no product hardcoded into the plugin
	c, err = parseSourceConfig(gjson.Parse(`{"headerName":"X-Product","allowedValues":["desktop"],"fallbackValue":"other"}`))
	require.NoError(t, err)
	require.Equal(t, "x-product", c.headerName)
	require.Equal(t, "desktop", c.normalize("desktop"))
	require.Equal(t, "other", c.normalize(""))
	require.Equal(t, "other", c.normalize("arbitrary-request-value"))
	for _, raw := range []string{`[]`, `{"headerName":":path"}`, `{"headerName":1}`, `{"allowedValues":"desktop"}`, `{"allowedValues":[false]}`, `{"allowedValues":["with.dots"]}`, `{"fallbackValue":""}`} {
		_, err := parseSourceConfig(gjson.Parse(raw))
		require.Error(t, err, raw)
	}
}

func TestReportSourceSnapshot(t *testing.T) {
	test.RunTest(t, func(t *testing.T) {
		for _, tc := range []struct {
			name, sourceConfig, header, value, expected string
			stream                                      bool
		}{
			{name: "old config remains valid", header: "x-wujie-source", value: "wujieai", expected: "unknown"},
			{name: "configured source JSON", sourceConfig: `,"source":{"allowedValues":["wujieai"]}`, header: "x-wujie-source", value: "wujieai", expected: "wujieai"},
			{name: "configured source SSE", sourceConfig: `,"source":{"allowedValues":["wujieai"]}`, header: "x-wujie-source", value: "wujieai", expected: "wujieai", stream: true},
			{name: "missing source", sourceConfig: `,"source":{"allowedValues":["wujieai"]}`, header: "x-wujie-source", expected: "unknown", stream: true},
			{name: "unknown source", sourceConfig: `,"source":{"allowedValues":["wujieai"]}`, header: "x-wujie-source", value: "other-product", expected: "unknown"},
			{name: "custom contract", sourceConfig: `,"source":{"headerName":"X-Product","allowedValues":["desktop"],"fallbackValue":"other"}`, header: "x-product", value: "desktop", expected: "desktop", stream: true},
		} {
			t.Run(tc.name, func(t *testing.T) {
				config := json.RawMessage(`{"reportApiUrl":"https://billing.test/model/usage/report","reportServiceName":"billing"` + tc.sourceConfig + `}`)
				host, status := test.NewTestHost(config)
				defer host.Reset()
				require.Equal(t, types.OnPluginStartStatusOK, status)
				require.NoError(t, host.SetProperty([]string{"wasm.providerType"}, []byte("openai")))
				require.NoError(t, host.SetProperty([]string{"wasm.requestModel"}, []byte("gpt-4")))
				id := host.InitializeHttpContext()
				headers := [][2]string{{":authority", "model.test"}, {":method", "POST"}, {":path", "/v1/chat/completions"}, {"x-original-api-key", "test-user-key"}, {"request-id", "business-request"}, {"x-request-id", "envoy-request"}, {"wujie_task_id", "999"}}
				original := append(append([][2]string{}, headers...), [2]string{tc.header, tc.value})
				require.Equal(t, types.ActionContinue, host.CallOnRequestHeaders(id, original, false))
				// Later routing/provider/header plugins rewrite path/host and source.
				// Reporting must retain the value captured by this decode callback.
				headers[0][1] = "upstream.test"
				headers[2][1] = "/rewritten/provider/path"
				host.SetHttpRequestHeaders(id, append(headers, [2]string{tc.header, "rewritten-source"}))
				require.Empty(t, host.GetCalloutAttributesFromContext(id))
				body := []byte(`{"model":"gpt-4","usage":{"prompt_tokens":10,"completion_tokens":5,"total_tokens":15}}`)
				contentType := "application/json"
				if tc.stream {
					contentType = "text/event-stream"
				}
				host.CallOnResponseHeaders(id, [][2]string{{":status", "200"}, {"content-type", contentType}}, false)
				if tc.stream {
					host.CallOnResponseBody(id, append(append([]byte("data: "), body...), []byte("\n\n")...), false)
					require.Empty(t, host.GetCalloutAttributesFromContext(id), "must not report before original end-of-stream timing")
					host.CallOnResponseBody(id, []byte("data: [DONE]\n\n"), true)
				} else {
					host.CallOnResponseBody(id, body, true)
				}
				calls := host.GetCalloutAttributesFromContext(id)
				require.Len(t, calls, 1)
				var report map[string]any
				require.NoError(t, json.Unmarshal(calls[0].Body, &report))
				require.Equal(t, tc.expected, report["productSource"])
				require.Equal(t, "business-request", report["requestId"])
				require.Equal(t, "gpt-4", report["model"])
				require.Equal(t, "test-user-key", report["userKey"])
				require.Equal(t, "999", report["taskId"])
				require.Equal(t, float64(10), report["inputToken"])
				require.Equal(t, float64(5), report["outputToken"])
				require.Equal(t, float64(15), report["totalToken"])
				require.Equal(t, "outbound|443||billing.dns", calls[0].Upstream)
				for _, field := range []string{"duration", "timestamp", "startTime", "endTime"} {
					require.IsType(t, float64(0), report[field])
					delete(report, field)
				}
				var usage map[string]any
				require.NoError(t, json.Unmarshal([]byte(report["tokenUsage"].(string)), &usage))
				require.Equal(t, float64(10), usage["input_token"])
				require.Equal(t, float64(5), usage["output_token"])
				require.Equal(t, float64(15), usage["total_token"])
				require.Len(t, report, 9, "only productSource is added to the existing report schema")
				// The serialized asynchronous callout survives stream cleanup.
				host.CompleteHttpContext(id)
				require.Equal(t, tc.expected, gjson.GetBytes(calls[0].Body, "productSource").String())
			})
		}
	})
}
