package main

import (
	"encoding/json"
	"fmt"
	"testing"

	"github.com/alibaba/higress/plugins/wasm-go/pkg/teambilling"
	"github.com/higress-group/proxy-wasm-go-sdk/proxywasm"
	"github.com/higress-group/proxy-wasm-go-sdk/proxywasm/proxytest"
	"github.com/higress-group/proxy-wasm-go-sdk/proxywasm/types"
	"github.com/higress-group/wasm-go/pkg/test"
	"github.com/stretchr/testify/require"
)

func TestUnlistedEmbeddingFilterChain(t *testing.T) {
	binary := buildBalanceFilter(t)
	const route = "custom-model"
	const balanceRule = `"_rules_":[{"_match_route_":["custom-model"],
		"teamBilling":true,"teamRouteMode":"compat","teamSettlement":"usage",
		"teamPaidPaths":["/v1/chat/completions"],"teamUnlistedPathConfig":{},
		"apiUrl":"https://billing.test/legacy","serviceName":"billing","teamApiUrl":"https://billing.test/admit"}]`
	for _, automatic := range []bool{false, true} {
		t.Run(fmt.Sprintf("automatic-%t", automatic), func(t *testing.T) {
			var headers [][2]string
			properties := map[string][]byte{}
			global := ""
			if automatic {
				global = `"teamApiUrl":"https://billing.test/admit","serviceName":"billing",`
			}
			test.RunWasmTestWithPath(t, binary, func(t *testing.T) {
				h, start := test.NewTestHost([]byte("{" + global + balanceRule + "}"))
				defer h.Reset()
				require.Equal(t, types.OnPluginStartStatusOK, start)
				require.NoError(t, h.SetRouteName(route))
				require.NoError(t, h.SetProperty([]string{"wasm.providerType"}, []byte("openai")))
				action := h.CallOnHttpRequestHeaders([][2]string{
					{":authority", "model.test"}, {":method", "POST"}, {":path", "/v1/embeddings"},
					{OriginalAPIKey, "fixture"}, {"request-id", "embedding-1"},
				})
				require.Equal(t, types.ActionContinue, action)
				require.Nil(t, h.GetLocalResponse())
				require.Empty(t, h.GetHttpCalloutAttributes())
				headers = h.GetRequestHeaders()
				for _, name := range []string{teambilling.OperationProperty, "wasm.teamBillingUnlistedPath", "wasm.teamId", "wasm.providerType"} {
					value, err := proxywasm.GetProperty([]string{name})
					if err == types.ErrorStatusNotFound {
						continue
					}
					require.NoError(t, err)
					properties[name] = append([]byte(nil), value...)
				}
				require.Equal(t, "1", string(properties["wasm.teamBillingUnlistedPath"]))
				require.NotContains(t, properties, "wasm.teamId")
			})
			for _, tc := range []struct {
				name, snapshot string
				report, reject bool
			}{
				{name: "old report enabled", snapshot: `{"reportApiUrl":"https://old.test/previous","reportServiceName":"previous"}`, report: true},
				{name: "old report disabled", snapshot: `{}`},
				{name: "no historical policy", reject: true},
			} {
				t.Run(tc.name, func(t *testing.T) {
					config := map[string]any{}
					if automatic {
						config["teamReportService"] = "team.internal.dns"
					}
					rule := map[string]any{
						"_match_route_": []string{route}, "requireTeamId": true,
						"teamReportService": "team.internal.dns", "reportApiUrl": "https://billing.test/current",
					}
					if tc.snapshot != "" {
						rule["teamUnlistedPathConfig"] = json.RawMessage(tc.snapshot)
					}
					config["_rules_"] = []any{rule}
					cfg, err := json.Marshal(config)
					require.NoError(t, err)
					test.RunTest(t, func(t *testing.T) {
						h, start := test.NewTestHost(cfg)
						defer h.Reset()
						require.Equal(t, types.OnPluginStartStatusOK, start)
						require.NoError(t, h.SetRouteName(route))
						for name, value := range properties {
							require.NoError(t, h.SetProperty([]string{name}, value))
						}
						require.NoError(t, h.SetProperty([]string{"wasm.requestModel"}, []byte("embedding-test")))
						action := h.CallOnHttpRequestHeaders(headers)
						if tc.reject {
							require.NotNil(t, h.GetLocalResponse())
							require.EqualValues(t, 503, h.GetLocalResponse().StatusCode)
							require.Empty(t, h.GetHttpCalloutAttributes())
							return
						}
						require.Nil(t, h.GetLocalResponse())
						require.Equal(t, types.ActionContinue, action)
						h.CallOnHttpResponseHeaders([][2]string{{":status", "200"}, {"content-type", "application/json"}})
						h.CallOnHttpResponseBody([]byte(`{"model":"embedding-test","data":[],"usage":{"prompt_tokens":7,"total_tokens":7}}`))
						calls := h.GetHttpCalloutAttributes()
						if tc.report {
							require.Len(t, calls, 1)
							require.Contains(t, calls[0].Headers, [2]string{":path", "/previous"})
							require.Equal(t, "outbound|443||previous.dns", calls[0].Upstream)
							var report TokenUsageReportRequest
							require.NoError(t, json.Unmarshal(calls[0].Body, &report))
							require.Equal(t, "embedding-1", report.RequestId)
							require.EqualValues(t, 7, report.TotalToken)
						} else {
							require.Empty(t, calls)
						}
						h.CompleteHttp()
						h.Tick()
						require.Empty(t, h.GetCalloutAttributesFromContext(proxytest.PluginContextID))
					})
				})
			}
		})
	}
}
