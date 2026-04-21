package chat_completions

import (
	"context"
	"testing"

	"github.com/tidwall/gjson"
)

func TestConvertCodexResponseToOpenAI_StreamSetsModelFromResponseCreated(t *testing.T) {
	ctx := context.Background()
	var param any

	modelName := "gpt-5.3-codex"

	out := ConvertCodexResponseToOpenAI(ctx, modelName, nil, nil, []byte(`data: {"type":"response.created","response":{"id":"resp_123","created_at":1700000000,"model":"gpt-5.3-codex"}}`), &param)
	if len(out) != 0 {
		t.Fatalf("expected no output for response.created, got %d chunks", len(out))
	}

	out = ConvertCodexResponseToOpenAI(ctx, modelName, nil, nil, []byte(`data: {"type":"response.output_text.delta","delta":"hello"}`), &param)
	if len(out) != 1 {
		t.Fatalf("expected 1 chunk, got %d", len(out))
	}

	gotModel := gjson.GetBytes(out[0], "model").String()
	if gotModel != modelName {
		t.Fatalf("expected model %q, got %q", modelName, gotModel)
	}
}

func TestConvertCodexResponseToOpenAI_FirstChunkUsesRequestModelName(t *testing.T) {
	ctx := context.Background()
	var param any

	modelName := "gpt-5.3-codex"

	out := ConvertCodexResponseToOpenAI(ctx, modelName, nil, nil, []byte(`data: {"type":"response.output_text.delta","delta":"hello"}`), &param)
	if len(out) != 1 {
		t.Fatalf("expected 1 chunk, got %d", len(out))
	}

	gotModel := gjson.GetBytes(out[0], "model").String()
	if gotModel != modelName {
		t.Fatalf("expected model %q, got %q", modelName, gotModel)
	}
}

func TestConvertCodexResponseToOpenAI_ToolCallChunkOmitsNullContentFields(t *testing.T) {
	ctx := context.Background()
	var param any

	out := ConvertCodexResponseToOpenAI(ctx, "gpt-5.4", nil, nil, []byte(`data: {"type":"response.output_item.added","item":{"type":"function_call","call_id":"call_123","name":"websearch"}}`), &param)
	if len(out) != 1 {
		t.Fatalf("expected 1 chunk, got %d", len(out))
	}

	if gjson.GetBytes(out[0], "choices.0.delta.content").Exists() {
		t.Fatalf("expected content to be omitted, got %s", string(out[0]))
	}
	if gjson.GetBytes(out[0], "choices.0.delta.reasoning_content").Exists() {
		t.Fatalf("expected reasoning_content to be omitted, got %s", string(out[0]))
	}
	if !gjson.GetBytes(out[0], "choices.0.delta.tool_calls").Exists() {
		t.Fatalf("expected tool_calls to exist, got %s", string(out[0]))
	}
}

func TestConvertCodexResponseToOpenAI_ToolCallArgumentsDeltaOmitsNullContentFields(t *testing.T) {
	ctx := context.Background()
	var param any

	out := ConvertCodexResponseToOpenAI(ctx, "gpt-5.4", nil, nil, []byte(`data: {"type":"response.output_item.added","item":{"type":"function_call","call_id":"call_123","name":"websearch"}}`), &param)
	if len(out) != 1 {
		t.Fatalf("expected tool call announcement chunk, got %d", len(out))
	}

	out = ConvertCodexResponseToOpenAI(ctx, "gpt-5.4", nil, nil, []byte(`data: {"type":"response.function_call_arguments.delta","delta":"{\"query\":\"OpenAI\"}"}`), &param)
	if len(out) != 1 {
		t.Fatalf("expected 1 chunk, got %d", len(out))
	}

	if gjson.GetBytes(out[0], "choices.0.delta.content").Exists() {
		t.Fatalf("expected content to be omitted, got %s", string(out[0]))
	}
	if gjson.GetBytes(out[0], "choices.0.delta.reasoning_content").Exists() {
		t.Fatalf("expected reasoning_content to be omitted, got %s", string(out[0]))
	}
	if !gjson.GetBytes(out[0], "choices.0.delta.tool_calls.0.function.arguments").Exists() {
		t.Fatalf("expected tool call arguments delta to exist, got %s", string(out[0]))
	}
}

func TestConvertCodexResponseToOpenAI_StreamPartialImageEmitsDataURL(t *testing.T) {
	ctx := context.Background()
	var param any

	out := ConvertCodexResponseToOpenAI(ctx, "gpt-5.4", nil, nil, []byte(`data: {"type":"response.image_generation_call.partial_image","item_id":"img_1","partial_image_b64":"QUJD","output_format":"webp"}`), &param)
	if len(out) != 1 {
		t.Fatalf("expected 1 chunk, got %d", len(out))
	}

	gotURL := gjson.GetBytes(out[0], "choices.0.delta.images.0.image_url.url").String()
	if gotURL != "data:image/webp;base64,QUJD" {
		t.Fatalf("image data URL = %q, want %q", gotURL, "data:image/webp;base64,QUJD")
	}
}

func TestConvertCodexResponseToOpenAI_StreamPartialImageDeduplicatesByItemID(t *testing.T) {
	ctx := context.Background()
	var param any

	first := ConvertCodexResponseToOpenAI(ctx, "gpt-5.4", nil, nil, []byte(`data: {"type":"response.image_generation_call.partial_image","item_id":"img_1","partial_image_b64":"QUJD","output_format":"png"}`), &param)
	if len(first) != 1 {
		t.Fatalf("expected first partial image chunk, got %d", len(first))
	}

	second := ConvertCodexResponseToOpenAI(ctx, "gpt-5.4", nil, nil, []byte(`data: {"type":"response.image_generation_call.partial_image","item_id":"img_1","partial_image_b64":"QUJD","output_format":"png"}`), &param)
	if len(second) != 0 {
		t.Fatalf("expected duplicate partial image to be suppressed, got %d chunks", len(second))
	}
}

func TestConvertCodexResponseToOpenAINonStream_ImageGenerationCall(t *testing.T) {
	ctx := context.Background()

	out := ConvertCodexResponseToOpenAINonStream(ctx, "", nil, nil, []byte(`{
		"type":"response.completed",
		"response":{
			"id":"resp_image",
			"created_at":1700000000,
			"model":"gpt-5.4",
			"status":"completed",
			"output":[
				{"type":"image_generation_call","result":"R0lG","output_format":"gif"}
			]
		}
	}`), nil)

	gotURL := gjson.GetBytes(out, "choices.0.message.images.0.image_url.url").String()
	if gotURL != "data:image/gif;base64,R0lG" {
		t.Fatalf("image data URL = %q, want %q", gotURL, "data:image/gif;base64,R0lG")
	}
}

func TestMimeTypeFromCodexOutputFormat_OpenAI(t *testing.T) {
	tests := []struct {
		name   string
		input  string
		expect string
	}{
		{name: "png", input: "png", expect: "image/png"},
		{name: "jpeg", input: "jpeg", expect: "image/jpeg"},
		{name: "webp", input: "webp", expect: "image/webp"},
		{name: "gif", input: "gif", expect: "image/gif"},
		{name: "full mime", input: "image/jpeg", expect: "image/jpeg"},
		{name: "default", input: "unknown", expect: "image/png"},
		{name: "empty", input: "", expect: "image/png"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := mimeTypeFromCodexOutputFormat(tt.input); got != tt.expect {
				t.Fatalf("mimeTypeFromCodexOutputFormat(%q) = %q, want %q", tt.input, got, tt.expect)
			}
		})
	}
}
