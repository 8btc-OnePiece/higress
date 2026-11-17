package main

import (
	"encoding/json"
	"net/http"
	"strconv"

	"github.com/higress-group/proxy-wasm-go-sdk/proxywasm"
	"github.com/higress-group/proxy-wasm-go-sdk/proxywasm/types"
	"github.com/higress-group/wasm-go/pkg/wrapper"
)

const teamUUIDHeader = "X-Wujie-Team-Id"
const teamIDProperty = "wasm.teamId"
const personalOwnerProperty = "wasm.teamPersonalOwnerUid"

func checkTeamAdmission(ctx wrapper.HttpContext, config BalanceCheckConfig, teamUUID string) types.Action {
	apiKey, _ := proxywasm.GetHttpRequestHeader(OriginalAPIKey)
	if apiKey == "" {
		return teamAdmissionError(http.StatusUnauthorized)
	}
	endpoint := config.teamEndpoint
	body, _ := json.Marshal(struct {
		TeamUUID string `json:"teamUuid"`
	}{TeamUUID: teamUUID})
	// The wrapper logs headers and bodies at info level. Keep this callout private.
	_, err := proxywasm.DispatchHttpCall(config.checkClient.ClusterName(), [][2]string{
		{":method", http.MethodPost},
		{":authority", endpoint.Hostname()},
		{":path", endpoint.RequestURI()},
		{"Authorization", "Bearer " + apiKey},
		{"Content-Type", "application/json"},
	}, body, nil, uint32(config.apiTimeout), func(_, bodySize, _ int) {
		status := teamAdmissionStatus()
		if status != http.StatusOK {
			teamAdmissionError(status)
			return
		}
		body, err := proxywasm.GetHttpCallResponseBody(0, bodySize)
		if err != nil {
			teamAdmissionError(http.StatusServiceUnavailable)
			return
		}
		var result struct {
			TeamID           string `json:"teamId"`
			PersonalOwnerUID string `json:"personalOwnerUid"`
		}
		if err := json.Unmarshal(body, &result); err != nil {
			teamAdmissionError(http.StatusServiceUnavailable)
			return
		}
		teamID, err := strconv.ParseInt(result.TeamID, 10, 64)
		if err != nil || teamID <= 0 {
			teamAdmissionError(http.StatusServiceUnavailable)
			return
		}
		if err := proxywasm.SetProperty([]string{teamIDProperty}, []byte(strconv.FormatInt(teamID, 10))); err != nil {
			teamAdmissionError(http.StatusServiceUnavailable)
			return
		}
		if result.PersonalOwnerUID != "" {
			owner, err := strconv.ParseInt(result.PersonalOwnerUID, 10, 64)
			if err != nil || owner <= 0 || proxywasm.SetProperty(
				[]string{personalOwnerProperty}, []byte(result.PersonalOwnerUID)) != nil {
				teamAdmissionError(http.StatusServiceUnavailable)
				return
			}
		}
		if err := proxywasm.RemoveHttpRequestHeader(teamUUIDHeader); err != nil {
			teamAdmissionError(http.StatusServiceUnavailable)
			return
		}
		ctx.SetContext(CtxKeyBalanceChecked, true)
		_ = proxywasm.ResumeHttpRequest()
	})
	if err != nil {
		return teamAdmissionError(http.StatusServiceUnavailable)
	}
	return types.ActionPause
}

func teamAdmissionStatus() int {
	headers, _ := proxywasm.GetHttpCallResponseHeaders()
	for _, h := range headers {
		if h[0] == ":status" {
			status, _ := strconv.Atoi(h[1])
			return status
		}
	}
	return http.StatusServiceUnavailable
}

func teamAdmissionError(status int) types.Action {
	var code string
	switch status {
	case http.StatusBadRequest:
		code = "invalid_team"
	case http.StatusUnauthorized:
		code = "authentication_required"
	case http.StatusForbidden:
		code = "team_access_denied"
	case http.StatusPaymentRequired:
		code = "insufficient_balance"
	default:
		status = http.StatusServiceUnavailable
		code = "team_billing_unavailable"
	}
	var response OpenAIError
	response.Error.Message = http.StatusText(status)
	response.Error.Type = code
	response.Error.Code = code
	body, _ := json.Marshal(response)
	_ = proxywasm.SendHttpResponse(uint32(status), [][2]string{{"Content-Type", "application/json"}}, body, -1)
	return types.HeaderStopAllIterationAndWatermark
}
