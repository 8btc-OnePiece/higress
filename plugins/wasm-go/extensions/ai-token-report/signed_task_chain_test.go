package main

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/alibaba/higress/plugins/wasm-go/pkg/teambilling"
	"github.com/higress-group/proxy-wasm-go-sdk/proxywasm"
	"github.com/higress-group/proxy-wasm-go-sdk/proxywasm/proxytest"
	"github.com/higress-group/proxy-wasm-go-sdk/proxywasm/types"
	"github.com/higress-group/wasm-go/pkg/test"
	"github.com/stretchr/testify/require"
)

func TestSignedTaskFilterChain(t *testing.T) {
	binary := filepath.Join(t.TempDir(), "balance.wasm")
	build := exec.Command("go", "build", "-buildmode=c-shared", "-o", binary, ".")
	build.Dir = "../ai-balance-check"
	build.Env = append(os.Environ(), "GOOS=wasip1", "GOARCH=wasm")
	output, err := build.CombinedOutput()
	require.NoError(t, err, "%s", output)

	const route = "wujieai-open-platform"
	const uuid = "3493d4d9-d20b-49db-809d-88586f950fc7"
	for _, settlement := range []string{"task_create", "task_create_sync"} {
		for _, automatic := range []bool{true, false} {
			t.Run(fmt.Sprintf("%s/automatic-admission-%t", settlement, automatic), func(t *testing.T) {
				config := map[string]any{
					"apiUrl": "https://billing.test/legacy", "serviceName": "billing",
					"teamApiUrl": "https://billing.test/admit", "teamBilling": true,
					"teamRouteMode": "strict", "teamSettlement": settlement,
					"teamSigningKey": "fixture-only-0123456789-0123456789",
				}
				if automatic {
					config["_match_route_"] = []string{route}
					config = map[string]any{
						"teamApiUrl": "https://billing.test/admit", "serviceName": "billing",
						"_rules_": []any{config},
					}
				}
				balanceConfig, err := json.Marshal(config)
				require.NoError(t, err)
				var headers [][2]string
				properties := map[string][]byte{}
				test.RunWasmTestWithPath(t, binary, func(t *testing.T) {
					h, start := test.NewTestHost(balanceConfig)
					defer h.Reset()
					require.Equal(t, types.OnPluginStartStatusOK, start)
					require.NoError(t, h.SetRouteName(route))
					h.CallOnHttpRequestHeaders([][2]string{
						{":method", "POST"}, {":authority", "test.example"},
						{":path", "/wj-open/v2/open-platform/task/" + settlement + "?source=test"},
						{OriginalAPIKey, "fixture"}, {"request-id", "signed-request"},
						{teamUUIDHeader, uuid}, {"X-Wujie-Billing-Version", "2"},
					})
					require.Len(t, h.GetHttpCalloutAttributes(), 1)
					h.CallOnHttpCall([][2]string{{":status", "200"}}, []byte(`{"teamId":"57","teamUuid":"`+uuid+`"}`))
					require.Nil(t, h.GetLocalResponse())
					headers = h.GetRequestHeaders()
					for _, name := range []string{teambilling.OperationProperty, "wasm.teamBillingSettlement", "wasm.teamId", "wasm.teamPersonalOwnerUid"} {
						v, err := proxywasm.GetProperty([]string{name})
						if err == types.ErrorStatusNotFound {
							continue
						}
						require.NoError(t, err)
						properties[name] = append([]byte(nil), v...)
					}
					require.Equal(t, settlement, string(properties["wasm.teamBillingSettlement"]))
					require.Equal(t, "57", string(properties["wasm.teamId"]))
					v, err := proxywasm.GetHttpRequestHeader("X-Wujie-Team-Signature")
					require.NoError(t, err)
					require.NotEmpty(t, v)
				})
				test.RunTest(t, func(t *testing.T) {
					h, start := test.NewTestHost([]byte(`{"teamReportService":"team.internal.dns","reportApiUrl":"https://billing.test/legacy-report"}`))
					defer h.Reset()
					require.Equal(t, types.OnPluginStartStatusOK, start)
					require.NoError(t, h.SetRouteName(route))
					for k, v := range properties {
						require.NoError(t, h.SetProperty([]string{k}, v))
					}
					require.Equal(t, types.ActionContinue, h.CallOnHttpRequestHeaders(headers))
					require.Nil(t, h.GetLocalResponse())
					h.CallOnHttpResponseHeaders([][2]string{{":status", "200"}, {"content-type", "application/json"}})
					h.CallOnHttpResponseBody([]byte(`{"taskKey":"fixture"}`))
					require.Empty(t, h.GetHttpCalloutAttributes())
					h.CompleteHttp()
					h.Tick()
					require.Empty(t, h.GetCalloutAttributesFromContext(proxytest.PluginContextID))
				})
			})
		}
	}
}
