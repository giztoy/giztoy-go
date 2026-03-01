package genx

import "context"

// Transformer converts a Stream into another Stream.
type Transformer interface {
	Transform(ctx context.Context, pattern string, input Stream) (Stream, error)
}
