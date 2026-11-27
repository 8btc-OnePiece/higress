package main

import (
	"testing"

	"github.com/alibaba/higress/plugins/wasm-go/pkg/teambilling"
	"github.com/higress-group/proxy-wasm-go-sdk/proxywasm"
	"github.com/higress-group/proxy-wasm-go-sdk/proxywasm/types"
	"github.com/higress-group/wasm-go/pkg/test"
	"github.com/stretchr/testify/require"
)

func TestAutomaticAdmission(t *testing.T) {
	const image = "ai-route-gemini-3.1-flash-image-preview-times.internal"
	const chat = "ai-route-deepseek-v4-flash.internal"
	const platform = "wujieai-open-platform"
	const create = "/wj-open/v2/open-platform/task/task_create"
	const selected = "9943d4d9-d20b-49db-809d-88586f950fc7"
	const config = `{"teamApiUrl":"https://admission.test/admit","serviceName":"billing","_rules_":[{"_match_route_":["wujieai-open-platform","ai-route-gemini-3.1-flash-image-preview-times.internal"],"apiUrl":"https://billing.test/personal","serviceName":"legacy"}]}`
	test.RunTest(t, func(t *testing.T) {
		for _, tc := range []struct {
			name, route, method, path, version, team, call, op string
			status                                             uint32
		}{
			{"old image selects default", image, "POST", "/v1/images/generations", "", "", "/admit", "image", 0},
			{"selected image", image, "POST", "/v1/images/edits?x=1", "2", selected, "/admit", "image", 0},
			{"chat uses selected team", chat, "POST", "/v1/chat/completions", "2", selected, "/admit", "usage", 0},
			{"legacy chat uses default", chat, "POST", "/v1/chat/completions", "", "", "/admit", "usage", 0},
			{"chat missing team", chat, "POST", "/v1/chat/completions", "2", "", "", "", 400},
			{"selected task", platform, "POST", create, "2", selected, "/admit", "forward", 0},
			{"old task uses default", platform, "POST", create, "", "", "/admit", "forward", 0},
			{"task missing team", platform, "POST", create, "2", "", "", "", 400},
			{"image missing team", image, "POST", "/v1/images/generations", "2", "", "", "", 400},
			{"task query is free", platform, "GET", "/wj-open/v2/open-platform/task/v2/task_info?taskKey=x", "2", selected, "", "read", 0},
			{"other unmarked prefix path retains old check", platform, "GET", "/wj-open/v2/other", "", "", "/personal", "", 0},
			{"unsupported marked operation", platform, "POST", create + "_sync", "2", selected, "", "", 503},
			{"unsupported explicitly selected team", "ai-route-banana.internal", "POST", "/v1/images/generations", "", selected, "", "", 503},
			{"other legacy route retains global pass", "unrelated", "POST", "/other", "", "", "", "", 0},
			{"invalid version", image, "POST", "/v1/images/generations", "3", selected, "", "", 400},
		} {
			t.Run(tc.name, func(t *testing.T) {
				h, status := test.NewTestHost([]byte(config))
				defer h.Reset()
				require.Equal(t, types.OnPluginStartStatusOK, status)
				require.NoError(t, h.SetRouteName(tc.route))
				headers := [][2]string{{":authority", "test.example"}, {":path", tc.path}, {":method", tc.method}, {OriginalAPIKey, "fixture"}}
				if tc.version != "" {
					headers = append(headers, [2]string{"X-Wujie-Billing-Version", tc.version})
				}
				if tc.team != "" {
					headers = append(headers, [2]string{teamUUIDHeader, tc.team})
				}
				action := h.CallOnHttpRequestHeaders(headers)
				calls := h.GetHttpCalloutAttributes()
				if tc.status != 0 {
					require.NotNil(t, h.GetLocalResponse())
					require.Equal(t, tc.status, h.GetLocalResponse().StatusCode)
					require.Empty(t, calls)
					return
				}
				require.Nil(t, h.GetLocalResponse())
				if tc.call != "" {
					require.Len(t, calls, 1)
					require.Contains(t, calls[0].Headers, [2]string{":path", tc.call})
					if tc.call == "/admit" {
						require.Equal(t, types.HeaderStopAllIterationAndWatermark, action)
						want := "{}"
						if tc.team != "" {
							want = `{"teamUuid":"` + selected + `"}`
						}
						require.JSONEq(t, want, string(calls[0].Body))
						h.CallOnHttpCall([][2]string{{":status", "200"}}, []byte(`{"teamId":"57","teamUuid":"`+selected+`"}`))
						require.Nil(t, h.GetLocalResponse())
						v, _ := proxywasm.GetProperty([]string{teamIDProperty})
						require.Equal(t, "57", string(v))
						if tc.op == "forward" {
							v, _ := proxywasm.GetHttpRequestHeader(teamUUIDHeader)
							require.Equal(t, selected, v)
						}
						if tc.op == "image" {
							v, _ := proxywasm.GetProperty([]string{"wasm.teamBillingPerCallModel"})
							require.Equal(t, teambilling.BananaSKU, string(v))
						}
					}
				} else {
					require.Empty(t, calls)
				}
				if tc.op != "" {
					v, _ := proxywasm.GetProperty([]string{teambilling.OperationProperty})
					require.Equal(t, tc.op, string(v))
				}
			})
		}
	})
}
