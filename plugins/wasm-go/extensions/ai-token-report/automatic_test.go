package main

import (
	"encoding/json"
	"testing"

	"github.com/alibaba/higress/plugins/wasm-go/pkg/teambilling"
	"github.com/higress-group/proxy-wasm-go-sdk/proxywasm/types"
	"github.com/higress-group/wasm-go/pkg/test"
	"github.com/stretchr/testify/require"
)

func TestAutomaticReporting(t *testing.T) {
	const image = "ai-route-gemini-3.1-flash-image-preview-times.internal"
	const chat = "ai-route-deepseek-v4-flash.internal"
	const platform = "wujieai-open-platform"
	const create = "/wj-open/v2/open-platform/task/task_create"
	const config = `{"teamReportService":"team.internal.dns","_rules_":[{"_match_route_":["ai-route-gemini-3.1-flash-image-preview-times.internal","legacy"],"reportApiUrl":"https://old.test/legacy-report","reportServiceName":"old"}]}`
	test.RunTest(t, func(t *testing.T) {
		for _, tc := range []struct {
			name, route, method, path, op, settlement, sku, id, owner string
			rejected                                                  bool
			report                                                    string
		}{
			{"chat after rewrite", chat, "POST", "/rewritten", "usage", "", "", "57", "", false, "/internal/model/usage/team-report"},
			{"image after rewrite", image, "POST", "/rewritten", "image", "", teambilling.BananaSKU, "57", "", false, "/internal/model/usage/team-report"},
			{"outer submission", platform, "POST", create, "forward", "forward", "", "57", "", false, ""},
			{"free query", platform, "GET", "/wj-open/v2/open-platform/task/v2/task_info", "read", "", "", "", "", false, ""},
			{"missing admission for legacy chat", chat, "POST", "/v1/chat/completions", "", "", "", "", "", true, ""},
			{"missing admission for image", image, "POST", "/v1/images/generations", "", "", "", "", "", true, ""},
			{"forward marker without admission", platform, "POST", create, "forward", "forward", "", "", "", true, ""},
			{"missing forward marker", platform, "POST", create, "", "", "", "57", "", true, ""},
			{"incorrect image SKU", image, "POST", "/v1/images/generations", "image", "", "wrong", "57", "", true, ""},
			{"personal owner on legacy chat", chat, "POST", "/v1/chat/completions", "usage", "", "", "57", "4", true, ""},
			{"unrelated legacy AI route", "legacy", "POST", "/v1/chat/completions", "", "", "", "", "", false, "/legacy-report"},
		} {
			t.Run(tc.name, func(t *testing.T) {
				h, start := test.NewTestHost([]byte(config))
				defer h.Reset()
				require.Equal(t, types.OnPluginStartStatusOK, start)
				id := h.InitializeHttpContext()
				require.NoError(t, h.SetRouteName(tc.route))
				for k, v := range map[string]string{teambilling.OperationProperty: tc.op, "wasm.teamBillingSettlement": tc.settlement, "wasm.teamId": tc.id, "wasm.teamPersonalOwnerUid": tc.owner, "wasm.teamBillingPerCallModel": tc.sku, "wasm.providerType": "openai", "wasm.requestModel": "test-chat"} {
					if v != "" {
						require.NoError(t, h.SetProperty([]string{k}, []byte(v)))
					}
				}
				headers := [][2]string{{":authority", "test.example"}, {":method", tc.method}, {":path", tc.path}, {OriginalAPIKey, "fixture"}, {"request-id", "run-1"}}
				action := h.CallOnRequestHeaders(id, headers, false)
				if tc.rejected {
					require.NotEqual(t, types.ActionContinue, action)
					require.Equal(t, uint32(503), h.GetSentLocalResponse(id).StatusCode)
					require.Empty(t, h.GetCalloutAttributesFromContext(id))
					return
				}
				require.Equal(t, types.ActionContinue, action)
				if tc.op == "image" {
					require.NoError(t, h.SetProperty([]string{"wasm.teamBillingPerCallModel"}, []byte("later-provider-model")))
				}
				h.CallOnResponseHeaders(id, [][2]string{{":status", "200"}, {"content-type", "application/json"}}, false)
				body := `{"model":"test-chat","usage":{"prompt_tokens":9,"completion_tokens":5,"total_tokens":14}}`
				if tc.op == "image" {
					body = `{"data":[{"url":"https://example.test/image.png"}]}`
				}
				h.CallOnResponseBody(id, []byte(body), true)
				calls := h.GetCalloutAttributesFromContext(id)
				if tc.report == "" {
					require.Empty(t, calls)
					return
				}
				require.Len(t, calls, 1)
				require.Contains(t, calls[0].Headers, [2]string{":path", tc.report})
				var payload TokenUsageReportRequest
				require.NoError(t, json.Unmarshal(calls[0].Body, &payload))
				require.Equal(t, "run-1", payload.RequestId)
				require.Zero(t, payload.PersonalOwnerUID)
				if tc.id != "" {
					require.EqualValues(t, 57, payload.TeamID)
				}
				if tc.op == "image" {
					require.Equal(t, teambilling.BananaSKU, payload.Model)
				}
			})
		}
	})
}
