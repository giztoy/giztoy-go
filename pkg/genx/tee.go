package genx

// Tee returns a Stream that reads from src and copies all chunks to builder.
func Tee(src Stream, builder *StreamBuilder) Stream {
	return &teeStream{src: src, builder: builder}
}

type teeStream struct {
	src     Stream
	builder *StreamBuilder
}

func (t *teeStream) Next() (*MessageChunk, error) {
	chunk, err := t.src.Next()
	if err != nil {
		if isDoneErr(err) {
			t.builder.Done(Usage{})
		} else {
			t.builder.Abort(err)
		}
		return nil, err
	}
	if chunk != nil {
		t.builder.Add(chunk)
	}
	return chunk, nil
}

func (t *teeStream) Close() error {
	return t.src.Close()
}

func (t *teeStream) CloseWithError(err error) error {
	return t.src.CloseWithError(err)
}
