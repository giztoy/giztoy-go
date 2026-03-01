package genx

import (
	"context"
	"strings"
	"testing"
)

func TestModelContextBuilderPromptMessageAndBuild(t *testing.T) {
	var mcb ModelContextBuilder

	mcb.AddPrompt(&Prompt{Name: "sys", Text: "line1"})
	mcb.AddPrompt(&Prompt{Name: "sys", Text: "line2"})
	mcb.AddPrompt(&Prompt{Name: "dev", Text: "line3"})

	mcb.AddMessage(&Message{Role: RoleUser, Name: "u", Payload: Contents{Text("a")}})
	mcb.AddMessage(&Message{Role: RoleUser, Name: "u", Payload: Contents{Text("b")}})
	mcb.AddMessage(&Message{Role: RoleModel, Name: "m", Payload: Contents{Text("c")}})

	mcb.SetCoT("cot-1", map[string]any{"k": "v"})
	mcb.AddTool(&SearchWebTool{})
	mcb.Params = &ModelParams{MaxTokens: 128}

	if err := mcb.Prompt("yaml", "k", map[string]int{"v": 1}); err != nil {
		t.Fatalf("Prompt() failed: %v", err)
	}

	ctx := mcb.Build()

	var prompts []*Prompt
	for p := range ctx.Prompts() {
		prompts = append(prompts, p)
	}
	if len(prompts) != 3 {
		t.Fatalf("unexpected prompts: %#v", prompts)
	}
	if prompts[0].Text != "line1\nline2" {
		t.Fatalf("expected merged prompt text, got: %q", prompts[0].Text)
	}

	var msgs []*Message
	for m := range ctx.Messages() {
		msgs = append(msgs, m)
	}
	if len(msgs) != 2 {
		t.Fatalf("unexpected message count: %#v", msgs)
	}
	userContents := msgs[0].Payload.(Contents)
	if len(userContents) != 2 {
		t.Fatalf("expected merged user contents, got: %#v", userContents)
	}

	var cots []string
	for c := range ctx.CoTs() {
		cots = append(cots, c)
	}
	if len(cots) != 2 || !strings.Contains(cots[1], "k") {
		t.Fatalf("unexpected cots: %#v", cots)
	}

	toolCount := 0
	for range ctx.Tools() {
		toolCount++
	}
	if toolCount != 1 {
		t.Fatalf("unexpected tools count: %d", toolCount)
	}

	if ctx.Params() == nil || ctx.Params().MaxTokens != 128 {
		t.Fatalf("unexpected params: %#v", ctx.Params())
	}
}

func TestModelContextBuilderConvenienceAndToolResult(t *testing.T) {
	var mcb ModelContextBuilder
	mcb.UserText("u", "hi")
	mcb.UserBlob("u", "image/png", []byte{1, 2})
	mcb.ModelText("m", "ok")
	mcb.ModelBlob("m", "audio/wav", []byte{3, 4})

	if err := mcb.AddToolCallResult("tool", map[string]any{"k": 1}, map[string]any{"ok": true}); err != nil {
		t.Fatalf("AddToolCallResult failed: %v", err)
	}

	if err := mcb.AddToolCallResult("tool", make(chan int), "x"); err == nil {
		t.Fatal("expected marshal error for tool call args")
	}
	if err := mcb.AddToolCallResult("tool", "{}", make(chan int)); err == nil {
		t.Fatal("expected marshal error for tool call result")
	}

	ctx := mcb.Build()
	var msgs []*Message
	for m := range ctx.Messages() {
		msgs = append(msgs, m)
	}
	if len(msgs) < 4 {
		t.Fatalf("expected tool call/result messages to be appended, got: %d", len(msgs))
	}

	last := msgs[len(msgs)-1]
	if _, ok := last.Payload.(*ToolResult); !ok {
		t.Fatalf("expected last payload to be ToolResult, got: %T", last.Payload)
	}
}

func TestModelContextBuilderInvokeTool(t *testing.T) {
	type invokeArg struct {
		N int `json:"n"`
	}

	tool := MustNewFuncTool[invokeArg]("sum", "sum tool", InvokeFunc[invokeArg](func(_ context.Context, _ *FuncCall, arg invokeArg) (any, error) {
		return map[string]any{"v": arg.N + 1}, nil
	}))

	call := tool.NewFuncCall(`{"n":1}`)
	tc := &ToolCall{ID: "call_1", FuncCall: call}

	var mcb ModelContextBuilder
	if err := mcb.InvokeTool(context.Background(), tc); err != nil {
		t.Fatalf("InvokeTool failed: %v", err)
	}

	if err := mcb.InvokeTool(context.Background(), &ToolCall{ID: "bad"}); err == nil {
		t.Fatal("expected nil func call error")
	}

	ctx := mcb.Build()
	msgCount := 0
	for range ctx.Messages() {
		msgCount++
	}
	if msgCount != 2 {
		t.Fatalf("expected two messages (tool call + result), got: %d", msgCount)
	}
}

func TestMultiModelContextIterationAndParams(t *testing.T) {
	var a, b ModelContextBuilder
	a.PromptText("a", "p1")
	a.UserText("a", "u1")
	a.SetCoT("cot1")
	a.AddTool(&SearchWebTool{})

	b.PromptText("b", "p2")
	b.UserText("b", "u2")
	b.SetCoT("cot2")
	b.Params = &ModelParams{TopP: 0.5}

	m := ModelContexts(a.Build(), b.Build())

	promptCount := 0
	for range m.Prompts() {
		promptCount++
	}
	if promptCount != 2 {
		t.Fatalf("unexpected prompt count: %d", promptCount)
	}

	messageCount := 0
	for range m.Messages() {
		messageCount++
	}
	if messageCount != 2 {
		t.Fatalf("unexpected message count: %d", messageCount)
	}

	cotCount := 0
	for range m.CoTs() {
		cotCount++
	}
	if cotCount != 2 {
		t.Fatalf("unexpected cot count: %d", cotCount)
	}

	toolCount := 0
	for range m.Tools() {
		toolCount++
	}
	if toolCount != 1 {
		t.Fatalf("unexpected tool count: %d", toolCount)
	}

	if m.Params() == nil || m.Params().TopP != 0.5 {
		t.Fatalf("unexpected params in multi context: %#v", m.Params())
	}
}
