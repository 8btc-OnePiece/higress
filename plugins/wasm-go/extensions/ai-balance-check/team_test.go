package main

import (
	"fmt"
	"testing"

	"github.com/higress-group/proxy-wasm-go-sdk/proxywasm"
	"github.com/higress-group/proxy-wasm-go-sdk/proxywasm/types"
	"github.com/higress-group/wasm-go/pkg/test"
)

func TestTeamAdmission(t *testing.T) {
	config := []byte(`{"teamBilling":true,"apiUrl":"https://example.com/legacy","teamApiUrl":"https://example.com/check","serviceName":"admission"}`)
	test.RunTest(t, func(t *testing.T) {
		for _, tc := range []struct {
			name      string
			upstream  string
			body      string
			wantError uint32
		}{
			{"accepted", "200", `{"teamId":"42"}`, 0},
			{"not a member", "403", "", 403},
			{"negative balance", "402", "", 402},
			{"missing internal id", "200", "{}", 503},
			{"upstream failure", "500", "", 503},
		} {
			t.Run(tc.name, func(t *testing.T) {
				host, status := test.NewTestHost(config)
				defer host.Reset()
				if status != types.OnPluginStartStatusOK {
					t.Fatal(status)
				}
				const uuid = "9943d4d9-d20b-49db-809d-88586f950fc7"
				if action := host.CallOnHttpRequestHeaders([][2]string{
					{":authority", "example.com"},
					{OriginalAPIKey, "unit-test-key"},
					{teamUUIDHeader, uuid},
				}); action != types.ActionPause {
					t.Fatalf("action = %v", action)
				}
				calls := host.GetHttpCalloutAttributes()
				if len(calls) != 1 || !hasCalloutHeader(calls[0].Headers, ":path", "/check") || string(calls[0].Body) != fmt.Sprintf(`{"teamUuid":%q}`, uuid) {
					t.Fatalf("unexpected admission calls: %+v", calls)
				}
				host.CallOnHttpCall([][2]string{{":status", tc.upstream}}, []byte(tc.body))
				local := host.GetLocalResponse()
				if tc.wantError == 0 {
					value, err := proxywasm.GetProperty([]string{teamIDProperty})
					if local != nil || err != nil || string(value) != "42" {
						t.Fatalf("expected validated ID and no local error: %q %v %+v", value, err, local)
					}
				} else if local == nil || local.StatusCode != tc.wantError {
					t.Fatalf("expected status %d, got %+v", tc.wantError, local)
				}
				host.CompleteHttp()
			})
		}
	})
}

func TestMissingTeamSelectorUsesLegacyBalance(t *testing.T) {
	config := []byte(`{"teamBilling":true,"apiUrl":"https://example.com/legacy","teamApiUrl":"https://example.com/check","serviceName":"admission"}`)
	test.RunTest(t, func(t *testing.T) {
		host, status := test.NewTestHost(config)
		defer host.Reset()
		if status != types.OnPluginStartStatusOK {
			t.Fatal(status)
		}
		if action := host.CallOnHttpRequestHeaders([][2]string{
			{":authority", "example.com"},
			{OriginalAPIKey, "unit-test-key"},
		}); action != types.ActionPause {
			t.Fatalf("action = %v", action)
		}
		calls := host.GetHttpCalloutAttributes()
		if len(calls) != 1 || len(calls[0].Body) != 0 ||
			!hasCalloutHeader(calls[0].Headers, ":method", "GET") || !hasCalloutHeader(calls[0].Headers, ":path", "/legacy") {
			t.Fatalf("unexpected legacy balance request: %+v", calls)
		}
		host.CallOnHttpCall([][2]string{{":status", "200"}}, []byte("true"))
		if _, err := proxywasm.GetProperty([]string{teamIDProperty}); err != types.ErrorStatusNotFound || host.GetLocalResponse() != nil {
			t.Fatalf("legacy request unexpectedly acquired a team ID or failed")
		}
		host.CompleteHttp()
	})
}

func hasCalloutHeader(headers [][2]string, name, value string) bool {
	for _, header := range headers {
		if header[0] == name && header[1] == value {
			return true
		}
	}
	return false
}
