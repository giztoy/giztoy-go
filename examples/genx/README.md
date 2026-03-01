# GX Model Capability Probe

This example runs live capability checks for the OpenAI-compatible models defined in:

- `examples/genx/models/*_openai.json`

It probes each model and prints a summary table for:

- `GENERATE`
- `INVOKE (JSON_OUTPUT)`
- `INVOKE (TOOL_CALLS)`
- expectation match (`EXPECT`)

## Run

```bash
go run ./examples/genx
```

## Latest Run Output

```text
MODEL                                  JSON_OUTPUT  TOOL_CALLS  GENERATE  EXPECT  NOTES
deepseek/chat (deepseek_openai.json)   no           yes         yes       ok      json_output: response_format_unsupported
minimax/text-01 (minimax_openai.json)  yes          yes         yes       ok      -
minimax/m2-her (minimax_openai.json)   yes          no          yes       ok      tool_calls: no tool calls
omg/gpt-4o-mini (omg_openai.json)      yes          yes         yes       ok      -
omg/gpt-4.1 (omg_openai.json)          yes          yes         yes       ok      -
omg/gpt-5 (omg_openai.json)            yes          unknown     yes       ok      -
qwen/turbo-latest (qwen_openai.json)   yes          yes         yes       ok      -
zhipu/glm-4 (zhipu_openai.json)        yes          yes         yes       ok      -
zhipu/glm-5 (zhipu_openai.json)        yes          no          yes       ok      tool_calls: no tool calls

result: all model expectations matched.
```
