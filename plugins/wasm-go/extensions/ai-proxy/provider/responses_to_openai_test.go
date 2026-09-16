package provider

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/tidwall/gjson"
)

// ---------------------------------------------------------------------------
// 请求向：Responses → Chat
// ---------------------------------------------------------------------------

func TestConvertResponsesRequestToChat_BasicStringInput(t *testing.T) {
	body := `{
		"model": "deepseek-v4-flash",
		"instructions": "You are a coding agent.",
		"input": "hi",
		"max_output_tokens": 1024,
		"store": false,
		"stream": true,
		"temperature": 0.2
	}`
	out, err := ConvertResponsesRequestToChat([]byte(body))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	g := gjson.ParseBytes(out)
	if g.Get("model").String() != "deepseek-v4-flash" {
		t.Errorf("model = %v", g.Get("model").String())
	}
	if int(g.Get("max_tokens").Int()) != 1024 {
		t.Errorf("max_tokens = %v, want 1024", g.Get("max_tokens"))
	}
	if !g.Get("stream").Bool() {
		t.Error("stream should be true")
	}
	if !g.Get("stream_options.include_usage").Bool() {
		t.Error("stream_options.include_usage must be forced true for billing")
	}
	msgs := g.Get("messages").Array()
	if len(msgs) != 2 {
		t.Fatalf("messages len = %d, want 2 (instructions + user)", len(msgs))
	}
	if msgs[0].Get("role").String() != "system" || msgs[0].Get("content").String() != "You are a coding agent." {
		t.Errorf("system message wrong: %v", msgs[0])
	}
	if msgs[1].Get("role").String() != "user" || msgs[1].Get("content").String() != "hi" {
		t.Errorf("user message wrong: %v", msgs[1])
	}
	if g.Get("temperature").Float() != 0.2 {
		t.Errorf("temperature = %v", g.Get("temperature"))
	}
	// store 等 responses 专属字段不得出现在 chat 请求里
	for _, forbidden := range []string{"store", "instructions", "input", "max_output_tokens", "previous_response_id"} {
		if g.Get(forbidden).Exists() {
			t.Errorf("responses-only field %q leaked into chat request", forbidden)
		}
	}
}

func TestConvertResponsesRequestToChat_CodexStatelessShape(t *testing.T) {
	// codex disable_response_storage 下的真实形态：input 是 item 数组，含
	// 历史 function_call / function_call_output 与 content parts。
	body := `{
		"model": "deepseek-v4-flash",
		"instructions": "sys prompt",
		"input": [
			{"type":"message","role":"user","content":[{"type":"input_text","text":"list files"}]},
			{"type":"reasoning","summary":[]},
			{"type":"function_call","name":"shell","arguments":"{\"cmd\":\"ls\"}","call_id":"call_1"},
			{"type":"function_call_output","call_id":"call_1","output":"file_a\nfile_b"},
			{"type":"message","role":"user","content":[{"type":"input_text","text":"thanks"}]}
		],
		"tools": [
			{"type":"function","name":"shell","description":"run a shell","parameters":{"type":"object"},"strict":true}
		],
		"tool_choice": {"type":"function","name":"shell"},
		"parallel_tool_calls": false,
		"stream": true,
		"store": false
	}`
	out, err := ConvertResponsesRequestToChat([]byte(body))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	g := gjson.ParseBytes(out)

	msgs := g.Get("messages").Array()
	// system + user(list files) + assistant(function_call) + tool(output) + user(thanks)
	if len(msgs) != 5 {
		t.Fatalf("messages len = %d, want 5: %s", len(msgs), out)
	}
	if msgs[0].Get("role").String() != "system" {
		t.Errorf("msg0 role = %v", msgs[0].Get("role"))
	}
	if msgs[1].Get("content.0.type").String() != "text" || msgs[1].Get("content.0.text").String() != "list files" {
		t.Errorf("user content parts wrong: %v", msgs[1].Get("content"))
	}
	if msgs[2].Get("role").String() != "assistant" {
		t.Errorf("msg2 role = %v", msgs[2].Get("role"))
	}
	tc := msgs[2].Get("tool_calls.0")
	if tc.Get("id").String() != "call_1" || tc.Get("function.name").String() != "shell" {
		t.Errorf("assistant tool_calls wrong: %v", tc)
	}
	if tc.Get("function.arguments").String() != `{"cmd":"ls"}` {
		t.Errorf("function arguments wrong: %v", tc.Get("function.arguments"))
	}
	if msgs[3].Get("role").String() != "tool" || msgs[3].Get("tool_call_id").String() != "call_1" {
		t.Errorf("tool message wrong: %v", msgs[3])
	}
	if msgs[3].Get("content").String() != "file_a\nfile_b" {
		t.Errorf("tool content = %q", msgs[3].Get("content").String())
	}

	tools := g.Get("tools").Array()
	if len(tools) != 1 || tools[0].Get("function.name").String() != "shell" {
		t.Fatalf("tools wrong: %v", g.Get("tools"))
	}
	if tools[0].Get("function.strict").Exists() || tools[0].Get("strict").Exists() {
		t.Error("strict field must be stripped (chat upstreams reject unknown openai-only fields)")
	}
	if g.Get("tool_choice.type").String() != "function" || g.Get("tool_choice.function.name").String() != "shell" {
		t.Errorf("tool_choice wrong: %v", g.Get("tool_choice"))
	}
	if g.Get("parallel_tool_calls").Bool() {
		t.Error("parallel_tool_calls should be false")
	}
}

func TestConvertResponsesRequestToChat_RejectsStateful(t *testing.T) {
	cases := map[string]string{
		"previous_response_id": `{"model":"m","input":"hi","previous_response_id":"resp_123"}`,
		"store":                `{"model":"m","input":"hi","store":true}`,
		"background":           `{"model":"m","input":"hi","background":true}`,
		"item_reference":       `{"model":"m","input":[{"type":"item_reference","id":"msg_1"}]}`,
	}
	for name, body := range cases {
		if _, err := ConvertResponsesRequestToChat([]byte(body)); err == nil {
			t.Errorf("%s: expected error, got nil", name)
		}
	}
}

func TestConvertResponsesRequestToChat_DeveloperRoleAndUnknownItem(t *testing.T) {
	body := `{"model":"m","input":[
		{"type":"message","role":"developer","content":"dev rules"},
		{"type":"web_search_call","action":{}},
		{"type":"message","role":"user","content":"q"}
	]}`
	out, err := ConvertResponsesRequestToChat([]byte(body))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	msgs := gjson.GetBytes(out, "messages").Array()
	if len(msgs) != 2 {
		t.Fatalf("messages len = %d, want 2 (developer→system, unknown dropped, user)", len(msgs))
	}
	if msgs[0].Get("role").String() != "system" {
		t.Errorf("developer should map to system, got %v", msgs[0].Get("role"))
	}
}

// ---------------------------------------------------------------------------
// 响应向（非流式）：Chat → Responses
// ---------------------------------------------------------------------------

func TestConvertChatResponseToResponses_Text(t *testing.T) {
	body := `{
		"id": "chatcmpl-abc123",
		"object": "chat.completion",
		"created": 1789441920,
		"model": "deepseek-v4-flash",
		"choices": [{"index":0,"message":{"role":"assistant","content":"hello world"},"finish_reason":"stop"}],
		"usage": {"prompt_tokens": 11, "completion_tokens": 7, "total_tokens": 18, "completion_tokens_details": {"reasoning_tokens": 3}}
	}`
	out, err := ConvertChatResponseToResponses([]byte(body))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	g := gjson.ParseBytes(out)
	if g.Get("object").String() != "response" {
		t.Errorf("object = %v", g.Get("object"))
	}
	if !strings.HasPrefix(g.Get("id").String(), "resp_") {
		t.Errorf("id = %v, want resp_ prefix", g.Get("id"))
	}
	if g.Get("status").String() != "completed" {
		t.Errorf("status = %v", g.Get("status"))
	}
	msg := g.Get("output.0")
	if msg.Get("type").String() != "message" || msg.Get("role").String() != "assistant" {
		t.Errorf("output item wrong: %v", msg)
	}
	if msg.Get("content.0.type").String() != "output_text" || msg.Get("content.0.text").String() != "hello world" {
		t.Errorf("content wrong: %v", msg.Get("content"))
	}
	u := g.Get("usage")
	if u.Get("input_tokens").Int() != 11 || u.Get("output_tokens").Int() != 7 || u.Get("total_tokens").Int() != 18 {
		t.Errorf("usage mapping wrong: %v", u)
	}
	if u.Get("output_tokens_details.reasoning_tokens").Int() != 3 {
		t.Errorf("reasoning tokens lost: %v", u)
	}
}

func TestConvertChatResponseToResponses_ToolCallsAndLength(t *testing.T) {
	body := `{
		"id": "chatcmpl-x",
		"created": 1,
		"model": "m",
		"choices": [{"index":0,"message":{"role":"assistant","content":null,"tool_calls":[
			{"id":"call_1","type":"function","function":{"name":"shell","arguments":"{\"cmd\":\"ls\"}"}}
		]},"finish_reason":"tool_calls"}],
		"usage": {"prompt_tokens": 5, "completion_tokens": 2, "total_tokens": 7}
	}`
	out, err := ConvertChatResponseToResponses([]byte(body))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	g := gjson.ParseBytes(out)
	fc := g.Get("output.0")
	if fc.Get("type").String() != "function_call" {
		t.Fatalf("output.0 type = %v, want function_call", fc.Get("type"))
	}
	if fc.Get("call_id").String() != "call_1" || fc.Get("name").String() != "shell" {
		t.Errorf("function_call fields wrong: %v", fc)
	}
	if g.Get("status").String() != "completed" {
		t.Errorf("tool_calls finish should be completed, got %v", g.Get("status"))
	}

	lengthBody := strings.Replace(string(body), `"finish_reason":"tool_calls"`, `"finish_reason":"length"`, 1)
	out2, _ := ConvertChatResponseToResponses([]byte(lengthBody))
	if gjson.GetBytes(out2, "status").String() != "incomplete" {
		t.Errorf("length finish should be incomplete")
	}
	if gjson.GetBytes(out2, "incomplete_details.reason").String() != "max_output_tokens" {
		t.Errorf("incomplete reason wrong: %v", gjson.GetBytes(out2, "incomplete_details.reason"))
	}
}

func TestConvertChatResponseToResponses_RejectsGarbage(t *testing.T) {
	if _, err := ConvertChatResponseToResponses([]byte(`{"error":{"message":"boom"}}`)); err == nil {
		t.Error("expected error for non-chat body")
	}
}

// ---------------------------------------------------------------------------
// 响应向（流式）：Chat SSE → Responses 事件流
// ---------------------------------------------------------------------------

// parseSSE 把转换器输出按事件拆开，返回 (event, dataJSON) 序列。
func parseSSE(t *testing.T, raw []byte) []gjson.Result {
	t.Helper()
	var events []gjson.Result
	for _, block := range strings.Split(strings.TrimSpace(string(raw)), "\n\n") {
		block = strings.TrimSpace(block)
		if block == "" {
			continue
		}
		var dataLine string
		for _, line := range strings.Split(block, "\n") {
			if strings.HasPrefix(line, "data: ") {
				dataLine = strings.TrimPrefix(line, "data: ")
			}
		}
		if dataLine == "" {
			t.Fatalf("SSE block without data line: %q", block)
		}
		events = append(events, gjson.Parse(dataLine))
	}
	return events
}

func eventTypes(events []gjson.Result) []string {
	types := make([]string, len(events))
	for i, e := range events {
		types[i] = e.Get("type").String()
	}
	return types
}

func feedChunks(t *testing.T, chunks []string) []gjson.Result {
	t.Helper()
	conv := NewChatToResponsesStreamConverter()
	var all strings.Builder
	for i, c := range chunks {
		out, err := conv.ProcessChunk([]byte(c), i == len(chunks)-1)
		if err != nil {
			t.Fatalf("chunk %d: unexpected error: %v", i, err)
		}
		if out == nil {
			t.Fatalf("chunk %d: nil output would pass original chunk through", i)
		}
		all.Write(out)
	}
	return parseSSE(t, []byte(all.String()))
}

func TestStreamConversion_TextHappyPath(t *testing.T) {
	chunks := []string{
		"data: {\"id\":\"chatcmpl-1\",\"created\":1789441920,\"model\":\"deepseek-v4-flash\",\"choices\":[{\"index\":0,\"delta\":{\"role\":\"assistant\",\"content\":\"\"},\"finish_reason\":null}]}\n\n",
		"data: {\"id\":\"chatcmpl-1\",\"choices\":[{\"index\":0,\"delta\":{\"content\":\"Hel\"},\"finish_reason\":null}]}\n\n",
		"data: {\"id\":\"chatcmpl-1\",\"choices\":[{\"index\":0,\"delta\":{\"content\":\"lo\"},\"finish_reason\":null}]}\n\n",
		"data: {\"id\":\"chatcmpl-1\",\"choices\":[{\"index\":0,\"delta\":{},\"finish_reason\":\"stop\"}]}\n\n",
		"data: {\"id\":\"chatcmpl-1\",\"choices\":[],\"usage\":{\"prompt_tokens\":9,\"completion_tokens\":4,\"total_tokens\":13}}\n\n",
		"data: [DONE]\n\n",
	}
	events := feedChunks(t, chunks)
	got := eventTypes(events)

	wantPrefix := []string{
		"response.created",
		"response.in_progress",
		"response.output_item.added",
		"response.content_part.added",
		"response.output_text.delta",
		"response.output_text.delta",
		"response.output_text.done",
		"response.content_part.done",
		"response.output_item.done",
		"response.completed",
	}
	if len(got) != len(wantPrefix) {
		t.Fatalf("event sequence mismatch:\ngot:  %v\nwant: %v", got, wantPrefix)
	}
	for i := range wantPrefix {
		if got[i] != wantPrefix[i] {
			t.Fatalf("event[%d] = %s, want %s\nfull: %v", i, got[i], wantPrefix[i], got)
		}
	}

	// delta 累积与顺序号
	var deltas []string
	for _, e := range events {
		if e.Get("type").String() == "response.output_text.delta" {
			deltas = append(deltas, e.Get("delta").String())
		}
	}
	if strings.Join(deltas, "") != "Hello" {
		t.Errorf("deltas = %v, want Hello", deltas)
	}

	completed := events[len(events)-1]
	if completed.Get("response.usage.input_tokens").Int() != 9 ||
		completed.Get("response.usage.output_tokens").Int() != 4 ||
		completed.Get("response.usage.total_tokens").Int() != 13 {
		t.Errorf("completed usage wrong: %v", completed.Get("response.usage"))
	}
	item := completed.Get("response.output.0")
	if item.Get("type").String() != "message" || item.Get("content.0.text").String() != "Hello" {
		t.Errorf("completed output wrong: %v", item)
	}
	if completed.Get("response.status").String() != "completed" {
		t.Errorf("completed status = %v", completed.Get("response.status"))
	}

	// sequence_number 单调
	prev := 0
	for _, e := range events {
		seq := int(e.Get("sequence_number").Int())
		if seq <= prev {
			t.Errorf("sequence_number not increasing: %d after %d", seq, prev)
		}
		prev = seq
	}
}

func TestStreamConversion_ToolCalls(t *testing.T) {
	chunks := []string{
		"data: {\"id\":\"c1\",\"created\":1,\"model\":\"m\",\"choices\":[{\"index\":0,\"delta\":{\"role\":\"assistant\",\"tool_calls\":[{\"index\":0,\"id\":\"call_1\",\"type\":\"function\",\"function\":{\"name\":\"shell\",\"arguments\":\"\"}}]},\"finish_reason\":null}]}\n\n",
		"data: {\"id\":\"c1\",\"choices\":[{\"index\":0,\"delta\":{\"tool_calls\":[{\"index\":0,\"function\":{\"arguments\":\"{\\\"cmd\\\":\\\"\"}}]},\"finish_reason\":null}]}\n\n",
		"data: {\"id\":\"c1\",\"choices\":[{\"index\":0,\"delta\":{\"tool_calls\":[{\"index\":0,\"function\":{\"arguments\":\"ls\\\"}\"}}]},\"finish_reason\":null}]}\n\n",
		"data: {\"id\":\"c1\",\"choices\":[{\"index\":0,\"delta\":{},\"finish_reason\":\"tool_calls\"}]}\n\n",
		"data: {\"id\":\"c1\",\"choices\":[],\"usage\":{\"prompt_tokens\":4,\"completion_tokens\":6,\"total_tokens\":10}}\n\n",
		"data: [DONE]\n\n",
	}
	events := feedChunks(t, chunks)
	got := eventTypes(events)

	want := []string{
		"response.created",
		"response.in_progress",
		"response.output_item.added",
		"response.function_call_arguments.delta",
		"response.function_call_arguments.delta",
		"response.function_call_arguments.done",
		"response.output_item.done",
		"response.completed",
	}
	if len(got) != len(want) {
		t.Fatalf("event sequence mismatch:\ngot:  %v\nwant: %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("event[%d] = %s, want %s\nfull: %v", i, got[i], want[i], got)
		}
	}
	fcItem := events[len(events)-1].Get("response.output.0")
	if fcItem.Get("type").String() != "function_call" {
		t.Fatalf("final output.0 = %v", fcItem)
	}
	if fcItem.Get("call_id").String() != "call_1" || fcItem.Get("name").String() != "shell" {
		t.Errorf("function_call fields wrong: %v", fcItem)
	}
	if fcItem.Get("arguments").String() != `{"cmd":"ls"}` {
		t.Errorf("assembled arguments = %v, want {\"cmd\":\"ls\"}", fcItem.Get("arguments"))
	}
}

func TestStreamConversion_MixedTextAndTools(t *testing.T) {
	chunks := []string{
		"data: {\"id\":\"c2\",\"created\":1,\"model\":\"m\",\"choices\":[{\"index\":0,\"delta\":{\"role\":\"assistant\",\"content\":\"Let me check.\"},\"finish_reason\":null}]}\n\n",
		"data: {\"id\":\"c2\",\"choices\":[{\"index\":0,\"delta\":{\"tool_calls\":[{\"index\":0,\"id\":\"call_a\",\"type\":\"function\",\"function\":{\"name\":\"read\",\"arguments\":\"{\\\"path\\\":\\\"x\\\"}\"}},{\"index\":1,\"id\":\"call_b\",\"type\":\"function\",\"function\":{\"name\":\"write\",\"arguments\":\"{}\"}}]},\"finish_reason\":null}]}\n\n",
		"data: {\"id\":\"c2\",\"choices\":[{\"index\":0,\"delta\":{},\"finish_reason\":\"tool_calls\"}]}\n\n",
		"data: [DONE]\n\n",
	}
	events := feedChunks(t, chunks)
	completed := events[len(events)-1]
	if completed.Get("type").String() != "response.completed" {
		t.Fatalf("last event = %v", completed.Get("type"))
	}
	output := completed.Get("response.output").Array()
	if len(output) != 3 {
		t.Fatalf("final output items = %d, want 3 (message + 2 function_calls): %v", len(output), completed.Get("response.output"))
	}
	if output[0].Get("type").String() != "message" || output[0].Get("content.0.text").String() != "Let me check." {
		t.Errorf("output[0] wrong: %v", output[0])
	}
	if output[1].Get("call_id").String() != "call_a" || output[2].Get("call_id").String() != "call_b" {
		t.Errorf("tool outputs wrong: %v / %v", output[1], output[2])
	}
	// output_index：message=0，两个 tool 依次 1、2
	if output[1].Get("id").String() == "" {
		t.Errorf("function_call id missing: %v", output[1])
	}
}

func TestStreamConversion_ChunkSplitAtLineBoundary(t *testing.T) {
	// TCP 分段把一行 data 撕成三段，转换器必须拼回完整 JSON 而不是泄漏原文。
	full := "data: {\"id\":\"c3\",\"created\":1,\"model\":\"m\",\"choices\":[{\"index\":0,\"delta\":{\"content\":\"torn line\"},\"finish_reason\":null}]}\n\ndata: [DONE]\n\n"
	parts := []string{full[:20], full[20:45], full[45:70], full[70:]}
	events := feedChunks(t, parts)

	var sawDelta bool
	for _, e := range events {
		if e.Get("type").String() == "response.output_text.delta" {
			sawDelta = true
			if e.Get("delta").String() != "torn line" {
				t.Errorf("delta = %q, want %q", e.Get("delta").String(), "torn line")
			}
		}
	}
	if !sawDelta {
		t.Fatal("torn-line delta never assembled")
	}
	if events[len(events)-1].Get("type").String() != "response.completed" {
		t.Error("completed missing after torn stream")
	}
}

func TestStreamConversion_MissingDoneFinalizesOnLastChunk(t *testing.T) {
	chunks := []string{
		"data: {\"id\":\"c4\",\"created\":1,\"model\":\"m\",\"choices\":[{\"index\":0,\"delta\":{\"content\":\"hi\"},\"finish_reason\":\"stop\"}]}\n\n",
		// 上游异常断流：没有 [DONE]，isLastChunk 兜底收尾
	}
	conv := NewChatToResponsesStreamConverter()
	var all strings.Builder
	for i, c := range chunks {
		out, _ := conv.ProcessChunk([]byte(c), i == len(chunks)-1)
		all.Write(out)
	}
	events := parseSSE(t, []byte(all.String()))
	if len(events) == 0 {
		t.Fatal("no events emitted")
	}
	last := events[len(events)-1]
	if last.Get("type").String() != "response.completed" {
		t.Fatalf("force-finalize failed, last = %v", last.Get("type"))
	}
	if last.Get("response.output.0.content.0.text").String() != "hi" {
		t.Errorf("finalized output wrong: %v", last.Get("response.output"))
	}
}

func TestStreamConversion_EmptyOutputChunkIsNonNil(t *testing.T) {
	conv := NewChatToResponsesStreamConverter()
	out, err := conv.ProcessChunk([]byte("data: {\"partial\": "), false)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if out == nil {
		t.Fatal("nil output would let the host pass the original chat chunk through")
	}
	if len(out) != 0 {
		t.Errorf("partial line should produce no output, got %q", out)
	}
}

// 编译期保证 wire 结构的 JSON 形态稳定（codex SDK 反序列化依赖字段名）。
func TestResponsesWireShapeStability(t *testing.T) {
	b, err := json.Marshal(responsesMessageItem{
		Id: "m", Type: "message", Role: "assistant", Status: "completed",
		Content: []responsesOutputText{{Type: "output_text", Text: "t", Annotations: []interface{}{}}},
	})
	if err != nil {
		t.Fatal(err)
	}
	g := gjson.ParseBytes(b)
	for _, key := range []string{"id", "type", "role", "status", "content"} {
		if !g.Get(key).Exists() {
			t.Errorf("responsesMessageItem missing wire key %q: %s", key, b)
		}
	}
	fc, _ := json.Marshal(responsesFunctionCallItem{Id: "f", Type: "function_call", Status: "completed", CallId: "c", Name: "n", Arguments: "{}"})
	gf := gjson.ParseBytes(fc)
	for _, key := range []string{"id", "type", "status", "call_id", "name", "arguments"} {
		if !gf.Get(key).Exists() {
			t.Errorf("responsesFunctionCallItem missing wire key %q: %s", key, fc)
		}
	}
}
