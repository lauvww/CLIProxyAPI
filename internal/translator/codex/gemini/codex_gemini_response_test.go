package gemini

import (
	"context"
	"testing"

	"github.com/tidwall/gjson"
)

func TestConvertCodexResponseToGemini_StreamEmptyOutputUsesOutputItemDoneMessageFallback(t *testing.T) {
	ctx := context.Background()
	originalRequest := []byte(`{"tools":[]}`)
	var param any

	chunks := [][]byte{
		[]byte("data: {\"type\":\"response.output_item.done\",\"item\":{\"type\":\"message\",\"role\":\"assistant\",\"content\":[{\"type\":\"output_text\",\"text\":\"ok\"}]},\"output_index\":0}"),
		[]byte("data: {\"type\":\"response.completed\",\"response\":{\"usage\":{\"input_tokens\":1,\"output_tokens\":1}}}"),
	}

	var outputs [][]byte
	for _, chunk := range chunks {
		outputs = append(outputs, ConvertCodexResponseToGemini(ctx, "gemini-2.5-pro", originalRequest, nil, chunk, &param)...)
	}

	found := false
	for _, out := range outputs {
		if gjson.GetBytes(out, "candidates.0.content.parts.0.text").String() == "ok" {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("expected fallback content from response.output_item.done message; outputs=%q", outputs)
	}
}

func TestConvertCodexResponseToGemini_StreamPartialImageEmitsInlineData(t *testing.T) {
	ctx := context.Background()
	var param any

	out := ConvertCodexResponseToGemini(ctx, "gemini-2.5-pro", nil, nil, []byte(`data: {"type":"response.image_generation_call.partial_image","item_id":"img_1","partial_image_b64":"QUJD","output_format":"png"}`), &param)
	if len(out) != 1 {
		t.Fatalf("expected 1 chunk, got %d", len(out))
	}

	gotData := gjson.GetBytes(out[0], "candidates.0.content.parts.0.inlineData.data").String()
	gotMime := gjson.GetBytes(out[0], "candidates.0.content.parts.0.inlineData.mimeType").String()
	if gotData != "QUJD" {
		t.Fatalf("inlineData.data = %q, want %q", gotData, "QUJD")
	}
	if gotMime != "image/png" {
		t.Fatalf("inlineData.mimeType = %q, want %q", gotMime, "image/png")
	}
}

func TestConvertCodexResponseToGemini_StreamPartialImageDeduplicatesByItemID(t *testing.T) {
	ctx := context.Background()
	var param any

	first := ConvertCodexResponseToGemini(ctx, "gemini-2.5-pro", nil, nil, []byte(`data: {"type":"response.image_generation_call.partial_image","item_id":"img_1","partial_image_b64":"QUJD","output_format":"png"}`), &param)
	if len(first) != 1 {
		t.Fatalf("expected first partial image chunk, got %d", len(first))
	}

	second := ConvertCodexResponseToGemini(ctx, "gemini-2.5-pro", nil, nil, []byte(`data: {"type":"response.image_generation_call.partial_image","item_id":"img_1","partial_image_b64":"QUJD","output_format":"png"}`), &param)
	if len(second) != 0 {
		t.Fatalf("expected duplicate partial image to be suppressed, got %d chunks", len(second))
	}
}

func TestConvertCodexResponseToGeminiNonStream_ImageGenerationCall(t *testing.T) {
	ctx := context.Background()

	out := ConvertCodexResponseToGeminiNonStream(ctx, "gemini-2.5-pro", nil, nil, []byte(`{
		"type":"response.completed",
		"response":{
			"id":"resp_image",
			"created_at":1700000000,
			"usage":{"input_tokens":1,"output_tokens":1},
			"output":[
				{"type":"image_generation_call","result":"R0lG","output_format":"image/jpeg"}
			]
		}
	}`), nil)

	gotData := gjson.GetBytes(out, "candidates.0.content.parts.0.inlineData.data").String()
	gotMime := gjson.GetBytes(out, "candidates.0.content.parts.0.inlineData.mimeType").String()
	if gotData != "R0lG" {
		t.Fatalf("inlineData.data = %q, want %q", gotData, "R0lG")
	}
	if gotMime != "image/jpeg" {
		t.Fatalf("inlineData.mimeType = %q, want %q", gotMime, "image/jpeg")
	}
}

func TestMimeTypeFromCodexOutputFormat_Gemini(t *testing.T) {
	tests := []struct {
		name   string
		input  string
		expect string
	}{
		{name: "png", input: "png", expect: "image/png"},
		{name: "jpg", input: "jpg", expect: "image/jpeg"},
		{name: "webp", input: "webp", expect: "image/webp"},
		{name: "gif", input: "gif", expect: "image/gif"},
		{name: "full mime", input: "image/webp", expect: "image/webp"},
		{name: "default", input: "foo", expect: "image/png"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := mimeTypeFromCodexOutputFormat(tt.input); got != tt.expect {
				t.Fatalf("mimeTypeFromCodexOutputFormat(%q) = %q, want %q", tt.input, got, tt.expect)
			}
		})
	}
}
