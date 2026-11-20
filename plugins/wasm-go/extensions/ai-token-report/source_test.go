package main

import (
	"encoding/json"
	"testing"

	"github.com/higress-group/proxy-wasm-go-sdk/proxywasm"
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
			name, sourceConfig, header, value, expected           string
			responseStatus                                        string
			finalStatus, responseBody                             string
			missingStatus                                         bool
			statusUnavailableDuringBody                           bool
			stream, teamRoute, hasAdmission, mixedRoute, rejected bool
		}{
			{name: "legacy 402 without team selector", responseStatus: "402"},
			{name: "legacy 402 SSE", responseStatus: "402", stream: true},
			{name: "missing response status", missingStatus: true},
			{name: "status changed to 402", finalStatus: "402"},
			{name: "status changed to 402 SSE", finalStatus: "402", stream: true},
			{name: "legacy 402 actual body", responseStatus: "402", responseBody: `{"error":{"code":"insufficient_balance"}}`},
			{name: "legacy 503 image data", responseStatus: "503", responseBody: `{"data":[]}`},
			{name: "completed zero-token image", responseBody: `{"data":[{"url":"image.png"}]}`, expected: "unknown"},
			{name: "success after response headers released", statusUnavailableDuringBody: true, expected: "unknown"},
			{name: "SSE success after response headers released", statusUnavailableDuringBody: true, stream: true, expected: "unknown"},
			{name: "image success after response headers released", statusUnavailableDuringBody: true, responseBody: `{"data":[{"url":"image.png"}]}`, expected: "unknown"},
			{name: "402 after response headers released", statusUnavailableDuringBody: true, responseStatus: "402"},
			{name: "503 SSE after response headers released", statusUnavailableDuringBody: true, responseStatus: "503", stream: true},
			{name: "legacy 401", responseStatus: "401"},
			{name: "legacy 403", responseStatus: "403"},
			{name: "legacy 429", responseStatus: "429"},
			{name: "legacy 503", responseStatus: "503"},
			{name: "legacy 503 SSE", responseStatus: "503", stream: true},
			{name: "admitted team upstream 503", responseStatus: "503", mixedRoute: true, hasAdmission: true},
			{name: "admitted team upstream 503 SSE", responseStatus: "503", mixedRoute: true, hasAdmission: true, stream: true},
			{name: "team usage SSE", teamRoute: true, hasAdmission: true, expected: "unknown", stream: true},
			{name: "team usage without admission", teamRoute: true, expected: "unknown"},
			{name: "mixed route with team admission", mixedRoute: true, hasAdmission: true, expected: "unknown"},
			{name: "mixed route with legacy usage", mixedRoute: true, expected: "unknown"},
			{name: "mixed route rejects team without personal billing", mixedRoute: true, rejected: true},
			{name: "old config remains valid", header: "x-wujie-source", value: "wujieai", expected: "unknown"},
			{name: "configured source JSON", sourceConfig: `,"source":{"allowedValues":["wujieai"]}`, header: "x-wujie-source", value: "wujieai", expected: "wujieai"},
			{name: "configured source SSE", sourceConfig: `,"source":{"allowedValues":["wujieai"]}`, header: "x-wujie-source", value: "wujieai", expected: "wujieai", stream: true},
			{name: "missing source", sourceConfig: `,"source":{"allowedValues":["wujieai"]}`, header: "x-wujie-source", expected: "unknown", stream: true},
			{name: "unknown source", sourceConfig: `,"source":{"allowedValues":["wujieai"]}`, header: "x-wujie-source", value: "other-product", expected: "unknown"},
			{name: "custom contract", sourceConfig: `,"source":{"headerName":"X-Product","allowedValues":["desktop"],"fallbackValue":"other"}`, header: "x-product", value: "desktop", expected: "desktop", stream: true},
		} {
			t.Run(tc.name, func(t *testing.T) {
				config := `{"reportApiUrl":"https://billing.test/model/usage/report","reportServiceName":"billing"` + tc.sourceConfig
				if tc.teamRoute {
					config += `,"requireTeamId":true`
				}
				if tc.teamRoute || tc.mixedRoute {
					config += `,"teamReportService":"onepiece-open-platform-internal.dns"`
				}
				hostConfig := json.RawMessage(config + `}`)
				host, status := test.NewTestHost(hostConfig)
				defer host.Reset()
				require.Equal(t, types.OnPluginStartStatusOK, status)
				require.NoError(t, host.SetProperty([]string{"wasm.providerType"}, []byte("openai")))
				require.NoError(t, host.SetProperty([]string{"wasm.requestModel"}, []byte("gpt-4")))
				id := host.InitializeHttpContext()
				if tc.hasAdmission {
					require.NoError(t, host.SetProperty([]string{"wasm.teamId"}, []byte("57")))
					require.NoError(t, host.SetProperty([]string{"wasm.teamPersonalOwnerUid"}, []byte("17")))
				}
				headers := [][2]string{{":authority", "model.test"}, {":method", "POST"}, {":path", "/v1/chat/completions"}, {"x-original-api-key", "test-user-key"}, {"request-id", "business-request"}, {"x-request-id", "envoy-request"}, {"wujie_task_id", "999"}}
				if tc.rejected {
					headers = append(headers, [2]string{teamUUIDHeader, "9943d4d9-d20b-49db-809d-88586f950fc7"})
				}
				original := append(append([][2]string{}, headers...), [2]string{tc.header, tc.value})
				require.Equal(t, types.ActionContinue, host.CallOnRequestHeaders(id, original, false))
				// Later routing/provider/header plugins rewrite path/host and source.
				// Reporting must retain the value captured by this decode callback.
				headers[0][1] = "upstream.test"
				headers[2][1] = "/rewritten/provider/path"
				host.SetHttpRequestHeaders(id, append(headers, [2]string{tc.header, "rewritten-source"}))
				require.Empty(t, host.GetCalloutAttributesFromContext(id))
				body := []byte(`{"model":"gpt-4","usage":{"prompt_tokens":10,"completion_tokens":5,"total_tokens":15}}`)
				if tc.responseBody != "" {
					body = []byte(tc.responseBody)
				}
				contentType := "application/json"
				if tc.stream {
					contentType = "text/event-stream"
				}
				responseStatus := "200"
				if tc.responseStatus != "" {
					responseStatus = tc.responseStatus
				}
				if tc.rejected {
					responseStatus = "403"
				}
				responseHeaders := [][2]string{{"content-type", contentType}}
				if !tc.missingStatus {
					responseHeaders = append(responseHeaders, [2]string{":status", responseStatus})
				}
				host.CallOnResponseHeaders(id, responseHeaders, false)
				if tc.statusUnavailableDuringBody {
					require.NoError(t, proxywasm.RemoveHttpResponseHeader(":status"))
				}
				if tc.finalStatus != "" {
					require.NoError(t, proxywasm.ReplaceHttpResponseHeader(":status", tc.finalStatus))
				}
				if tc.stream {
					host.CallOnResponseBody(id, append(append([]byte("data: "), body...), []byte("\n\n")...), false)
					require.Empty(t, host.GetCalloutAttributesFromContext(id), "must not report before original end-of-stream timing")
					host.CallOnResponseBody(id, []byte("data: [DONE]\n\n"), true)
				} else {
					host.CallOnResponseBody(id, body, true)
				}
				calls := host.GetCalloutAttributesFromContext(id)
				if tc.rejected || tc.responseStatus != "" || tc.finalStatus != "" || tc.missingStatus || (tc.teamRoute && !tc.hasAdmission) {
					require.Empty(t, calls, "failed or unadmitted requests must never reach either billing endpoint")
					host.CompleteHttpContext(id)
					return
				}
				require.Len(t, calls, 1)
				if tc.responseBody != "" {
					var report TokenUsageReportRequest
					require.NoError(t, json.Unmarshal(calls[0].Body, &report))
					require.Zero(t, report.TotalToken, "successful images remain billable without token usage")
					host.CompleteHttpContext(id)
					return
				}
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
				if tc.hasAdmission {
					require.Equal(t, "outbound|8080||onepiece-open-platform-internal.dns", calls[0].Upstream)
					require.Contains(t, calls[0].Headers, [2]string{":path", "/internal/model/usage/team-report"})
				} else {
					require.Equal(t, "outbound|443||billing.dns", calls[0].Upstream)
				}
				for _, field := range []string{"duration", "timestamp", "startTime", "endTime"} {
					require.IsType(t, float64(0), report[field])
					delete(report, field)
				}
				var usage map[string]any
				require.NoError(t, json.Unmarshal([]byte(report["tokenUsage"].(string)), &usage))
				require.Equal(t, float64(10), usage["input_token"])
				require.Equal(t, float64(5), usage["output_token"])
				require.Equal(t, float64(15), usage["total_token"])
				if tc.hasAdmission {
					require.Equal(t, float64(57), report["teamId"])
					require.Equal(t, float64(17), report["personalOwnerUid"])
					delete(report, "teamId")
					delete(report, "personalOwnerUid")
				}
				require.Len(t, report, 9, "legacy report shape is preserved")
				// The serialized asynchronous callout survives stream cleanup.
				host.CompleteHttpContext(id)
				require.Equal(t, tc.expected, gjson.GetBytes(calls[0].Body, "productSource").String())
			})
		}
	})
}
