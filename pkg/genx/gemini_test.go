package genx

import (
	"errors"
	"testing"

	"github.com/google/jsonschema-go/jsonschema"
	"google.golang.org/genai"
)

func TestGeminiConvSchemaTypeAndEnum(t *testing.T) {
	s := &jsonschema.Schema{Type: "string", Enum: []any{"a", 1}}
	got := geminiConvSchema(s)
	if got == nil {
		t.Fatal("expected schema")
	}
	if got.Type != genai.TypeString {
		t.Fatalf("unexpected type: %v", got.Type)
	}
	if len(got.Enum) != 2 || got.Enum[0] != "a" || got.Enum[1] != "1" {
		t.Fatalf("unexpected enum conversion: %#v", got.Enum)
	}
}

func TestGeminiConvMessageToolCallFallbackArgs(t *testing.T) {
	msg := &Message{
		Role: RoleModel,
		Payload: &ToolCall{
			ID: "fn",
			FuncCall: &FuncCall{
				Name:      "fn",
				Arguments: "not-json",
			},
		},
	}

	content, err := geminiConvMessage(nil, msg)
	if err != nil {
		t.Fatalf("geminiConvMessage failed: %v", err)
	}
	if content == nil || content.Role != "model" || len(content.Parts) != 1 {
		t.Fatalf("unexpected content: %#v", content)
	}
	fc := content.Parts[0].FunctionCall
	if fc == nil || fc.Args["text"] != "not-json" {
		t.Fatalf("expected fallback text args, got: %#v", fc)
	}
}

func TestGeminiPullHandlesFinishChunkWithoutContent(t *testing.T) {
	sb := NewStreamBuilder((&ModelContextBuilder{}).Build(), 8)

	seq := func(yield func(*genai.GenerateContentResponse, error) bool) {
		if !yield(&genai.GenerateContentResponse{
			Candidates: []*genai.Candidate{{
				Index:   0,
				Content: &genai.Content{Parts: []*genai.Part{genai.NewPartFromText("hello")}},
			}},
		}, nil) {
			return
		}

		yield(&genai.GenerateContentResponse{
			Candidates: []*genai.Candidate{{
				Index:        0,
				FinishReason: genai.FinishReasonStop,
			}},
			UsageMetadata: &genai.GenerateContentResponseUsageMetadata{
				PromptTokenCount:     1,
				CandidatesTokenCount: 1,
			},
		}, nil)
	}

	if err := geminiPull(sb, seq); err != nil {
		t.Fatalf("geminiPull failed: %v", err)
	}

	stream := sb.Stream()
	chunk, err := stream.Next()
	if err != nil {
		t.Fatalf("read first chunk failed: %v", err)
	}
	if chunk == nil || chunk.Part == nil {
		t.Fatalf("unexpected first chunk: %#v", chunk)
	}
	if text, ok := chunk.Part.(Text); !ok || string(text) != "hello" {
		t.Fatalf("unexpected first chunk text: %#v", chunk.Part)
	}

	if _, err := stream.Next(); !errors.Is(err, ErrDone) {
		t.Fatalf("expected ErrDone, got: %v", err)
	}
}
