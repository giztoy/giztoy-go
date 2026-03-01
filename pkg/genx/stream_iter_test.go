package genx

import (
	"errors"
	"fmt"
	"io"
	"testing"
	"time"
)

func TestIterDoesNotDropEventsUnderBackpressure(t *testing.T) {
	const n = 20
	chunks := make([]*MessageChunk, 0, n)
	for i := range n {
		chunks = append(chunks, &MessageChunk{
			Role: RoleModel,
			Name: "assistant",
			ToolCall: &ToolCall{
				ID: fmt.Sprintf("id-%d", i),
				FuncCall: &FuncCall{
					Name:      "tool",
					Arguments: "{}",
				},
			},
		})
	}

	itr := Iter(&sliceStream{chunks: chunks, doneErr: ErrDone})

	// 先让生产者跑起来，模拟消费者慢导致的背压场景。
	time.Sleep(30 * time.Millisecond)

	count := 0
	for {
		el, err := itr.Next()
		if err != nil {
			if errors.Is(err, ErrDone) {
				break
			}
			t.Fatalf("iter next failed: %v", err)
		}
		if _, ok := el.(*ToolCallElement); !ok {
			t.Fatalf("unexpected element type: %T", el)
		}
		count++
	}

	if count != n {
		t.Fatalf("expected %d events, got %d", n, count)
	}
}

func TestIterTreatsEOFAsDone(t *testing.T) {
	itr := Iter(&sliceStream{chunks: []*MessageChunk{{Part: Text("x")}}, doneErr: io.EOF})

	if _, err := itr.Next(); err != nil {
		t.Fatalf("first next failed: %v", err)
	}
	if _, err := itr.Next(); !errors.Is(err, ErrDone) {
		t.Fatalf("expected ErrDone after EOF source, got: %v", err)
	}
}
